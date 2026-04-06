package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the running WARP proxy backend",
	Long:  `down stops the managed sing-box process and removes transient runtime files.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		// TODO(task-6): implement process supervisor + down
		fmt.Fprintln(c.OutOrStdout(), "down: not yet implemented")
		return nil
	},
}
