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

//go:build !windows

package main

import (
	"log/slog"
	"net"
	"os"
	"syscall"
)

// listenUnix creates the plugin unix socket with mode 0770 atomically.
//
// On POSIX we set umask=0o007 BEFORE net.Listen so the socket file is
// created with mode 0770 in the same syscall that binds it — no race window
// between Listen and Chmod (audit §14.11). The previous umask is restored
// before returning so we don't affect the rest of the process.
//
// os.Chmod after Listen is a defensive fallback: some FS/kernel combos
// honour umask differently, and an explicit chmod makes the intent visible.
// A chmod failure is logged via slog but does not abort (socket is usable).
func listenUnix(path string) (net.Listener, error) {
	os.Remove(path)
	oldUmask := syscall.Umask(0o007)
	listener, err := net.Listen("unix", path)
	syscall.Umask(oldUmask)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0770); err != nil {
		// Non-fatal: socket already listens, but expected mode could not be set.
		slog.Warn("unix socket chmod failed", "path", path, "err", err)
	}
	return listener, nil
}
