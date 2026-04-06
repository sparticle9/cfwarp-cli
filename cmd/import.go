package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

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

		dirs := state.Resolve(globalStateDir, "")
		if err := dirs.MkdirAll(); err != nil {
			return fmt.Errorf("prepare state directories: %w", err)
		}

		// Guard against accidental overwrite without --force.
		if _, err := state.LoadAccount(dirs); err == nil && !importForce {
			return fmt.Errorf("account already exists at %s; use --force to overwrite", dirs.AccountFile())
		} else if err != nil && !errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("read existing account: %w", err)
		}

		raw, err := os.ReadFile(importFile)
		if err != nil {
			return fmt.Errorf("read import file: %w", err)
		}

		var acc state.AccountState
		if err := json.Unmarshal(raw, &acc); err != nil {
			return fmt.Errorf("parse import file: %w", err)
		}
		if err := validateAccount(acc); err != nil {
			return fmt.Errorf("invalid import file: %w", err)
		}

		acc.Source = "import"
		if acc.CreatedAt.IsZero() {
			acc.CreatedAt = time.Now().UTC()
		}

		if err := state.SaveAccount(dirs, acc, importForce); err != nil {
			return fmt.Errorf("save account: %w", err)
		}

		fmt.Fprintf(c.OutOrStdout(), "Imported successfully (account: %s)\n", acc.AccountID)
		fmt.Fprintf(c.OutOrStdout(), "State saved to: %s\n", dirs.AccountFile())
		return nil
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
