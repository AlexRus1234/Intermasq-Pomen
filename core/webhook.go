package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
// Реальный JSON богаче, но нам нужны только Names, Status и Labels.
type rawPodmanContainer struct {
	Names  []string          `json:"Names"`
	Status string            `json:"Status"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// ListContainers стучится на вебхук ВМ и возвращает нормализованный список.
// endpoint — путь на вебхуке (по умолчанию /hooks/podman — стандартный mount adnanh/webhook).
func (w *WebhookClient) ListContainers(vm VMConfig, endpoint string) ([]ContainerInfo, error) {
	if endpoint == "" {
		endpoint = "/hooks/podman"
	}
	url := strings.TrimRight(vm.WebhookURL, "/") + endpoint

	// adnanh/webhook по умолчанию ожидает POST. GET может вернуть 404/405.
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
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

	result := make([]ContainerInfo, 0, len(raw))
	for _, c := range raw {
		realName := ""
		if len(c.Names) > 0 {
			realName = c.Names[0]
		}

		running := strings.EqualFold(c.State, "running") || strings.EqualFold(c.Status, "running") ||
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

		if info.Name == "" {
			info.Name = realName
		}

		result = append(result, info)
	}
	return result, nil
}
