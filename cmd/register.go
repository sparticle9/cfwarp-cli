package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/warp"
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

		dirs := state.Resolve(globalStateDir, "")
		if err := dirs.MkdirAll(); err != nil {
			return fmt.Errorf("prepare state directories: %w", err)
		}

		// Guard against accidental overwrite without --force.
		if _, err := state.LoadAccount(dirs); err == nil && !registerForce {
			return fmt.Errorf("account already registered at %s; use --force to overwrite", dirs.AccountFile())
		} else if err != nil && !errors.Is(err, state.ErrNotFound) {
			return fmt.Errorf("read existing account: %w", err)
		}

		fmt.Fprintln(c.OutOrStdout(), "Generating X25519 keypair…")
		kp, err := warp.GenerateKeypair()
		if err != nil {
			return fmt.Errorf("generate keypair: %w", err)
		}

		fmt.Fprintln(c.OutOrStdout(), "Registering with Cloudflare WARP…")
		client := warp.NewClient()
		result, err := client.Register(c.Context(), kp.PublicKey)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		acc := state.AccountState{
			AccountID:        result.AccountID,
			Token:            result.Token,
			License:          result.License,
			ClientID:         result.ClientID,
			WARPPrivateKey:   kp.PrivateKey,
			WARPPeerPubKey:   result.PeerPublicKey,
			WARPReserved:     result.Reserved,
			WARPPeerEndpoint: result.PeerEndpoint,
			WARPIPV4:         result.IPv4,
			WARPIPV6:         result.IPv6,
			CreatedAt:        time.Now().UTC(),
			Source:           "register",
		}
		if err := state.SaveAccount(dirs, acc, registerForce); err != nil {
			return fmt.Errorf("save account: %w", err)
		}

		fmt.Fprintf(c.OutOrStdout(), "Registered successfully (account: %s)\n", result.AccountID)
		fmt.Fprintf(c.OutOrStdout(), "State saved to: %s\n", dirs.AccountFile())
		return nil
	},
}

func init() {
	registerCmd.Flags().BoolVar(&registerForce, "force", false, "overwrite existing registration data")
}
