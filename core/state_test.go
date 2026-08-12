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
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestStateStore_LoadMissingFile(t *testing.T) {
	s := NewStateStore(filepath.Join(t.TempDir(), "missing.json"))
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load on missing file: unexpected err %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Load on missing file: want empty slice, got %v", got)
	}
}

func TestStateStore_UpsertAndLoad(t *testing.T) {
	s := NewStateStore(filepath.Join(t.TempDir(), "routes.json"))
	rec := RouteRecord{
		Domain:     "athens.vm0.node0.internal",
		TargetIP:   "10.0.0.5",
		TargetPort: "8080",
		Protocol:   "http",
		RouteID:    "pod-vm0-athens-node0",
		TLSID:      "podtls-vm0-athens-node0",
		Node:       "node0",
		VMName:     "vm0",
	}
	if err := s.Upsert(rec); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].RouteID != rec.RouteID {
		t.Fatalf("Load = %+v, want [%+v]", got, rec)
	}
	if got[0].UpdatedAt == "" {
		t.Fatalf("UpdatedAt not auto-filled on insert")
	}
}

func TestStateStore_UpsertReplacesByRouteID(t *testing.T) {
	s := NewStateStore(filepath.Join(t.TempDir(), "routes.json"))
	rec := RouteRecord{RouteID: "r1", Domain: "old.internal", TargetPort: "80"}
	other := RouteRecord{RouteID: "r2", Domain: "other.internal", TargetPort: "81"}
	if err := s.Upsert(rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(other); err != nil {
		t.Fatal(err)
	}
	rec.TargetPort = "8080"
	if err := s.Upsert(rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 records after upsert, got %d (%+v)", len(got), got)
	}
	var r1 *RouteRecord
	for i := range got {
		if got[i].RouteID == "r1" {
			r1 = &got[i]
		}
	}
	if r1 == nil || r1.TargetPort != "8080" {
		t.Fatalf("upsert did not replace r1 in place: %+v", got)
	}
}

func TestStateStore_Remove(t *testing.T) {
	s := NewStateStore(filepath.Join(t.TempDir(), "routes.json"))
	if err := s.Upsert(RouteRecord{RouteID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(RouteRecord{RouteID: "r2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("r1"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RouteID != "r2" {
		t.Fatalf("Remove left unexpected state: %+v", got)
	}
}

// TestStateStore_ConcurrentUpsert — узкий тест на data race, который мы
// чинили в этапе 2 (раньше StateStore был без мьютекса). Запускается с
// `go test -race`. 100 горутин пишут разные RouteID; итоговый файл должен
// содержать ровно 100 записей.
func TestStateStore_ConcurrentUpsert(t *testing.T) {
	s := NewStateStore(filepath.Join(t.TempDir(), "routes.json"))
	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if err := s.Upsert(RouteRecord{
				RouteID:    "r" + strconv.Itoa(i),
				Domain:     "x",
				TargetPort: "80",
			}); err != nil {
				t.Errorf("goroutine %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != n {
		t.Fatalf("race: want %d records after concurrent upsert, got %d", n, len(got))
	}
}
