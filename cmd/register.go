package cmd

import (
	"github.com/spf13/cobra"
)

var registerForce bool
var registerMasque bool

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
		return runRegister(c)
	},
}

func init() {
	registerCmd.Flags().BoolVar(&registerForce, "force", false, "overwrite existing registration data")
	registerCmd.Flags().BoolVar(&registerMasque, "masque", false, "also enroll a MASQUE key and persist MASQUE transport state")
}
