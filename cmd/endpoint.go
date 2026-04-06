package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// endpointCmd is the parent for endpoint sub-commands.
var endpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Manage and test WARP peer endpoint candidates",
}

var endpointTestCmd = &cobra.Command{
	Use:   "test [host:port ...]",
	Short: "Validate and probe one or more endpoint candidates",
	Long: `test validates each candidate's syntax and optionally performs a
lightweight backend preflight check. It does not mutate stored runtime state.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		// TODO(task-8): implement endpoint candidate validation
		for _, ep := range args {
			fmt.Fprintf(c.OutOrStdout(), "endpoint test: %s — not yet implemented\n", ep)
		}
		return nil
	},
}

func init() {
	endpointCmd.AddCommand(endpointTestCmd)
}
