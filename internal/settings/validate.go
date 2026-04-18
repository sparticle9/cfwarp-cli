package settings

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nexus/cfwarp-cli/internal/state"
)

var validRuntimeFamilies = map[string]bool{
	state.RuntimeFamilyLegacy: true,
	state.RuntimeFamilyNative: true,
}

var validTransports = map[string]bool{
	state.TransportWireGuard: true,
	state.TransportMasque:    true,
}

var validModes = map[string]bool{
	state.ModeSocks5: true,
	state.ModeHTTP:   true,
	state.ModeTUN:    true,
}

var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

var validCapProbes = map[string]bool{
	"internet": true,
	"warp":     true,
	"gemini":   true,
	"chatgpt":  true,
}

var validDNSModes = map[string]bool{
	"local": true,
	"udp":   true,
	"https": true,
}

var validDNSStrategies = map[string]bool{
	"prefer_ipv4": true,
	"prefer_ipv6": true,
	"ipv4_only":   true,
	"ipv6_only":   true,
}

var validRotationDistinctness = map[string]bool{
	state.RotationDistinctnessEither: true,
	state.RotationDistinctnessIPv4:   true,
	state.RotationDistinctnessIPv6:   true,
	state.RotationDistinctnessBoth:   true,
}

// Validate checks s for invalid or inconsistent values.
// It returns the first error found, naming the offending field.
func Validate(s state.Settings) error {
	s.Normalize()

	if !validRuntimeFamilies[s.RuntimeFamily] {
		return fmt.Errorf("invalid runtime_family %q: must be one of %v", s.RuntimeFamily, keys(validRuntimeFamilies))
	}
	if !validTransports[s.Transport] {
		return fmt.Errorf("invalid transport %q: must be one of %v", s.Transport, keys(validTransports))
	}
	if !validModes[s.Mode] {
		return fmt.Errorf("invalid mode %q: must be one of %v", s.Mode, keys(validModes))
	}
	if !validLogLevels[s.LogLevel] {
		return fmt.Errorf("invalid log_level %q: must be one of %v", s.LogLevel, keys(validLogLevels))
	}
	if s.EndpointOverride != "" {
		if err := ValidateEndpoint(s.EndpointOverride); err != nil {
			return fmt.Errorf("invalid endpoint_override: %w", err)
		}
	}
	if len(s.Access) == 0 {
		return fmt.Errorf("at least one access entry is required")
	}
	seen := map[string]bool{}
	for i, access := range s.Access {
		access.Type = strings.ToLower(strings.TrimSpace(access.Type))
		if !validModes[access.Type] {
			return fmt.Errorf("invalid access[%d].type %q: must be one of %v", i, access.Type, keys(validModes))
		}
		if access.Type == state.ModeTUN {
			return fmt.Errorf("access[%d].type %q is not yet supported", i, access.Type)
		}
		if access.ListenHost == "" {
			return fmt.Errorf("access[%d].listen_host is required", i)
		}
		if access.ListenPort < 1 || access.ListenPort > 65535 {
			return fmt.Errorf("invalid access[%d].listen_port %d: must be 1–65535", i, access.ListenPort)
		}
		if (access.Username == "") != (access.Password == "") {
			return fmt.Errorf("access[%d].username and access[%d].password must both be set or both be empty", i, i)
		}
		key := net.JoinHostPort(access.ListenHost, strconv.Itoa(access.ListenPort))
		if seen[key] {
			return fmt.Errorf("duplicate access listener %s", key)
		}
		seen[key] = true
	}

	supportedProxyModes := []string{state.ModeSocks5, state.ModeHTTP}
	switch s.RuntimeFamily {
	case state.RuntimeFamilyLegacy:
		if s.Transport != state.TransportWireGuard {
			return fmt.Errorf("invalid transport %q for runtime_family %q: must be %q", s.Transport, s.RuntimeFamily, state.TransportWireGuard)
		}
		if s.Mode != state.ModeSocks5 && s.Mode != state.ModeHTTP {
			return fmt.Errorf("invalid mode %q for runtime_family %q: must be one of %v", s.Mode, s.RuntimeFamily, supportedProxyModes)
		}
	case state.RuntimeFamilyNative:
		if s.Transport != state.TransportMasque {
			return fmt.Errorf("invalid transport %q for runtime_family %q: must be %q", s.Transport, s.RuntimeFamily, state.TransportMasque)
		}
		if s.Mode != state.ModeSocks5 && s.Mode != state.ModeHTTP {
			return fmt.Errorf("invalid mode %q for runtime_family %q: must be one of %v", s.Mode, s.RuntimeFamily, supportedProxyModes)
		}
	}

	if s.Caps != nil {
		if s.Caps.IntervalSeconds <= 0 {
			return fmt.Errorf("invalid caps.interval_seconds %d: must be > 0", s.Caps.IntervalSeconds)
		}
		for i, check := range s.Caps.Checks {
			if !validCapProbes[check.Probe] {
				return fmt.Errorf("invalid caps.checks[%d].probe %q: must be one of %v", i, check.Probe, keys(validCapProbes))
			}
			if check.TimeoutSeconds <= 0 {
				return fmt.Errorf("invalid caps.checks[%d].timeout_seconds %d: must be > 0", i, check.TimeoutSeconds)
			}
		}
	}

	if s.DNS != nil {
		if !validDNSModes[s.DNS.Mode] {
			return fmt.Errorf("invalid dns.mode %q: must be one of %v", s.DNS.Mode, keys(validDNSModes))
		}
		if s.DNS.Strategy != "" && !validDNSStrategies[s.DNS.Strategy] {
			return fmt.Errorf("invalid dns.strategy %q: must be one of %v", s.DNS.Strategy, keys(validDNSStrategies))
		}
		switch s.DNS.Mode {
		case "local":
			if s.DNS.Server != "" {
				return fmt.Errorf("dns.server is not supported when dns.mode=%q", s.DNS.Mode)
			}
			if s.DNS.ServerPort != 0 {
				return fmt.Errorf("dns.server_port is not supported when dns.mode=%q", s.DNS.Mode)
			}
			if s.DNS.Path != "" {
				return fmt.Errorf("dns.path is not supported when dns.mode=%q", s.DNS.Mode)
			}
		case "udp", "https":
			if strings.TrimSpace(s.DNS.Server) == "" {
				return fmt.Errorf("dns.server is required when dns.mode=%q", s.DNS.Mode)
			}
			if s.DNS.ServerPort < 0 || s.DNS.ServerPort > 65535 {
				return fmt.Errorf("invalid dns.server_port %d: must be 1–65535 when set", s.DNS.ServerPort)
			}
			if s.DNS.Mode == "udp" && s.DNS.Path != "" {
				return fmt.Errorf("dns.path is only supported when dns.mode=%q", "https")
			}
		}
	}

	if s.Rotation != nil {
		if s.Rotation.Distinctness != "" && !validRotationDistinctness[s.Rotation.Distinctness] {
			return fmt.Errorf("invalid rotation.distinctness %q: must be one of %v", s.Rotation.Distinctness, keys(validRotationDistinctness))
		}
		if s.Rotation.Enabled {
			if s.Rotation.MaxAttemptsPerIncident <= 0 {
				return fmt.Errorf("invalid rotation.max_attempts_per_incident %d: must be > 0", s.Rotation.MaxAttemptsPerIncident)
			}
			if s.Rotation.SettleTimeSeconds <= 0 {
				return fmt.Errorf("invalid rotation.settle_time_seconds %d: must be > 0", s.Rotation.SettleTimeSeconds)
			}
			if s.Rotation.HistorySize <= 0 {
				return fmt.Errorf("invalid rotation.history_size %d: must be > 0", s.Rotation.HistorySize)
			}
		}
		if s.Rotation.CooldownSeconds < 0 {
			return fmt.Errorf("invalid rotation.cooldown_seconds %d: must be >= 0", s.Rotation.CooldownSeconds)
		}
		if s.Rotation.HistorySize < 0 {
			return fmt.Errorf("invalid rotation.history_size %d: must be >= 0", s.Rotation.HistorySize)
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
