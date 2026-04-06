package state

import "time"

// AccountState holds the WARP device registration data persisted to account.json.
type AccountState struct {
	AccountID       string    `json:"account_id"`
	Token           string    `json:"token"`
	License         string    `json:"license,omitempty"`
	ClientID        string    `json:"client_id"`
	WARPPrivateKey  string    `json:"warp_private_key"`
	WARPPeerPubKey  string    `json:"warp_peer_public_key"`
	WARPIPV4        string    `json:"warp_ipv4"`
	WARPIPV6        string    `json:"warp_ipv6"`
	CreatedAt       time.Time `json:"created_at"`
	Source          string    `json:"source"` // "register" | "import"
}

// Settings holds operator-supplied configuration persisted to settings.json.
type Settings struct {
	Backend          string `json:"backend"`           // e.g. "singbox-wireguard"
	ListenHost       string `json:"listen_host"`       // e.g. "0.0.0.0"
	ListenPort       int    `json:"listen_port"`       // e.g. 1080
	ProxyMode        string `json:"proxy_mode"`        // "socks5" | "http"
	ProxyUsername    string `json:"proxy_username,omitempty"`
	ProxyPassword    string `json:"proxy_password,omitempty"`
	EndpointOverride string `json:"endpoint_override,omitempty"` // "host:port"
	StateDir         string `json:"state_dir,omitempty"`
	LogLevel         string `json:"log_level"` // "debug" | "info" | "warn" | "error"
}

// DefaultSettings returns a Settings with sane defaults.
func DefaultSettings() Settings {
	return Settings{
		Backend:    "singbox-wireguard",
		ListenHost: "0.0.0.0",
		ListenPort: 1080,
		ProxyMode:  "socks5",
		LogLevel:   "info",
	}
}

// RuntimeState holds ephemeral process metadata persisted to runtime.json.
type RuntimeState struct {
	PID              int       `json:"pid"`
	Backend          string    `json:"backend"`
	ConfigPath       string    `json:"config_path"`
	StdoutLogPath    string    `json:"stdout_log_path"`
	StderrLogPath    string    `json:"stderr_log_path"`
	StartedAt        time.Time `json:"started_at"`
	LastError        string    `json:"last_error,omitempty"`
	LocalReachable   bool      `json:"local_reachable"`
	LastTraceSummary string    `json:"last_trace_summary,omitempty"`
}

// EndpointCandidate describes a WireGuard peer endpoint under consideration.
type EndpointCandidate struct {
	Value        string     `json:"value"`                    // "host:port"
	Source       string     `json:"source"`                   // "manual" | "default" | "env"
	Valid        bool       `json:"valid"`
	LastTestedAt *time.Time `json:"last_tested_at,omitempty"`
	LastResult   string     `json:"last_result,omitempty"` // "success" | "failure"
	Notes        string     `json:"notes,omitempty"`
}
