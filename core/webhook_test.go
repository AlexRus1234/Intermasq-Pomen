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
	"strconv"
	"testing"
)

// TestNormalizeContainer покрывает правила нормализации (audit §13.13):
// срез префикса systemd-, срез нумерационного префикса NN-, override labels,
// дефолты Protocol="http", выбор первого host_port.
func TestNormalizeContainer(t *testing.T) {
	vm := VMConfig{Name: "vm0", Node: "node0", IP: "10.0.0.5"}

	cases := []struct {
		name string
		raw  rawPodmanContainer
		want ContainerInfo
	}{
		{
			name: "systemd-prefix stripped",
			raw:  rawPodmanContainer{Names: []string{"systemd-athens"}, State: "running"},
			want: ContainerInfo{
				Name: "athens", RealName: "systemd-athens", Status: "",
				Running: true, Protocol: "http", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0",
			},
		},
		{
			name: "numeric prefix stripped",
			raw:  rawPodmanContainer{Names: []string{"01-athens"}, State: "running"},
			want: ContainerInfo{Name: "athens", RealName: "01-athens", Running: true, Protocol: "http", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0"},
		},
		{
			name: "name without prefix unchanged",
			raw:  rawPodmanContainer{Names: []string{"plain"}, State: "exited"},
			want: ContainerInfo{Name: "plain", RealName: "plain", Running: false, Protocol: "http", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0"},
		},
		{
			name: "running detection via Status prefix",
			raw:  rawPodmanContainer{Names: []string{"x"}, Status: "Up 3 hours"},
			want: ContainerInfo{Name: "x", RealName: "x", Status: "Up 3 hours", Running: true, Protocol: "http", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0"},
		},
		{
			name: "first host_port picked",
			raw: rawPodmanContainer{
				Names: []string{"srv"},
				Ports: []rawPort{{HostPort: 8081, ContainerPort: 8080, Protocol: "tcp"}},
			},
			want: ContainerInfo{Name: "srv", RealName: "srv", Protocol: "tcp", Port: "8081", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0"},
		},
		{
			name: "labels override port/proto/name",
			raw: rawPodmanContainer{
				Names:  []string{"systemd-svc"},
				State:  "running",
				Ports:  []rawPort{{HostPort: 9999}},
				Labels: map[string]string{"port-8080": "", "proto-https": "", "name-x": "pretty"},
			},
			want: ContainerInfo{
				Name: "pretty", RealName: "systemd-svc", Running: true,
				Port: "8080", Protocol: "https", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0",
			},
		},
		{
			name: "label port-NN with value overrides prefix",
			raw: rawPodmanContainer{
				Names:  []string{"x"},
				Labels: map[string]string{"port-XXX": "1234"},
			},
			want: ContainerInfo{Name: "x", RealName: "x", Protocol: "http", Port: "1234", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0"},
		},
		{
			name: "empty Names does not panic",
			raw:  rawPodmanContainer{State: "running"},
			want: ContainerInfo{Name: "", RealName: "", Running: true, Protocol: "http", VMName: "vm0", VMIP: "10.0.0.5", Node: "node0"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeContainer(tc.raw, vm)
			if got != tc.want {
				t.Errorf("normalizeContainer mismatch\n got:  %+v\n want: %+v", got, tc.want)
			}
		})
	}
}

// Бонусная проверка: первый host_port конвертируется в строку без паники
// на int→string через strconv (это отдельный путь кода, не покрытый
// табличными кейсами выше).
func TestNormalizeContainer_HostPortStrConv(t *testing.T) {
	raw := rawPodmanContainer{
		Names: []string{"x"},
		Ports: []rawPort{{HostPort: 65535}},
	}
	got := normalizeContainer(raw, VMConfig{Name: "v", Node: "n", IP: "1.2.3.4"})
	if got.Port != strconv.Itoa(65535) {
		t.Fatalf("Port = %q, want %q", got.Port, "65535")
	}
}
