package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	capspkg "github.com/nexus/cfwarp-cli/internal/caps"
	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

type daemonCtlRequest struct {
	Action string `json:"action"`
}

type daemonCtlResponse struct {
	OK      bool          `json:"ok"`
	Message string        `json:"message,omitempty"`
	Status  *daemonStatus `json:"status,omitempty"`
}

type daemonStatus struct {
	RuntimeFamily        string                 `json:"runtime_family,omitempty"`
	Transport            string                 `json:"transport,omitempty"`
	Access               []state.AccessConfig   `json:"access,omitempty"`
	LastResults          []capspkg.Result       `json:"last_results,omitempty"`
	LastCheckedAt        string                 `json:"last_checked_at,omitempty"`
	CooldownUntil        string                 `json:"cooldown_until,omitempty"`
	LastError            string                 `json:"last_error,omitempty"`
	LastRotation         *state.RotationNovelty `json:"last_rotation,omitempty"`
	RotationHistoryCount int                    `json:"rotation_history_count,omitempty"`
}

type daemonManager struct {
	cmd        *cobra.Command
	dirs       state.Dirs
	socketPath string

	mu                   sync.Mutex
	settings             state.Settings
	lastResults          []capspkg.Result
	lastChecked          time.Time
	cooldownUntil        time.Time
	lastError            string
	lastRotation         *state.RotationNovelty
	rotationHistoryCount int
}

func newDaemonManager(cmd *cobra.Command, dirs state.Dirs, sett state.Settings) *daemonManager {
	socketPath := dirs.DaemonSocketFile()
	if sett.Daemon != nil && sett.Daemon.ControlSocket != "" {
		socketPath = sett.Daemon.ControlSocket
	}
	return &daemonManager{cmd: cmd, dirs: dirs, settings: sett, socketPath: socketPath}
}

func (m *daemonManager) snapshot() daemonStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := daemonStatus{
		RuntimeFamily:        m.settings.RuntimeFamily,
		Transport:            m.settings.Transport,
		Access:               append([]state.AccessConfig(nil), m.settings.Access...),
		LastResults:          append([]capspkg.Result(nil), m.lastResults...),
		LastError:            m.lastError,
		RotationHistoryCount: m.rotationHistoryCount,
	}
	if m.lastRotation != nil {
		rotation := *m.lastRotation
		status.LastRotation = &rotation
	}
	if !m.lastChecked.IsZero() {
		status.LastCheckedAt = m.lastChecked.Format(time.RFC3339)
	}
	if !m.cooldownUntil.IsZero() {
		status.CooldownUntil = m.cooldownUntil.Format(time.RFC3339)
	}
	return status
}

func (m *daemonManager) setSettings(sett state.Settings) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings = sett
}

func (m *daemonManager) setResults(results []capspkg.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastResults = append([]capspkg.Result(nil), results...)
	m.lastChecked = time.Now().UTC()
	m.lastError = ""
}

func (m *daemonManager) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		m.lastError = ""
		return
	}
	m.lastError = err.Error()
}

func (m *daemonManager) setRotationStatus(status state.RotationNovelty) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := status
	m.lastRotation = &copied
}

func (m *daemonManager) setRotationHistoryCount(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotationHistoryCount = n
}

func (m *daemonManager) inCooldown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.cooldownUntil.IsZero() && time.Now().Before(m.cooldownUntil)
}

func (m *daemonManager) setCooldown(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d <= 0 {
		m.cooldownUntil = time.Time{}
		return
	}
	m.cooldownUntil = time.Now().Add(d).UTC()
}

func (m *daemonManager) ensureBackendRunning(ctx context.Context) error {
	if rt, err := state.LoadRuntime(m.dirs); err == nil && orchestrator.IsRuntimeActive(rt) {
		return nil
	}
	sett, err := resolveSettings(m.cmd, m.dirs)
	if err != nil {
		return err
	}
	m.setSettings(sett)
	_, err = startBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs, sett, false)
	return err
}

func (m *daemonManager) runChecks(ctx context.Context) ([]capspkg.Result, error) {
	sett, err := resolveSettings(m.cmd, m.dirs)
	if err != nil {
		return nil, err
	}
	m.setSettings(sett)
	if sett.Caps == nil || len(sett.Caps.Checks) == 0 {
		return nil, nil
	}
	target, err := probeTargetFromSettings(sett)
	if err != nil {
		return nil, err
	}
	results := make([]capspkg.Result, 0, len(sett.Caps.Checks))
	for _, check := range sett.Caps.Checks {
		checkCtx, cancel := context.WithTimeout(ctx, time.Duration(check.TimeoutSeconds)*time.Second)
		results = append(results, capspkg.ProbeCheck(checkCtx, target, check))
		cancel()
	}
	m.setResults(results)
	return results, nil
}

func checkFailurePolicy(checks []state.CapCheck, results []capspkg.Result) (activeFailed bool, requiredFailed bool, anyRotate bool) {
	for i, check := range checks {
		if i >= len(results) {
			break
		}
		if results[i].OK {
			continue
		}
		if check.Required || check.RotateOnFail {
			activeFailed = true
		}
		if check.Required {
			requiredFailed = true
		}
		if check.RotateOnFail {
			anyRotate = true
		}
	}
	return
}

func (m *daemonManager) markLastGood() {
	acc, err := state.LoadAccount(m.dirs)
	if err != nil {
		return
	}
	if status, err := ensureRotationAccount(m.dirs, acc, m.settings); err == nil {
		m.setRotationStatus(status)
		m.setRotationHistoryCount(status.HistoryEntries)
	}
	_ = state.SaveLastGoodAccount(m.dirs, acc)
}

func (m *daemonManager) restoreLastGood(ctx context.Context) error {
	acc, err := state.LoadLastGoodAccount(m.dirs)
	if err != nil {
		return err
	}
	if err := state.SaveAccount(m.dirs, acc, true); err != nil {
		return err
	}
	if err := stopBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs); err != nil {
		return err
	}
	sett, err := resolveSettings(m.cmd, m.dirs)
	if err != nil {
		return err
	}
	m.setSettings(sett)
	_, err = startBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs, sett, false)
	return err
}

func (m *daemonManager) rotateOnce(ctx context.Context) error {
	sett, err := resolveSettings(m.cmd, m.dirs)
	if err != nil {
		return err
	}
	m.setSettings(sett)
	if err := stopBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs); err != nil {
		return err
	}
	masqueEnroll := sett.Transport == state.TransportMasque
	if sett.Rotation != nil && sett.Rotation.EnrollMasque {
		masqueEnroll = true
	}
	if err := registerAccount(m.cmd, m.dirs, true, masqueEnroll); err != nil {
		return err
	}
	acc, err := state.LoadAccount(m.dirs)
	if err != nil {
		return err
	}
	status, err := rememberRotationAccount(m.dirs, acc, sett, time.Now().UTC())
	if err != nil {
		return err
	}
	m.setRotationStatus(status)
	m.setRotationHistoryCount(status.HistoryEntries)
	fmt.Fprintf(m.cmd.OutOrStdout(), "%s\n", formatRotationNovelty(status))
	if !status.Qualifies {
		return fmt.Errorf("rotation did not produce a new address assignment under distinctness=%s", status.Distinctness)
	}
	_, err = startBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs, sett, false)
	return err
}

func (m *daemonManager) remediate(ctx context.Context, force bool) error {
	sett, err := resolveSettings(m.cmd, m.dirs)
	if err != nil {
		return err
	}
	m.setSettings(sett)
	if !force && (sett.Rotation == nil || !sett.Rotation.Enabled) {
		return nil
	}

	attempts := 1
	settle := 12 * time.Second
	cooldown := time.Duration(0)
	restoreLastGood := false
	if sett.Rotation != nil {
		if sett.Rotation.MaxAttemptsPerIncident > 0 {
			attempts = sett.Rotation.MaxAttemptsPerIncident
		}
		if sett.Rotation.SettleTimeSeconds > 0 {
			settle = time.Duration(sett.Rotation.SettleTimeSeconds) * time.Second
		}
		if sett.Rotation.CooldownSeconds > 0 {
			cooldown = time.Duration(sett.Rotation.CooldownSeconds) * time.Second
		}
		restoreLastGood = sett.Rotation.RestoreLastGood
	}
	var lastErr error
	for i := 1; i <= attempts; i++ {
		fmt.Fprintf(m.cmd.OutOrStdout(), "daemon remediation rotate attempt %d/%d…\n", i, attempts)
		if err := m.rotateOnce(ctx); err != nil {
			lastErr = err
			m.setError(err)
			continue
		}
		if !waitForPrimaryAccess(sett, settle) {
			lastErr = fmt.Errorf("proxy did not become reachable within %s", settle)
			m.setError(lastErr)
			continue
		}
		if sett.Caps == nil || len(sett.Caps.Checks) == 0 {
			m.markLastGood()
			m.setCooldown(0)
			return nil
		}
		results, err := m.runChecks(ctx)
		if err != nil {
			lastErr = err
			m.setError(err)
			continue
		}
		activeFailed, _, _ := checkFailurePolicy(sett.Caps.Checks, results)
		if !activeFailed {
			m.markLastGood()
			m.setCooldown(0)
			return nil
		}
		lastErr = fmt.Errorf("capability checks still failing after rotation")
		m.setError(lastErr)
	}

	if restoreLastGood {
		if err := m.restoreLastGood(ctx); err == nil {
			_, _ = m.runChecks(ctx)
		}
	}
	m.setCooldown(cooldown)
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("remediation failed")
}

func waitForPrimaryAccess(sett state.Settings, deadline time.Duration) bool {
	access, err := firstProxyAccess(sett)
	if err != nil {
		return false
	}
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if netProbe(access) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func netProbe(access state.AccessConfig) bool {
	addr := net.JoinHostPort(access.ListenHost, fmt.Sprintf("%d", access.ListenPort))
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return health.ProbeLocal(host, port, 0)
}

func (m *daemonManager) scheduledCheck(ctx context.Context) error {
	if err := m.ensureBackendRunning(ctx); err != nil {
		m.setError(err)
		return err
	}
	results, err := m.runChecks(ctx)
	if err != nil {
		m.setError(err)
		return err
	}
	sett := m.settings
	if sett.Caps == nil || len(sett.Caps.Checks) == 0 {
		return nil
	}
	activeFailed, requiredFailed, anyRotate := checkFailurePolicy(sett.Caps.Checks, results)
	if !activeFailed {
		m.markLastGood()
		m.setCooldown(0)
		return nil
	}
	if anyRotate && !m.inCooldown() {
		_ = m.remediate(ctx, false)
		results, err = m.runChecks(ctx)
		if err != nil {
			m.setError(err)
			return err
		}
		activeFailed, requiredFailed, _ = checkFailurePolicy(sett.Caps.Checks, results)
		if !activeFailed {
			m.markLastGood()
			return nil
		}
	}
	if requiredFailed {
		return fmt.Errorf("required capability probes failed")
	}
	return nil
}

func (m *daemonManager) reload(ctx context.Context) error {
	sett, err := resolveSettings(m.cmd, m.dirs)
	if err != nil {
		return err
	}
	m.setSettings(sett)
	if err := stopBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs); err != nil {
		return err
	}
	_, err = startBackendRuntime(ctx, m.cmd.OutOrStdout(), m.dirs, sett, false)
	return err
}

func (m *daemonManager) serveControl(ctx context.Context) error {
	_ = os.Remove(m.socketPath)
	if err := os.MkdirAll(filepath.Dir(m.socketPath), 0o700); err != nil {
		return err
	}
	ln, err := net.Listen("unix", m.socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(m.socketPath)
	}()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go m.handleControlConn(ctx, conn)
	}
}

func (m *daemonManager) handleControlConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	var req daemonCtlRequest
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(daemonCtlResponse{OK: false, Message: err.Error()})
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	var resp daemonCtlResponse
	switch action {
	case "status":
		resp = daemonCtlResponse{OK: true, Status: ptrDaemonStatus(m.snapshot())}
	case "check":
		err := m.scheduledCheck(ctx)
		resp = daemonCtlResponse{OK: err == nil, Message: errorString(err), Status: ptrDaemonStatus(m.snapshot())}
	case "rotate":
		err := m.remediate(ctx, true)
		resp = daemonCtlResponse{OK: err == nil, Message: errorString(err), Status: ptrDaemonStatus(m.snapshot())}
	case "reload":
		err := m.reload(ctx)
		resp = daemonCtlResponse{OK: err == nil, Message: errorString(err), Status: ptrDaemonStatus(m.snapshot())}
	default:
		resp = daemonCtlResponse{OK: false, Message: fmt.Sprintf("unsupported action %q", action)}
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func ptrDaemonStatus(s daemonStatus) *daemonStatus { return &s }

var daemonCmd = &cobra.Command{Use: "daemon", Short: "Run or control the cfwarp service daemon"}
var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the long-lived daemon and capability loop",
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		dirs := state.Resolve(globalStateDir, "")
		if err := dirs.MkdirAll(); err != nil {
			return err
		}
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		if err := platformCheckSettings(sett); err != nil {
			return err
		}
		mgr := newDaemonManager(c, dirs, sett)
		ctx, cancel := signal.NotifyContext(c.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
		if err := mgr.ensureBackendRunning(ctx); err != nil {
			return err
		}
		if status, err := rememberCurrentRotationAccount(dirs, sett); err == nil && status != nil {
			mgr.setRotationStatus(*status)
			mgr.setRotationHistoryCount(status.HistoryEntries)
		}
		if err := mgr.scheduledCheck(ctx); err != nil {
			return err
		}
		errCh := make(chan error, 1)
		go func() { errCh <- mgr.serveControl(ctx) }()
		interval := 30 * time.Second
		if sett.Caps != nil && sett.Caps.IntervalSeconds > 0 {
			interval = time.Duration(sett.Caps.IntervalSeconds) * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case err := <-errCh:
				if err == nil || errors.Is(err, net.ErrClosed) {
					return nil
				}
				return err
			case <-ticker.C:
				if err := mgr.scheduledCheck(ctx); err != nil {
					return err
				}
			}
		}
	},
}

var daemonCtlCmd = &cobra.Command{Use: "ctl", Short: "Send commands to a running daemon"}

func newDaemonCtlCommand(action string) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: fmt.Sprintf("Send %s to the running daemon", action),
		RunE: func(c *cobra.Command, args []string) error {
			dirs := state.Resolve(globalStateDir, "")
			sett, err := resolveSettings(c, dirs)
			if err != nil {
				return err
			}
			socketPath := dirs.DaemonSocketFile()
			if sett.Daemon != nil && sett.Daemon.ControlSocket != "" {
				socketPath = sett.Daemon.ControlSocket
			}
			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				return err
			}
			defer conn.Close()
			if err := json.NewEncoder(conn).Encode(daemonCtlRequest{Action: action}); err != nil {
				return err
			}
			var resp daemonCtlResponse
			if err := json.NewDecoder(conn).Decode(&resp); err != nil {
				return err
			}
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(resp); err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("%s", resp.Message)
			}
			return nil
		},
	}
}

func init() {
	daemonCtlCmd.AddCommand(
		newDaemonCtlCommand("status"),
		newDaemonCtlCommand("check"),
		newDaemonCtlCommand("rotate"),
		newDaemonCtlCommand("reload"),
	)
	daemonCmd.AddCommand(daemonRunCmd, daemonCtlCmd)
}
