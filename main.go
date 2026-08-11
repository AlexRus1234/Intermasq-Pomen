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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"pomen/api"
	"pomen/core"
	"pomen/internal/version"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	BaseDomain string                     `json:"base_domain"`
	Nodes      map[string]core.NodeConfig `json:"nodes"`
}

func loadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(file, &cfg)
	return &cfg, err
}

func main() {
	showVersion := flag.Bool("version", false, "print build version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("pomen", version.Version)
		return
	}

	cfg, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Ошибка чтения config.json: %v", err)
	}

	caddyURLs := make(map[string]string)
	for name, data := range cfg.Nodes {
		caddyURLs[name] = data.CaddyURL
	}
	caddyClient := core.NewCaddyClient(caddyURLs)

	statePath := os.Getenv("STATE_FILE")
	if statePath == "" {
		statePath = "/etc/intermasq/plugins/pomen/routes.json"
	}
	stateStore := core.NewStateStore(statePath)

	vmsPath := os.Getenv("VMS_FILE")
	if vmsPath == "" {
		vmsPath = "/etc/intermasq/plugins/pomen/vms.json"
	}
	vmStore := core.NewVMStore(vmsPath)

	webhookClient := core.NewWebhookClient()

	engine := core.NewEngine(caddyClient, stateStore, vmStore, webhookClient, cfg.BaseDomain, cfg.Nodes)

	mux := http.NewServeMux()
	api.Register(mux, engine, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	socketPath := os.Getenv("PLUGIN_SOCKET")
	devPort := os.Getenv("POMEN_DEV_PORT")

	srv := &http.Server{Handler: mux}
	errCh := make(chan error, 1)

	switch {
	case socketPath != "":
		// Контракт Intermasq: unix-сокет с правами 0770.
		// listenUnix (платформо-зависимый, см. socket_{unix,windows}.go)
		// создаёт сокет с правильными правами atomically — без race window
		// между net.Listen и os.Chmod (audit §14.11).
		listener, err := listenUnix(socketPath)
		if err != nil {
			log.Fatalf("Ошибка создания сокета: %v", err)
		}
		fmt.Printf("Pomen %s started on unix socket: %s\n", version.Version, socketPath)
		go func() { errCh <- srv.Serve(listener) }()

	case devPort != "":
		// Dev-режим для локальной разработки и smoke/E2E тестов в CI.
		// НЕ для production: unix-сокета нет, secrets через env.
		addr := strings.TrimPrefix(devPort, ":")
		log.Printf("WARNING: dev mode on TCP :%s — not for production", addr)
		fmt.Printf("Pomen %s started on TCP :%s (DEV)\n", version.Version, addr)
		srv.Addr = ":" + addr
		go func() { errCh <- srv.ListenAndServe() }()

	default:
		log.Fatal("PLUGIN_SOCKET (production) or POMEN_DEV_PORT (dev) must be set")
	}

	// Graceful shutdown: SIGINT/SIGTERM → Shutdown(ctx), exit 0.
	// Раньше был os.Exit(1) на SIGTERM — systemd мог трактовать это как
	// падение и писать ошибку в журнал (audit §14.6).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-stop:
		log.Printf("received %s, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		<-errCh // ждём выхода srv.Serve
	}

	if socketPath != "" {
		os.Remove(socketPath)
	}
}
