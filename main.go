package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"pomen/api"
	"pomen/core"
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

		fmt.Printf("Pomen started on unix socket: %s\n", socketPath)
		os.Chmod(socketPath, 0770)
		http.Serve(listener, mux)
	} else {
		fmt.Printf("Pomen started on TCP :5001\n")
		log.Fatal(http.ListenAndServe(":5001", mux))
	}
}
