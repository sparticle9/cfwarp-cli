package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var validateJSON bool

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the resolved configuration integrity",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		if validateJSON {
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"ok":       true,
				"settings": sett,
			})
		}
		fmt.Fprintln(c.OutOrStdout(), "configuration valid")
		return nil
	},
}

func init() {
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "emit resolved validated settings as JSON")
}
