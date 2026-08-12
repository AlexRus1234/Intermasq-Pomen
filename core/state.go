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
	"slices"
	"time"
)

type RouteRecord struct {
	Domain     string `json:"domain"`
	TargetIP   string `json:"target_ip"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
	RouteID    string `json:"route_id"`
	TLSID      string `json:"tls_id"`
	Node       string `json:"node"`
	VMName     string `json:"vm_name"`
	UpdatedAt  string `json:"updated_at"`
}

// StateStore — реестр выданных маршрутов (routes.json). Потокобезопасен
// через JSONStore. Раньше был без мьютекса — data race при параллельных
// Provision/Replay (audit §14.1).
type StateStore struct {
	*JSONStore[RouteRecord]
}

func NewStateStore(path string) *StateStore {
	return &StateStore{JSONStore: NewJSONStore[RouteRecord](path)}
}

func (s *StateStore) Upsert(rec RouteRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.load()
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = now
	}
	found := false
	for i, r := range records {
		if r.RouteID == rec.RouteID {
			rec.UpdatedAt = now
			records[i] = rec
			found = true
			break
		}
	}
	if !found {
		records = append(records, rec)
	}
	return s.save(records)
}

func (s *StateStore) Remove(routeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.load()
	if err != nil {
		return err
	}
	filtered := slices.DeleteFunc(records, func(r RouteRecord) bool {
		return r.RouteID == routeID
	})
	return s.save(filtered)
}
