package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var registerForce bool

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Generate a keypair and register a new Cloudflare WARP device",
	Long: `register calls the Cloudflare consumer registration API, generates a local
X25519 keypair, and persists the returned account data to local state.

Use --force to overwrite an existing registration.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		// TODO(task-3): implement registration client
		fmt.Fprintln(c.OutOrStdout(), "register: not yet implemented")
		return nil
	},
}

func init() {
	registerCmd.Flags().BoolVar(&registerForce, "force", false, "overwrite existing registration data")
}
