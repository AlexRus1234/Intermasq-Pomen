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

package api

import (
	"encoding/json"
	"net/http"
	"pomen/core"
	"strings"
)

// Server — HTTP API Pomen: оборачивает Engine и регистрирует его обработчики
// на http.ServeMux через Register.
//
// Раньше назывался ApiServer — нарушал Go-конвенцию для аббревиатур
// (initialisms должны быть ALLCAPS: API, не Api). Переименовано в Server
// (audit §14.23): в контексте пакета api этого достаточно и читается естественно.
type Server struct {
	Engine  *core.Engine
	Version string
}

func NewServer(e *core.Engine, version string) *Server {
	return &Server{Engine: e, Version: version}
}

// VMView — публичное представление ВМ без секретов.
// Раньше GET /api/vms отдавал VMConfig целиком, включая поле Secret —
// это утечка вебхук-секретов через API (audit §14.3). Принимаем Secret
// при POST, но никогда не возвращаем.
type VMView struct {
	Name       string `json:"name"`
	Node       string `json:"node"`
	IP         string `json:"ip"`
	WebhookURL string `json:"webhook_url"`
}

func toVMView(v core.VMConfig) VMView {
	return VMView{Name: v.Name, Node: v.Node, IP: v.IP, WebhookURL: v.WebhookURL}
}

func jsonResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// === Реестр ВМ (CRUD) ===

// HandleNodes: GET /api/nodes — список ключей нод из config.json (для дропдауна UI).
func (s *Server) HandleNodes(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, s.Engine.ListNodes())
}

// HandleVMs: GET — список, POST — добавить/обновить.
func (s *Server) HandleVMs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		vms, err := s.Engine.ListVMs()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		views := make([]VMView, len(vms))
		for i, v := range vms {
			views[i] = toVMView(v)
		}
		jsonResponse(w, http.StatusOK, views)

	case http.MethodPost:
		var vm core.VMConfig
		if err := json.NewDecoder(r.Body).Decode(&vm); err != nil {
			jsonError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		if strings.TrimSpace(vm.Name) == "" || strings.TrimSpace(vm.Node) == "" ||
			strings.TrimSpace(vm.IP) == "" || strings.TrimSpace(vm.WebhookURL) == "" {
			jsonError(w, http.StatusBadRequest, "Поля name, node, ip, webhook_url обязательны")
			return
		}
		if err := s.Engine.AddVM(vm); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "ВМ сохранена"})

	default:
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleDeleteVM: DELETE /api/vms/:name
func (s *Server) HandleDeleteVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/vms/")
	name = strings.Trim(name, "/")
	if name == "" {
		jsonError(w, http.StatusBadRequest, "Имя ВМ обязательно")
		return
	}
	if err := s.Engine.RemoveVM(name); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "ВМ удалена из реестра"})
}

// === Контейнеры (on-demand pull) ===

// HandleGetContainers: GET /api/containers?vm=<name>
func (s *Server) HandleGetContainers(w http.ResponseWriter, r *http.Request) {
	vm := r.URL.Query().Get("vm")
	if vm == "" {
		jsonError(w, http.StatusBadRequest, "Параметр vm обязателен")
		return
	}
	containers, err := s.Engine.GetContainers(vm)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, containers)
}

// === Маршруты ===

// HandleProvision: POST {vm, container_name}
// Контейнер ищется в свежем списке от вебхука ВМ (чтобы гарантировать актуальный IP/порт).
func (s *Server) HandleProvision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VMName        string `json:"vm"`
		ContainerName string `json:"container_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.VMName == "" || req.ContainerName == "" {
		jsonError(w, http.StatusBadRequest, "Поля vm и container_name обязательны")
		return
	}

	containers, err := s.Engine.GetContainers(req.VMName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var target *core.ContainerInfo
	for i := range containers {
		if containers[i].Name == req.ContainerName || containers[i].RealName == req.ContainerName {
			target = &containers[i]
			break
		}
	}
	if target == nil {
		jsonError(w, http.StatusNotFound, "Контейнер не найден в свежем списке ВМ")
		return
	}

	msg, err := s.Engine.Provision(*target)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": msg})
}

// HandleDeprovision — оставлен для совместимости (POST {route_id}).
func (s *Server) HandleDeprovision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RouteID string `json:"route_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if req.RouteID == "" {
		jsonError(w, http.StatusBadRequest, "route_id обязателен")
		return
	}
	if err := s.Engine.DeprovisionByID(req.RouteID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Маршрут удалён"})
}

// HandleDeprovisionByID: DELETE /api/deprovision/:routeID
func (s *Server) HandleDeprovisionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	routeID := strings.TrimPrefix(r.URL.Path, "/api/deprovision/")
	routeID = strings.Trim(routeID, "/")
	if routeID == "" {
		jsonError(w, http.StatusBadRequest, "route_id обязателен")
		return
	}
	if err := s.Engine.DeprovisionByID(routeID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Маршрут удалён"})
}

// HandleGetState: GET /api/state
func (s *Server) HandleGetState(w http.ResponseWriter, r *http.Request) {
	records, err := s.Engine.GetState()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, records)
}

// HandleReplay: POST /api/replay
func (s *Server) HandleReplay(w http.ResponseWriter, r *http.Request) {
	ok, errs, err := s.Engine.ReplayCaddy()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"ok":     ok,
		"errors": errs,
	})
}

// HandleVersion: GET /api/version — отдаёт версию сборки (из internal/version,
// в CI перекрывается через -ldflags). Используется UI для отображения и
// диагностическими скриптами для проверки, что запущен ожидаемый билд.
func (s *Server) HandleVersion(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"version": s.Version})
}
