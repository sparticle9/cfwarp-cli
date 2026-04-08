package native_test

import (
	"encoding/json"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/backend"
	_ "github.com/nexus/cfwarp-cli/internal/backend/native"
	"github.com/nexus/cfwarp-cli/internal/state"
)

func TestNativeMasqueBackendRegistered(t *testing.T) {
	b, err := backend.Lookup(state.BackendNativeMasque)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", state.BackendNativeMasque, err)
	}
	if b.Name() != state.BackendNativeMasque {
		t.Fatalf("unexpected backend name %q", b.Name())
	}
}

func TestNativeMasqueBackendRenderConfig(t *testing.T) {
	b, err := backend.Lookup(state.BackendNativeMasque)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", state.BackendNativeMasque, err)
	}
	sett := state.DefaultSettings()
	sett.RuntimeFamily = state.RuntimeFamilyNative
	sett.Transport = state.TransportMasque
	sett.Mode = state.ModeHTTP
	sett.Normalize()
	res, err := b.RenderConfig(backend.RenderInput{Account: state.AccountState{}, Settings: sett})
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(res.ConfigJSON, &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if payload["runtime_family"] != state.RuntimeFamilyNative || payload["transport"] != state.TransportMasque {
		t.Fatalf("unexpected rendered payload: %v", payload)
	}
	if payload["masque_state"] != "missing" {
		t.Fatalf("expected missing masque_state marker, got %v", payload["masque_state"])
	}
}
