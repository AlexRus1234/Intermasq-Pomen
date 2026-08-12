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

// Дефолтные значения для конфигурации Pomen. Любое из них перекрывается
// соответствующим полем в config.json (audit §5 — все эти значения раньше
// были захардкожены в разброс по коду).

const (
	// DefaultCaddyTimeout — HTTP-таймаут запросов к Caddy Admin API.
	DefaultCaddyTimeout = 10 * time.Second

	// DefaultWebhookTimeout — HTTP-таймаут запросов к вебхукам ВМ.
	// Выше, чем Caddy, потому что podman ps может долго отвечать под нагрузкой.
	DefaultWebhookTimeout = 15 * time.Second

	// DefaultRestartDelay — пауза между записью последнего маршрута и
	// вызовом /stop в ReplayCaddy. Даёт Caddy время завершить CertManager.
	DefaultRestartDelay = 2 * time.Second

	// DefaultWebhookPath — стандартный mount point adnanh/webhook.
	DefaultWebhookPath = "/hooks/podman"

	// DefaultVMSecretHeader — HTTP-заголовок для вебхук-секрета ВМ.
	DefaultVMSecretHeader = "X-VM-Secret"
)

// Дефолты для TLS-config (раньше были захардкожены в config.go —
// audit §5). Перекрываются полями tls.* в config.json.
const (
	// DefaultACMECA — URL ACME-директории внутреннего Step-CA.
	DefaultACMECA = "https://172.20.0.1:9000/acme/acme/directory"

	// DefaultRootCAPath — путь к root CA PEM на ноде Caddy.
	DefaultRootCAPath = "/etc/caddy/root_ca.crt"
)

// Шаблоны идентификаторов и доменов. Раньше захардкожены в Engine.Provision
// (audit §5). Параметры: vm, name, node.
const (
	// FQDNFormat — формат домена контейнера: <name>.<vm>.<node><base_domain>.
	// base_domain обычно ".internal".
	FQDNFormat = "%s.%s.%s%s"

	// RouteIDFormat — ID маршрута в Caddy: pod-<vm>-<name>-<node>.
	// Префикс "pod-" отличает Pomen-маршруты от Povez ("proxy-").
	RouteIDFormat = "pod-%s-%s-%s"

	// TLSIDFormat — ID TLS-политики в Caddy: podtls-<vm>-<name>-<node>.
	TLSIDFormat = "podtls-%s-%s-%s"
)

// Префиксы имён контейнеров (webhook normalization).
const (
	// ContainerSystemdPrefix — префикс Quadlet для systemd-unit'ов.
	// Срезается при нормализации имени (например "systemd-athens" → "athens").
	ContainerSystemdPrefix = "systemd-"
)
