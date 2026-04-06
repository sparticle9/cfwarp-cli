package settings

import (
	"fmt"
	"net"
	"strconv"

	"github.com/nexus/cfwarp-cli/internal/state"
)

var validBackends = map[string]bool{
	"singbox-wireguard": true,
}

var validProxyModes = map[string]bool{
	"socks5": true,
	"http":   true,
}

var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Validate checks s for invalid or inconsistent values.
// It returns the first error found, naming the offending field.
func Validate(s state.Settings) error {
	if !validBackends[s.Backend] {
		return fmt.Errorf("invalid backend %q: must be one of %v", s.Backend, keys(validBackends))
	}
	if !validProxyModes[s.ProxyMode] {
		return fmt.Errorf("invalid proxy_mode %q: must be one of %v", s.ProxyMode, keys(validProxyModes))
	}
	if !validLogLevels[s.LogLevel] {
		return fmt.Errorf("invalid log_level %q: must be one of %v", s.LogLevel, keys(validLogLevels))
	}
	if s.ListenPort < 1 || s.ListenPort > 65535 {
		return fmt.Errorf("invalid listen_port %d: must be 1–65535", s.ListenPort)
	}
	// Auth: username and password must both be set or both be empty.
	if (s.ProxyUsername == "") != (s.ProxyPassword == "") {
		return fmt.Errorf("proxy_username and proxy_password must both be set or both be empty")
	}
	if s.EndpointOverride != "" {
		if err := ValidateEndpoint(s.EndpointOverride); err != nil {
			return fmt.Errorf("invalid endpoint_override: %w", err)
		}
	}
	return nil
}

// ValidateEndpoint checks that ep is a valid "host:port" string where port is 1–65535.
func ValidateEndpoint(ep string) error {
	host, portStr, err := net.SplitHostPort(ep)
	if err != nil {
		return fmt.Errorf("%q is not a valid host:port — %w", ep, err)
	}
	if host == "" {
		return fmt.Errorf("host portion of %q is empty", ep)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("port %q in %q is not numeric", portStr, ep)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d in %q is out of range (1–65535)", port, ep)
	}
	return nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
