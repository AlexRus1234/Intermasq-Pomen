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
	"strconv"
	"strings"
	"time"
)

// WebhookClient дёргает вебхук на ВМ, который исполняет `podman ps --format json`
// от бесправного пользователя. Auth — секрет в заголовке X-VM-Secret (per-VM).
type WebhookClient struct {
	client *http.Client
}

func NewWebhookClient() *WebhookClient {
	return &WebhookClient{client: &http.Client{Timeout: 15 * time.Second}}
}

// rawPodmanContainer — селективный подмножество полей `podman ps --format json`.
type rawPodmanContainer struct {
	Names  []string          `json:"Names"`
	Status string            `json:"Status"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
	Ports  []rawPort         `json:"Ports"`
}

type rawPort struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// ListContainers стучится на вебхук ВМ и возвращает нормализованный список.
// endpoint — путь на вебхуке (по умолчанию /hooks/podman — стандартный mount adnanh/webhook).
func (w *WebhookClient) ListContainers(vm VMConfig, endpoint string) ([]ContainerInfo, error) {
	raw, err := w.fetchContainers(vm, endpoint)
	if err != nil {
		return nil, err
	}
	return normalizeContainers(raw, vm), nil
}

// fetchContainers делает HTTP-запрос к вебхуку ВМ и декодирует ответ podman ps.
// Изолирует транспортную часть от нормализации (которая тестируется отдельно).
func (w *WebhookClient) fetchContainers(vm VMConfig, endpoint string) ([]rawPodmanContainer, error) {
	if endpoint == "" {
		endpoint = "/hooks/podman"
	}
	url := strings.TrimRight(vm.WebhookURL, "/") + endpoint

	// WORKAROUND: adnanh/webhook падает с "unsupported content type" ДО
	// проверки trigger-rule, если у запроса нет body. Шлём пустой JSON-объект
	// и Content-Type: application/json, чтобы триггер отработал. Когда перейдём
	// на другой вебхук-демон — body можно убрать.
	req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-VM-Secret", vm.Secret)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("вебхук ВМ %s недоступен: %w", vm.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("вебхук ВМ %s вернул %d: %s", vm.Name, resp.StatusCode, string(body))
	}

	var raw []rawPodmanContainer
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("парсинг podman ps от ВМ %s: %w", vm.Name, err)
	}
	return raw, nil
}

// normalizeContainers превращает «сырые» поля podman ps в ContainerInfo с
// корректными Name/Port/Protocol. Чистая функция — не делает I/O, легко
// покрывается табличными юнит-тестами (этап 6).
//
// Правила нормализации:
//   - Name — реальное имя без префикса "systemd-" (Quadlet) и без ведущего
//     числового префикса вида "01-athens" → "athens";
//   - Port — первый host_port из Ports, переопределяется label "port-NNN";
//   - Protocol — "http" по умолчанию, переопределяется label "proto-NAME"
//     или полем Port.Protocol;
//   - label "name-NAME" полностью заменяет Name (для человекочитаемых доменов).
func normalizeContainers(raw []rawPodmanContainer, vm VMConfig) []ContainerInfo {
	result := make([]ContainerInfo, 0, len(raw))
	for _, c := range raw {
		result = append(result, normalizeContainer(c, vm))
	}
	return result
}

func normalizeContainer(c rawPodmanContainer, vm VMConfig) ContainerInfo {
	realName := ""
	if len(c.Names) > 0 {
		realName = c.Names[0]
	}

	running := strings.EqualFold(c.State, "running") ||
		strings.HasPrefix(strings.ToLower(c.Status), "up")

	info := ContainerInfo{
		RealName: realName,
		Status:   c.Status,
		Running:  running,
		Protocol: "http",
		VMName:   vm.Name,
		VMIP:     vm.IP,
		Node:     vm.Node,
	}

	// Порт: приоритет у label port-XXXX, иначе первый host_port из Ports.
	if c.Ports != nil {
		for _, p := range c.Ports {
			if p.HostPort > 0 {
				info.Port = strconv.Itoa(p.HostPort)
				if p.Protocol != "" {
					info.Protocol = p.Protocol
				}
				break
			}
		}
	}

	// Имя: по умолчанию реальное имя без префикса systemd-.
	info.Name = strings.TrimPrefix(realName, "systemd-")
	// Срезаем ведущий нумерационный префикс вида "01-athens" -> "athens".
	if parts := strings.SplitN(info.Name, "-", 2); len(parts) == 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			info.Name = parts[1]
		}
	}

	// Labels переопределяют.
	if c.Labels != nil {
		for k, v := range c.Labels {
			lk := strings.ToLower(strings.TrimSpace(k))
			lv := strings.TrimSpace(v)
			switch {
			case strings.HasPrefix(lk, "port-"):
				info.Port = strings.TrimPrefix(lk, "port-")
				if lv != "" {
					info.Port = lv
				}
			case strings.HasPrefix(lk, "proto-"):
				info.Protocol = strings.TrimPrefix(lk, "proto-")
				if lv != "" {
					info.Protocol = lv
				}
			case strings.HasPrefix(lk, "name-"):
				info.Name = lv
			}
		}
	}

	return info
}
