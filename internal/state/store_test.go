package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func sampleAccount() state.AccountState {
	return state.AccountState{
		AccountID:      "acct-123",
		Token:          "tok-abc",
		ClientID:       "cid-xyz",
		WARPPrivateKey: "priv",
		WARPPeerPubKey: "pub",
		WARPIPV4:       "172.16.0.2/32",
		WARPIPV6:       "fd01::2/128",
		CreatedAt:      time.Now().UTC().Truncate(time.Second),
		Source:         "register",
	}
}

// --- AccountState ---

func TestSaveLoadAccount(t *testing.T) {
	d := tempDirs(t)
	acc := sampleAccount()

	if err := state.SaveAccount(d, acc, false); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}

	got, err := state.LoadAccount(d)
	if err != nil {
		t.Fatalf("LoadAccount: %v", err)
	}
	if got.AccountID != acc.AccountID || got.Token != acc.Token {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, acc)
	}
	if got.SchemaVersion != state.CurrentAccountSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", state.CurrentAccountSchemaVersion, got.SchemaVersion)
	}
	if got.WireGuard == nil {
		t.Fatal("expected WireGuard transport data to be populated")
	}
	if got.WireGuard.PrivateKey != acc.WARPPrivateKey || got.WireGuard.PeerPubKey != acc.WARPPeerPubKey {
		t.Errorf("unexpected nested wireguard data: %+v", got.WireGuard)
	}
}

func TestLoadAccount_LegacyJSONMigratesToWireGuardState(t *testing.T) {
	d := tempDirs(t)
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{
  "account_id": "acct-123",
  "token": "tok-abc",
  "client_id": "cid-xyz",
  "warp_private_key": "priv",
  "warp_peer_public_key": "pub",
  "warp_ipv4": "172.16.0.2/32",
  "warp_ipv6": "fd01::2/128",
  "warp_reserved": [1,2,3],
  "warp_peer_endpoint": "162.159.192.1:2408",
  "created_at": "2026-04-07T00:00:00Z",
  "source": "import"
}`
	if err := os.WriteFile(d.AccountFile(), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy account: %v", err)
	}

	got, err := state.LoadAccount(d)
	if err != nil {
		t.Fatalf("LoadAccount: %v", err)
	}
	if got.WireGuard == nil {
		t.Fatal("expected migrated wireguard state")
	}
	if got.WireGuard.PrivateKey != "priv" || got.WireGuard.PeerEndpoint != "162.159.192.1:2408" {
		t.Errorf("unexpected migrated wireguard state: %+v", got.WireGuard)
	}
	if got.WARPPrivateKey != "priv" || got.WARPPeerEndpoint != "162.159.192.1:2408" {
		t.Errorf("expected legacy aliases to remain populated, got %+v", got)
	}
}

func TestSaveAccount_NoOverwrite(t *testing.T) {
	d := tempDirs(t)
	acc := sampleAccount()

	if err := state.SaveAccount(d, acc, false); err != nil {
		t.Fatalf("first SaveAccount: %v", err)
	}
	err := state.SaveAccount(d, acc, false)
	if err == nil {
		t.Fatal("expected error on second SaveAccount without force")
	}
}

func TestSaveAccount_ForceOverwrite(t *testing.T) {
	d := tempDirs(t)
	acc := sampleAccount()

	if err := state.SaveAccount(d, acc, false); err != nil {
		t.Fatalf("first SaveAccount: %v", err)
	}
	acc.Token = "tok-new"
	if err := state.SaveAccount(d, acc, true); err != nil {
		t.Fatalf("force SaveAccount: %v", err)
	}
	got, _ := state.LoadAccount(d)
	if got.Token != "tok-new" {
		t.Errorf("expected updated token, got %q", got.Token)
	}
}

func TestLoadAccount_NotFound(t *testing.T) {
	d := tempDirs(t)
	_, err := state.LoadAccount(d)
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- Settings ---

func TestSaveLoadSettings(t *testing.T) {
	d := tempDirs(t)
	s := state.DefaultSettings()
	s.RuntimeFamily = state.RuntimeFamilyLegacy
	s.Transport = state.TransportWireGuard
	s.Mode = state.ModeHTTP
	s.ListenPort = 9090
	s.EndpointOverride = "162.159.192.1:4500"
	s.MasqueOptions = &state.MasqueOptions{SNI: "consumer-masque.cloudflareclient.com", ConnectPort: 443}

	if err := state.SaveSettings(d, s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := state.LoadSettings(d)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.ListenPort != 9090 || got.EndpointOverride != s.EndpointOverride {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
	if got.RuntimeFamily != state.RuntimeFamilyLegacy || got.Transport != state.TransportWireGuard || got.Mode != state.ModeHTTP {
		t.Errorf("unexpected runtime selection after round-trip: %+v", got)
	}
	if got.MasqueOptions == nil || got.MasqueOptions.SNI != "consumer-masque.cloudflareclient.com" {
		t.Errorf("expected nested masque options to round-trip, got %+v", got.MasqueOptions)
	}
}

func TestLoadSettings_LegacyJSONMigratesRuntimeSelection(t *testing.T) {
	d := tempDirs(t)
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{
  "backend": "singbox-wireguard",
  "listen_host": "127.0.0.1",
  "listen_port": 8080,
  "proxy_mode": "http",
  "log_level": "debug"
}`
	if err := os.WriteFile(d.SettingsFile(), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy settings: %v", err)
	}

	got, err := state.LoadSettings(d)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.RuntimeFamily != state.RuntimeFamilyLegacy || got.Transport != state.TransportWireGuard {
		t.Errorf("expected migrated runtime selection, got %+v", got)
	}
	if got.Mode != state.ModeHTTP || got.ProxyMode != state.ModeHTTP {
		t.Errorf("expected migrated mode http, got %+v", got)
	}
	if got.Backend != state.BackendSingboxWireGuard {
		t.Errorf("expected backend alias %q, got %q", state.BackendSingboxWireGuard, got.Backend)
	}
}

func TestLoadSettings_DefaultsOnNotFound(t *testing.T) {
	d := tempDirs(t)
	got, err := state.LoadSettings(d)
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if got.Backend != state.BackendSingboxWireGuard || got.ListenPort != 1080 {
		t.Errorf("expected defaults, got %+v", got)
	}
	if got.RuntimeFamily != state.RuntimeFamilyLegacy || got.Transport != state.TransportWireGuard || got.Mode != state.ModeSocks5 {
		t.Errorf("expected default runtime selection, got %+v", got)
	}
}

// --- RuntimeState ---

func TestSaveLoadRuntime(t *testing.T) {
	d := tempDirs(t)
	rt := state.RuntimeState{
		PID:                42,
		Backend:            state.BackendSingboxWireGuard,
		RuntimeFamily:      state.RuntimeFamilyLegacy,
		Transport:          state.TransportWireGuard,
		Mode:               state.ModeHTTP,
		Phase:              state.RuntimePhaseConnected,
		ListenHost:         "127.0.0.1",
		ListenPort:         8080,
		SelectedEndpoint:   "162.159.192.1:2408",
		LastTransportError: "",
		StartedAt:          time.Now().UTC().Truncate(time.Second),
	}

	if err := state.SaveRuntime(d, rt); err != nil {
		t.Fatalf("SaveRuntime: %v", err)
	}
	got, err := state.LoadRuntime(d)
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if got.PID != 42 {
		t.Errorf("expected PID 42, got %d", got.PID)
	}
	if got.SchemaVersion != state.CurrentRuntimeSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", state.CurrentRuntimeSchemaVersion, got.SchemaVersion)
	}
	if got.RuntimeFamily != state.RuntimeFamilyLegacy || got.Transport != state.TransportWireGuard || got.Mode != state.ModeHTTP {
		t.Errorf("unexpected runtime selection after round-trip: %+v", got)
	}
	if got.Phase != state.RuntimePhaseConnected {
		t.Errorf("expected connected phase, got %q", got.Phase)
	}
}

func TestLoadRuntime_LegacyJSONMigratesRuntimeMetadata(t *testing.T) {
	d := tempDirs(t)
	if err := os.MkdirAll(d.Runtime, 0o700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	legacy := `{
  "pid": 42,
  "backend": "singbox-wireguard",
  "config_path": "/run/cfwarp-cli/backend.json",
  "stdout_log_path": "/tmp/stdout.log",
  "stderr_log_path": "/tmp/stderr.log",
  "started_at": "2026-04-07T00:00:00Z",
  "last_error": "",
  "local_reachable": true
}`
	if err := os.WriteFile(d.RuntimeFile(), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy runtime: %v", err)
	}
	got, err := state.LoadRuntime(d)
	if err != nil {
		t.Fatalf("LoadRuntime: %v", err)
	}
	if got.RuntimeFamily != state.RuntimeFamilyLegacy || got.Transport != state.TransportWireGuard {
		t.Errorf("expected migrated runtime selection, got %+v", got)
	}
	if got.Phase != state.RuntimePhaseConnected {
		t.Errorf("expected connected phase for legacy runtime, got %q", got.Phase)
	}
}

func TestLoadRuntime_NotFound(t *testing.T) {
	d := tempDirs(t)
	_, err := state.LoadRuntime(d)
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestClearRuntime(t *testing.T) {
	d := tempDirs(t)
	rt := state.RuntimeState{PID: 1}
	_ = state.SaveRuntime(d, rt)

	// Write a fake backend config file too.
	_ = os.MkdirAll(d.Runtime, 0o700)
	_ = os.WriteFile(d.BackendConfigFile(), []byte("{}"), 0o600)

	if err := state.ClearRuntime(d); err != nil {
		t.Fatalf("ClearRuntime: %v", err)
	}
	if _, err := os.Stat(d.RuntimeFile()); !errors.Is(err, os.ErrNotExist) {
		t.Error("runtime.json should have been removed")
	}
	if _, err := os.Stat(d.BackendConfigFile()); !errors.Is(err, os.ErrNotExist) {
		t.Error("backend.json should have been removed")
	}
}

// --- File permissions ---

func TestSaveAccount_FilePermissions(t *testing.T) {
	d := tempDirs(t)
	if err := state.SaveAccount(d, sampleAccount(), false); err != nil {
		t.Fatalf("SaveAccount: %v", err)
	}
	info, err := os.Stat(d.AccountFile())
	if err != nil {
		t.Fatalf("stat account file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected permissions 0600, got %04o", perm)
	}
}
