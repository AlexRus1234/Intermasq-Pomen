package core

// NodeConfig описывает ноду из статического config.json.
// В Pomen нода хранит только URL Caddy (всё остальное относится к ВМ).
type NodeConfig struct {
	CaddyURL string `json:"caddy_url"`
}

// VMConfig описывает одну ВМ с Podman, зарегистрированную через UI.
// VM-ы короткоживущие и управляются динамически (vms.json).
type VMConfig struct {
	Name        string `json:"name"`
	Node        string `json:"node"`
	IP          string `json:"ip"`
	WebhookURL  string `json:"webhook_url"`
	Secret      string `json:"secret"`
}

// ContainerInfo — распарсенный контейнер Podman с нормализованными полями.
// Port/Protocol/Name берутся из labels port-/proto-/name- (как теги в PVE).
type ContainerInfo struct {
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Status   string `json:"status"`
	Running  bool   `json:"running"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	VMName   string `json:"vm_name"`
	VMIP     string `json:"vm_ip"`
	Node     string `json:"node"`
}
