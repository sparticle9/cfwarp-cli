package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
	"github.com/spf13/cobra"
)

func startBackendRuntime(ctx context.Context, out io.Writer, dirs state.Dirs, sett state.Settings, foreground bool) (state.RuntimeState, error) {
	acc, err := state.LoadAccount(dirs)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) && autoRegisterOnStart() {
			masque := sett.RuntimeFamily == state.RuntimeFamilyNative && sett.Transport == state.TransportMasque
			if out != nil {
				fmt.Fprintln(out, "No account registered; auto-registering on start…")
			}
			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			if out != nil {
				cmd.SetOut(out)
				cmd.SetErr(out)
			}
			if regErr := registerAccount(cmd, dirs, false, masque); regErr != nil {
				return state.RuntimeState{}, fmt.Errorf("auto-register on start: %w", regErr)
			}
			acc, err = state.LoadAccount(dirs)
		}
		if err != nil {
			return state.RuntimeState{}, fmt.Errorf("load account: %w", err)
		}
	}

	b, err := configuredBackend(sett)
	if err != nil {
		return state.RuntimeState{}, err
	}
	if err := b.ValidatePrereqs(ctx); err != nil {
		return state.RuntimeState{}, err
	}

	if rt, err := state.LoadRuntime(dirs); err == nil {
		if supervisor.CheckStale(rt) {
			return state.RuntimeState{}, fmt.Errorf("backend already running (PID %d)", rt.PID)
		}
		if out != nil {
			fmt.Fprintln(out, "Removing stale runtime state…")
		}
		_ = state.ClearRuntime(dirs)
	}

	result, err := b.RenderConfig(backend.RenderInput{Account: acc, Settings: sett})
	if err != nil {
		return state.RuntimeState{}, fmt.Errorf("render config: %w", err)
	}

	if out != nil {
		fmt.Fprintf(out, "Starting %s (foreground=%v)…\n", sett.Backend, foreground)
	}
	info, startErr := b.Start(ctx, result, dirs, foreground)

	rt := orchestrator.BuildRuntimeState(sett, info, dirs)
	if startErr != nil {
		orchestrator.MarkStopped(&rt, startErr.Error())
	}
	if saveErr := state.SaveRuntime(dirs, rt); saveErr != nil && out != nil {
		fmt.Fprintf(out, "warning: could not save runtime state: %v\n", saveErr)
	}
	if startErr != nil {
		return rt, fmt.Errorf("backend exited: %w", startErr)
	}
	return rt, nil
}

func stopBackendRuntime(ctx context.Context, out io.Writer, dirs state.Dirs) error {
	rt, err := state.LoadRuntime(dirs)
	if err != nil {
		if err == state.ErrNotFound {
			return nil
		}
		return fmt.Errorf("load runtime state: %w", err)
	}
	if !orchestrator.IsRuntimeActive(rt) {
		if out != nil {
			fmt.Fprintln(out, "Backend is not running (stale runtime). Cleaning up…")
		}
		return state.ClearRuntime(dirs)
	}
	b, err := runtimeBackend(rt)
	if err != nil {
		return err
	}
	if out != nil {
		fmt.Fprintf(out, "Stopping backend (PID %d)…\n", rt.PID)
	}
	if err := b.Stop(ctx, runtimeInfo(rt)); err != nil {
		return fmt.Errorf("stop backend: %w", err)
	}
	if err := state.ClearRuntime(dirs); err != nil {
		return fmt.Errorf("clear runtime state: %w", err)
	}
	return nil
}

func waitForProxyReachable(sett state.Settings, deadline time.Duration) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if health.ProbeLocal(sett.ListenHost, sett.ListenPort, 0) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
