package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/unlock"
	"github.com/spf13/cobra"
)

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Run lightweight region/unlock checks through the proxy",
}

var unlockServices []string
var unlockJSON bool
var unlockTimeout time.Duration

var unlockTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test unlock targets through the configured proxy (gemini, chatgpt, claude, netflix, youtube)",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		target, err := probeTargetFromSettings(sett)
		if err != nil {
			return err
		}
		services := unlockServices
		if len(services) == 0 {
			services = []string{unlock.ServiceGemini, unlock.ServiceChatGPT}
		}
		results, err := unlock.ProbeMany(c.Context(), unlock.Config{
			ProxyMode: target.Type,
			ProxyAddr: target.Address,
			Username:  target.Username,
			Password:  target.Password,
			Timeout:   unlockTimeout,
		}, services)
		if err != nil {
			return err
		}
		if unlockJSON {
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}
		for _, result := range results {
			line := fmt.Sprintf("%s: %s", result.Service, result.Status)
			if result.Region != "" {
				line += fmt.Sprintf(" (%s)", result.Region)
			}
			if result.Detail != "" {
				line += fmt.Sprintf(" — %s", result.Detail)
			}
			if len(result.Supplement) > 0 {
				keys := make([]string, 0, len(result.Supplement))
				for key := range result.Supplement {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				bits := make([]string, 0, len(keys))
				for _, key := range keys {
					bits = append(bits, fmt.Sprintf("%s=%s", key, result.Supplement[key]))
				}
				line += fmt.Sprintf(" — %s", strings.Join(bits, "; "))
			}
			fmt.Fprintln(c.OutOrStdout(), line)
		}
		return requireUnlockResults(results)
	},
}

func requireUnlockResults(results []unlock.Result) error {
	failed := make([]string, 0)
	for _, result := range results {
		if result.OK {
			continue
		}
		failed = append(failed, fmt.Sprintf("%s=%s", result.Service, result.Status))
	}
	if len(failed) == 0 {
		return nil
	}
	return fmt.Errorf("unlock checks failed: %s", strings.Join(failed, ", "))
}

func probeUnlockResults(ctx context.Context, sett state.Settings, services []string, timeout time.Duration) ([]unlock.Result, error) {
	target, err := probeTargetFromSettings(sett)
	if err != nil {
		return nil, err
	}
	return unlock.ProbeMany(ctx, unlock.Config{
		ProxyMode: target.Type,
		ProxyAddr: target.Address,
		Username:  target.Username,
		Password:  target.Password,
		Timeout:   timeout,
	}, services)
}

func init() {
	unlockTestCmd.Flags().StringSliceVar(&unlockServices, "service", nil, "unlock checks to run (gemini, chatgpt/openai, claude, netflix, youtube)")
	unlockTestCmd.Flags().BoolVar(&unlockJSON, "json", false, "emit unlock results as JSON")
	unlockTestCmd.Flags().DurationVar(&unlockTimeout, "timeout", 15*time.Second, "per-service HTTP timeout")
	unlockCmd.AddCommand(unlockTestCmd)
}
