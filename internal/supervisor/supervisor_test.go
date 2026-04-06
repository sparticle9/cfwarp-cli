package supervisor_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
)

func logDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "logs")
}

// sleepCmd returns a shell one-liner that traps SIGTERM so it exits promptly.
func sleepCmd(secs string) []string {
	return []string{"sh", "-c", "trap 'exit 0' TERM; sleep " + secs + " & wait"}
}

// --- IsRunning ---

func TestIsRunning_CurrentProcess(t *testing.T) {
	if !supervisor.IsRunning(os.Getpid()) {
		t.Error("IsRunning should return true for the current process")
	}
}

func TestIsRunning_InvalidPID(t *testing.T) {
	if supervisor.IsRunning(0) {
		t.Error("IsRunning(0) should return false")
	}
	if supervisor.IsRunning(-1) {
		t.Error("IsRunning(-1) should return false")
	}
}

func TestIsRunning_DeadPID(t *testing.T) {
	// Start a process that exits immediately, then check it's gone.
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command:    []string{"true"},
		LogDir:     logDir(t),
		Backend:    "test",
		Foreground: true,
	})
	if err != nil {
		t.Fatalf("Start(true): %v", err)
	}
	// Give the OS a moment to reap.
	time.Sleep(50 * time.Millisecond)
	if supervisor.IsRunning(rt.PID) {
		t.Errorf("IsRunning should be false for exited PID %d", rt.PID)
	}
}

// --- Start (background) ---

func TestStart_Background(t *testing.T) {
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command: sleepCmd("10"),
		LogDir:  logDir(t),
		Backend: "test",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(rt) })

	if rt.PID <= 0 {
		t.Errorf("expected positive PID, got %d", rt.PID)
	}
	if rt.Backend != "test" {
		t.Errorf("expected Backend=test, got %s", rt.Backend)
	}
	if rt.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if rt.StdoutLogPath == "" || rt.StderrLogPath == "" {
		t.Error("expected log paths to be set")
	}
	if !supervisor.IsRunning(rt.PID) {
		t.Errorf("expected process %d to be running", rt.PID)
	}
}

func TestStart_LogFilesCreated(t *testing.T) {
	dir := logDir(t)
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command: sleepCmd("10"),
		LogDir:  dir,
		Backend: "test",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(rt) })

	for _, path := range []string{rt.StdoutLogPath, rt.StderrLogPath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("log file not created: %s: %v", path, err)
		}
	}
}

func TestStart_LogFilePermissions(t *testing.T) {
	dir := logDir(t)
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command: sleepCmd("10"),
		LogDir:  dir,
		Backend: "test",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(rt) })

	for _, path := range []string{rt.StdoutLogPath, rt.StderrLogPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("stat %s: %v", path, err)
			continue
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("expected 0600 on %s, got %04o", path, perm)
		}
	}
}

// --- Start (foreground) ---

func TestStart_Foreground_Success(t *testing.T) {
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command:    []string{"true"},
		LogDir:     logDir(t),
		Backend:    "test",
		Foreground: true,
	})
	if err != nil {
		t.Fatalf("foreground 'true' should succeed, got: %v", err)
	}
	if rt.PID <= 0 {
		t.Error("expected valid PID even in foreground mode")
	}
	if rt.LastError != "" {
		t.Errorf("expected no LastError, got %q", rt.LastError)
	}
}

func TestStart_Foreground_Crash(t *testing.T) {
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command:    []string{"false"},
		LogDir:     logDir(t),
		Backend:    "test",
		Foreground: true,
	})
	if err == nil {
		t.Fatal("expected error for 'false' (non-zero exit)")
	}
	if rt.LastError == "" {
		t.Error("expected LastError to be set on crash")
	}
}

func TestStart_EmptyCommand(t *testing.T) {
	_, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		LogDir:  logDir(t),
		Backend: "test",
	})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// --- Stop ---

func TestStop_RunningProcess(t *testing.T) {
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command: sleepCmd("30"),
		LogDir:  logDir(t),
		Backend: "test",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := supervisor.Stop(rt); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if supervisor.IsRunning(rt.PID) {
		t.Errorf("process %d should be stopped", rt.PID)
	}
}

func TestStop_AlreadyDead(t *testing.T) {
	// Start a process that exits immediately (foreground), then try to stop it.
	rt, _ := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command:    []string{"true"},
		LogDir:     logDir(t),
		Backend:    "test",
		Foreground: true,
	})
	// Should not error even though the process is already gone.
	if err := supervisor.Stop(rt); err != nil {
		t.Errorf("Stop on dead process should not error, got: %v", err)
	}
}

func TestStop_InvalidPID(t *testing.T) {
	for _, pid := range []int{0, -5} {
		rt := state.RuntimeState{PID: pid}
		if err := supervisor.Stop(rt); err == nil {
			t.Errorf("Stop(PID=%d) should return error", pid)
		}
	}
}

// --- CheckStale ---

func TestCheckStale_Running(t *testing.T) {
	rt, err := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command: sleepCmd("10"),
		LogDir:  logDir(t),
		Backend: "test",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(rt) })

	if !supervisor.CheckStale(rt) {
		t.Error("CheckStale should return true (running) for live process")
	}
}

func TestCheckStale_Dead(t *testing.T) {
	rt, _ := supervisor.Start(context.Background(), supervisor.StartConfig{
		Command:    []string{"true"},
		LogDir:     logDir(t),
		Backend:    "test",
		Foreground: true,
	})
	time.Sleep(50 * time.Millisecond)
	if supervisor.CheckStale(rt) {
		t.Error("CheckStale should return false (stale) for dead process")
	}
}


