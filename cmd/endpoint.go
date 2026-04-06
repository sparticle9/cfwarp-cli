package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexus/cfwarp-cli/internal/endpoint"
	"github.com/spf13/cobra"
)

// endpointCmd is the parent for endpoint sub-commands.
var endpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Manage and test WARP peer endpoint candidates",
}

var endpointTestJSON bool

var endpointTestCmd = &cobra.Command{
	Use:   "test [host:port ...]",
	Short: "Validate and probe one or more endpoint candidates",
	Long: `test validates each candidate's syntax and optionally performs a
lightweight backend preflight check. It does not mutate stored runtime state.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		results := make([]endpoint.Result, 0, len(args))
		anyInvalid := false

		for _, arg := range args {
			r := endpoint.Probe(c.Context(), arg, 5*time.Second)
			results = append(results, r)
			if !r.Valid {
				anyInvalid = true
			}
		}

		if endpointTestJSON {
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return err
			}
		} else {
			tick := func(ok bool) string {
				if ok {
					return "✓"
				}
				return "✗"
			}
			for _, r := range results {
				notes := ""
				if r.Notes != "" {
					notes = "  [" + r.Notes + "]"
				}
				fmt.Fprintf(c.OutOrStdout(), "%-40s  valid %s  reachable %s%s\n",
					r.Candidate, tick(r.Valid), tick(r.Reachable), notes)
			}
		}

		if anyInvalid {
			return fmt.Errorf("one or more endpoint candidates are invalid")
		}
		return nil
	},
}

func init() {
	endpointTestCmd.Flags().BoolVar(&endpointTestJSON, "json", false, "emit results as JSON")
	endpointCmd.AddCommand(endpointTestCmd)
}
