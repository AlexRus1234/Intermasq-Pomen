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

//go:build windows

package main

import (
	"net"
	"os"
)

// listenUnix on Windows: umask is not available (POSIX-only). On Windows the
// unix-socket path is never used in production — Pomen runs behind PLUGIN_SOCKET
// on Linux hosts only. This stub exists so `go build ./...` succeeds on Windows
// dev machines. Race window between Listen and Chmod is irrelevant here.
//
// Production behaviour is unchanged: CI and the Intermasq host run on Linux,
// where the !windows build (socket_unix.go) is selected.
func listenUnix(path string) (net.Listener, error) {
	os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0770)
	return listener, nil
}
