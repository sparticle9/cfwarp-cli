package cmd

import (
	"fmt"
	"os"
	"runtime"

	_ "github.com/nexus/cfwarp-cli/internal/backend/native"
	_ "github.com/nexus/cfwarp-cli/internal/backend/singbox"
	"github.com/nexus/cfwarp-cli/internal/state"
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
		daemonCmd,
		validateCmd,
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

// platformCheck returns an error when the current host is outside the broad
// control-plane support matrix.
func platformCheck() error {
	switch {
	case runtime.GOOS == "linux":
		return nil
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return nil
	default:
		return fmt.Errorf("unsupported platform %q/%q: cfwarp-cli currently supports Linux, plus macOS on Apple Silicon for selected workflows", runtime.GOOS, runtime.GOARCH)
	}
}

// platformCheckSettings enforces the runtime connectivity matrix for the
// resolved settings on the current platform.
func platformCheckSettings(sett state.Settings) error {
	if err := platformCheck(); err != nil {
		return err
	}
	if runtime.GOOS == "linux" {
		return nil
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if sett.RuntimeFamily == state.RuntimeFamilyLegacy && sett.Transport == state.TransportWireGuard {
			return nil
		}
		return fmt.Errorf("unsupported runtime on %s/%s: macOS on Apple Silicon currently supports the WireGuard legacy lane only", runtime.GOOS, runtime.GOARCH)
	}
	return nil
}
