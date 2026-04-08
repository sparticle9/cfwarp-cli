package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func execCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
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
