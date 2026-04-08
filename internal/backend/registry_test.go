package backend_test

import (
	"strings"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/backend"
	_ "github.com/nexus/cfwarp-cli/internal/backend/singbox"
	"github.com/nexus/cfwarp-cli/internal/state"
)

func TestLookup_SingboxWireGuard(t *testing.T) {
	b, err := backend.Lookup(state.BackendSingboxWireGuard)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", state.BackendSingboxWireGuard, err)
	}
	if b.Name() != state.BackendSingboxWireGuard {
		t.Fatalf("expected backend name %q, got %q", state.BackendSingboxWireGuard, b.Name())
	}
}

func TestLookup_UnsupportedBackend(t *testing.T) {
	_, err := backend.Lookup("does-not-exist")
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
	if !strings.Contains(err.Error(), "unsupported backend") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNames_ContainsSingboxWireGuard(t *testing.T) {
	names := backend.Names()
	found := false
	for _, name := range names {
		if name == state.BackendSingboxWireGuard {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected registered names to include %q, got %v", state.BackendSingboxWireGuard, names)
	}
}
