package cmd

import (
	"fmt"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var importFile string
var importForce bool

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing WARP credentials from a JSON file",
	Long: `import reads previously generated WARP account data from a JSON file
and stores it in local state, skipping re-registration.

The file must be a JSON object with at minimum: warp_private_key,
warp_peer_public_key, warp_ipv4, warp_ipv6, account_id, and token.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		return runImport(c)
	},
}

// validateAccount checks required fields are present.
func validateAccount(acc state.AccountState) error {
	missing := []string{}
	if acc.AccountID == "" {
		missing = append(missing, "account_id")
	}
	if acc.Token == "" {
		missing = append(missing, "token")
	}
	if acc.WARPPrivateKey == "" {
		missing = append(missing, "warp_private_key")
	}
	if acc.WARPPeerPubKey == "" {
		missing = append(missing, "warp_peer_public_key")
	}
	if acc.WARPIPV4 == "" {
		missing = append(missing, "warp_ipv4")
	}
	if acc.WARPIPV6 == "" {
		missing = append(missing, "warp_ipv6")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %v", missing)
	}
	return nil
}

func init() {
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "path to account JSON file (required)")
	importCmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing account data")
	_ = importCmd.MarkFlagRequired("file")
}
