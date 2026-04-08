package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
)

func renderStateDirs(t *testing.T) state.Dirs {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "state")
	return state.Dirs{
		Config:  cfg,
		Runtime: filepath.Join(cfg, "run"),
	}
}

func writeRenderAccount(t *testing.T, d state.Dirs) {
	t.Helper()
	acc := state.AccountState{
		AccountID:      "acct-render",
		Token:          "tok-render",
		WARPPrivateKey: "privKeyBase64==",
		WARPPeerPubKey: "peerPubKeyBase64==",
		WARPReserved:   [3]int{0, 1, 2},
		WARPIPV4:       "172.16.0.2",
		WARPIPV6:       "fd01::2",
		CreatedAt:      time.Now().UTC(),
		Source:         "register",
	}
	if err := state.SaveAccount(d, acc, false); err != nil {
		t.Fatalf("save account: %v", err)
	}
}

func executeRender(t *testing.T, args ...string) (string, error) {
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

func TestRender_UsesRegisteredBackend(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("render command enforces linux-only platform checks")
	}
	d := renderStateDirs(t)
	writeRenderAccount(t, d)

	fakeBinDir := t.TempDir()
	fakeSingbox := filepath.Join(fakeBinDir, "sing-box")
	if err := os.WriteFile(fakeSingbox, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake sing-box: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+origPath)

	out, err := executeRender(t, "render", "--state-dir", d.Config)
	if err != nil {
		t.Fatalf("render: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, `"type": "wireguard"`) {
		t.Fatalf("expected wireguard config in output, got: %s", out)
	}
	if !strings.Contains(out, `"type": "socks"`) {
		t.Fatalf("expected socks inbound in output, got: %s", out)
	}
}
