package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var upForeground bool

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the WARP proxy backend",
	Long: `up validates prerequisites, renders the backend configuration, and
launches the sing-box WireGuard proxy. Use --foreground to keep it in the
foreground (useful for Docker entrypoints).`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		// TODO(task-6): implement process supervisor + up
		fmt.Fprintln(c.OutOrStdout(), "up: not yet implemented")
		return nil
	},
}

func init() {
	upCmd.Flags().BoolVar(&upForeground, "foreground", false, "run in the foreground instead of daemonizing")
}
