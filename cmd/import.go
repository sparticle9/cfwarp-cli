package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var importFile string

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing WARP credentials from a file",
	Long: `import reads previously generated WARP account data from a JSON file
and stores it in local state, skipping re-registration.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		// TODO(task-3): implement import logic
		fmt.Fprintf(c.OutOrStdout(), "import: not yet implemented (file: %s)\n", importFile)
		return nil
	},
}

func init() {
	importCmd.Flags().StringVarP(&importFile, "file", "f", "", "path to account JSON file (required)")
	_ = importCmd.MarkFlagRequired("file")
}
