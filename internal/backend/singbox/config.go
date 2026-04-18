// Package singbox implements the sing-box WireGuard backend for cfwarp-cli.
// Config targets sing-box >= 1.13 which uses the endpoints section for WireGuard
// (the legacy wireguard outbound type was removed in 1.13).
package singbox

// singboxConfig is the top-level sing-box configuration structure (v1.13+).
type singboxConfig struct {
	Log       logConfig    `json:"log"`
	DNS       *dnsConfig   `json:"dns,omitempty"`
	Inbounds  []inbound    `json:"inbounds"`
	Endpoints []wgEndpoint `json:"endpoints"`
	Route     routeConfig  `json:"route"`
}

// logConfig configures sing-box logging.
type logConfig struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp"`
}

// inbound represents a proxy listener (socks or http).
type inbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listen_port"`
	Users      []user `json:"users"` // empty slice = no auth
}

// user is a proxy auth credential pair.
type user struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// wgEndpoint is the sing-box 1.13 WireGuard endpoint (replaces outbound).
type wgEndpoint struct {
	Type           string   `json:"type"`
	Tag            string   `json:"tag"`
	System         bool     `json:"system"` // false = userspace (no NET_ADMIN)
	MTU            int      `json:"mtu"`
	Address        []string `json:"address"` // local WARP-assigned addresses with prefix length
	PrivateKey     string   `json:"private_key"`
	DomainResolver string   `json:"domain_resolver,omitempty"`
	Peers          []wgPeer `json:"peers"`
}

// wgPeer configures the Cloudflare WARP peer.
type wgPeer struct {
	Address                     string   `json:"address"`
	Port                        int      `json:"port"`
	PublicKey                   string   `json:"public_key"`
	AllowedIPs                  []string `json:"allowed_ips"`
	PersistentKeepaliveInterval int      `json:"persistent_keepalive_interval"`
	Reserved                    [3]int   `json:"reserved"`
}

type dnsConfig struct {
	Servers  []dnsServer `json:"servers,omitempty"`
	Strategy string      `json:"strategy,omitempty"`
}

type dnsServer struct {
	Type       string `json:"type"`
	Tag        string `json:"tag,omitempty"`
	PreferGo   bool   `json:"prefer_go,omitempty"`
	Server     string `json:"server,omitempty"`
	ServerPort int    `json:"server_port,omitempty"`
	Path       string `json:"path,omitempty"`
}

type routeRule struct {
	Action   string `json:"action"`
	Server   string `json:"server,omitempty"`
	Strategy string `json:"strategy,omitempty"`
}

// routeConfig tells sing-box which endpoint handles all traffic.
type routeConfig struct {
	Rules                 []routeRule `json:"rules,omitempty"`
	Final                 string      `json:"final"`
	DefaultDomainResolver string      `json:"default_domain_resolver,omitempty"`
}
