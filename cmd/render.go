package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/backend/singbox"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var renderOutput string

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render the backend configuration without launching the proxy",
	Long: `render generates the sing-box WireGuard configuration from stored
account state and settings, and writes it to stdout or a file (--output).`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}

		dirs := state.Resolve(globalStateDir, "")

		acc, err := state.LoadAccount(dirs)
		if err != nil {
			if errors.Is(err, state.ErrNotFound) {
				return fmt.Errorf("no account registered; run 'cfwarp-cli register' first")
			}
			return fmt.Errorf("load account: %w", err)
		}

		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return fmt.Errorf("resolve settings: %w", err)
		}

		if err := singbox.ValidatePrereqs(c.Context()); err != nil {
			return err
		}

		data, err := singbox.Render(backend.RenderInput{Account: acc, Settings: sett})
		if err != nil {
			return fmt.Errorf("render config: %w", err)
		}

		if renderOutput != "" {
			if err := os.WriteFile(renderOutput, data, 0o600); err != nil {
				return fmt.Errorf("write config to %s: %w", renderOutput, err)
			}
			fmt.Fprintf(c.OutOrStdout(), "Config written to: %s\n", renderOutput)
			return nil
		}

		_, err = fmt.Fprintln(c.OutOrStdout(), string(data))
		return err
	},
}

func init() {
	renderCmd.Flags().StringVarP(&renderOutput, "output", "o", "", "write rendered config to file instead of stdout")
}
