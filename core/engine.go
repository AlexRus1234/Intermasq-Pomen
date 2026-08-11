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
	"fmt"
	"strings"
	"time"
)

// Engine — оркестратор Pomen.
// В отличие от Povez, не обращается к Proxmox/Intermasq: источник контейнеров —
// вебхуки ВМ (по кнопке), DNS не пишется (wildcard в dnsmasq), Caddy — те же, что в Povez.
type Engine struct {
	Caddy   *CaddyClient
	State   *StateStore
	VMs     *VMStore
	Webhook *WebhookClient
	Domain  string
	Nodes   map[string]NodeConfig
}

func NewEngine(caddy *CaddyClient, state *StateStore, vms *VMStore, wh *WebhookClient, domain string, nodes map[string]NodeConfig) *Engine {
	cleanNodes := make(map[string]NodeConfig)
	for k, v := range nodes {
		cleanNodes[strings.ToLower(k)] = v
	}
	return &Engine{
		Caddy:   caddy,
		State:   state,
		VMs:     vms,
		Webhook: wh,
		Domain:  domain,
		Nodes:   cleanNodes,
	}
}

// ListVMs пробрасывает реестр ВМ (для UI).
func (e *Engine) ListVMs() ([]VMConfig, error) {
	return e.VMs.List()
}

// ListNodes отдаёт ключи известных нод (для дропдауна в UI).
func (e *Engine) ListNodes() []string {
	out := make([]string, 0, len(e.Nodes))
	for k := range e.Nodes {
		out = append(out, k)
	}
	return out
}

// AddVM регистрирует новую ВМ (запрет коллизий по имени — в VMStore.Upsert).
func (e *Engine) AddVM(vm VMConfig) error {
	if _, ok := e.Nodes[strings.ToLower(vm.Node)]; !ok {
		return fmt.Errorf("неизвестная нода: %s", vm.Node)
	}
	return e.VMs.Upsert(vm)
}

// RemoveVM удаляет ВМ из реестра. Маршруты Caddy/state НЕ трогаются (вариант A).
func (e *Engine) RemoveVM(name string) error {
	return e.VMs.Delete(name)
}

// GetContainers дёргает вебхук указанной ВМ и возвращает контейнеры.
func (e *Engine) GetContainers(vmName string) ([]ContainerInfo, error) {
	vm, err := e.VMs.Get(vmName)
	if err != nil {
		return nil, err
	}
	return e.Webhook.ListContainers(vm, "/hooks/podman")
}

// Provision выдаёт домен контейнеру: пишет маршрут в Caddy и запись в state.
// DNS не трогается — wildcard в dnsmasq уже ведёт *.<node>.internal на Caddy.
//
// Параметры берутся из ContainerInfo (уже распарсенные labels).
func (e *Engine) Provision(c ContainerInfo) (string, error) {
	if c.Port == "" {
		return "", fmt.Errorf("у контейнера %s нет label port-XXXX", c.RealName)
	}
	if _, ok := e.Nodes[strings.ToLower(c.Node)]; !ok {
		return "", fmt.Errorf("неизвестная нода: %s", c.Node)
	}

	name := strings.ToLower(c.Name)
	vmName := strings.ToLower(c.VMName)
	nodeKey := strings.ToLower(c.Node)

	domain := fmt.Sprintf("%s.%s.%s%s", name, vmName, nodeKey, e.Domain)
	routeID := fmt.Sprintf("pod-%s-%s-%s", vmName, name, nodeKey)
	tlsID := fmt.Sprintf("podtls-%s-%s-%s", vmName, name, nodeKey)

	// Используем ReplayRoute (upsertByID) вместо AddRoute (чистый POST),
	// чтобы повторный Provision того же домена не создавал дубль TLS-политики
	// в Caddy ("cannot apply more than one automation policy to host").
	if err := e.Caddy.ReplayRoute(nodeKey, domain, c.VMIP, c.Port, c.Protocol, routeID, tlsID); err != nil {
		return "", fmt.Errorf("Caddy ошибка: %w", err)
	}

	// The Nuke: жесткий рестарт Caddy для применения сертификата (как AddRoute в Povez).
	if err := e.Caddy.RestartCaddy(nodeKey); err != nil {
		return "", fmt.Errorf("Caddy restart: %w", err)
	}

	if e.State != nil {
		if err := e.State.Upsert(RouteRecord{
			Domain:     domain,
			TargetIP:   c.VMIP,
			TargetPort: c.Port,
			Protocol:   c.Protocol,
			RouteID:    routeID,
			TLSID:      tlsID,
			Node:       nodeKey,
			VMName:     c.VMName,
		}); err != nil {
			return "", fmt.Errorf("state ошибка: %w", err)
		}
	}

	return fmt.Sprintf("Успех! %s -> %s:%s", domain, c.VMIP, c.Port), nil
}

// DeprovisionByID удаляет маршрут по routeID (ручная очистка).
// Не проверяет статус контейнера (offline держим по решению A).
func (e *Engine) DeprovisionByID(routeID string) error {
	if e.State == nil {
		return fmt.Errorf("state store не инициализирован")
	}
	records, err := e.State.Load()
	if err != nil {
		return err
	}
	var rec RouteRecord
	found := false
	for _, r := range records {
		if r.RouteID == routeID {
			rec = r
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("маршрут %s не найден", routeID)
	}

	e.Caddy.DeleteRouteAndTLS(rec.Node, rec.RouteID, rec.TLSID)
	if err := e.State.Remove(rec.RouteID); err != nil {
		return fmt.Errorf("state remove: %w", err)
	}
	return nil
}

// ReplayCaddy перезаписывает конфиг Caddy из файла-таблицы (восстановление после сброса).
func (e *Engine) ReplayCaddy() (int, []string, error) {
	if e.State == nil {
		return 0, nil, fmt.Errorf("state store не инициализирован")
	}
	records, err := e.State.Load()
	if err != nil {
		return 0, nil, err
	}
	if len(records) == 0 {
		return 0, nil, nil
	}

	ok := 0
	var errors []string
	touchedNodes := make(map[string]bool)

	for _, rec := range records {
		if err := e.Caddy.ReplayRoute(rec.Node, rec.Domain, rec.TargetIP, rec.TargetPort, rec.Protocol, rec.RouteID, rec.TLSID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", rec.Domain, err))
			continue
		}
		ok++
		touchedNodes[rec.Node] = true
	}

	time.Sleep(2 * time.Second)
	for node := range touchedNodes {
		if err := e.Caddy.RestartCaddy(node); err != nil {
			errors = append(errors, fmt.Sprintf("рестарт %s: %v", node, err))
		}
	}

	return ok, errors, nil
}

// GetState отдаёт содержимое файла-таблицы для UI.
func (e *Engine) GetState() ([]RouteRecord, error) {
	if e.State == nil {
		return []RouteRecord{}, nil
	}
	return e.State.Load()
}
