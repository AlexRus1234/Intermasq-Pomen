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
	"strings"
	"testing"
)

func TestVMStore_UpsertAndGet(t *testing.T) {
	s := NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	vm := VMConfig{Name: "vm0", Node: "node0", IP: "10.0.0.5", WebhookURL: "http://vm0:9000", Secret: "topsecret"}
	if err := s.Upsert(vm); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := s.Get("VM0") // case-insensitive lookup
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "vm0" {
		t.Fatalf("Get returned wrong VM: %+v", got)
	}
}

func TestVMStore_UpsertReplacesCaseInsensitive(t *testing.T) {
	s := NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	if err := s.Upsert(VMConfig{Name: "vm0", IP: "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(VMConfig{Name: "VM0", IP: "10.0.0.2"}); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("Upsert by case-variant name created duplicate: %+v", list)
	}
	if list[0].IP != "10.0.0.2" {
		t.Fatalf("Upsert did not replace: %+v", list[0])
	}
}

func TestVMStore_UpsertRejectsEmptyName(t *testing.T) {
	s := NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	err := s.Upsert(VMConfig{Name: "   "})
	if err == nil {
		t.Fatalf("Upsert with whitespace-only name should fail")
	}
	if !strings.Contains(err.Error(), "имя") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVMStore_GetMissing(t *testing.T) {
	s := NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	if _, err := s.Get("nope"); err == nil {
		t.Fatalf("Get on missing VM should fail")
	}
}

func TestVMStore_Delete(t *testing.T) {
	s := NewVMStore(filepath.Join(t.TempDir(), "vms.json"))
	if err := s.Upsert(VMConfig{Name: "vm0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Upsert(VMConfig{Name: "vm1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("VM0"); err != nil { // case-insensitive
		t.Fatal(err)
	}
	list, _ := s.List()
	if len(list) != 1 || list[0].Name != "vm1" {
		t.Fatalf("Delete left wrong state: %+v", list)
	}
	if err := s.Delete("missing"); err == nil {
		t.Fatalf("Delete on missing should fail")
	}
}
