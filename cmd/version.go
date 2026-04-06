package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/nexus/cfwarp-cli/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, backend, and binary path information",
	RunE: func(c *cobra.Command, args []string) error {
		fmt.Fprintf(c.OutOrStdout(), "cfwarp-cli %s (commit: %s, built: %s)\n", version.Version, version.Commit, version.Date)
		fmt.Fprintf(c.OutOrStdout(), "platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)

		singboxPath, err := exec.LookPath("sing-box")
		if err != nil {
			fmt.Fprintln(c.OutOrStdout(), "backend:  sing-box — not found in PATH")
		} else {
			fmt.Fprintf(c.OutOrStdout(), "backend:  sing-box — %s\n", singboxPath)
		}
		return nil
	},
}
