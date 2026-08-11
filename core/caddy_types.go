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

// Caddy Admin API JSON-структуры.
//
// Раньше код строил map[string]interface{} вручную (audit §4.2): это работало,
// но не давало ни проверки типов на этапе компиляции, ни удобного чтения.
// Здесь — точные типы для маршрута, TLS-политики и их родительских контейнеров
// (initIfMissing payloads). Сериализуются через encoding/json с json-тегов.

// CaddyRoute — один маршрут под /config/apps/http/servers/<name>/routes[].
type CaddyRoute struct {
	ID     string        `json:"@id"`
	Match  []CaddyMatch  `json:"match"`
	Handle []CaddyHandle `json:"handle"`
}

// CaddyMatch — условие матчинга входящего запроса (по Host).
type CaddyMatch struct {
	Host []string `json:"host"`
}

// CaddyHandle — обработчик reverse_proxy.
type CaddyHandle struct {
	Handler   string          `json:"handler"` // всегда "reverse_proxy"
	Upstreams []CaddyUpstream `json:"upstreams"`
	Transport CaddyTransport  `json:"transport"`
}

// CaddyUpstream — dial-адрес upstream-сервиса (IP:port).
type CaddyUpstream struct {
	Dial string `json:"dial"`
}

// CaddyTransport — transport-конфиг reverse_proxy.
// При protocol="https" добавляется TLS-блок с insecure_skip_verify
// (upstream — самоподписанные сертификаты в .internal сети).
type CaddyTransport struct {
	Protocol string            `json:"protocol"` // "http"
	TLS      *CaddyUpstreamTLS `json:"tls,omitempty"`
}

// CaddyUpstreamTLS — TLS-настройки reverse_proxy в сторону upstream.
type CaddyUpstreamTLS struct {
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
}

// CaddyTLSPolicy — автоматизация TLS-сертификата для домена.
type CaddyTLSPolicy struct {
	ID       string        `json:"@id"`
	Subjects []string      `json:"subjects"`
	Issuers  []CaddyIssuer `json:"issuers"`
}

// CaddyIssuer — ACME-issuer через внутренний Step-CA.
type CaddyIssuer struct {
	Module               string          `json:"module"` // "acme"
	CA                   string          `json:"ca"`
	TrustedRootsPEMFiles []string        `json:"trusted_roots_pem_files"`
	Challenges           CaddyChallenges `json:"challenges"`
}

// CaddyChallenges — конфиг challenges для ACME.
type CaddyChallenges struct {
	HTTP CaddyHTTPChallenge `json:"http"`
}

// CaddyHTTPChallenge — HTTP-01 challenge (отключён, используем DNS-01 / TLS-ALPN).
type CaddyHTTPChallenge struct {
	Disabled bool `json:"disabled"`
}

// caddyAutomationConfig — payload для PUT /config/apps/tls при первичной
// инициализации (parent missing → 500 на POST в policies).
type caddyAutomationConfig struct {
	Automation caddyAutomation `json:"automation"`
}

type caddyAutomation struct {
	Policies []CaddyTLSPolicy `json:"policies"`
}

// caddyServerConfig — payload для PUT /config/apps/http/servers/srv0 при
// первичной инициализации HTTP-сервера.
type caddyServerConfig struct {
	Listen []string     `json:"listen"`
	Routes []CaddyRoute `json:"routes"`
}
