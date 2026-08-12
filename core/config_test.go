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
	"testing"
	"time"
)

func TestConfig_WithDefaults_FillsEverything(t *testing.T) {
	cfg := Config{
		BaseDomain: ".internal",
		Nodes:      map[string]NodeConfig{"n0": {CaddyURL: "http://caddy:2019"}},
		// остальные секции пустые — должны подтянуться defaults
	}
	r := cfg.WithDefaults()

	if r.BaseDomain != ".internal" {
		t.Errorf("BaseDomain lost: %q", r.BaseDomain)
	}
	if r.TLS.ACMECA == "" {
		t.Error("TLS.ACMECA not defaulted")
	}
	if r.TLS.RootCAPath == "" {
		t.Error("TLS.RootCAPath not defaulted")
	}
	if r.Webhook.Path != DefaultWebhookPath {
		t.Errorf("Webhook.Path = %q, want %q", r.Webhook.Path, DefaultWebhookPath)
	}
	if r.Webhook.SecretHeader != DefaultVMSecretHeader {
		t.Errorf("Webhook.SecretHeader = %q, want %q", r.Webhook.SecretHeader, DefaultVMSecretHeader)
	}
	if r.CaddyTimeout != DefaultCaddyTimeout {
		t.Errorf("CaddyTimeout = %v, want %v", r.CaddyTimeout, DefaultCaddyTimeout)
	}
	if r.WebhookTimeout != DefaultWebhookTimeout {
		t.Errorf("WebhookTimeout = %v, want %v", r.WebhookTimeout, DefaultWebhookTimeout)
	}
	if r.RestartDelayDur != DefaultRestartDelay {
		t.Errorf("RestartDelayDur = %v, want %v", r.RestartDelayDur, DefaultRestartDelay)
	}
}

func TestConfig_WithDefaults_OverridesApplied(t *testing.T) {
	cfg := Config{
		BaseDomain: ".internal",
		Nodes:      map[string]NodeConfig{"n0": {}},
		TLS:        TLSConfig{ACMECA: "https://custom/ca", RootCAPath: "/x.crt"},
		Webhook:    WebhookConfig{Path: "/custom", SecretHeader: "X-Custom"},
		Timeouts: TimeoutsConfig{
			Caddy:        "30s",
			Webhook:      "45s",
			RestartDelay: "5s",
		},
	}
	r := cfg.WithDefaults()
	if r.TLS.ACMECA != "https://custom/ca" {
		t.Errorf("TLS.ACMECA override lost: %q", r.TLS.ACMECA)
	}
	if r.Webhook.Path != "/custom" {
		t.Errorf("Webhook.Path override lost: %q", r.Webhook.Path)
	}
	if r.CaddyTimeout != 30*time.Second {
		t.Errorf("CaddyTimeout = %v, want 30s", r.CaddyTimeout)
	}
	if r.RestartDelayDur != 5*time.Second {
		t.Errorf("RestartDelayDur = %v, want 5s", r.RestartDelayDur)
	}
}

func TestConfig_WithDefaults_BadDurationFallsBack(t *testing.T) {
	cfg := Config{
		BaseDomain: ".internal",
		Nodes:      map[string]NodeConfig{"n0": {}},
		Timeouts:   TimeoutsConfig{Caddy: "not-a-duration"},
	}
	r := cfg.WithDefaults()
	if r.CaddyTimeout != DefaultCaddyTimeout {
		t.Errorf("bad duration should fall back to default; got %v", r.CaddyTimeout)
	}
}

// Гарантируем что исходный cfg не мутируется (WithDefaults возвращает копию).
func TestConfig_WithDefaults_DoesNotMutateInput(t *testing.T) {
	cfg := Config{
		BaseDomain: ".internal",
		Nodes:      map[string]NodeConfig{"n0": {}},
	}
	_ = cfg.WithDefaults()
	if cfg.TLS.ACMECA != "" {
		t.Errorf("WithDefaults mutated input cfg.TLS.ACMECA = %q", cfg.TLS.ACMECA)
	}
	if cfg.Webhook.Path != "" {
		t.Errorf("WithDefaults mutated input cfg.Webhook.Path = %q", cfg.Webhook.Path)
	}
}
