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

// NodeConfig описывает ноду из статического config.json.
// В Pomen нода хранит только URL Caddy (всё остальное относится к ВМ).
type NodeConfig struct {
	CaddyURL string `json:"caddy_url"`
}

// VMConfig описывает одну ВМ с Podman, зарегистрированную через UI.
// VM-ы короткоживущие и управляются динамически (vms.json).
type VMConfig struct {
	Name       string `json:"name"`
	Node       string `json:"node"`
	IP         string `json:"ip"`
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"`
}

// ContainerInfo — распарсенный контейнер Podman с нормализованными полями.
// Port/Protocol/Name берутся из labels port-/proto-/name- (как теги в PVE).
type ContainerInfo struct {
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Status   string `json:"status"`
	Running  bool   `json:"running"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	VMName   string `json:"vm_name"`
	VMIP     string `json:"vm_ip"`
	Node     string `json:"node"`
}
