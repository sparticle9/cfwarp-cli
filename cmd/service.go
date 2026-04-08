package cmd

import (
	"fmt"

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
		return fmt.Errorf("native runtime service entrypoint reserved but not yet implemented")
	},
}

func init() {
	serviceCmd.AddCommand(serviceRunNativeCmd)
}
