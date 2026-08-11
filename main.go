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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"pomen/api"
	"pomen/core"
	"pomen/internal/version"
	"syscall"
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

	apiServer := api.NewApiServer(engine)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	mux.HandleFunc("/api/nodes", apiServer.HandleNodes)
	mux.HandleFunc("/api/vms", apiServer.HandleVMs)
	mux.HandleFunc("/api/vms/", apiServer.HandleDeleteVM)
	mux.HandleFunc("/api/containers", apiServer.HandleGetContainers)
	mux.HandleFunc("/api/provision", apiServer.HandleProvision)
	mux.HandleFunc("/api/deprovision", apiServer.HandleDeprovision)
	mux.HandleFunc("/api/deprovision/", apiServer.HandleDeprovisionByID)
	mux.HandleFunc("/api/state", apiServer.HandleGetState)
	mux.HandleFunc("/api/replay", apiServer.HandleReplay)

	socketPath := os.Getenv("PLUGIN_SOCKET")

	if socketPath != "" {
		os.Remove(socketPath)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			log.Fatalf("Ошибка создания сокета: %v", err)
		}

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Remove(socketPath)
			os.Exit(1)
		}()

		fmt.Printf("Pomen %s started on unix socket: %s\n", version.Version, socketPath)
		os.Chmod(socketPath, 0770)
		http.Serve(listener, mux)
	} else {
		fmt.Printf("Pomen %s started on TCP :5001\n", version.Version)
		log.Fatal(http.ListenAndServe(":5001", mux))
	}
}
