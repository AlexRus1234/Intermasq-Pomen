// Pomen - plugin for Intermasq
// Copyright (C) 2026 AlexRus1234
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type CaddyClient struct {
	BaseURLs map[string]string
	client   *http.Client
}

func NewCaddyClient(urls map[string]string) *CaddyClient {
	// Копируем map и триммим '/' — НЕ мутируем входной map вызывающего кода
	// (audit §14.7: побочный эффект, трудно отлаживать).
	trimmed := make(map[string]string, len(urls))
	for k, v := range urls {
		trimmed[k] = strings.TrimRight(v, "/")
	}
	return &CaddyClient{BaseURLs: trimmed, client: &http.Client{Timeout: 10 * time.Second}}
}

// baseURLFor возвращает базовый URL Caddy для ноды (case-insensitive lookup).
func (c *CaddyClient) baseURLFor(nodeName string) (string, error) {
	if u, ok := c.BaseURLs[strings.ToLower(nodeName)]; ok {
		return u, nil
	}
	return "", fmt.Errorf("URL Caddy не найден для ноды %s", nodeName)
}

// GenerateRoute собирает CaddyRoute для домена → upstream.
func GenerateRoute(domain, targetIP, targetPort, protocol, routeID string) CaddyRoute {
	transport := CaddyTransport{Protocol: "http"}
	if protocol == "https" {
		transport.TLS = &CaddyUpstreamTLS{InsecureSkipVerify: true}
	}
	return CaddyRoute{
		ID:    routeID,
		Match: []CaddyMatch{{Host: []string{domain}}},
		Handle: []CaddyHandle{{
			Handler:   "reverse_proxy",
			Upstreams: []CaddyUpstream{{Dial: fmt.Sprintf("%s:%s", targetIP, targetPort)}},
			Transport: transport,
		}},
	}
}

// GenerateTLSPolicy собирает CaddyTLSPolicy для domain.
// ACME-issuer указывает на внутренний Step-CA (URL/путь к root CA сейчас
// захардкожены — будут вынесены в config на этапе 5).
func GenerateTLSPolicy(domain, tlsID string) CaddyTLSPolicy {
	return CaddyTLSPolicy{
		ID:       tlsID,
		Subjects: []string{domain},
		Issuers: []CaddyIssuer{{
			Module: "acme",
			CA:     "https://172.20.0.1:9000/acme/acme/directory",
			TrustedRootsPEMFiles: []string{"/etc/caddy/root_ca.crt"},
			Challenges: CaddyChallenges{
				HTTP: CaddyHTTPChallenge{Disabled: true},
			},
		}},
	}
}

func (c *CaddyClient) DeleteRouteAndTLS(nodeName, routeID, tlsID string) error {
	baseURL, err := c.baseURLFor(nodeName)
	if err != nil {
		return nil
	}

	// Best-effort удаление маршрута и TLS-политики. Ошибки логируем, но не
	// возвращаем — это компенсационное действие при Deprovision, и fail
	// одного из двух DELETE не должен блокировать чистку state.
	for _, id := range []string{routeID, tlsID} {
		req, err := http.NewRequest("DELETE", caddyIDURL(baseURL, id), nil)
		if err != nil {
			slog.Warn("caddy delete: build request failed", "node", nodeName, "id", id, "err", err)
			continue
		}
		resp, err := c.client.Do(req)
		if err != nil {
			slog.Warn("caddy delete: request failed", "node", nodeName, "id", id, "err", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			slog.Warn("caddy delete: non-OK status", "node", nodeName, "id", id, "status", resp.StatusCode)
		}
	}
	return nil
}

func (c *CaddyClient) RestartCaddy(nodeName string) error {
	baseURL, err := c.baseURLFor(nodeName)
	if err != nil {
		return err
	}
	slog.Info("caddy restart", "node", nodeName, "endpoint", caddyStop)
	resp, err := http.Post(baseURL+caddyStop, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s (%d): %s", caddyStop, resp.StatusCode, string(body))
	}
	return nil
}

// upsertByID выполняет GET /id/<id>; 200 → PUT (update), иначе POST (create).
// Если POST возвращает 500, мы различаем два случая:
//   - родительский путь отсутствует (например /config/apps/tls/automation
//     ещё не создан) — тогда вызываем initIfMissing и повторяем POST;
//   - реальная внутренняя ошибка Caddy — возвращаем ошибку с телом ответа.
//
// Различение делается по результату повторного POST после init: если он тоже
// падает, это точно не "parent missing", а что-то другое (невалидный payload,
// конфликт ID, истёкший сертификат и т.п.) — тогда ошибка возвращается наверх
// с понятным сообщением. Первичный 500 до init логируем, чтобы дать след в
// журнале даже при успешном autorecovery.
func (c *CaddyClient) upsertByID(baseURL, id, createPath string, payload interface{}, initIfMissing func() error) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// GET → если есть, делаем PUT (update).
	if getResp, gerr := c.client.Get(caddyIDURL(baseURL, id)); gerr == nil {
		getResp.Body.Close()
		if getResp.StatusCode == 200 {
			req, _ := http.NewRequest("PUT", caddyIDURL(baseURL, id), bytes.NewBuffer(data))
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("PUT %s (%d): %s", id, resp.StatusCode, string(body))
			}
			return nil
		}
	}

	// Нет → POST (create).
	req, _ := http.NewRequest("POST", baseURL+createPath, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 500 {
		firstBody, _ := io.ReadAll(resp.Body)
		if initIfMissing != nil {
			slog.Info("caddy POST 500 — assuming missing parent, retrying after init",
				"path", createPath, "id", id, "body", string(firstBody))
			if err := initIfMissing(); err != nil {
				return fmt.Errorf("init parent для %s: %w", createPath, err)
			}
			req2, _ := http.NewRequest("POST", baseURL+createPath, bytes.NewBuffer(data))
			req2.Header.Set("Content-Type", "application/json")
			resp2, err := c.client.Do(req2)
			if err != nil {
				return err
			}
			defer resp2.Body.Close()
			if resp2.StatusCode >= 400 {
				body2, _ := io.ReadAll(resp2.Body)
				return fmt.Errorf("POST %s после init всё ещё падает (%d); первичная ошибка была: %s | вторичная: %s",
					createPath, resp2.StatusCode, string(firstBody), string(body2))
			}
			return nil
		}
		return fmt.Errorf("POST %s (500, initIfMissing не задан): %s", createPath, string(firstBody))
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s (%d): %s", createPath, resp.StatusCode, string(body))
	}
	return nil
}

func (c *CaddyClient) ReplayRoute(nodeName, domain, targetIP, targetPort, protocol, routeID, tlsID string) error {
	baseURL, err := c.baseURLFor(nodeName)
	if err != nil {
		return err
	}

	slog.Info("caddy replay", "domain", domain, "route_id", routeID, "tls_id", tlsID, "node", nodeName)

	tlsPolicy := GenerateTLSPolicy(domain, tlsID)
	initTLS := func() error {
		payload := caddyAutomationConfig{Automation: caddyAutomation{Policies: []CaddyTLSPolicy{tlsPolicy}}}
		data, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", baseURL+caddyPathTLS, bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	if err := c.upsertByID(baseURL, tlsID, caddyPathTLSPolicies, tlsPolicy, initTLS); err != nil {
		return fmt.Errorf("TLS policy: %w", err)
	}

	// Добавляем domain в automate-список (выпуск сертификата в фоне).
	automatePayload, _ := json.Marshal([]string{domain})
	reqAuth, _ := http.NewRequest("POST", baseURL+caddyPathTLSCertAutomate, bytes.NewBuffer(automatePayload))
	reqAuth.Header.Set("Content-Type", "application/json")
	respAuth, err := c.client.Do(reqAuth)
	if err != nil {
		return fmt.Errorf("automate: %w", err)
	}
	defer respAuth.Body.Close()
	if respAuth.StatusCode >= 400 {
		body, _ := io.ReadAll(respAuth.Body)
		return fmt.Errorf("automate (%d): %s", respAuth.StatusCode, string(body))
	}

	routeConfig := GenerateRoute(domain, targetIP, targetPort, protocol, routeID)
	initRoute := func() error {
		payload := caddyServerConfig{
			Listen: []string{caddyHTTPListen},
			Routes: []CaddyRoute{routeConfig},
		}
		data, _ := json.Marshal(payload)
		req, _ := http.NewRequest("PUT", baseURL+caddyPathHTTPServer, bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	if err := c.upsertByID(baseURL, routeID, caddyPathHTTPServerRoutes, routeConfig, initRoute); err != nil {
		return fmt.Errorf("route: %w", err)
	}

	return nil
}
