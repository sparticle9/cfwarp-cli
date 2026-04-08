package orchestrator

import (
	"os"
	"path/filepath"
	"time"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
)

const serviceSocketFile = "service.sock"

// ServiceSocketPath returns the reserved path for a future native runtime local socket.
func ServiceSocketPath(dirs state.Dirs) string {
	return filepath.Join(dirs.Runtime, serviceSocketFile)
}

// BuildRuntimeState converts settings and runtime info into the persisted runtime model.
func BuildRuntimeState(sett state.Settings, info backend.RuntimeInfo, dirs state.Dirs) state.RuntimeState {
	sett.Normalize()
	rt := state.RuntimeState{
		SchemaVersion:     state.CurrentRuntimeSchemaVersion,
		PID:               info.PID,
		Backend:           sett.Backend,
		RuntimeFamily:     sett.RuntimeFamily,
		Transport:         sett.Transport,
		Mode:              sett.Mode,
		ListenHost:        sett.ListenHost,
		ListenPort:        sett.ListenPort,
		ConfigPath:        info.ConfigPath,
		StdoutLogPath:     info.StdoutLogPath,
		StderrLogPath:     info.StderrLogPath,
		StartedAt:         info.StartedAt,
		LastError:         info.LastError,
		ServiceSocketPath: ServiceSocketPath(dirs),
	}

	if rt.RuntimeFamily == state.RuntimeFamilyNative {
		rt.Phase = state.RuntimePhaseConnecting
	} else if rt.PID > 0 {
		rt.Phase = state.RuntimePhaseConnected
	} else {
		rt.Phase = state.RuntimePhaseIdle
	}
	return rt
}

// IsRuntimeActive reports whether the persisted runtime still appears live.
func IsRuntimeActive(rt state.RuntimeState) bool {
	rt.Normalize()
	switch rt.RuntimeFamily {
	case state.RuntimeFamilyNative:
		if rt.ServiceSocketPath == "" {
			return false
		}
		_, err := os.Stat(rt.ServiceSocketPath)
		return err == nil
	default:
		return supervisor.IsRunning(rt.PID)
	}
}

// DerivePhase computes the current runtime phase from persisted state plus liveness and reachability.
func DerivePhase(rt state.RuntimeState, running, localReachable bool) string {
	rt.Normalize()
	if !running {
		if rt.LastTransportError != "" || rt.LastError != "" || rt.PID > 0 || rt.ServiceSocketPath != "" {
			return state.RuntimePhaseStopped
		}
		if rt.Phase != "" {
			return rt.Phase
		}
		return state.RuntimePhaseIdle
	}
	if !localReachable {
		return state.RuntimePhaseDegraded
	}
	return state.RuntimePhaseConnected
}

// MarkTransportError records transport-level degradation metadata.
func MarkTransportError(rt *state.RuntimeState, reason string, at time.Time) {
	rt.LastTransportError = reason
	rt.LastReconnectReason = reason
	rt.LastReconnectAt = at.UTC()
	rt.Phase = state.RuntimePhaseDegraded
}

// MarkStopped records a terminal stop condition.
func MarkStopped(rt *state.RuntimeState, reason string) {
	if reason != "" {
		rt.LastError = reason
	}
	rt.Phase = state.RuntimePhaseStopped
}
