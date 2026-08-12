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
	"errors"
	"path/filepath"
	"testing"
)

// mockCaddy — реализация CaddyAPI для тестов Engine без живого Caddy.
// Каждое поле — функция; nil = no-op / nil error (имитация успеха).
type mockCaddy struct {
	replayFn      func(node, domain, ip, port, proto, routeID, tlsID string) error
	restartFn     func(node string) error
	deleteRouteFn func(node, routeID, tlsID string) error

	replayCalls      int
	restartCalls     int
	deleteRouteCalls []string // собранные routeID для assert'ов
}

func (m *mockCaddy) ReplayRoute(node, domain, ip, port, proto, routeID, tlsID string) error {
	m.replayCalls++
	if m.replayFn != nil {
		return m.replayFn(routeID, domain, ip, port, proto, routeID, tlsID)
	}
	return nil
}
func (m *mockCaddy) RestartCaddy(node string) error {
	m.restartCalls++
	if m.restartFn != nil {
		return m.restartFn(node)
	}
	return nil
}
func (m *mockCaddy) DeleteRouteAndTLS(node, routeID, tlsID string) error {
	m.deleteRouteCalls = append(m.deleteRouteCalls, routeID)
	if m.deleteRouteFn != nil {
		return m.deleteRouteFn(node, routeID, tlsID)
	}
	return nil
}

func newTestEngine(t *testing.T, caddy CaddyAPI) (*Engine, *StateStore) {
	t.Helper()
	state := NewStateStore(filepath.Join(t.TempDir(), "routes.json"))
	vms := NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	return &Engine{
		Caddy:        caddy,
		State:        state,
		VMs:          vms,
		Webhook:      nil, // не нужен для Provision/Deprovision
		Domain:       ".internal",
		Nodes:        map[string]NodeConfig{"node0": {}},
		WebhookPath:  DefaultWebhookPath,
		RestartDelay: 0,
	}, state
}

func TestEngine_Provision_Success(t *testing.T) {
	mc := &mockCaddy{}
	e, state := newTestEngine(t, mc)

	msg, err := e.Provision(ContainerInfo{
		Name: "athens", RealName: "systemd-athens", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0",
		Port: "8080", Protocol: "http",
	})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if mc.replayCalls != 1 || mc.restartCalls != 1 {
		t.Errorf("want 1 ReplayRoute + 1 RestartCaddy, got replay=%d restart=%d", mc.replayCalls, mc.restartCalls)
	}
	if mc.deleteRouteCalls != nil {
		t.Errorf("no rollback expected on success, got %v", mc.deleteRouteCalls)
	}
	records, _ := state.Load()
	if len(records) != 1 || records[0].Domain != "athens.vm0.node0.internal" {
		t.Errorf("state record wrong: %+v", records)
	}
	if !contains(msg, "athens.vm0.node0.internal") {
		t.Errorf("success message lost domain: %q", msg)
	}
}

// TestEngine_Provision_CaddyRestartFails_Rollback — Regression для этапа 4:
// при ошибке RestartCaddy Engine должен попытаться удалить маршрут через
// DeleteRouteAndTLS и НЕ записать ничего в state.
func TestEngine_Provision_CaddyRestartFails_Rollback(t *testing.T) {
	mc := &mockCaddy{
		restartFn: func(node string) error { return errors.New("caddy 500") },
	}
	e, state := newTestEngine(t, mc)

	_, err := e.Provision(ContainerInfo{
		Name: "athens", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0",
		Port: "8080", Protocol: "http",
	})
	if err == nil {
		t.Fatal("expected error on RestartCaddy failure")
	}
	if len(mc.deleteRouteCalls) != 1 || mc.deleteRouteCalls[0] != "pod-vm0-athens-node0" {
		t.Errorf("expected rollback DeleteRouteAndTLS for route_id, got %v", mc.deleteRouteCalls)
	}
	records, _ := state.Load()
	if len(records) != 0 {
		t.Errorf("state should be empty after rollback, got %+v", records)
	}
}

func TestEngine_Provision_CaddyReplayFails_NoRollback(t *testing.T) {
	mc := &mockCaddy{
		replayFn: func(node, domain, ip, port, proto, routeID, tlsID string) error {
			return errors.New("caddy unreachable")
		},
	}
	e, state := newTestEngine(t, mc)

	_, err := e.Provision(ContainerInfo{
		Name: "athens", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0",
		Port: "8080", Protocol: "http",
	})
	if err == nil {
		t.Fatal("expected error on ReplayRoute failure")
	}
	if len(mc.deleteRouteCalls) != 0 {
		t.Errorf("no rollback expected when ReplayRoute itself fails (nothing to delete), got %v", mc.deleteRouteCalls)
	}
	if mc.restartCalls != 0 {
		t.Errorf("RestartCaddy should NOT be called when ReplayRoute failed")
	}
	records, _ := state.Load()
	if len(records) != 0 {
		t.Errorf("state should be empty, got %+v", records)
	}
}

func TestEngine_Provision_UnknownNode(t *testing.T) {
	mc := &mockCaddy{}
	e, _ := newTestEngine(t, mc)
	_, err := e.Provision(ContainerInfo{
		Name: "x", VMName: "vm0", Node: "no-such-node", Port: "80",
	})
	if err == nil {
		t.Fatal("expected error for unknown node")
	}
	if mc.replayCalls != 0 {
		t.Errorf("ReplayRoute should not be called for unknown node")
	}
}

func TestEngine_Provision_NoPort(t *testing.T) {
	mc := &mockCaddy{}
	e, _ := newTestEngine(t, mc)
	_, err := e.Provision(ContainerInfo{
		Name: "x", VMName: "vm0", Node: "node0", // Port=""
	})
	if err == nil {
		t.Fatal("expected error when container has no port label")
	}
}

// TestEngine_Deprovision_StateFirst — Regression для этапа 4: State.Remove
// должен идти ДО Caddy.DeleteRouteAndTLS, чтобы при падении Caddy запись
// уже была убрана и не воскрешалась при следующем Replay.
func TestEngine_Deprovision_StateFirst(t *testing.T) {
	mc := &mockCaddy{}
	e, state := newTestEngine(t, mc)

	// Записываем маршрут напрямую в state, имитируя успешный Provision.
	if err := state.Upsert(RouteRecord{
		RouteID: "pod-vm0-athens-node0", TLSID: "podtls-vm0-athens-node0",
		Domain: "athens.vm0.node0.internal", Node: "node0", VMName: "vm0",
		TargetIP: "10.0.0.5", TargetPort: "8080", Protocol: "http",
	}); err != nil {
		t.Fatal(err)
	}

	if err := e.DeprovisionByID("pod-vm0-athens-node0"); err != nil {
		t.Fatalf("DeprovisionByID: %v", err)
	}
	if len(mc.deleteRouteCalls) != 1 {
		t.Errorf("expected 1 DeleteRouteAndTLS call, got %d", len(mc.deleteRouteCalls))
	}
	records, _ := state.Load()
	if len(records) != 0 {
		t.Errorf("state should be empty after Deprovision, got %+v", records)
	}
}

func TestEngine_Deprovision_UnknownRouteID(t *testing.T) {
	mc := &mockCaddy{}
	e, _ := newTestEngine(t, mc)
	if err := e.DeprovisionByID("missing"); err == nil {
		t.Fatal("expected error for unknown route_id")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && containsStr(s, sub)))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
