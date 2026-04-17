package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/nexus/cfwarp-cli/internal/settings"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var transportCmd = &cobra.Command{Use: "transport", Short: "Inspect or update transport selection"}
var modeCmd = &cobra.Command{Use: "mode", Short: "Inspect or update service mode"}
var statsCmd = &cobra.Command{Use: "stats", Short: "Show runtime selection and current status snapshot"}

var transportSetRuntimeFamily string
var transportJSON bool
var statsJSON bool

var transportShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the resolved transport configuration",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		if transportJSON {
			return printAnyJSON(map[string]any{
				"backend":        sett.Backend,
				"runtime_family": sett.RuntimeFamily,
				"transport":      sett.Transport,
			}, json.NewEncoder(c.OutOrStdout()))
		}
		fmt.Fprintf(c.OutOrStdout(), "backend:        %s\n", sett.Backend)
		fmt.Fprintf(c.OutOrStdout(), "runtime_family: %s\n", sett.RuntimeFamily)
		fmt.Fprintf(c.OutOrStdout(), "transport:      %s\n", sett.Transport)
		return nil
	},
}

var transportSetCmd = &cobra.Command{
	Use:   "set <transport>",
	Short: "Persist the selected transport",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := loadMutableSettings(dirs)
		if err != nil {
			return err
		}
		sett.Transport = args[0]
		if transportSetRuntimeFamily != "" {
			sett.RuntimeFamily = transportSetRuntimeFamily
		} else {
			switch args[0] {
			case state.TransportMasque:
				sett.RuntimeFamily = state.RuntimeFamilyNative
			case state.TransportWireGuard:
				sett.RuntimeFamily = state.RuntimeFamilyLegacy
			}
		}
		sett.Normalize()
		if err := saveSettingsValidated(dirs, sett); err != nil {
			return err
		}
		fmt.Fprintf(c.OutOrStdout(), "Transport set to %s (%s)\n", sett.Transport, sett.RuntimeFamily)
		return nil
	},
}

var modeShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the resolved service mode",
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		return printAnyJSON(map[string]any{
			"mode":          sett.Mode,
			"listen_host":   sett.ListenHost,
			"listen_port":   sett.ListenPort,
			"proxy_enabled": sett.Mode == state.ModeHTTP || sett.Mode == state.ModeSocks5,
		}, json.NewEncoder(c.OutOrStdout()))
	},
}

var modeSetCmd = &cobra.Command{
	Use:   "set <mode>",
	Short: "Persist the selected service mode",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := loadMutableSettings(dirs)
		if err != nil {
			return err
		}
		sett.Mode = args[0]
		sett.ProxyMode = args[0]
		if len(sett.Access) == 0 {
			sett.Access = []state.AccessConfig{{Type: args[0], ListenHost: sett.ListenHost, ListenPort: sett.ListenPort, Username: sett.ProxyUsername, Password: sett.ProxyPassword}}
		} else {
			sett.Access[0].Type = args[0]
		}
		sett.Normalize()
		if err := saveSettingsValidated(dirs, sett); err != nil {
			return err
		}
		fmt.Fprintf(c.OutOrStdout(), "Mode set to %s\n", sett.Mode)
		return nil
	},
}

var statsCmdRun = &cobra.Command{
	Use:    "show",
	Short:  "Show a compact status snapshot",
	Hidden: true,
}

func init() {
	transportShowCmd.Flags().BoolVar(&transportJSON, "json", false, "emit transport settings as JSON")
	transportSetCmd.Flags().StringVar(&transportSetRuntimeFamily, "runtime-family", "", "override runtime family when setting the transport")
	transportCmd.AddCommand(transportShowCmd, transportSetCmd)
	modeCmd.AddCommand(modeShowCmd, modeSetCmd)
	statsCmd.Flags().BoolVar(&statsJSON, "json", true, "emit stats as JSON")
	statsCmd.RunE = func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")
		sett, err := resolveSettings(c, dirs)
		if err != nil {
			return err
		}
		rt, _ := state.LoadRuntime(dirs)
		payload := map[string]any{
			"backend":        sett.Backend,
			"runtime_family": sett.RuntimeFamily,
			"transport":      sett.Transport,
			"mode":           sett.Mode,
			"listen_host":    sett.ListenHost,
			"listen_port":    sett.ListenPort,
			"phase":          rt.Phase,
			"pid":            rt.PID,
			"started_at":     rt.StartedAt,
		}
		enc := json.NewEncoder(c.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
}

func loadMutableSettings(dirs state.Dirs) (state.Settings, error) {
	sett, err := state.LoadSettings(dirs)
	if err == nil {
		return sett, nil
	}
	if err == state.ErrNotFound {
		return state.DefaultSettings(), nil
	}
	return state.Settings{}, err
}

func saveSettingsValidated(dirs state.Dirs, sett state.Settings) error {
	if err := settings.Validate(sett); err != nil {
		return err
	}
	if err := dirs.MkdirAll(); err != nil {
		return err
	}
	return state.SaveSettings(dirs, sett)
}
