package singbox

import (
	"testing"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func TestBuildDNSConfig_DefaultIsDisabled(t *testing.T) {
	cfg, resolver := buildDNSConfig(nil)
	if cfg != nil {
		t.Fatalf("expected nil dns config by default, got %+v", cfg)
	}
	if resolver != "" {
		t.Fatalf("expected empty resolver by default, got %q", resolver)
	}
}

func TestBuildDNSConfig_LocalResolver(t *testing.T) {
	cfg, resolver := buildDNSConfig(&state.DNSOptions{Mode: "local", Strategy: "ipv4_only"})
	if resolver != localDNSTag {
		t.Fatalf("expected resolver tag %q, got %q", localDNSTag, resolver)
	}
	if cfg == nil || len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 dns server, got %+v", cfg)
	}
	server := cfg.Servers[0]
	if server.Type != "local" || server.Tag != localDNSTag {
		t.Fatalf("unexpected local dns server: %+v", server)
	}
	if cfg.Strategy != "ipv4_only" {
		t.Fatalf("expected ipv4_only strategy, got %q", cfg.Strategy)
	}
}

func TestBuildDNSConfig_HTTPSResolver(t *testing.T) {
	cfg, resolver := buildDNSConfig(&state.DNSOptions{
		Mode:       "https",
		Server:     "1.1.1.1",
		ServerPort: 443,
		Path:       "/dns-query",
		Strategy:   "ipv4_only",
	})
	if resolver != customDNSTag {
		t.Fatalf("expected resolver tag %q, got %q", customDNSTag, resolver)
	}
	if cfg == nil || len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 dns server, got %+v", cfg)
	}
	server := cfg.Servers[0]
	if server.Type != "https" || server.Tag != customDNSTag || server.Server != "1.1.1.1" || server.ServerPort != 443 || server.Path != "/dns-query" {
		t.Fatalf("unexpected https dns server: %+v", server)
	}
	if cfg.Strategy != "ipv4_only" {
		t.Fatalf("expected ipv4_only strategy, got %q", cfg.Strategy)
	}
}
