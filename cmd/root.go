package cmd

import (
	"fmt"
	"os"
	"runtime"

	_ "github.com/nexus/cfwarp-cli/internal/backend/native"
	_ "github.com/nexus/cfwarp-cli/internal/backend/singbox"
	"github.com/spf13/cobra"
)

var globalStateDir string

var rootCmd = &cobra.Command{
	Use:   "cfwarp-cli",
	Short: "CLI toolkit for Cloudflare WARP-backed proxy",
	Long: `cfwarp-cli manages Cloudflare WARP connectivity via selectable proxy
backends, exposing an explicit proxy (SOCKS5/HTTP) on Linux/Docker.`,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&globalStateDir, "state-dir", "", "override config/state root directory")
	rootCmd.AddCommand(
		registerCmd,
		importCmd,
		registrationCmd,
		addressCmd,
		transportCmd,
		modeCmd,
		statsCmd,
		renderCmd,
		upCmd,
		downCmd,
		connectCmd,
		disconnectCmd,
		statusCmd,
		unlockCmd,
		rotateCmd,
		endpointCmd,
		versionCmd,
		serviceCmd,
	)
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// platformCheck returns an error when the OS is unsupported.
func platformCheck() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("unsupported platform %q: cfwarp-cli requires Linux (or a Linux container)", runtime.GOOS)
	}
	return nil
}
