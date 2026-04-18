package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "github.com/nexus/cfwarp-cli/internal/backend/native"
	_ "github.com/nexus/cfwarp-cli/internal/backend/singbox"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh]",
	Short:     "Generate shell completion script",
	Long:      "Generate completion scripts for bash and zsh.",
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: []string{"bash", "zsh"},
	RunE: func(cmd *cobra.Command, args []string) error {
		initFile, err := cmd.Flags().GetString("init-file")
		if err != nil {
			return err
		}
		return writeShellCompletion(cmd, args[0], initFile)
	},
	Example: `  # bash
	cfwarp-cli completion bash | source

  # zsh
  cfwarp-cli completion zsh | source

  # write to a custom zsh startup file
  cfwarp-cli completion zsh --init-file ~/.zshrc.local`,
}

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
	completionCmd.Flags().String("init-file", "", "append generated completion snippet to an existing shell init file")
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
		completionCmd,
		versionCmd,
		serviceCmd,
	)
}

func resolveCompletionInitFile(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	expanded := os.ExpandEnv(strings.TrimSpace(raw))
	if !strings.HasPrefix(expanded, "~/") {
		return expanded, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	homeExpanded := filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
	return homeExpanded, nil
}

func writeShellCompletion(cmd *cobra.Command, shell string, initFile string) error {
	var buf bytes.Buffer
	switch shell {
	case "bash":
		if err := cmd.Root().GenBashCompletionV2(&buf, false); err != nil {
			return err
		}
	case "zsh":
		if err := cmd.Root().GenZshCompletion(&buf); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported shell %q: supported shells are bash and zsh", shell)
	}

	if initFile == "" {
		_, err := cmd.OutOrStdout().Write(buf.Bytes())
		return err
	}

	resolvedInitFile, err := resolveCompletionInitFile(initFile)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(resolvedInitFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString("\n# cfwarp-cli completion (" + shell + ")\n"); err != nil {
		return err
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		return err
	}
	if _, err := f.WriteString("\n"); err != nil {
		return err
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Appended %s completion to %s\n", shell, resolvedInitFile)
	return err
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
