package cmd

import (
	"bytes"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
)

// execStatus runs the status command with the given extra args and a temp state dir.
func execStatus(t *testing.T, dirs state.Dirs, extraArgs ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	args := append([]string{"status", "--state-dir", dirs.Config}, extraArgs...)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return buf.String(), err
}

func tempStatusDirs(t *testing.T) state.Dirs {
	t.Helper()
	// When --state-dir=/x is passed, Resolve sets Runtime=/x/run.
	// Mirror that here so SaveRuntime and LoadRuntime agree.
	cfg := filepath.Join(t.TempDir(), "state")
	return state.Dirs{
		Config:  cfg,
		Runtime: filepath.Join(cfg, "run"),
	}
}

func writeAccount(t *testing.T, d state.Dirs) {
	t.Helper()
	acc := state.AccountState{
		AccountID:      "acct-test",
		Token:          "tok-test",
		WARPPrivateKey: "priv",
		WARPPeerPubKey: "pub",
		WARPIPV4:       "172.16.0.2",
		WARPIPV6:       "fd01::2",
		CreatedAt:      time.Now().UTC(),
		Source:         "register",
	}
	if err := state.SaveAccount(d, acc, false); err != nil {
		t.Fatalf("save account: %v", err)
	}
}

func writeRuntime(t *testing.T, d state.Dirs, pid int, lastErr string) {
	t.Helper()
	rt := state.RuntimeState{
		PID:       pid,
		Backend:   "singbox-wireguard",
		StartedAt: time.Now().UTC(),
		LastError: lastErr,
	}
	if err := state.SaveRuntime(d, rt); err != nil {
		t.Fatalf("save runtime: %v", err)
	}
}

// --- no account ---

func TestStatus_NoAccount(t *testing.T) {
	d := tempStatusDirs(t)
	out, err := execStatus(t, d)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "not configured") {
		t.Errorf("expected 'not configured', got: %s", out)
	}
}

// --- account present, no runtime ---

func TestStatus_AccountOnly(t *testing.T) {
	d := tempStatusDirs(t)
	writeAccount(t, d)
	out, err := execStatus(t, d)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "configured") {
		t.Errorf("expected 'configured', got: %s", out)
	}
	if !strings.Contains(out, "not started") {
		t.Errorf("expected 'not started', got: %s", out)
	}
}

// --- running process ---

func TestStatus_Running(t *testing.T) {
	d := tempStatusDirs(t)
	writeAccount(t, d)

	// Start a real background process so IsRunning returns true.
	rt, err := supervisor.Start(t.Context(), supervisor.StartConfig{
		Command: []string{"sh", "-c", "trap 'exit 0' TERM; sleep 30 & wait"},
		LogDir:  d.LogDir(),
		Backend: "singbox-wireguard",
	})
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(rt) })

	if err := state.SaveRuntime(d, rt); err != nil {
		t.Fatalf("save runtime: %v", err)
	}

	out, execErr := execStatus(t, d)
	if execErr != nil {
		t.Fatalf("status: %v", execErr)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("expected 'running', got: %s", out)
	}
}

// --- stale (crashed) process ---

func TestStatus_Crashed(t *testing.T) {
	d := tempStatusDirs(t)
	writeAccount(t, d)
	writeRuntime(t, d, 99999999, "exit status 1") // unlikely PID

	out, err := execStatus(t, d)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "not running") {
		t.Errorf("expected 'not running', got: %s", out)
	}
	if !strings.Contains(out, "exit status 1") {
		t.Errorf("expected last error in output, got: %s", out)
	}
}

// --- local reachability ---

func TestStatus_LocalReachable(t *testing.T) {
	d := tempStatusDirs(t)
	writeAccount(t, d)

	// Start a TCP listener to simulate a running proxy.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())

	out, err := execStatus(t, d, "--listen-port", portStr)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "reachable") {
		t.Errorf("expected 'reachable', got: %s", out)
	}
}

// --- JSON output ---

func TestStatus_JSON(t *testing.T) {
	d := tempStatusDirs(t)
	writeAccount(t, d)
	writeRuntime(t, d, 99999999, "")

	out, err := execStatus(t, d, "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}

	var report StatusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("unmarshal JSON: %v\noutput: %s", err, out)
	}
	if !report.AccountConfigured {
		t.Error("expected account_configured=true")
	}
	if report.BackendRunning {
		t.Error("expected backend_running=false for stale PID")
	}
}

func TestStatus_JSON_NoAccount(t *testing.T) {
	d := tempStatusDirs(t)
	out, err := execStatus(t, d, "--json")
	if err != nil {
		t.Fatalf("status --json: %v", err)
	}

	var report StatusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("unmarshal JSON: %v\noutput: %s", err, out)
	}
	if report.AccountConfigured {
		t.Error("expected account_configured=false")
	}
	if report.BackendRunning {
		t.Error("expected backend_running=false")
	}
}
