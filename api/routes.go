package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"pomen/core"
)

type ApiServer struct {
	Engine *core.Engine
}

func NewApiServer(e *core.Engine) *ApiServer {
	return &ApiServer{Engine: e}
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

// HandleVMs: GET — список, POST — добавить/обновить.
func (s *ApiServer) HandleVMs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		vms, err := s.Engine.ListVMs()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, vms)

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
func (s *ApiServer) HandleDeleteVM(w http.ResponseWriter, r *http.Request) {
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
func (s *ApiServer) HandleGetContainers(w http.ResponseWriter, r *http.Request) {
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
func (s *ApiServer) HandleProvision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VMName         string `json:"vm"`
		ContainerName  string `json:"container_name"`
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
func (s *ApiServer) HandleDeprovision(w http.ResponseWriter, r *http.Request) {
	var req struct{ RouteID string `json:"route_id"` }
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
func (s *ApiServer) HandleDeprovisionByID(w http.ResponseWriter, r *http.Request) {
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
func (s *ApiServer) HandleGetState(w http.ResponseWriter, r *http.Request) {
	records, err := s.Engine.GetState()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, records)
}

// HandleReplay: POST /api/replay
func (s *ApiServer) HandleReplay(w http.ResponseWriter, r *http.Request) {
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
