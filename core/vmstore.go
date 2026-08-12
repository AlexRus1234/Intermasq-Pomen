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
	"slices"
	"strings"
)

// VMStore — потокобезопасный реестр короткоживущих ВМ (vms.json).
// В отличие от статического config.json, этот файл правится через UI.
type VMStore struct {
	*JSONStore[VMConfig]
}

func NewVMStore(path string) *VMStore {
	return &VMStore{JSONStore: NewJSONStore[VMConfig](path)}
}

// List возвращает все зарегистрированные ВМ.
func (s *VMStore) List() ([]VMConfig, error) {
	return s.Load()
}

// Get возвращает ВМ по имени (регистронезависимо).
func (s *VMStore) Get(name string) (VMConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vms, err := s.load()
	if err != nil {
		return VMConfig{}, err
	}
	lname := strings.ToLower(name)
	for _, v := range vms {
		if strings.ToLower(v.Name) == lname {
			return v, nil
		}
	}
	return VMConfig{}, fmt.Errorf("%w: ВМ %s не найдена", ErrNotFound, name)
}

// Upsert добавляет или обновляет ВМ по имени.
// Возвращает ошибку, если name пустой.
func (s *VMStore) Upsert(vm VMConfig) error {
	if strings.TrimSpace(vm.Name) == "" {
		return fmt.Errorf("%w: имя ВМ обязательно", ErrBadRequest)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	vms, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for i, v := range vms {
		if strings.EqualFold(v.Name, vm.Name) {
			vms[i] = vm
			found = true
			break
		}
	}
	if !found {
		vms = append(vms, vm)
	}
	return s.save(vms)
}

// Delete удаляет ВМ из реестра по имени.
// Маршруты Caddy/state НЕ трогаются (вариант A: offline-записи чистятся вручную).
func (s *VMStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vms, err := s.load()
	if err != nil {
		return err
	}
	lname := strings.ToLower(name)
	before := len(vms)
	filtered := slices.DeleteFunc(vms, func(v VMConfig) bool {
		return strings.ToLower(v.Name) == lname
	})
	if len(filtered) == before {
		return fmt.Errorf("%w: ВМ %s не найдена", ErrNotFound, name)
	}
	return s.save(filtered)
}
