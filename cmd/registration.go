package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var registrationJSON bool

var registrationCmd = &cobra.Command{
	Use:   "registration",
	Short: "Manage local registration state",
}

var registrationNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Register a new Cloudflare WARP device",
	RunE: func(c *cobra.Command, args []string) error {
		return runRegister(c)
	},
}

var registrationImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing WARP credentials",
	RunE: func(c *cobra.Command, args []string) error {
		return runImport(c)
	},
}

var registrationShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current local registration",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		acc, err := state.LoadAccount(dirs)
		if err != nil {
			if errors.Is(err, state.ErrNotFound) {
				return fmt.Errorf("no account registered")
			}
			return err
		}
		if registrationJSON {
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(acc)
		}
		fmt.Fprintf(c.OutOrStdout(), "account_id: %s\n", acc.AccountID)
		fmt.Fprintf(c.OutOrStdout(), "source:     %s\n", acc.Source)
		fmt.Fprintf(c.OutOrStdout(), "created_at: %s\n", acc.CreatedAt.Format(time.RFC3339))
		fmt.Fprintf(c.OutOrStdout(), "wireguard:  %t\n", acc.WireGuard != nil)
		fmt.Fprintf(c.OutOrStdout(), "masque:     %t\n", acc.Masque != nil)
		return nil
	},
}

func init() {
	registrationNewCmd.Flags().BoolVar(&registerForce, "force", false, "overwrite existing registration data")
	registrationNewCmd.Flags().BoolVar(&registerMasque, "masque", false, "also enroll a MASQUE key and persist MASQUE transport state")
	registrationImportCmd.Flags().StringVarP(&importFile, "file", "f", "", "path to account JSON file (required)")
	registrationImportCmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing account data")
	_ = registrationImportCmd.MarkFlagRequired("file")
	registrationShowCmd.Flags().BoolVar(&registrationJSON, "json", false, "emit registration state as JSON")
	registrationCmd.AddCommand(registrationNewCmd, registrationImportCmd, registrationShowCmd)
}
