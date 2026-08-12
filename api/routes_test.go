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

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"pomen/core"
)

// stubCaddy — no-op реализация CaddyAPI для тестов api-слоя.
// Проверяем API handlers, а не оркестрацию, поэтому Caddy не нужен.
type stubCaddy struct{}

func (stubCaddy) ReplayRoute(node, domain, ip, port, proto, routeID, tlsID string) error {
	return nil
}
func (stubCaddy) RestartCaddy(node string) error { return nil }
func (stubCaddy) DeleteRouteAndTLS(node, routeID, tlsID string) error {
	return nil
}

func newTestServer(t *testing.T) (*Server, *core.VMStore) {
	t.Helper()
	state := core.NewStateStore(filepath.Join(t.TempDir(), "routes.json"))
	vms := core.NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	engine := core.NewEngine(core.EngineOptions{
		Caddy:        stubCaddy{},
		State:        state,
		VMs:          vms,
		Domain:       ".internal",
		Nodes:        map[string]core.NodeConfig{"node0": {}},
		WebhookPath:  core.DefaultWebhookPath,
		RestartDelay: 0,
	})
	return NewServer(engine, "test-version"), vms
}

func do(t *testing.T, srv *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	switch {
	case strings.HasPrefix(target, "/api/version"):
		srv.HandleVersion(w, req)
	case strings.HasPrefix(target, "/api/nodes"):
		srv.HandleNodes(w, req)
	case target == "/api/vms":
		srv.HandleVMs(w, req)
	default:
		t.Fatalf("do: no handler mapping for %s", target)
	}
	return w
}

// TestHandleVMs_GET_HidesSecret — regression для этапа 2: VMConfig.Secret
// должен идти только на запись, в ответе GET /api/vms его быть не должно.
func TestHandleVMs_GET_HidesSecret(t *testing.T) {
	srv, vms := newTestServer(t)
	if err := vms.Upsert(core.VMConfig{
		Name: "vm0", Node: "node0", IP: "10.0.0.5",
		WebhookURL: "http://vm0:9000", Secret: "TOPSECRET-value",
	}); err != nil {
		t.Fatal(err)
	}

	w := do(t, srv, "GET", "/api/vms", "")
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "TOPSECRET-value") {
		t.Errorf("SECRET LEAKED in response: %s", body)
	}
	var views []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &views); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("want 1 VM, got %d", len(views))
	}
	for k := range views[0] {
		if k == "secret" {
			t.Errorf("response contains 'secret' field: %v", views[0])
		}
	}
}

// TestHandleVMs_POST_ThenGET — round-trip: POST с Secret принимается, при
// GET Secret не возвращается, остальные поля сохраняются.
func TestHandleVMs_POST_ThenGET(t *testing.T) {
	srv, _ := newTestServer(t)

	w := do(t, srv, "POST", "/api/vms",
		`{"name":"vm0","node":"node0","ip":"10.0.0.5","webhook_url":"http://vm0:9000","secret":"hidden"}`)
	if w.Code != 200 {
		t.Fatalf("POST status = %d body=%s", w.Code, w.Body.String())
	}

	w = do(t, srv, "GET", "/api/vms", "")
	if !strings.Contains(w.Body.String(), "vm0") {
		t.Errorf("GET after POST lost vm0: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "hidden") {
		t.Errorf("Secret leaked after round-trip: %s", w.Body.String())
	}
}

func TestHandleVMs_POST_RejectsMissingFields(t *testing.T) {
	srv, _ := newTestServer(t)
	w := do(t, srv, "POST", "/api/vms", `{"name":"vm0"}`)
	if w.Code != 400 {
		t.Errorf("missing fields should give 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestHandleVMs_POST_RejectsUnknownNode(t *testing.T) {
	srv, _ := newTestServer(t)
	w := do(t, srv, "POST", "/api/vms",
		`{"name":"vm0","node":"no-such","ip":"10.0.0.5","webhook_url":"http://x"}`)
	if w.Code != 400 {
		t.Errorf("unknown node should give 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestHandleNodes(t *testing.T) {
	srv, _ := newTestServer(t)
	w := do(t, srv, "GET", "/api/nodes", "")
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "node0") {
		t.Errorf("expected node0 in response, got %s", body)
	}
}

func TestHandleVersion(t *testing.T) {
	srv, _ := newTestServer(t)
	w := do(t, srv, "GET", "/api/version", "")
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["version"] != "test-version" {
		t.Errorf("version = %q, want test-version", got["version"])
	}
}

// Sanity-check: 405 на недопустимые методы (через http.ServeMux это
// происходит автоматически, но мы хотим видеть поведение Handler'а).
func TestHandleVMs_PUT_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	w := do(t, srv, http.MethodPut, "/api/vms", `{}`)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/vms status = %d, want 405", w.Code)
	}
}
