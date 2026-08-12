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
	"strings"
	"testing"
)

// TestGenerateRoute_HTTPS_IncludesTLS проверяет, что GenerateRoute для
// HTTPS-протокола добавляет transport.tls.insecure_skip_verify, а для HTTP —
// не добавляет (раньше это была map[string]interface{} каша, теперь типизировано).
func TestGenerateRoute_HTTPS_IncludesTLS(t *testing.T) {
	r := GenerateRoute("athens.vm0.node0.internal", "10.0.0.5", "8080", "https", "pod-vm0-athens-node0")
	if r.Handle[0].Transport.TLS == nil {
		t.Fatal("HTTPS route should have TLS block")
	}
	if !r.Handle[0].Transport.TLS.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true for HTTPS upstream")
	}
}

func TestGenerateRoute_HTTP_NoTLS(t *testing.T) {
	r := GenerateRoute("athens.vm0.node0.internal", "10.0.0.5", "8080", "http", "pod-vm0-athens-node0")
	if r.Handle[0].Transport.TLS != nil {
		t.Error("HTTP route should NOT have TLS block")
	}
}

// TestGenerateRoute_JSONRoundTrip — marshal/unmarshal должен давать
// идентичную структуру. Заодно ловит опечатки в json-тегах.
func TestGenerateRoute_JSONRoundTrip(t *testing.T) {
	original := GenerateRoute("athens.vm0.node0.internal", "10.0.0.5", "8080", "https", "pod-vm0-athens-node0")
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded CaddyRoute
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: %q vs %q", decoded.ID, original.ID)
	}
	if decoded.Handle[0].Upstreams[0].Dial != "10.0.0.5:8080" {
		t.Errorf("Dial mismatch: %+v", decoded.Handle[0].Upstreams[0])
	}
}

// TestGenerateRoute_JSONShape — выкуривает регрессии в формате JSON:
// "@id", "match[0].host[0]", "handle[0].handler" должны быть на месте.
// Caddy валидирует это на своей стороне, и опечатка в теге = молчаливый
// 500, который у нас сейчас ловится только в интеграции.
func TestGenerateRoute_JSONShape(t *testing.T) {
	r := GenerateRoute("foo.internal", "1.2.3.4", "80", "http", "rid")
	data, _ := json.Marshal(r)
	s := string(data)
	required := []string{
		`"@id":"rid"`,
		`"match":[{"host":["foo.internal"]}]`,
		`"handler":"reverse_proxy"`,
		`"dial":"1.2.3.4:80"`,
		`"protocol":"http"`,
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q\nfull: %s", want, s)
		}
	}
}

func TestGenerateTLSPolicy_JSONShape(t *testing.T) {
	p := GenerateTLSPolicy(TLSConfig{
		ACMECA:     "https://ca.example/directory",
		RootCAPath: "/etc/caddy/root_ca.crt",
	}, "athens.vm0.node0.internal", "podtls-vm0-athens-node0")
	data, _ := json.Marshal(p)
	s := string(data)
	required := []string{
		`"@id":"podtls-vm0-athens-node0"`,
		`"subjects":["athens.vm0.node0.internal"]`,
		`"module":"acme"`,
		`"ca":"https://ca.example/directory"`,
		`"trusted_roots_pem_files":["/etc/caddy/root_ca.crt"]`,
		`"disabled":true`,
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Errorf("TLS JSON missing %q\nfull: %s", want, s)
		}
	}
}
