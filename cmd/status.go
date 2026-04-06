package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusJSON  bool
var statusTrace bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report proxy configuration, process, and health state",
	Long: `status checks whether account state is configured, the backend process is
running, and the local proxy port is reachable. Use --json for machine-readable
output. Use --trace to also probe cdn-cgi/trace through the proxy.`,
	RunE: func(c *cobra.Command, args []string) error {
		// status is available on all platforms for inspection
		// TODO(task-7): implement status + health probing
		fmt.Fprintln(c.OutOrStdout(), "status: not yet implemented")
		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "emit status as JSON")
	statusCmd.Flags().BoolVar(&statusTrace, "trace", false, "probe cdn-cgi/trace through the proxy (requires live network)")
}
