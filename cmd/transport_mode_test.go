package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/pflag"
)

func execCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
	transportSetRuntimeFamily = ""
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return buf.String(), err
}

func tempConfigDirs(t *testing.T) state.Dirs {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "state")
	return state.Dirs{Config: cfg, Runtime: filepath.Join(cfg, "run")}
}

func TestTransportSetAndShow(t *testing.T) {
	d := tempConfigDirs(t)
	out, err := execCmd(t, "transport", "set", "masque", "--runtime-family", "native", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("transport set: %v\n%s", err, out)
	}
	out, err = execCmd(t, "transport", "show", "--json", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("transport show: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"transport": "masque"`) || !strings.Contains(out, `"runtime_family": "native"`) {
		t.Fatalf("unexpected transport show output: %s", out)
	}
}

func TestTransportSetWireGuardLegacyShow(t *testing.T) {
	d := tempConfigDirs(t)
	out, err := execCmd(t, "transport", "set", "wireguard", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("transport set legacy: %v\n%s", err, out)
	}
	out, err = execCmd(t, "transport", "show", "--json", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("transport show legacy: %v\n%s", err, out)
	}
	for _, fragment := range []string{`"backend": "singbox-wireguard"`, `"runtime_family": "legacy"`, `"transport": "wireguard"`} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected %q in output: %s", fragment, out)
		}
	}
}

func TestModeSetAndShow(t *testing.T) {
	d := tempConfigDirs(t)
	// native+masque is required before setting mode http in this path.
	_, err := execCmd(t, "transport", "set", "masque", "--runtime-family", "native", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("transport set: %v", err)
	}
	out, err := execCmd(t, "mode", "set", "http", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("mode set: %v\n%s", err, out)
	}
	out, err = execCmd(t, "mode", "show", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("mode show: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"mode": "http"`) {
		t.Fatalf("unexpected mode show output: %s", out)
	}
}

func TestStats_LegacyRuntimeSnapshot(t *testing.T) {
	d := tempConfigDirs(t)
	sett := state.DefaultSettings()
	if err := state.SaveSettings(d, sett); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	rt := state.RuntimeState{
		PID:           4242,
		Backend:       state.BackendSingboxWireGuard,
		RuntimeFamily: state.RuntimeFamilyLegacy,
		Transport:     state.TransportWireGuard,
		Mode:          state.ModeSocks5,
		Phase:         state.RuntimePhaseConnected,
	}
	if err := state.SaveRuntime(d, rt); err != nil {
		t.Fatalf("SaveRuntime: %v", err)
	}
	out, err := execCmd(t, "stats", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("stats: %v\n%s", err, out)
	}
	for _, fragment := range []string{`"backend": "singbox-wireguard"`, `"runtime_family": "legacy"`, `"transport": "wireguard"`, `"phase": "connected"`} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("expected %q in output: %s", fragment, out)
		}
	}
}
