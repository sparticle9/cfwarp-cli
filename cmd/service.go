package cmd

import (
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:    "service",
	Short:  "Internal runtime service commands",
	Hidden: true,
}

var serviceRunNativeCmd = &cobra.Command{
	Use:    "run-native",
	Short:  "Internal native runtime entrypoint",
	Hidden: true,
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		return runNativeRuntime(c.Context(), dirs, c)
	},
}

func init() {
	serviceCmd.AddCommand(serviceRunNativeCmd)
}
