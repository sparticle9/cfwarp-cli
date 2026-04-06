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
	s.ListenPort = 9090
	s.EndpointOverride = "162.159.192.1:4500"

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
}

func TestLoadSettings_DefaultsOnNotFound(t *testing.T) {
	d := tempDirs(t)
	got, err := state.LoadSettings(d)
	if !errors.Is(err, state.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if got.Backend != "singbox-wireguard" || got.ListenPort != 1080 {
		t.Errorf("expected defaults, got %+v", got)
	}
}

// --- RuntimeState ---

func TestSaveLoadRuntime(t *testing.T) {
	d := tempDirs(t)
	rt := state.RuntimeState{
		PID:       42,
		Backend:   "singbox-wireguard",
		StartedAt: time.Now().UTC().Truncate(time.Second),
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
