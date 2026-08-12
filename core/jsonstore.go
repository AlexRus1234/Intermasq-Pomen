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
	"sync"
)

// JSONStore — потокобезопасное JSON-хранилище слайса записей любого типа.
// Устраняет дубликацию между StateStore (RouteRecord) и VMStore (VMConfig):
// до этого оба копировали один и тот же load/save/atomic-rename/mutex код
// (audit §4.3). Конкретные сторы встраивают *JSONStore[T] и добавляют
// только свою доменную логику (Upsert/Remove/Get/Delete).
//
// Файл хранится как JSON-массив. Запись идёт через tmp-файл + atomic rename,
// чтобы при сбое посреди записи не осталось половинчатого файла.
type JSONStore[T any] struct {
	path string
	mu   sync.Mutex
}

// NewJSONStore создаёт стор для файла по указанному пути.
func NewJSONStore[T any](path string) *JSONStore[T] {
	return &JSONStore[T]{path: path}
}

// Path возвращает путь к файлу (read-only, для логов/диагностики).
func (s *JSONStore[T]) Path() string { return s.path }

// Load читает файл и возвращает слайс записей. Не существует файл → пустой
// слайс без ошибки (это нормальное стартовое состояние). Потокобезопасен.
func (s *JSONStore[T]) Load() ([]T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// Save атомарно перезаписывает файл новым содержимым. Потокобезопасен.
// Вызывающий код должен сам построить полный слайс — partial-update
// поверх существующего файла делается через Load + модификация + Save.
func (s *JSONStore[T]) Save(records []T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save(records)
}

// load — внутренний read БЕЗ мьютекса. Используется из методов стора,
// которые уже держат лок через Update/Delete/etc.
func (s *JSONStore[T]) load() ([]T, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []T{}, nil
		}
		return nil, err
	}
	var records []T
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	if records == nil {
		records = []T{}
	}
	return records, nil
}

// save — внутренний write БЕЗ мьютекса. atomic: tmp + rename.
func (s *JSONStore[T]) save(records []T) error {
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
