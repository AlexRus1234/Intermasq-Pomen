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

// Имена путей Caddy Admin API. Раньше захардкожены строками по всему
// caddy.go (audit §4.2) — теперь в одном месте, проще искать и менять
// при переходе на новую версию Caddy.

const (
	// caddyPathTLS — корень TLS-конфигурации Caddy.
	caddyPathTLS = "/config/apps/tls"

	// caddyPathTLSPolicies — массив политик автоматизации TLS.
	caddyPathTLSPolicies = "/config/apps/tls/automation/policies"

	// caddyPathTLSCertAutomate — список доменов для automate (выпуск
	// сертификатов в фоне без привязки к конкретному ingress).
	caddyPathTLSCertAutomate = "/config/apps/tls/certificates/automate"

	// caddyPathHTTPServer — единственный HTTP-сервер Pomen ("srv0").
	caddyPathHTTPServer = "/config/apps/http/servers/srv0"

	// caddyPathHTTPServerRoutes — массив маршрутов HTTP-сервера.
	caddyPathHTTPServerRoutes = "/config/apps/http/servers/srv0/routes"

	// caddyStop — endpoint "остановить процесс" (systemd поднимет обратно).
	caddyStop = "/stop"
)

// Имя HTTP-сервера Caddy (srv0) — захардкожено в Caddy-конфиге Pomen.
const caddyHTTPServerName = "srv0"

// Listen-адрес Caddy-сервера — Pomen всегда терминирует :443.
const caddyHTTPListen = ":443"

// caddyIDURL формирует URL для операций с конкретным ID через Caddy Admin API.
func caddyIDURL(baseURL, id string) string {
	return baseURL + "/id/" + id
}
