package settings_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/settings"
	"github.com/nexus/cfwarp-cli/internal/state"
)

func tempDirs(t *testing.T) state.Dirs {
	t.Helper()
	root := t.TempDir()
	return state.Dirs{
		Config:  filepath.Join(root, "config"),
		Runtime: filepath.Join(root, "runtime"),
	}
}

func writeSettings(t *testing.T, d state.Dirs, s state.Settings) {
	t.Helper()
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.Marshal(s)
	if err := os.WriteFile(d.SettingsFile(), data, 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// --- defaults ---

func TestLoad_Defaults(t *testing.T) {
	d := tempDirs(t)
	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Backend != "singbox-wireguard" {
		t.Errorf("default backend wrong: %s", s.Backend)
	}
	if s.ListenPort != 1080 {
		t.Errorf("default port wrong: %d", s.ListenPort)
	}
	if s.ProxyMode != "socks5" {
		t.Errorf("default proxy_mode wrong: %s", s.ProxyMode)
	}
	if s.LogLevel != "info" {
		t.Errorf("default log_level wrong: %s", s.LogLevel)
	}
}

// --- persisted file overrides defaults ---

func TestLoad_PersistedOverridesDefaults(t *testing.T) {
	d := tempDirs(t)
	persisted := state.DefaultSettings()
	persisted.ListenPort = 9090
	persisted.EndpointOverride = "162.159.192.1:4500"
	writeSettings(t, d, persisted)

	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ListenPort != 9090 {
		t.Errorf("expected port 9090, got %d", s.ListenPort)
	}
	if s.EndpointOverride != "162.159.192.1:4500" {
		t.Errorf("expected endpoint override, got %q", s.EndpointOverride)
	}
}

// --- env vars override persisted ---

func TestLoad_EnvOverridesPersisted(t *testing.T) {
	d := tempDirs(t)
	persisted := state.DefaultSettings()
	persisted.ListenPort = 9090
	writeSettings(t, d, persisted)

	t.Setenv("CFWARP_LISTEN_PORT", "7070")
	t.Setenv("CFWARP_LOG_LEVEL", "debug")

	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ListenPort != 7070 {
		t.Errorf("expected env port 7070, got %d", s.ListenPort)
	}
	if s.LogLevel != "debug" {
		t.Errorf("expected env log_level debug, got %s", s.LogLevel)
	}
}

func TestLoad_EnvProxyMode_Normalised(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_PROXY_MODE", "HTTP") // uppercase
	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ProxyMode != "http" {
		t.Errorf("expected normalised proxy_mode 'http', got %q", s.ProxyMode)
	}
}

func TestLoad_EnvPortInvalid_Ignored(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_LISTEN_PORT", "not-a-number")
	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Falls back to default.
	if s.ListenPort != 1080 {
		t.Errorf("expected fallback port 1080, got %d", s.ListenPort)
	}
}

// --- overrides (flags) win over env ---

func TestLoad_FlagOverridesEnv(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_LISTEN_PORT", "7070")

	s, err := settings.Load(d, settings.Overrides{ListenPort: intPtr(5050)})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ListenPort != 5050 {
		t.Errorf("expected flag port 5050, got %d", s.ListenPort)
	}
}

func TestLoad_FlagEndpointOverride(t *testing.T) {
	d := tempDirs(t)
	s, err := settings.Load(d, settings.Overrides{EndpointOverride: strPtr("162.159.192.1:4500")})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.EndpointOverride != "162.159.192.1:4500" {
		t.Errorf("expected endpoint override, got %q", s.EndpointOverride)
	}
}

// --- precedence chain: flag > env > file > default ---

func TestLoad_FullPrecedenceChain(t *testing.T) {
	d := tempDirs(t)

	// File sets port=9090.
	p := state.DefaultSettings()
	p.ListenPort = 9090
	writeSettings(t, d, p)

	// Env overrides to 7070.
	t.Setenv("CFWARP_LISTEN_PORT", "7070")

	// Flag overrides to 5050.
	s, err := settings.Load(d, settings.Overrides{ListenPort: intPtr(5050)})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.ListenPort != 5050 {
		t.Errorf("flag should win (5050), got %d", s.ListenPort)
	}
}
