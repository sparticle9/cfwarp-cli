package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cfwarp-cli",
	Short: "CLI toolkit for Cloudflare WARP-backed proxy",
	Long: `cfwarp-cli manages Cloudflare WARP connectivity via a userspace
WireGuard backend, exposing an explicit proxy (SOCKS5/HTTP) on Linux/Docker.`,
	SilenceUsage: true,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		registerCmd,
		importCmd,
		renderCmd,
		upCmd,
		downCmd,
		statusCmd,
		endpointCmd,
		versionCmd,
	)
}

// platformCheck returns an error when the OS is unsupported.
func platformCheck() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("unsupported platform %q: cfwarp-cli requires Linux (or a Linux container)", runtime.GOOS)
	}
	return nil
}
