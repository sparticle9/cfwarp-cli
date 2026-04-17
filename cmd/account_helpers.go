package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/nexus/cfwarp-cli/internal/cloudflare"
	"github.com/nexus/cfwarp-cli/internal/state"
	masquetransport "github.com/nexus/cfwarp-cli/internal/transport/masque"
	"github.com/nexus/cfwarp-cli/internal/warp"
	"github.com/spf13/cobra"
)

func runRegister(c *cobra.Command) error {
	dirs := state.Resolve(globalStateDir, "")
	return registerAccount(c, dirs, registerForce, registerMasque)
}

func registerAccount(c *cobra.Command, dirs state.Dirs, force bool, masque bool) error {
	if err := dirs.MkdirAll(); err != nil {
		return fmt.Errorf("prepare state directories: %w", err)
	}

	if _, err := state.LoadAccount(dirs); err == nil && !force {
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
	client := cloudflare.NewClient()
	result, err := client.RegisterConsumer(c.Context(), kp.PublicKey)
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
	if masque {
		fmt.Fprintln(c.OutOrStdout(), "Enrolling MASQUE key…")
		privDER, pubDER, err := masquetransport.GenerateECDSAKeypairDER()
		if err != nil {
			return fmt.Errorf("generate MASQUE keypair: %w", err)
		}
		enrollment, err := client.EnrollMasqueKey(c.Context(), result.AccountID, result.Token, pubDER, "")
		if err != nil {
			return fmt.Errorf("MASQUE enrollment failed: %w", err)
		}
		acc.Masque = cloudflare.BuildMasqueState(privDER, enrollment)
	}
	if err := state.SaveAccount(dirs, acc, force); err != nil {
		return fmt.Errorf("save account: %w", err)
	}

	fmt.Fprintf(c.OutOrStdout(), "Registered successfully (account: %s)\n", result.AccountID)
	fmt.Fprintf(c.OutOrStdout(), "State saved to: %s\n", dirs.AccountFile())
	return nil
}

func runImport(c *cobra.Command) error {
	dirs := state.Resolve(globalStateDir, "")
	if err := dirs.MkdirAll(); err != nil {
		return fmt.Errorf("prepare state directories: %w", err)
	}

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
}
