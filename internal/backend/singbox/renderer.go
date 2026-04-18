package singbox

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/settings"
	"github.com/nexus/cfwarp-cli/internal/state"
)

const (
	defaultPeerEndpoint = "engage.cloudflareclient.com:2408"
	wgMTU               = 1280
	wgKeepalive         = 25
	wgTag               = "wg-ep"
	localDNSTag         = "local"
	customDNSTag        = "dns-custom"
)

// Render builds the sing-box JSON config from the provided RenderInput.
func Render(input backend.RenderInput) ([]byte, error) {
	input.Settings.Normalize()
	peerAddr, peerPort, err := resolvePeerEndpoint(input)
	if err != nil {
		return nil, err
	}

	localAddrs, err := buildLocalAddresses(input.Account.WARPIPV4, input.Account.WARPIPV6)
	if err != nil {
		return nil, err
	}

	inbounds, err := buildInbounds(input.Settings.Access)
	if err != nil {
		return nil, err
	}

	dnsCfg, resolverTag := buildDNSConfig(input.Settings.DNS)
	cfg := singboxConfig{
		Log:      buildLogConfig(input.Settings.LogLevel),
		DNS:      dnsCfg,
		Inbounds: inbounds,
		Endpoints: []wgEndpoint{
			{
				Type:           "wireguard",
				Tag:            wgTag,
				System:         false,
				MTU:            wgMTU,
				Address:        localAddrs,
				PrivateKey:     input.Account.WARPPrivateKey,
				DomainResolver: resolverTag,
				Peers: []wgPeer{
					{
						Address:                     peerAddr,
						Port:                        peerPort,
						PublicKey:                   input.Account.WARPPeerPubKey,
						AllowedIPs:                  []string{"0.0.0.0/0", "::/0"},
						PersistentKeepaliveInterval: wgKeepalive,
						Reserved:                    input.Account.WARPReserved,
					},
				},
			},
		},
		Route: routeConfig{
			Rules:                 buildRouteRules(resolverTag, input.Settings.DNS),
			Final:                 wgTag,
			DefaultDomainResolver: resolverTag,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return data, nil
}

func buildLogConfig(level string) logConfig {
	if level == "" {
		level = "info"
	}
	return logConfig{Level: level, Timestamp: true}
}

func buildDNSConfig(opts *state.DNSOptions) (*dnsConfig, string) {
	if opts == nil || opts.Mode == "" {
		return nil, ""
	}
	server := dnsServer{Type: opts.Mode}
	resolverTag := localDNSTag
	if opts.Mode != "local" {
		resolverTag = customDNSTag
		server.Server = opts.Server
		server.ServerPort = opts.ServerPort
		if opts.Mode == "https" {
			server.Path = opts.Path
		}
		return &dnsConfig{Servers: []dnsServer{{
			Type:       server.Type,
			Tag:        resolverTag,
			Server:     server.Server,
			ServerPort: server.ServerPort,
			Path:       server.Path,
		}}, Strategy: opts.Strategy}, resolverTag
	}
	return &dnsConfig{Servers: []dnsServer{{Type: server.Type, Tag: resolverTag}}, Strategy: opts.Strategy}, resolverTag
}

func buildRouteRules(resolverTag string, opts *state.DNSOptions) []routeRule {
	if resolverTag == "" {
		return nil
	}
	rule := routeRule{Action: "resolve", Server: resolverTag}
	if opts != nil {
		rule.Strategy = opts.Strategy
	}
	return []routeRule{rule}
}

// resolvePeerEndpoint returns the peer host and port to use, applying override if set.
func resolvePeerEndpoint(input backend.RenderInput) (string, int, error) {
	raw := input.Settings.EndpointOverride
	if raw == "" {
		raw = input.Account.WARPPeerEndpoint
	}
	if raw == "" {
		raw = defaultPeerEndpoint
	}
	if err := settings.ValidateEndpoint(raw); err != nil {
		return "", 0, fmt.Errorf("peer endpoint %q: %w", raw, err)
	}
	host, portStr, _ := net.SplitHostPort(raw)
	port, _ := strconv.Atoi(portStr)
	return host, port, nil
}

// buildLocalAddresses normalises the WARP-assigned IPv4/IPv6 into CIDR notation.
// The API may return bare IPs (no prefix) or IPs with prefix; we ensure /32 and /128.
func buildLocalAddresses(ipv4, ipv6 string) ([]string, error) {
	if ipv4 == "" {
		return nil, fmt.Errorf("warp_ipv4 is required for WireGuard config")
	}
	addrs := []string{ensurePrefix(ipv4, 32)}
	if ipv6 != "" {
		addrs = append(addrs, ensurePrefix(ipv6, 128))
	}
	return addrs, nil
}

// ensurePrefix appends /bits if the address doesn't already have a prefix.
func ensurePrefix(addr string, bits int) string {
	if strings.Contains(addr, "/") {
		return addr
	}
	return fmt.Sprintf("%s/%d", addr, bits)
}

// inboundTypeFor maps an access type to the sing-box inbound type string.
func inboundTypeFor(mode string) (string, error) {
	switch mode {
	case "socks5":
		return "socks", nil
	case "http":
		return "http", nil
	default:
		return "", fmt.Errorf("unsupported access type %q: must be socks5 or http", mode)
	}
}

func buildInbounds(accesses []state.AccessConfig) ([]inbound, error) {
	inbounds := make([]inbound, 0, len(accesses))
	for i, access := range accesses {
		inboundType, err := inboundTypeFor(access.Type)
		if err != nil {
			return nil, err
		}
		inbounds = append(inbounds, inbound{
			Type:       inboundType,
			Tag:        fmt.Sprintf("proxy-in-%d", i+1),
			Listen:     access.ListenHost,
			ListenPort: access.ListenPort,
			Users:      buildUsers(access.Username, access.Password),
		})
	}
	return inbounds, nil
}

// buildUsers returns the users slice for the inbound.
// An empty slice means no authentication required.
func buildUsers(username, password string) []user {
	if username == "" {
		return []user{}
	}
	return []user{{Username: username, Password: password}}
}
