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
	if s.Backend != state.BackendSingboxWireGuard {
		t.Errorf("default backend wrong: %s", s.Backend)
	}
	if s.RuntimeFamily != state.RuntimeFamilyLegacy {
		t.Errorf("default runtime_family wrong: %s", s.RuntimeFamily)
	}
	if s.Transport != state.TransportWireGuard {
		t.Errorf("default transport wrong: %s", s.Transport)
	}
	if s.ListenPort != 1080 {
		t.Errorf("default port wrong: %d", s.ListenPort)
	}
	if s.Mode != state.ModeSocks5 {
		t.Errorf("default mode wrong: %s", s.Mode)
	}
	if s.ProxyMode != state.ModeSocks5 {
		t.Errorf("default proxy_mode alias wrong: %s", s.ProxyMode)
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
	persisted.Mode = state.ModeHTTP
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
	if s.Mode != state.ModeHTTP || s.ProxyMode != state.ModeHTTP {
		t.Errorf("expected mode/proxy alias to be http, got %+v", s)
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

func TestLoad_EnvRuntimeSelection(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_RUNTIME_FAMILY", "LEGACY")
	t.Setenv("CFWARP_TRANSPORT", "WIREGUARD")
	t.Setenv("CFWARP_MODE", "HTTP")

	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.RuntimeFamily != state.RuntimeFamilyLegacy || s.Transport != state.TransportWireGuard || s.Mode != state.ModeHTTP {
		t.Errorf("expected env runtime selection to be normalised, got %+v", s)
	}
}

func TestLoad_EnvProxyMode_Normalised(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_PROXY_MODE", "HTTP") // legacy env name
	s, err := settings.Load(d, settings.Overrides{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Mode != state.ModeHTTP || s.ProxyMode != state.ModeHTTP {
		t.Errorf("expected normalised mode/proxy alias 'http', got %+v", s)
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

func TestLoad_FlagModeOverride(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_MODE", "http")

	s, err := settings.Load(d, settings.Overrides{Mode: strPtr(state.ModeSocks5)})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Mode != state.ModeSocks5 || s.ProxyMode != state.ModeSocks5 {
		t.Errorf("expected flag mode socks5, got %+v", s)
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

func TestLoad_NativeRuntimeCurrentlyRejected(t *testing.T) {
	d := tempDirs(t)
	t.Setenv("CFWARP_RUNTIME_FAMILY", "native")
	t.Setenv("CFWARP_TRANSPORT", "masque")

	_, err := settings.Load(d, settings.Overrides{})
	if err == nil {
		t.Fatal("expected native runtime selection to be rejected for now")
	}
}
