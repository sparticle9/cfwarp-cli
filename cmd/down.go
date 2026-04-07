package cmd

import (
	"errors"
	"fmt"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the running WARP proxy backend",
	Long:  `down stops the managed backend process and removes transient runtime files.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}

		dirs := state.Resolve(globalStateDir, "")

		rt, err := state.LoadRuntime(dirs)
		if err != nil {
			if errors.Is(err, state.ErrNotFound) {
				return fmt.Errorf("no backend is running (no runtime state found)")
			}
			return fmt.Errorf("load runtime state: %w", err)
		}

		if !supervisor.CheckStale(rt) {
			fmt.Fprintln(c.OutOrStdout(), "Backend is not running (stale runtime). Cleaning up…")
			return state.ClearRuntime(dirs)
		}

		b, err := runtimeBackend(rt)
		if err != nil {
			return err
		}

		fmt.Fprintf(c.OutOrStdout(), "Stopping backend (PID %d)…\n", rt.PID)
		if err := b.Stop(c.Context(), runtimeInfo(rt)); err != nil {
			return fmt.Errorf("stop backend: %w", err)
		}

		if err := state.ClearRuntime(dirs); err != nil {
			return fmt.Errorf("clear runtime state: %w", err)
		}

		fmt.Fprintln(c.OutOrStdout(), "Backend stopped.")
		return nil
	},
}
