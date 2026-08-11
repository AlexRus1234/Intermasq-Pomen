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
	"net/http"

	"pomen/core"
)

// Register навешивает все обработчики Pomen на mux.
// Применяется в main.go (production/dev запуск) и в тестах через
// httptest.NewServer+custom mux. По образцу webapi.Register в Intermasq:
// единая точка регистрации, чтобы main.go не знал о деталях роутинга.
//
// uiHandler — обработчик корня "/" (index.html, embedнутый в бинарь).
// Передаётся снаружи, чтобы api-пакет не зависел от //go:embed в main.
func Register(mux *http.ServeMux, engine *core.Engine, uiHandler http.HandlerFunc) {
	s := NewServer(engine)

	mux.HandleFunc("/", uiHandler)
	mux.HandleFunc("/api/nodes", s.HandleNodes)
	mux.HandleFunc("/api/vms", s.HandleVMs)
	mux.HandleFunc("/api/vms/", s.HandleDeleteVM)
	mux.HandleFunc("/api/containers", s.HandleGetContainers)
	mux.HandleFunc("/api/provision", s.HandleProvision)
	mux.HandleFunc("/api/deprovision", s.HandleDeprovision)
	mux.HandleFunc("/api/deprovision/", s.HandleDeprovisionByID)
	mux.HandleFunc("/api/state", s.HandleGetState)
	mux.HandleFunc("/api/replay", s.HandleReplay)
}
