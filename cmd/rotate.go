package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/unlock"
	"github.com/spf13/cobra"
)

var rotateAttempts int
var rotateServices []string
var rotateTimeout time.Duration
var rotateSettleTime time.Duration
var rotateMasque bool

var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate the WARP registration and optionally validate unlock targets",
	Long: `rotate re-registers the local WARP device to obtain new assigned addresses.

If unlock services are requested, cfwarp-cli will bring the configured backend up,
run the requested checks through the local proxy, and retry until a passing set of
results is found or the attempt budget is exhausted.`,
	RunE: func(c *cobra.Command, args []string) error {
		if err := platformCheck(); err != nil {
			return err
		}
		if rotateAttempts < 1 {
			return fmt.Errorf("attempts must be >= 1")
		}

		dirs := state.Resolve(globalStateDir, "")
		if err := dirs.MkdirAll(); err != nil {
			return fmt.Errorf("prepare state directories: %w", err)
		}
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return fmt.Errorf("resolve settings: %w", err)
		}
		if err := platformCheckSettings(sett); err != nil {
			return err
		}

		services, err := unlock.NormalizeServices(rotateServices)
		if err != nil {
			return err
		}

		previousAccount, previousAccountErr := state.LoadAccount(dirs)
		hadPreviousAccount := previousAccountErr == nil
		if previousAccountErr != nil && !errors.Is(previousAccountErr, state.ErrNotFound) {
			return fmt.Errorf("load current account: %w", previousAccountErr)
		}
		if hadPreviousAccount {
			if status, err := ensureRotationAccount(dirs, previousAccount, sett); err == nil {
				fmt.Fprintf(c.OutOrStdout(), "Current %s\n", formatRotationNovelty(status))
			}
		}

		wasRunning := false
		if rt, err := state.LoadRuntime(dirs); err == nil {
			wasRunning = orchestrator.IsRuntimeActive(rt)
		}
		if err := stopBackendRuntime(c.Context(), c.OutOrStdout(), dirs); err != nil {
			return err
		}

		masqueEnroll := rotateMasque || sett.Transport == state.TransportMasque || (hadPreviousAccount && previousAccount.Masque != nil)
		keepRunning := wasRunning
		useTemporaryBackend := len(services) > 0

		var lastErr error
		for attempt := 1; attempt <= rotateAttempts; attempt++ {
			fmt.Fprintf(c.OutOrStdout(), "Rotate attempt %d/%d…\n", attempt, rotateAttempts)
			if err := registerAccount(c, dirs, true, masqueEnroll); err != nil {
				lastErr = err
				fmt.Fprintf(c.ErrOrStderr(), "registration failed: %v\n", err)
				continue
			}

			acc, err := state.LoadAccount(dirs)
			if err != nil {
				lastErr = fmt.Errorf("load rotated account: %w", err)
				fmt.Fprintf(c.ErrOrStderr(), "%v\n", lastErr)
				continue
			}
			fmt.Fprintf(c.OutOrStdout(), "wireguard_ipv4=%s wireguard_ipv6=%s\n", acc.WARPIPV4, acc.WARPIPV6)
			if acc.Masque != nil {
				fmt.Fprintf(c.OutOrStdout(), "masque_ipv4=%s masque_ipv6=%s\n", acc.Masque.IPv4, acc.Masque.IPv6)
			}
			status, statusErr := rememberRotationAccount(dirs, acc, sett, time.Now().UTC())
			if statusErr != nil {
				lastErr = statusErr
				fmt.Fprintf(c.ErrOrStderr(), "rotation memory update failed: %v\n", statusErr)
				continue
			}
			fmt.Fprintf(c.OutOrStdout(), "%s\n", formatRotationNovelty(status))
			if !status.Qualifies {
				lastErr = fmt.Errorf("rotation did not produce a new address assignment under distinctness=%s", status.Distinctness)
				fmt.Fprintf(c.ErrOrStderr(), "%v\n", lastErr)
				continue
			}

			shouldStart := keepRunning || useTemporaryBackend
			if !shouldStart {
				return nil
			}

			if _, err := startBackendRuntime(c.Context(), c.OutOrStdout(), dirs, sett, false); err != nil {
				lastErr = err
				fmt.Fprintf(c.ErrOrStderr(), "start backend failed: %v\n", err)
				_ = stopBackendRuntime(c.Context(), c.OutOrStdout(), dirs)
				continue
			}
			if !waitForProxyReachable(sett, rotateSettleTime) {
				lastErr = fmt.Errorf("proxy did not become reachable within %s", rotateSettleTime)
				fmt.Fprintf(c.ErrOrStderr(), "%v\n", lastErr)
				_ = stopBackendRuntime(c.Context(), c.OutOrStdout(), dirs)
				continue
			}

			if len(services) == 0 {
				return nil
			}

			results, err := probeUnlockResults(c.Context(), sett, services, rotateTimeout)
			if err != nil {
				lastErr = err
				fmt.Fprintf(c.ErrOrStderr(), "unlock checks failed: %v\n", err)
				_ = stopBackendRuntime(c.Context(), c.OutOrStdout(), dirs)
				continue
			}
			for _, result := range results {
				line := fmt.Sprintf("%s: %s", result.Service, result.Status)
				if result.Region != "" {
					line += fmt.Sprintf(" (%s)", result.Region)
				}
				if result.Detail != "" {
					line += fmt.Sprintf(" — %s", result.Detail)
				}
				fmt.Fprintln(c.OutOrStdout(), line)
			}
			if err := requireUnlockResults(results); err == nil {
				if !keepRunning {
					_ = stopBackendRuntime(c.Context(), c.OutOrStdout(), dirs)
				}
				return nil
			} else {
				lastErr = err
				fmt.Fprintf(c.ErrOrStderr(), "%v\n", err)
				_ = stopBackendRuntime(c.Context(), c.OutOrStdout(), dirs)
			}
		}

		if hadPreviousAccount {
			fmt.Fprintln(c.OutOrStdout(), "Restoring previous account after failed rotation attempts…")
			if err := state.SaveAccount(dirs, previousAccount, true); err != nil {
				return fmt.Errorf("restore previous account: %w", err)
			}
			if keepRunning {
				if _, err := startBackendRuntime(c.Context(), c.OutOrStdout(), dirs, sett, false); err != nil {
					return fmt.Errorf("restore backend with previous account: %w", err)
				}
			}
		}
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("rotation failed")
	},
}

func init() {
	rotateCmd.Flags().IntVar(&rotateAttempts, "attempts", 3, "maximum rotation attempts")
	rotateCmd.Flags().StringSliceVar(&rotateServices, "service", nil, "unlock checks to use as the rotation driver (gemini, chatgpt/openai, claude, netflix, youtube)")
	rotateCmd.Flags().DurationVar(&rotateTimeout, "timeout", 15*time.Second, "per-service HTTP timeout")
	rotateCmd.Flags().DurationVar(&rotateSettleTime, "settle-time", 12*time.Second, "how long to wait for the local proxy to become reachable after each rotation")
	rotateCmd.Flags().BoolVar(&rotateMasque, "masque", false, "also enroll MASQUE state while rotating the account")
}
