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
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
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

// StateStore — потокобезопасное хранилище выданных маршрутов (routes.json).
// Ранее был без мьютекса (в отличие от VMStore), что приводило к data race
// при параллельных Provision/Replay — см. audit §14.1.
type StateStore struct {
	path string
	mu   sync.Mutex
}

func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

// load читает routes.json без блокировки — вызывающий код должен держать mu.
func (s *StateStore) load() ([]RouteRecord, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RouteRecord{}, nil
		}
		return nil, err
	}
	var records []RouteRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	if records == nil {
		records = []RouteRecord{}
	}
	return records, nil
}

// save записывает routes.json атомарно (tmp + rename) без блокировки —
// вызывающий код должен держать mu.
func (s *StateStore) save(records []RouteRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0660); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Load возвращает все записи. Потокобезопасен.
func (s *StateStore) Load() ([]RouteRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *StateStore) Upsert(rec RouteRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.load()
	if err != nil {
		return err
	}
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	found := false
	for i, r := range records {
		if r.RouteID == rec.RouteID {
			rec.UpdatedAt = time.Now().Format(time.RFC3339)
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
