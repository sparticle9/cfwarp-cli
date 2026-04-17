package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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
	Short: "Test Gemini / ChatGPT / Claude availability through the configured proxy",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		services := unlockServices
		if len(services) == 0 {
			services = []string{unlock.ServiceGemini, unlock.ServiceChatGPT}
		}
		results, err := unlock.ProbeMany(c.Context(), unlock.Config{
			ProxyMode: sett.Mode,
			ProxyAddr: fmt.Sprintf("%s:%d", sett.ListenHost, sett.ListenPort),
			Username:  sett.ProxyUsername,
			Password:  sett.ProxyPassword,
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
	return unlock.ProbeMany(ctx, unlock.Config{
		ProxyMode: sett.Mode,
		ProxyAddr: fmt.Sprintf("%s:%d", sett.ListenHost, sett.ListenPort),
		Username:  sett.ProxyUsername,
		Password:  sett.ProxyPassword,
		Timeout:   timeout,
	}, services)
}

func init() {
	unlockTestCmd.Flags().StringSliceVar(&unlockServices, "service", nil, "unlock checks to run (gemini, chatgpt/openai, claude)")
	unlockTestCmd.Flags().BoolVar(&unlockJSON, "json", false, "emit unlock results as JSON")
	unlockTestCmd.Flags().DurationVar(&unlockTimeout, "timeout", 15*time.Second, "per-service HTTP timeout")
	unlockCmd.AddCommand(unlockTestCmd)
}
