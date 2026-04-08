package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

var statusJSON bool
var statusTrace bool

// StatusReport is the machine-readable status output.
type StatusReport struct {
	AccountConfigured  bool   `json:"account_configured"`
	BackendRunning     bool   `json:"backend_running"`
	LocalReachable     bool   `json:"local_reachable"`
	PID                int    `json:"pid,omitempty"`
	Backend            string `json:"backend,omitempty"`
	RuntimeFamily      string `json:"runtime_family,omitempty"`
	Transport          string `json:"transport,omitempty"`
	Mode               string `json:"mode,omitempty"`
	Phase              string `json:"phase,omitempty"`
	ListenAddr         string `json:"listen_addr,omitempty"`
	StartedAt          string `json:"started_at,omitempty"`
	LastError          string `json:"last_error,omitempty"`
	LastTransportError string `json:"last_transport_error,omitempty"`
	WARPVerified       *bool  `json:"warp_verified,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report proxy configuration, process, and health state",
	Long: `status checks whether account state is configured, the backend process is
running, and the local proxy port is reachable.

Use --json for machine-readable output.
Use --trace to also probe cdn-cgi/trace through the proxy (requires live network).`,
	RunE: func(c *cobra.Command, args []string) error {
		dirs := state.Resolve(globalStateDir, "")

		report := StatusReport{}

		// 1. Account configured?
		_, accErr := state.LoadAccount(dirs)
		report.AccountConfigured = accErr == nil

		// 2. Runtime state + process liveness.
		var listenAddr string
		rt, rtErr := state.LoadRuntime(dirs)
		if rtErr == nil {
			report.PID = rt.PID
			report.Backend = rt.Backend
			report.RuntimeFamily = rt.RuntimeFamily
			report.Transport = rt.Transport
			report.Mode = rt.Mode
			report.Phase = rt.Phase
			report.LastError = rt.LastError
			report.LastTransportError = rt.LastTransportError
			if !rt.StartedAt.IsZero() {
				report.StartedAt = rt.StartedAt.Format(time.RFC3339)
			}
		}

		// 3. Local reachability — need listen host:port from settings.
		sett, settErr := resolveSettings(c, dirs)
		if settErr == nil {
			listenAddr = fmt.Sprintf("%s:%d", sett.ListenHost, sett.ListenPort)
			report.ListenAddr = listenAddr
			report.LocalReachable = health.ProbeLocal(sett.ListenHost, sett.ListenPort, 0)
		}

		// 4. Backend-specific status.
		if rtErr == nil && settErr == nil {
			b, err := runtimeBackend(rt)
			if err != nil {
				if report.LastError == "" {
					report.LastError = err.Error()
				}
			} else {
				st, statusErr := b.Status(c.Context(), runtimeInfo(rt), sett)
				if statusErr != nil {
					if report.LastError == "" {
						report.LastError = statusErr.Error()
					}
				} else {
					report.BackendRunning = st.Running
					report.LocalReachable = st.LocalReachable
					report.Phase = orchestrator.DerivePhase(rt, st.Running, st.LocalReachable)
					if report.LastError == "" {
						report.LastError = st.LastError
					}
				}
			}
		} else if rtErr == nil {
			report.BackendRunning = orchestrator.IsRuntimeActive(rt)
			report.Phase = orchestrator.DerivePhase(rt, report.BackendRunning, report.LocalReachable)
		}

		// 5. Optional cdn-cgi/trace probe.
		if statusTrace && settErr == nil {
			proxyAddr := fmt.Sprintf("%s:%d", sett.ListenHost, sett.ListenPort)
			result, err := health.ProbeTrace(c.Context(), sett.Mode, proxyAddr, sett.ProxyUsername, sett.ProxyPassword)
			if err != nil {
				errMsg := fmt.Sprintf("trace probe failed: %v", err)
				if statusJSON {
					// embed error in report
					report.LastError = errMsg
				} else {
					fmt.Fprintf(c.ErrOrStderr(), "warning: %s\n", errMsg)
				}
			} else {
				warpOn := result.WARPOn
				report.WARPVerified = &warpOn
			}
		}

		if statusJSON {
			return printJSON(c, report)
		}
		printHuman(c, report, accErr, rtErr)
		return nil
	},
}

func printJSON(c *cobra.Command, r StatusReport) error {
	enc := json.NewEncoder(c.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func printHuman(c *cobra.Command, r StatusReport, accErr, rtErr error) {
	tick := func(ok bool) string {
		if ok {
			return "✓"
		}
		return "✗"
	}

	// Account
	if r.AccountConfigured {
		fmt.Fprintf(c.OutOrStdout(), "Account:  configured %s\n", tick(true))
	} else {
		fmt.Fprintf(c.OutOrStdout(), "Account:  not configured %s", tick(false))
		if accErr != nil && !errors.Is(accErr, state.ErrNotFound) {
			fmt.Fprintf(c.OutOrStdout(), " (%v)", accErr)
		}
		fmt.Fprintln(c.OutOrStdout())
	}

	// Backend process
	if errors.Is(rtErr, state.ErrNotFound) {
		fmt.Fprintf(c.OutOrStdout(), "Backend:  not started %s\n", tick(false))
	} else if r.BackendRunning {
		fmt.Fprintf(c.OutOrStdout(), "Backend:  %s running %s (PID %d, started %s, phase %s)\n",
			r.Backend, tick(true), r.PID, r.StartedAt, r.Phase)
	} else {
		fmt.Fprintf(c.OutOrStdout(), "Backend:  not running %s", tick(false))
		if r.LastError != "" {
			fmt.Fprintf(c.OutOrStdout(), " — last error: %s", r.LastError)
		}
		if r.Phase != "" {
			fmt.Fprintf(c.OutOrStdout(), " — phase: %s", r.Phase)
		}
		fmt.Fprintln(c.OutOrStdout())
	}

	// Local proxy
	if r.ListenAddr != "" {
		if r.LocalReachable {
			fmt.Fprintf(c.OutOrStdout(), "Proxy:    reachable at %s %s\n", r.ListenAddr, tick(true))
		} else {
			fmt.Fprintf(c.OutOrStdout(), "Proxy:    not reachable at %s %s\n", r.ListenAddr, tick(false))
		}
	}

	if r.LastTransportError != "" {
		fmt.Fprintf(c.OutOrStdout(), "Transport: last error: %s\n", r.LastTransportError)
	}

	// WARP trace (optional)
	if r.WARPVerified != nil {
		if *r.WARPVerified {
			fmt.Fprintf(c.OutOrStdout(), "WARP:     verified (warp=on) %s\n", tick(true))
		} else {
			fmt.Fprintf(c.OutOrStdout(), "WARP:     not verified (warp=off) %s\n", tick(false))
		}
	}
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "emit status as JSON")
	statusCmd.Flags().BoolVar(&statusTrace, "trace", false, "probe cdn-cgi/trace through the proxy (requires live network)")
}
