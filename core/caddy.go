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

func GenerateRouteJSON(domain, targetIP, targetPort, protocol, routeID string) map[string]interface{} {
	upstream := map[string]interface{}{"dial": fmt.Sprintf("%s:%s", targetIP, targetPort)}
	transport := map[string]interface{}{"protocol": "http"}
	if protocol == "https" {
		transport["tls"] = map[string]interface{}{"insecure_skip_verify": true}
	}
	handler := map[string]interface{}{
		"handler":   "reverse_proxy",
		"upstreams": []interface{}{upstream},
		"transport": transport,
	}
	return map[string]interface{}{
		"@id":    routeID,
		"match":  []interface{}{map[string]interface{}{"host": []string{domain}}},
		"handle": []interface{}{handler},
	}
}

func GenerateTLSPolicyJSON(domain, tlsID string) map[string]interface{} {
	return map[string]interface{}{
		"@id":      tlsID,
		"subjects": []string{domain},
		"issuers": []map[string]interface{}{
			{
				"module":                  "acme",
				"ca":                      "https://172.20.0.1:9000/acme/acme/directory",
				"trusted_roots_pem_files": []string{"/etc/caddy/root_ca.crt"},
				"challenges": map[string]interface{}{
					"http": map[string]interface{}{
						"disabled": true,
					},
				},
			},
		},
	}
}

func (c *CaddyClient) DeleteRouteAndTLS(nodeName, routeID, tlsID string) error {
	baseURL, ok := c.BaseURLs[strings.ToLower(nodeName)]
	if !ok {
		return nil
	}

	// Best-effort удаление маршрута и TLS-политики. Ошибки логируем, но не
	// возвращаем — это компенсационное действие при Deprovision, и fail
	// одного из двух DELETE не должен блокировать чистку state.
	for _, id := range []string{routeID, tlsID} {
		req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/id/%s", baseURL, id), nil)
		if err != nil {
			fmt.Printf("[CADDY] delete %s: build request: %v\n", id, err)
			continue
		}
		resp, err := c.client.Do(req)
		if err != nil {
			fmt.Printf("[CADDY] delete %s: %v\n", id, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			fmt.Printf("[CADDY] delete %s: HTTP %d\n", id, resp.StatusCode)
		}
	}
	return nil
}

func (c *CaddyClient) RestartCaddy(nodeName string) error {
	baseURL, ok := c.BaseURLs[strings.ToLower(nodeName)]
	if !ok {
		return fmt.Errorf("URL Caddy не найден для ноды %s", nodeName)
	}
	fmt.Printf("[CADDY] Рестарт ноды %s (POST /stop)...\n", nodeName)
	resp, err := http.Post(baseURL+"/stop", "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST /stop (%d): %s", resp.StatusCode, string(body))
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
func (c *CaddyClient) upsertByID(baseURL, id, createPath string, payload map[string]interface{}, initIfMissing func() error) error {
	data, _ := json.Marshal(payload)

	getResp, err := c.client.Get(fmt.Sprintf("%s/id/%s", baseURL, id))
	if err == nil {
		getResp.Body.Close()
		if getResp.StatusCode == 200 {
			req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/id/%s", baseURL, id), bytes.NewBuffer(data))
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
			fmt.Printf("[CADDY] POST %s → 500 (предположительно нет родителя): %s; init+retry\n", createPath, string(firstBody))
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
	baseURL, ok := c.BaseURLs[strings.ToLower(nodeName)]
	if !ok {
		return fmt.Errorf("URL Caddy не найден для ноды %s", nodeName)
	}

	fmt.Printf("[CADDY] Replay %s (%s)...\n", domain, routeID)

	tlsPolicy := GenerateTLSPolicyJSON(domain, tlsID)
	initTLS := func() error {
		initTlsPayload, _ := json.Marshal(map[string]interface{}{
			"automation": map[string]interface{}{
				"policies": []interface{}{tlsPolicy},
			},
		})
		req, _ := http.NewRequest("PUT", baseURL+"/config/apps/tls", bytes.NewBuffer(initTlsPayload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	if err := c.upsertByID(baseURL, tlsID, "/config/apps/tls/automation/policies", tlsPolicy, initTLS); err != nil {
		return fmt.Errorf("TLS policy: %w", err)
	}

	automatePayload, _ := json.Marshal([]string{domain})
	reqAuth, _ := http.NewRequest("POST", baseURL+"/config/apps/tls/certificates/automate", bytes.NewBuffer(automatePayload))
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

	routeConfig := GenerateRouteJSON(domain, targetIP, targetPort, protocol, routeID)
	initRoute := func() error {
		initPayload, _ := json.Marshal(map[string]interface{}{
			"listen": []string{":443"},
			"routes": []interface{}{routeConfig},
		})
		req, _ := http.NewRequest("PUT", baseURL+"/config/apps/http/servers/srv0", bytes.NewBuffer(initPayload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}
	if err := c.upsertByID(baseURL, routeID, "/config/apps/http/servers/srv0/routes", routeConfig, initRoute); err != nil {
		return fmt.Errorf("route: %w", err)
	}

	return nil
}
