package orchestrator_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
)

func testDirs(t *testing.T) state.Dirs {
	t.Helper()
	root := t.TempDir()
	return state.Dirs{Config: filepath.Join(root, "config"), Runtime: filepath.Join(root, "run")}
}

func TestBuildRuntimeState_Legacy(t *testing.T) {
	dirs := testDirs(t)
	sett := state.DefaultSettings()
	info := backend.RuntimeInfo{PID: 1234, ConfigPath: dirs.BackendConfigFile(), StartedAt: time.Now().UTC()}

	rt := orchestrator.BuildRuntimeState(sett, info, dirs)
	if rt.RuntimeFamily != state.RuntimeFamilyLegacy || rt.Transport != state.TransportWireGuard || rt.Mode != state.ModeSocks5 {
		t.Fatalf("unexpected runtime selection: %+v", rt)
	}
	if rt.Phase != state.RuntimePhaseConnected {
		t.Fatalf("expected connected phase, got %q", rt.Phase)
	}
}

func TestBuildRuntimeState_Native(t *testing.T) {
	dirs := testDirs(t)
	sett := state.DefaultSettings()
	sett.RuntimeFamily = state.RuntimeFamilyNative
	sett.Transport = state.TransportMasque
	sett.Mode = state.ModeHTTP
	sett.Normalize()
	info := backend.RuntimeInfo{ConfigPath: dirs.BackendConfigFile(), StartedAt: time.Now().UTC()}

	rt := orchestrator.BuildRuntimeState(sett, info, dirs)
	if rt.RuntimeFamily != state.RuntimeFamilyNative || rt.Transport != state.TransportMasque || rt.Mode != state.ModeHTTP {
		t.Fatalf("unexpected runtime selection: %+v", rt)
	}
	if rt.Phase != state.RuntimePhaseConnecting {
		t.Fatalf("expected connecting phase, got %q", rt.Phase)
	}
	if rt.ServiceSocketPath == "" {
		t.Fatal("expected native runtime to reserve a service socket path")
	}
}

func TestIsRuntimeActive_External(t *testing.T) {
	rt, err := supervisor.Start(t.Context(), supervisor.StartConfig{
		Command: []string{"sh", "-c", "trap 'exit 0' TERM; sleep 5 & wait"},
		LogDir:  filepath.Join(t.TempDir(), "logs"),
		Backend: state.BackendSingboxWireGuard,
	})
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(rt) })

	persisted := state.RuntimeState{PID: rt.PID, Backend: rt.Backend, RuntimeFamily: state.RuntimeFamilyLegacy}
	if !orchestrator.IsRuntimeActive(persisted) {
		t.Fatal("expected external runtime to be active")
	}
}

func TestIsRuntimeActive_Native(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "service.sock")
	if err := os.WriteFile(socketPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write socket placeholder: %v", err)
	}
	rt := state.RuntimeState{RuntimeFamily: state.RuntimeFamilyNative, ServiceSocketPath: socketPath}
	if !orchestrator.IsRuntimeActive(rt) {
		t.Fatal("expected native runtime with socket path to be active")
	}

	stale := state.RuntimeState{RuntimeFamily: state.RuntimeFamilyNative, ServiceSocketPath: filepath.Join(t.TempDir(), "missing.sock")}
	if orchestrator.IsRuntimeActive(stale) {
		t.Fatal("expected native runtime without socket path target to be stale")
	}
}

func TestDerivePhase(t *testing.T) {
	rt := state.RuntimeState{}
	if got := orchestrator.DerivePhase(rt, true, true); got != state.RuntimePhaseConnected {
		t.Fatalf("expected connected, got %q", got)
	}
	if got := orchestrator.DerivePhase(rt, true, false); got != state.RuntimePhaseDegraded {
		t.Fatalf("expected degraded, got %q", got)
	}
	rt.LastError = "boom"
	if got := orchestrator.DerivePhase(rt, false, false); got != state.RuntimePhaseStopped {
		t.Fatalf("expected stopped, got %q", got)
	}
}

func TestMarkTransportError(t *testing.T) {
	var rt state.RuntimeState
	now := time.Now().UTC()
	orchestrator.MarkTransportError(&rt, "reconnect needed", now)
	if rt.Phase != state.RuntimePhaseDegraded {
		t.Fatalf("expected degraded phase, got %q", rt.Phase)
	}
	if rt.LastTransportError != "reconnect needed" || rt.LastReconnectReason != "reconnect needed" {
		t.Fatalf("unexpected transport error metadata: %+v", rt)
	}
	if rt.LastReconnectAt.IsZero() {
		t.Fatal("expected reconnect timestamp to be set")
	}
}
