package singbox_test

import (
	"encoding/json"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/backend/singbox"
	"github.com/nexus/cfwarp-cli/internal/state"
)

// baseAccount returns a minimal valid AccountState for testing.
func baseAccount() state.AccountState {
	return state.AccountState{
		AccountID:        "acct-123",
		Token:            "tok-abc",
		WARPPrivateKey:   "privKeyBase64==",
		WARPPeerPubKey:   "peerPubKeyBase64==",
		WARPReserved:     [3]int{0, 1, 2},
		WARPPeerEndpoint: "162.159.192.1:2408",
		WARPIPV4:         "172.16.0.2",
		WARPIPV6:         "fd01::2",
	}
}

func baseSettings() state.Settings {
	return state.DefaultSettings() // socks5, 0.0.0.0:1080, no auth, info log
}

// parseConfig is a helper to unmarshal rendered JSON into a generic map.
func parseConfig(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("rendered config is not valid JSON: %v\n%s", err, data)
	}
	return m
}

// --- no auth, default endpoint ---

func TestRender_NoAuth_DefaultEndpoint(t *testing.T) {
	data, err := singbox.Render(backend.RenderInput{
		Account:  baseAccount(),
		Settings: baseSettings(),
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	m := parseConfig(t, data)

	// log
	log := m["log"].(map[string]any)
	if log["level"] != "info" {
		t.Errorf("expected log.level=info, got %v", log["level"])
	}
	if log["timestamp"] != true {
		t.Errorf("expected log.timestamp=true")
	}

	// inbound: socks
	inbounds := m["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %d", len(inbounds))
	}
	ib := inbounds[0].(map[string]any)
	if ib["type"] != "socks" {
		t.Errorf("expected inbound type socks, got %v", ib["type"])
	}
	if ib["listen"] != "0.0.0.0" {
		t.Errorf("expected listen 0.0.0.0, got %v", ib["listen"])
	}
	if ib["listen_port"] != float64(1080) {
		t.Errorf("expected listen_port 1080, got %v", ib["listen_port"])
	}
	// no auth: empty users array
	users := ib["users"].([]any)
	if len(users) != 0 {
		t.Errorf("expected empty users, got %v", users)
	}

	// endpoint: wireguard
	endpoints := m["endpoints"].([]any)
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	ep := endpoints[0].(map[string]any)
	if ep["type"] != "wireguard" {
		t.Errorf("expected endpoint type wireguard, got %v", ep["type"])
	}
	if ep["system"] != false {
		t.Errorf("expected system=false (userspace), got %v", ep["system"])
	}
	if ep["private_key"] != "privKeyBase64==" {
		t.Errorf("expected private_key, got %v", ep["private_key"])
	}

	addrs := ep["address"].([]any)
	if len(addrs) < 2 {
		t.Errorf("expected IPv4 and IPv6 address, got %v", addrs)
	}
	if addrs[0] != "172.16.0.2/32" {
		t.Errorf("expected 172.16.0.2/32, got %v", addrs[0])
	}
	if addrs[1] != "fd01::2/128" {
		t.Errorf("expected fd01::2/128, got %v", addrs[1])
	}

	peers := ep["peers"].([]any)
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	peer := peers[0].(map[string]any)
	if peer["address"] != "162.159.192.1" {
		t.Errorf("expected peer address 162.159.192.1, got %v", peer["address"])
	}
	if peer["port"] != float64(2408) {
		t.Errorf("expected peer port 2408, got %v", peer["port"])
	}
	if peer["public_key"] != "peerPubKeyBase64==" {
		t.Errorf("expected peer public_key, got %v", peer["public_key"])
	}
	allowedIPs := peer["allowed_ips"].([]any)
	if len(allowedIPs) != 2 || allowedIPs[0] != "0.0.0.0/0" || allowedIPs[1] != "::/0" {
		t.Errorf("expected allowed_ips [0.0.0.0/0, ::/0], got %v", allowedIPs)
	}
	reserved := peer["reserved"].([]any)
	if len(reserved) != 3 || reserved[1] != float64(1) || reserved[2] != float64(2) {
		t.Errorf("expected reserved [0,1,2], got %v", reserved)
	}

	// route
	route := m["route"].(map[string]any)
	if route["final"] != "wg-ep" {
		t.Errorf("expected route.final=wg-ep, got %v", route["final"])
	}
}

// --- auth enabled ---

func TestRender_AuthEnabled(t *testing.T) {
	s := baseSettings()
	s.ProxyUsername = "alice"
	s.ProxyPassword = "secret"

	data, err := singbox.Render(backend.RenderInput{Account: baseAccount(), Settings: s})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	m := parseConfig(t, data)

	inbounds := m["inbounds"].([]any)
	ib := inbounds[0].(map[string]any)
	users := ib["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	u := users[0].(map[string]any)
	if u["username"] != "alice" || u["password"] != "secret" {
		t.Errorf("unexpected user: %v", u)
	}
}

// --- endpoint override ---

func TestRender_EndpointOverride(t *testing.T) {
	s := baseSettings()
	s.EndpointOverride = "162.159.193.5:4500"

	data, err := singbox.Render(backend.RenderInput{Account: baseAccount(), Settings: s})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	m := parseConfig(t, data)

	ep := m["endpoints"].([]any)[0].(map[string]any)
	peer := ep["peers"].([]any)[0].(map[string]any)
	if peer["address"] != "162.159.193.5" {
		t.Errorf("expected overridden address 162.159.193.5, got %v", peer["address"])
	}
	if peer["port"] != float64(4500) {
		t.Errorf("expected overridden port 4500, got %v", peer["port"])
	}
}

// --- HTTP proxy mode ---

func TestRender_HTTPMode(t *testing.T) {
	s := baseSettings()
	s.ProxyMode = "http"
	s.ListenPort = 8080

	data, err := singbox.Render(backend.RenderInput{Account: baseAccount(), Settings: s})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	m := parseConfig(t, data)

	ib := m["inbounds"].([]any)[0].(map[string]any)
	if ib["type"] != "http" {
		t.Errorf("expected inbound type http, got %v", ib["type"])
	}
	if ib["listen_port"] != float64(8080) {
		t.Errorf("expected listen_port 8080, got %v", ib["listen_port"])
	}
}

// --- fallback to default peer endpoint when account has none ---

func TestRender_DefaultPeerEndpointFallback(t *testing.T) {
	acc := baseAccount()
	acc.WARPPeerEndpoint = "" // not captured from API

	data, err := singbox.Render(backend.RenderInput{Account: acc, Settings: baseSettings()})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	m := parseConfig(t, data)
	ep := m["endpoints"].([]any)[0].(map[string]any)
	peer := ep["peers"].([]any)[0].(map[string]any)
	if peer["address"] != "engage.cloudflareclient.com" {
		t.Errorf("expected default peer address, got %v", peer["address"])
	}
}

// --- IPv4-only account (no IPv6) ---

func TestRender_IPv4Only(t *testing.T) {
	acc := baseAccount()
	acc.WARPIPV6 = ""

	data, err := singbox.Render(backend.RenderInput{Account: acc, Settings: baseSettings()})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	m := parseConfig(t, data)
	ep := m["endpoints"].([]any)[0].(map[string]any)
	addrs := ep["address"].([]any)
	if len(addrs) != 1 {
		t.Errorf("expected 1 address (IPv4 only), got %v", addrs)
	}
}

// --- address already has prefix (no double-slash) ---

func TestRender_AddressAlreadyHasPrefix(t *testing.T) {
	acc := baseAccount()
	acc.WARPIPV4 = "172.16.0.2/32"
	acc.WARPIPV6 = "fd01::2/128"

	data, err := singbox.Render(backend.RenderInput{Account: acc, Settings: baseSettings()})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	m := parseConfig(t, data)
	ep := m["endpoints"].([]any)[0].(map[string]any)
	addrs := ep["address"].([]any)
	if addrs[0] != "172.16.0.2/32" {
		t.Errorf("unexpected address[0]: %v", addrs[0])
	}
	if addrs[1] != "fd01::2/128" {
		t.Errorf("unexpected address[1]: %v", addrs[1])
	}
}

// --- invalid endpoint override is rejected ---

func TestRender_InvalidEndpointOverride(t *testing.T) {
	s := baseSettings()
	s.EndpointOverride = "not-valid"

	_, err := singbox.Render(backend.RenderInput{Account: baseAccount(), Settings: s})
	if err == nil {
		t.Fatal("expected error for invalid endpoint override")
	}
}

// --- missing IPv4 is rejected ---

func TestRender_MissingIPv4(t *testing.T) {
	acc := baseAccount()
	acc.WARPIPV4 = ""

	_, err := singbox.Render(backend.RenderInput{Account: acc, Settings: baseSettings()})
	if err == nil {
		t.Fatal("expected error when warp_ipv4 is empty")
	}
}
