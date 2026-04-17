package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var addressJSON bool

var addressCmd = &cobra.Command{
	Use:   "address",
	Short: "Inspect WARP-assigned interface addresses",
}

var addressShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the currently allocated WARP IPv4/IPv6 addresses",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		acc, err := state.LoadAccount(dirs)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"account_id": acc.AccountID,
			"wireguard": map[string]any{
				"ipv4":          acc.WARPIPV4,
				"ipv6":          acc.WARPIPV6,
				"peer_endpoint": acc.WARPPeerEndpoint,
			},
		}
		if acc.Masque != nil {
			payload["masque"] = map[string]any{
				"ipv4":        acc.Masque.IPv4,
				"ipv6":        acc.Masque.IPv6,
				"endpoint_v4": acc.Masque.EndpointV4,
				"endpoint_v6": acc.Masque.EndpointV6,
			}
		}
		if addressJSON {
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}
		fmt.Fprintf(c.OutOrStdout(), "account_id: %s\n", acc.AccountID)
		fmt.Fprintf(c.OutOrStdout(), "wireguard_ipv4: %s\n", acc.WARPIPV4)
		fmt.Fprintf(c.OutOrStdout(), "wireguard_ipv6: %s\n", acc.WARPIPV6)
		fmt.Fprintf(c.OutOrStdout(), "wireguard_peer_endpoint: %s\n", acc.WARPPeerEndpoint)
		if acc.Masque != nil {
			fmt.Fprintf(c.OutOrStdout(), "masque_ipv4: %s\n", acc.Masque.IPv4)
			fmt.Fprintf(c.OutOrStdout(), "masque_ipv6: %s\n", acc.Masque.IPv6)
			fmt.Fprintf(c.OutOrStdout(), "masque_endpoint_v4: %s\n", acc.Masque.EndpointV4)
			fmt.Fprintf(c.OutOrStdout(), "masque_endpoint_v6: %s\n", acc.Masque.EndpointV6)
		}
		return nil
	},
}

func init() {
	addressShowCmd.Flags().BoolVar(&addressJSON, "json", false, "emit addresses as JSON")
	addressCmd.AddCommand(addressShowCmd)
}
