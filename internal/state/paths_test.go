package state_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func TestResolve_Override(t *testing.T) {
	d := state.Resolve("/my/config", "/my/runtime")
	if d.Config != "/my/config" {
		t.Errorf("expected /my/config, got %s", d.Config)
	}
	if d.Runtime != "/my/runtime" {
		t.Errorf("expected /my/runtime, got %s", d.Runtime)
	}
}

func TestResolve_XDGEnv(t *testing.T) {
	// XDG paths only apply on non-container Linux.
	if runtime.GOOS != "linux" {
		t.Skip("XDG path resolution only applies on Linux hosts")
	}
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			t.Skipf("skipping XDG test: running inside a container (%s found)", f)
		}
	}

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	d := state.Resolve("", "")
	if !strings.HasPrefix(d.Config, filepath.Join(tmp, "cfg")) {
		t.Errorf("expected config under XDG_CONFIG_HOME, got %s", d.Config)
	}
	if !strings.HasPrefix(d.Runtime, filepath.Join(tmp, "state")) {
		t.Errorf("expected runtime under XDG_STATE_HOME, got %s", d.Runtime)
	}
}

func TestMkdirAll(t *testing.T) {
	root := t.TempDir()
	d := state.Dirs{
		Config:  filepath.Join(root, "config"),
		Runtime: filepath.Join(root, "runtime"),
	}
	if err := d.MkdirAll(); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, dir := range []string{d.Config, d.Runtime, d.LogDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected dir %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("expected 0700 on %s, got %04o", dir, perm)
		}
	}
}

func TestDirPaths(t *testing.T) {
	d := state.Dirs{Config: "/cfg", Runtime: "/run"}
	cases := map[string]string{
		"AccountFile":     d.AccountFile(),
		"SettingsFile":    d.SettingsFile(),
		"RuntimeFile":     d.RuntimeFile(),
		"BackendConfig":   d.BackendConfigFile(),
		"LogDir":          d.LogDir(),
	}
	for name, path := range cases {
		if path == "" {
			t.Errorf("%s returned empty path", name)
		}
	}
}
