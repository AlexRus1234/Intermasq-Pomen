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
	"log/slog"
	"strings"
	"time"
)

// Engine — оркестратор Pomen.
// В отличие от Povez, не обращается к Proxmox/Intermasq: источник контейнеров —
// вебхуки ВМ (по кнопке), DNS не пишется (wildcard в dnsmasq), Caddy — те же, что в Povez.
//
// Caddy и Webhook держит через интерфейсы (CaddyAPI/WebhookAPI), что позволяет
// подменять их моками в тестах Engine без поднятия реальной инфраструктуры.
type Engine struct {
	Caddy   CaddyAPI
	State   *StateStore
	VMs     *VMStore
	Webhook WebhookAPI
	Domain  string
	Nodes   map[string]NodeConfig
}

func NewEngine(caddy CaddyAPI, state *StateStore, vms *VMStore, wh WebhookAPI, domain string, nodes map[string]NodeConfig) *Engine {
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
//
// Компенсация частичных отказов (audit §14.5):
//   - Caddy.ReplayRoute → error: ничего не записано, выходим.
//   - Caddy.RestartCaddy → error: маршрут в Caddy уже настроен, но сертификат
//     может не примениться. Пытаемся откатить — DeleteRouteAndTLS (best-effort),
//     возвращаем ошибку. Это не гарантирует полную чистоту (Caddy не
//     транзакционен), но в штатных случаях спасает от фантомных маршрутов.
//   - State.Upsert → error: домен фактически уже настроен в Caddy; пишем
//     в лог warning и возвращаем ошибку. State-файл — источник правды для UI,
//     пользователь не увидит домен в списке, но сам домен работает.
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

	// ReplayRoute использует upsertByID: GET /id/<id> → PUT если существует,
	// иначе POST с инициализацией родительского пути. Это гарантирует, что
	// повторный Provision того же домена не создаст дубль TLS-политики в
	// Caddy ("cannot apply more than one automation policy to host").
	if err := e.Caddy.ReplayRoute(nodeKey, domain, c.VMIP, c.Port, c.Protocol, routeID, tlsID); err != nil {
		return "", fmt.Errorf("Caddy ошибка: %w", err)
	}

	// The Nuke: жёсткий рестарт Caddy для применения выпущенного сертификата.
	// POST /stop завершает процесс; systemd (Restart=always) поднимает его
	// снова с чистым кэшем. Альтернатива — reload — иногда не подхватывает
	// новый сертификат сразу; /stop надёжнее. Согласованная цена — даунтайм
	// ~1-2 секунды на ноде.
	if err := e.Caddy.RestartCaddy(nodeKey); err != nil {
		// Компенсация: маршрут и TLS-политика уже в Caddy, но без рестарта
		// сертификат не применён. Пытаемся удалить запись, чтобы UI/state
		// и Caddy не разошлись. Caddy.DeleteRouteAndTLS — best-effort,
		// внутренние ошибки логируются в самом методе.
		slog.Warn("caddy restart failed after ReplayRoute, rolling back route", "node", nodeKey, "route_id", routeID, "err", err)
		_ = e.Caddy.DeleteRouteAndTLS(nodeKey, routeID, tlsID)
		return "", fmt.Errorf("Caddy restart (попытка отката выполнена): %w", err)
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
			// Caddy уже настроен и перезапущен — домен РАБОТАЕТ, но мы не
			// смогли записать факт этого в state. UI не покажет домен;
			// пользователь может пере-выдать (State.Upsert идемпотентен по
			// route_id) или вручную добавить запись. Не откатываем Caddy,
			// чтобы не сломать уже выданный сертификат.
			slog.Error("state write failed AFTER caddy configured; domain is live but invisible to UI",
				"domain", domain, "route_id", routeID, "err", err)
			return "", fmt.Errorf("state ошибка (домен уже активен в Caddy, но не записан в state): %w", err)
		}
	}

	return fmt.Sprintf("Успех! %s -> %s:%s", domain, c.VMIP, c.Port), nil
}

// DeprovisionByID удаляет маршрут по routeID (ручная очистка).
// Не проверяет статус контейнера (offline держим по решению A).
//
// Порядок: сначала State.Remove, потом Caddy.DeleteRouteAndTLS. State для нас
// источник правды — если Caddy-удаление упадёт, запись уже убрана и при
// следующем Replay не будет воскрешена. Обратный порядок (как было раньше)
// при сбое state.Remove оставлял бы "висящий" в state маршрут без домена в
// Caddy, который потом нельзя повторно удалить (Caddy уже пуст).
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

	if err := e.State.Remove(rec.RouteID); err != nil {
		return fmt.Errorf("state remove: %w", err)
	}
	// State уже почищен — Caddy-удаление best-effort. Если оно упадёт, в Caddy
	// останется фантомный маршрут, но UI/state будет консистентен; при
	// следующем полном ReplayCaddy фантом не вернётся (его нет в state).
	// Caddy.DeleteRouteAndTLS логирует ошибки внутри себя.
	_ = e.Caddy.DeleteRouteAndTLS(rec.Node, rec.RouteID, rec.TLSID)
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
