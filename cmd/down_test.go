package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func execDown(t *testing.T, dirs state.Dirs) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"down", "--state-dir", dirs.Config})
	err := rootCmd.Execute()
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return buf.String(), err
}

func tempDownDirs(t *testing.T) state.Dirs {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "state")
	return state.Dirs{Config: cfg, Runtime: filepath.Join(cfg, "run")}
}

func TestDown_CleansUpStaleLegacyRuntime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("down command enforces linux-only platform checks")
	}
	d := tempDownDirs(t)
	rt := state.RuntimeState{
		PID:           99999999,
		Backend:       state.BackendSingboxWireGuard,
		RuntimeFamily: state.RuntimeFamilyLegacy,
		Transport:     state.TransportWireGuard,
		Mode:          state.ModeSocks5,
		StartedAt:     time.Now().UTC(),
	}
	if err := state.SaveRuntime(d, rt); err != nil {
		t.Fatalf("save runtime: %v", err)
	}

	out, err := execDown(t, d)
	if err != nil {
		t.Fatalf("down: %v\noutput: %s", err, out)
	}
	if _, err := state.LoadRuntime(d); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("expected runtime to be cleaned up, got %v", err)
	}
}

func TestDown_CleansUpStaleNativeRuntime(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("down command enforces linux-only platform checks")
	}
	d := tempDownDirs(t)
	rt := state.RuntimeState{
		Backend:           state.BackendNativeMasque,
		RuntimeFamily:     state.RuntimeFamilyNative,
		Transport:         state.TransportMasque,
		Mode:              state.ModeHTTP,
		Phase:             state.RuntimePhaseConnecting,
		ServiceSocketPath: filepath.Join(d.Runtime, "missing.sock"),
		StartedAt:         time.Now().UTC(),
	}
	if err := state.SaveRuntime(d, rt); err != nil {
		t.Fatalf("save runtime: %v", err)
	}

	out, err := execDown(t, d)
	if err != nil {
		t.Fatalf("down: %v\noutput: %s", err, out)
	}
	if _, err := state.LoadRuntime(d); !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("expected runtime to be cleaned up, got %v", err)
	}
}
