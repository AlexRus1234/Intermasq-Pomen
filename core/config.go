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

import "time"

// Config — содержимое config.json. Любое поле, кроме BaseDomain и Nodes,
// можно опустить — будут применены defaults (см. constants.go).
type Config struct {
	BaseDomain string                `json:"base_domain"`
	Nodes      map[string]NodeConfig `json:"nodes"`
	TLS        TLSConfig             `json:"tls,omitempty"`
	Webhook    WebhookConfig         `json:"webhook,omitempty"`
	Timeouts   TimeoutsConfig        `json:"timeouts,omitempty"`
}

// TLSConfig — параметры ACME-issuer'а, который Caddy использует для
// выпуска сертификатов. Дефолты — в constants.go (DefaultACMECA,
// DefaultRootCAPath); до рефакторинга были захардкожены (audit §5).
type TLSConfig struct {
	ACMECA     string `json:"acme_ca"`      // ACME directory URL
	RootCAPath string `json:"root_ca_path"` // путь к root CA PEM на ноде Caddy
}

// WebhookConfig — параметры доступа к вебхукам ВМ.
type WebhookConfig struct {
	Path         string `json:"path"`          // путь на вебхуке (DefaultWebhookPath)
	SecretHeader string `json:"secret_header"` // имя заголовка (DefaultVMSecretHeader)
}

// TimeoutsConfig — таймауты HTTP-клиентов и задержки оркестратора.
// Строки парсятся через time.ParseDuration (например "10s", "2s").
type TimeoutsConfig struct {
	Caddy        string `json:"caddy"`
	Webhook      string `json:"webhook"`
	RestartDelay string `json:"restart_delay"`
}

// WithDefaults возвращает копию Config, в которой пустые поля заполнены
// дефолтами. Парсит строковые таймауты в duration, чтобы потомкам не
// пришлось это делать на каждом вызове. Не мутирует исходный cfg.
func (cfg Config) WithDefaults() ResolvedConfig {
	r := ResolvedConfig{
		BaseDomain: cfg.BaseDomain,
		Nodes:      cfg.Nodes,
		TLS:        cfg.TLS,
		Webhook: WebhookConfig{
			Path:         orDefault(cfg.Webhook.Path, DefaultWebhookPath),
			SecretHeader: orDefault(cfg.Webhook.SecretHeader, DefaultVMSecretHeader),
		},
		Timeouts: TimeoutsConfig{
			Caddy:        orDefault(cfg.Timeouts.Caddy, DefaultCaddyTimeout.String()),
			Webhook:      orDefault(cfg.Timeouts.Webhook, DefaultWebhookTimeout.String()),
			RestartDelay: orDefault(cfg.Timeouts.RestartDelay, DefaultRestartDelay.String()),
		},
	}
	r.TLS.ACMECA = orDefault(r.TLS.ACMECA, DefaultACMECA)
	r.TLS.RootCAPath = orDefault(r.TLS.RootCAPath, DefaultRootCAPath)
	applyDur := func(s string, def time.Duration, dst *time.Duration) {
		if d, err := time.ParseDuration(s); err == nil {
			*dst = d
		} else {
			*dst = def
		}
	}
	applyDur(r.Timeouts.Caddy, DefaultCaddyTimeout, &r.CaddyTimeout)
	applyDur(r.Timeouts.Webhook, DefaultWebhookTimeout, &r.WebhookTimeout)
	applyDur(r.Timeouts.RestartDelay, DefaultRestartDelay, &r.RestartDelayDur)
	return r
}

// ResolvedConfig — Config, в котором все поля заполнены (нет пустых строк,
// Duration уже распарсены). Удобна для передачи в NewCaddyClient и т.п.
type ResolvedConfig struct {
	BaseDomain      string
	Nodes           map[string]NodeConfig
	TLS             TLSConfig
	Webhook         WebhookConfig
	Timeouts        TimeoutsConfig
	CaddyTimeout    time.Duration
	WebhookTimeout  time.Duration
	RestartDelayDur time.Duration
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
