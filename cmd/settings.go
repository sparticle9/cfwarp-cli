package cmd

import (
	"github.com/nexus/cfwarp-cli/internal/settings"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/spf13/cobra"
)

// settingsFlags holds values bound to the persistent settings flags on rootCmd.
// Cobra populates these; resolveSettings inspects Changed() to build Overrides.
var settingsFlags struct {
	backend          string
	listenHost       string
	listenPort       int
	proxyMode        string
	proxyUsername    string
	proxyPassword    string
	endpointOverride string
	logLevel         string
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&settingsFlags.backend, "backend", "", "backend to use (default: singbox-wireguard)")
	pf.StringVar(&settingsFlags.listenHost, "listen-host", "", "proxy listen address (default: 0.0.0.0)")
	pf.IntVar(&settingsFlags.listenPort, "listen-port", 0, "proxy listen port (default: 1080)")
	pf.StringVar(&settingsFlags.proxyMode, "proxy-mode", "", "proxy mode: socks5 or http (default: socks5)")
	pf.StringVar(&settingsFlags.proxyUsername, "proxy-username", "", "proxy auth username")
	pf.StringVar(&settingsFlags.proxyPassword, "proxy-password", "", "proxy auth password")
	pf.StringVar(&settingsFlags.endpointOverride, "endpoint", "", "WireGuard peer endpoint override (host:port)")
	pf.StringVar(&settingsFlags.logLevel, "log-level", "", "log level: debug, info, warn, error (default: info)")
}

// resolveSettings builds the final Settings for the given command by applying
// flag > env > persisted file > defaults precedence.
// Only flags that were explicitly set by the user (Changed) are included as overrides.
func resolveSettings(c *cobra.Command, dirs state.Dirs) (state.Settings, error) {
	changed := func(name string) bool { return c.Root().PersistentFlags().Changed(name) }

	var o settings.Overrides
	if changed("backend") {
		o.Backend = &settingsFlags.backend
	}
	if changed("listen-host") {
		o.ListenHost = &settingsFlags.listenHost
	}
	if changed("listen-port") {
		o.ListenPort = &settingsFlags.listenPort
	}
	if changed("proxy-mode") {
		o.ProxyMode = &settingsFlags.proxyMode
	}
	if changed("proxy-username") {
		o.ProxyUsername = &settingsFlags.proxyUsername
	}
	if changed("proxy-password") {
		o.ProxyPassword = &settingsFlags.proxyPassword
	}
	if changed("endpoint") {
		o.EndpointOverride = &settingsFlags.endpointOverride
	}
	if changed("log-level") {
		o.LogLevel = &settingsFlags.logLevel
	}

	return settings.Load(dirs, o)
}
