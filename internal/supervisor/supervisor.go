// Package supervisor manages the lifecycle of the sing-box backend process.
package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
)

const (
	stopTimeout   = 5 * time.Second
	pollInterval  = 100 * time.Millisecond
	stdoutLogFile = "backend.stdout.log"
	stderrLogFile = "backend.stderr.log"
)

// StartConfig defines how to launch the backend process.
type StartConfig struct {
	// Command is the full argv. e.g. ["sing-box", "run", "-c", "/path/config.json"]
	Command []string
	// LogDir is where stdout/stderr log files are written (mode 0700).
	LogDir string
	// Backend is the backend tag stored in RuntimeState (e.g. "singbox-wireguard").
	Backend string
	// Foreground blocks Start until the process exits (Docker entrypoint mode).
	Foreground bool
}

// Start launches the backend process described by cfg.
// In background mode it returns immediately after the process is spawned.
// In foreground mode it blocks until the process exits; any non-zero exit is
// returned as an error with the exit message stored in RuntimeState.LastError.
func Start(ctx context.Context, cfg StartConfig) (state.RuntimeState, error) {
	if len(cfg.Command) == 0 {
		return state.RuntimeState{}, fmt.Errorf("supervisor: command must not be empty")
	}
	if err := os.MkdirAll(cfg.LogDir, 0o700); err != nil {
		return state.RuntimeState{}, fmt.Errorf("create log dir: %w", err)
	}

	stdoutPath := filepath.Join(cfg.LogDir, stdoutLogFile)
	stderrPath := filepath.Join(cfg.LogDir, stderrLogFile)

	stdoutF, err := os.OpenFile(stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return state.RuntimeState{}, fmt.Errorf("open stdout log: %w", err)
	}
	defer stdoutF.Close()

	stderrF, err := os.OpenFile(stderrPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return state.RuntimeState{}, fmt.Errorf("open stderr log: %w", err)
	}
	defer stderrF.Close()

	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	cmd.Stdout = stdoutF
	cmd.Stderr = stderrF

	if !cfg.Foreground {
		// Detach from parent process group: child survives cfwarp-cli exit.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if err := cmd.Start(); err != nil {
		return state.RuntimeState{}, fmt.Errorf("start backend: %w", err)
	}

	rt := state.RuntimeState{
		PID:           cmd.Process.Pid,
		Backend:       cfg.Backend,
		StdoutLogPath: stdoutPath,
		StderrLogPath: stderrPath,
		StartedAt:     time.Now().UTC(),
	}

	if cfg.Foreground {
		waitErr := cmd.Wait()
		if waitErr != nil {
			rt.LastError = waitErr.Error()
			return rt, fmt.Errorf("backend exited with error: %w", waitErr)
		}
	}

	return rt, nil
}

// Stop sends SIGTERM to the process identified by rt.PID, waits up to
// stopTimeout, then sends SIGKILL if the process has not yet exited.
// Returns nil if the process is already gone.
func Stop(rt state.RuntimeState) error {
	if rt.PID <= 0 {
		return fmt.Errorf("invalid PID %d in runtime state", rt.PID)
	}
	if !IsRunning(rt.PID) {
		return nil
	}

	proc, err := os.FindProcess(rt.PID)
	if err != nil {
		return nil // already gone
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return nil // already gone
	}

	deadline := time.Now().Add(stopTimeout)
	for time.Now().Before(deadline) {
		// Try to reap zombie on each poll iteration (no-op if not our child).
		_, _ = proc.Wait()
		if !IsRunning(rt.PID) {
			return nil
		}
		time.Sleep(pollInterval)
	}

	// Still alive after grace period — force kill.
	_ = proc.Signal(syscall.SIGKILL)
	// Reap zombie if this process is our direct child.
	// proc.Wait returns ECHILD when we're not the parent; ignore that.
	_, _ = proc.Wait()
	// Final poll: give the OS a moment to update the process table.
	for i := 0; i < 30; i++ {
		time.Sleep(pollInterval)
		if !IsRunning(rt.PID) {
			return nil
		}
	}
	return nil
}

// IsRunning reports whether a process with the given PID is currently alive.
func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks for process existence without sending a real signal.
	return proc.Signal(syscall.Signal(0)) == nil
}

// CheckStale examines a persisted RuntimeState and returns:
//   - (true, nil)  if the PID is alive and the backend is running normally
//   - (false, nil) if the PID is stale (process is gone)
func CheckStale(rt state.RuntimeState) (running bool) {
	return IsRunning(rt.PID)
}
