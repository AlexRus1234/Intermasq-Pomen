// caddy-mock — мини-Caddy Admin API для тестов Pomen.
// Реализует только те эндпоинты, которые реально дёргает core.CaddyClient:
// GET/PUT/DELETE /id/<id>, POST-init для parent-путей, /stop.
//
// Запуск: CADDY_MOCK_ADDR=:18090 go run .
// Не претендует на полное соответствие Caddy — только enough для smoke.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	mu   sync.Mutex
	data = map[string]interface{}{}
)

func main() {
	addr := os.Getenv("CADDY_MOCK_ADDR")
	if addr == "" {
		addr = ":18090"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)

	log.Printf("caddy-mock listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	body, _ := io.ReadAll(r.Body)
	var payload interface{}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}

	mu.Lock()
	defer mu.Unlock()

	log.Printf("[caddy-mock] %s %s (body=%dB)", r.Method, path, len(body))

	switch {
	// === /id/<id> CRUD ===
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/id/"):
		id := strings.TrimPrefix(path, "/id/")
		if v, ok := data[id]; ok {
			writeJSON(w, http.StatusOK, v)
		} else {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		}
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/id/"):
		id := strings.TrimPrefix(path, "/id/")
		data[id] = payload
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/id/"):
		id := strings.TrimPrefix(path, "/id/")
		delete(data, id)
		w.WriteHeader(http.StatusOK)

	// === POST policies/routes: имитируем "нет родителя" при первом вызове
	case r.Method == http.MethodPost && path == "/config/apps/tls/automation/policies":
		if _, ok := data["__tls_automation_init__"]; !ok {
			data["__tls_automation_init__"] = true
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no parent /config/apps/tls/automation"})
			return
		}
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && path == "/config/apps/http/servers/srv0/routes":
		if _, ok := data["__srv0_init__"]; !ok {
			data["__srv0_init__"] = true
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no server srv0"})
			return
		}
		w.WriteHeader(http.StatusOK)

	// === automate / parent init / stop
	case r.Method == http.MethodPost && path == "/config/apps/tls/certificates/automate":
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut && (path == "/config/apps/tls" || path == "/config/apps/http/servers/srv0"):
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && path == "/stop":
		w.WriteHeader(http.StatusOK)

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":  "mock: unsupported",
			"path":   path,
			"method": r.Method,
		})
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "caddy-mock encode: %v\n", err)
	}
}
