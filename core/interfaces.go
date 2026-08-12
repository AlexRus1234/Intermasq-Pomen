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

// CaddyAPI — поверхность CaddyClient, которую использует Engine.
// Конкретный *CaddyClient реализует интерфейс неявно; в тестах Engine
// подменяется моком (audit §13.20: раньше Engine был жёстко связан с
// конкретными типами — невозможно было тестировать бизнес-логику без
// живого Caddy/вебхука).
type CaddyAPI interface {
	ReplayRoute(nodeName, domain, targetIP, targetPort, protocol, routeID, tlsID string) error
	RestartCaddy(nodeName string) error
	DeleteRouteAndTLS(nodeName, routeID, tlsID string)
}

// WebhookAPI — поверхность WebhookClient, используемая Engine.
type WebhookAPI interface {
	ListContainers(vm VMConfig, endpoint string) ([]ContainerInfo, error)
}
