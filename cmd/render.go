package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var renderOutput string

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render the backend configuration without launching the proxy",
	Long: `render generates the sing-box WireGuard configuration from stored
account state and settings, and writes it to stdout or a file.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		// TODO(task-5): implement backend config renderer
		fmt.Fprintln(c.OutOrStdout(), "render: not yet implemented")
		return nil
	},
}

func init() {
	renderCmd.Flags().StringVarP(&renderOutput, "output", "o", "", "write rendered config to file instead of stdout")
}
