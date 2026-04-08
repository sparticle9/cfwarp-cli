package cmd

import "github.com/spf13/cobra"

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect using the configured backend",
	Long:  `connect is a compatibility wrapper around 'up'.`,
	RunE: func(c *cobra.Command, args []string) error {
		return upCmd.RunE(c, args)
	},
}

var disconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect the running backend",
	Long:  `disconnect is a compatibility wrapper around 'down'.`,
	RunE: func(c *cobra.Command, args []string) error {
		return downCmd.RunE(c, args)
	},
}
