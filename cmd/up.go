package cmd

import (
	"errors"
	"fmt"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
	"github.com/spf13/cobra"
)

var upForeground bool

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the WARP proxy backend",
	Long: `up validates prerequisites, renders the selected backend configuration,
and launches the configured proxy backend. Use --foreground to keep it in the
foreground (useful for Docker entrypoints).`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}

		dirs := state.Resolve(globalStateDir, "")
		if err := dirs.MkdirAll(); err != nil {
			return fmt.Errorf("prepare state directories: %w", err)
		}

		acc, err := state.LoadAccount(dirs)
		if err != nil {
			if errors.Is(err, state.ErrNotFound) {
				return fmt.Errorf("no account registered; run 'cfwarp-cli register' first")
			}
			return fmt.Errorf("load account: %w", err)
		}

		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return fmt.Errorf("resolve settings: %w", err)
		}

		b, err := configuredBackend(sett)
		if err != nil {
			return err
		}
		if err := b.ValidatePrereqs(c.Context()); err != nil {
			return err
		}

		// Guard against starting twice; clean up stale runtime if process is gone.
		if rt, err := state.LoadRuntime(dirs); err == nil {
			if supervisor.CheckStale(rt) {
				return fmt.Errorf("backend already running (PID %d); run 'cfwarp-cli down' first", rt.PID)
			}
			fmt.Fprintln(c.OutOrStdout(), "Removing stale runtime state…")
			_ = state.ClearRuntime(dirs)
		}

		result, err := b.RenderConfig(backend.RenderInput{Account: acc, Settings: sett})
		if err != nil {
			return fmt.Errorf("render config: %w", err)
		}

		fmt.Fprintf(c.OutOrStdout(), "Starting %s (foreground=%v)…\n", sett.Backend, upForeground)
		info, startErr := b.Start(c.Context(), result, dirs, upForeground)

		rt := orchestrator.BuildRuntimeState(sett, info, dirs)
		if startErr != nil {
			orchestrator.MarkStopped(&rt, startErr.Error())
		}

		// Persist runtime metadata even on error so 'status' can report it.
		if saveErr := state.SaveRuntime(dirs, rt); saveErr != nil {
			fmt.Fprintf(c.ErrOrStderr(), "warning: could not save runtime state: %v\n", saveErr)
		}

		if startErr != nil {
			return fmt.Errorf("backend exited: %w", startErr)
		}

		if !upForeground {
			fmt.Fprintf(c.OutOrStdout(), "Backend started (PID %d)\n", info.PID)
			fmt.Fprintf(c.OutOrStdout(), "Proxy listening on %s:%d (%s)\n",
				sett.ListenHost, sett.ListenPort, sett.Mode)
		}
		return nil
	},
}

func init() {
	upCmd.Flags().BoolVar(&upForeground, "foreground", false, "run in foreground mode (for Docker entrypoints)")
}
