// webhook-mock — мини-вебхук ВМ для тестов Pomen. Эмулирует adnanh/webhook
// на path /hooks/podman: проверяет X-VM-Secret (если задан) и возвращает
// фиксированный `podman ps --format json` ответ.
//
// Запуск: WEBHOOK_MOCK_ADDR=:18091 WEBHOOK_MOCK_SECRET=s3cr3t go run .

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// 2 контейнера: один "systemd-athens" (running, port 8081 + label override),
// второй "01-backend" (exited, без port label — Provision такого должен
// отвергнуть). Этого хватает для smoke: happy-path Provision + негативный
// сценарий "нет порта".
const podmanPSJSON = `[
  {
    "Names": ["systemd-athens"],
    "State": "running",
    "Status": "Up 5 minutes",
    "Ports": [{"host_port": 8081, "container_port": 8080, "protocol": "tcp"}],
    "Labels": {"port-8080": "", "proto-http": ""}
  },
  {
    "Names": ["01-backend"],
    "State": "exited",
    "Labels": {"name-x": "back"}
  }
]`

func main() {
	addr := os.Getenv("WEBHOOK_MOCK_ADDR")
	if addr == "" {
		addr = ":18091"
	}
	secret := os.Getenv("WEBHOOK_MOCK_SECRET")

	mux := http.NewServeMux()
	mux.HandleFunc("/hooks/podman", func(w http.ResponseWriter, r *http.Request) {
		// adnanh/webhook требует POST с non-empty JSON body.
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if secret != "" && r.Header.Get("X-VM-Secret") != secret {
			http.Error(w, "bad secret", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, podmanPSJSON)
	})

	log.Printf("webhook-mock listening on %s (secret set: %t)", addr, secret != "")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
