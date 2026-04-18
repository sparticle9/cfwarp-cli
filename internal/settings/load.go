// Package settings resolves operator configuration from flags, env vars,
// persisted file, and defaults — in that precedence order.
package settings

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nexus/cfwarp-cli/internal/state"
)

// Overrides carries values that were explicitly supplied (flags or env vars).
// A nil pointer means "not set by this source"; a non-nil pointer overrides.
type Overrides struct {
	Backend          *string
	RuntimeFamily    *string
	Transport        *string
	ProxyMode        *string
	Mode             *string
	ListenHost       *string
	ListenPort       *int
	ProxyUsername    *string
	ProxyPassword    *string
	EndpointOverride *string
	LogLevel         *string
	SettingsFile     *string
}

func ensurePrimaryAccess(s *state.Settings) {
	if len(s.Access) == 0 {
		s.Access = []state.AccessConfig{{}}
	}
}

func syncPrimaryAccessFromLegacy(s *state.Settings) {
	ensurePrimaryAccess(s)
	primary := s.Access[0]
	if s.Mode != "" {
		primary.Type = s.Mode
	}
	if s.ListenHost != "" {
		primary.ListenHost = s.ListenHost
	}
	if s.ListenPort != 0 {
		primary.ListenPort = s.ListenPort
	}
	if s.ProxyUsername != "" || s.ProxyPassword != "" {
		primary.Username = s.ProxyUsername
		primary.Password = s.ProxyPassword
	}
	if primary.Type == "" {
		primary.Type = state.ModeSocks5
	}
	if primary.ListenHost == "" && primary.Type != state.ModeTUN {
		primary.ListenHost = "0.0.0.0"
	}
	if primary.ListenPort == 0 && primary.Type != state.ModeTUN {
		primary.ListenPort = 1080
	}
	s.Access[0] = primary
}

// Load builds the final Settings by layering:
//
//  1. Defaults (from state.DefaultSettings)
//  2. Persisted settings.json (if present)
//  3. CFWARP_* environment variables
//  4. Explicit overrides (caller-supplied, typically from CLI flags)
//
// The resolved settings are validated before being returned.
func Load(dirs state.Dirs, overrides Overrides) (state.Settings, error) {
	// 1. Start from defaults.
	s := state.DefaultSettings()

	// 2. Overlay persisted file from the durable data dir when present.
	persisted, err := state.LoadSettings(dirs)
	if err != nil && !errors.Is(err, state.ErrNotFound) {
		return s, fmt.Errorf("load persisted settings: %w", err)
	}
	if err == nil {
		applyPersisted(&s, persisted)
	}

	// 2.5. Optionally overlay a standalone settings file.
	settingsFile := os.Getenv("CFWARP_SETTINGS_FILE")
	if overrides.SettingsFile != nil {
		settingsFile = strings.TrimSpace(*overrides.SettingsFile)
	}
	if settingsFile != "" {
		external, err := state.LoadSettingsFile(settingsFile)
		if err != nil {
			return s, fmt.Errorf("load settings file %s: %w", settingsFile, err)
		}
		applyPersisted(&s, external)
	}

	// 3. Overlay env vars.
	applyEnv(&s)

	// 4. Overlay explicit overrides.
	applyOverrides(&s, overrides)
	s.Normalize()

	// 5. Validate.
	if err := Validate(s); err != nil {
		return s, err
	}
	return s, nil
}

// applyPersisted copies non-zero persisted fields onto s.
func applyPersisted(s *state.Settings, p state.Settings) {
	if p.SchemaVersion != 0 {
		s.SchemaVersion = p.SchemaVersion
	}
	if p.Backend != "" {
		s.Backend = p.Backend
	}
	if p.RuntimeFamily != "" {
		s.RuntimeFamily = p.RuntimeFamily
	}
	if p.Transport != "" {
		s.Transport = p.Transport
	}
	if p.ProxyMode != "" {
		s.ProxyMode = p.ProxyMode
		s.Mode = p.ProxyMode
	}
	if p.Mode != "" {
		s.Mode = p.Mode
	}
	if p.ListenHost != "" {
		s.ListenHost = p.ListenHost
	}
	if p.ListenPort != 0 {
		s.ListenPort = p.ListenPort
	}
	if p.ProxyUsername != "" {
		s.ProxyUsername = p.ProxyUsername
	}
	if p.ProxyPassword != "" {
		s.ProxyPassword = p.ProxyPassword
	}
	if p.EndpointOverride != "" {
		s.EndpointOverride = p.EndpointOverride
	}
	if p.StateDir != "" {
		s.StateDir = p.StateDir
	}
	if p.LogLevel != "" {
		s.LogLevel = p.LogLevel
	}
	if p.MasqueOptions != nil {
		s.MasqueOptions = p.MasqueOptions
	}
	if p.Access != nil {
		s.Access = p.Access
	}
	if p.Caps != nil {
		s.Caps = p.Caps
	}
	if p.DNS != nil {
		s.DNS = p.DNS
	}
	if p.Rotation != nil {
		s.Rotation = p.Rotation
	}
	if p.Daemon != nil {
		s.Daemon = p.Daemon
	}
}

// applyEnv reads CFWARP_* environment variables.
func applyEnv(s *state.Settings) {
	legacyAccessTouched := false
	if v := os.Getenv("CFWARP_BACKEND"); v != "" {
		s.Backend = v
	}
	if v := os.Getenv("CFWARP_RUNTIME_FAMILY"); v != "" {
		s.RuntimeFamily = strings.ToLower(v)
	}
	if v := os.Getenv("CFWARP_TRANSPORT"); v != "" {
		s.Transport = strings.ToLower(v)
	}
	if v := os.Getenv("CFWARP_LISTEN_HOST"); v != "" {
		s.ListenHost = v
		legacyAccessTouched = true
	}
	if v := os.Getenv("CFWARP_LISTEN_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			s.ListenPort = p
			legacyAccessTouched = true
		}
	}
	if v := os.Getenv("CFWARP_PROXY_MODE"); v != "" {
		normalized := strings.ToLower(v)
		s.ProxyMode = normalized
		s.Mode = normalized
		legacyAccessTouched = true
	}
	if v := os.Getenv("CFWARP_MODE"); v != "" {
		s.Mode = strings.ToLower(v)
		legacyAccessTouched = true
	}
	if v := os.Getenv("CFWARP_PROXY_USERNAME"); v != "" {
		s.ProxyUsername = v
		legacyAccessTouched = true
	}
	if v := os.Getenv("CFWARP_PROXY_PASSWORD"); v != "" {
		s.ProxyPassword = v
		legacyAccessTouched = true
	}
	if v := os.Getenv("CFWARP_ENDPOINT_OVERRIDE"); v != "" {
		s.EndpointOverride = v
	}
	if v := os.Getenv("CFWARP_LOG_LEVEL"); v != "" {
		s.LogLevel = strings.ToLower(v)
	}
	applyMasqueEnv(s)
	applyCapsEnv(s)
	applyDNSEnv(s)
	applyRotationEnv(s)
	applyDaemonEnv(s)
	if legacyAccessTouched {
		syncPrimaryAccessFromLegacy(s)
	}
}

func applyMasqueEnv(s *state.Settings) {
	var touched bool
	ensureMasqueOptions := func() *state.MasqueOptions {
		if s.MasqueOptions == nil {
			s.MasqueOptions = &state.MasqueOptions{}
		}
		touched = true
		return s.MasqueOptions
	}
	if v := os.Getenv("CFWARP_MASQUE_SNI"); v != "" {
		o := ensureMasqueOptions()
		o.SNI = v
	}
	if v := os.Getenv("CFWARP_MASQUE_CONNECT_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			o := ensureMasqueOptions()
			o.ConnectPort = p
		}
	}
	if v := os.Getenv("CFWARP_MASQUE_USE_IPV6"); v != "" {
		if parsed, ok := parseBool(v); ok {
			o := ensureMasqueOptions()
			o.UseIPv6 = parsed
		}
	}
	if v := os.Getenv("CFWARP_MASQUE_MTU"); v != "" {
		if mtu, err := strconv.Atoi(v); err == nil {
			o := ensureMasqueOptions()
			o.MTU = mtu
		}
	}
	if v := os.Getenv("CFWARP_MASQUE_INITIAL_PACKET_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil && size >= 0 && size <= 65535 {
			o := ensureMasqueOptions()
			o.InitialPacketSize = uint16(size)
		}
	}
	if v := os.Getenv("CFWARP_MASQUE_KEEPALIVE_SECONDS"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			o := ensureMasqueOptions()
			o.KeepAlivePeriodSeconds = sec
		}
	}
	if v := os.Getenv("CFWARP_MASQUE_RECONNECT_DELAY_MILLIS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			o := ensureMasqueOptions()
			o.ReconnectDelayMillis = ms
		}
	}
	if !touched && s.MasqueOptions != nil && isZeroMasqueOptions(*s.MasqueOptions) {
		s.MasqueOptions = nil
	}
}

func applyCapsEnv(s *state.Settings) {
	ensureCaps := func() *state.CapsOptions {
		if s.Caps == nil {
			s.Caps = &state.CapsOptions{}
		}
		return s.Caps
	}
	if v := os.Getenv("CFWARP_CAPS_INTERVAL_SECONDS"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			ensureCaps().IntervalSeconds = sec
		}
	}
	if v := os.Getenv("CFWARP_CAPS_CHECKS"); v != "" {
		parts := strings.Split(v, ",")
		checks := make([]state.CapCheck, 0, len(parts))
		for _, part := range parts {
			probe := strings.ToLower(strings.TrimSpace(part))
			if probe == "" {
				continue
			}
			checks = append(checks, state.CapCheck{Probe: probe, RotateOnFail: true, TimeoutSeconds: 15})
		}
		ensureCaps().Checks = checks
	}
}

func applyDNSEnv(s *state.Settings) {
	ensureDNS := func() *state.DNSOptions {
		if s.DNS == nil {
			s.DNS = &state.DNSOptions{}
		}
		return s.DNS
	}
	if v := os.Getenv("CFWARP_DNS_MODE"); v != "" {
		ensureDNS().Mode = v
	}
	if v := os.Getenv("CFWARP_DNS_SERVER"); v != "" {
		ensureDNS().Server = v
	}
	if v := os.Getenv("CFWARP_DNS_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			ensureDNS().ServerPort = p
		}
	}
	if v := os.Getenv("CFWARP_DNS_PATH"); v != "" {
		ensureDNS().Path = v
	}
	if v := os.Getenv("CFWARP_DNS_STRATEGY"); v != "" {
		ensureDNS().Strategy = v
	}
	if s.DNS != nil && s.DNS.Mode == "" && s.DNS.Server == "" && s.DNS.ServerPort == 0 && s.DNS.Path == "" && s.DNS.Strategy == "" {
		s.DNS = nil
	}
}

func applyRotationEnv(s *state.Settings) {
	ensureRotation := func() *state.RotationOptions {
		if s.Rotation == nil {
			s.Rotation = &state.RotationOptions{}
		}
		return s.Rotation
	}
	if v := os.Getenv("CFWARP_ROTATION_ENABLED"); v != "" {
		if parsed, ok := parseBool(v); ok {
			ensureRotation().Enabled = parsed
		}
	}
	if v := os.Getenv("CFWARP_ROTATION_MAX_ATTEMPTS_PER_INCIDENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ensureRotation().MaxAttemptsPerIncident = n
		}
	}
	if v := os.Getenv("CFWARP_ROTATION_SETTLE_TIME_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ensureRotation().SettleTimeSeconds = n
		}
	}
	if v := os.Getenv("CFWARP_ROTATION_COOLDOWN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ensureRotation().CooldownSeconds = n
		}
	}
	if v := os.Getenv("CFWARP_ROTATION_RESTORE_LAST_GOOD"); v != "" {
		if parsed, ok := parseBool(v); ok {
			ensureRotation().RestoreLastGood = parsed
		}
	}
	if v := os.Getenv("CFWARP_ROTATION_ENROLL_MASQUE"); v != "" {
		if parsed, ok := parseBool(v); ok {
			ensureRotation().EnrollMasque = parsed
		}
	}
	if v := os.Getenv("CFWARP_ROTATION_DISTINCTNESS"); v != "" {
		ensureRotation().Distinctness = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("CFWARP_ROTATION_HISTORY_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ensureRotation().HistorySize = n
		}
	}
}

func applyDaemonEnv(s *state.Settings) {
	if v := os.Getenv("CFWARP_DAEMON_CONTROL_SOCKET"); v != "" {
		if s.Daemon == nil {
			s.Daemon = &state.DaemonOptions{}
		}
		s.Daemon.ControlSocket = v
	}
}

func parseBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func isZeroMasqueOptions(o state.MasqueOptions) bool {
	return o.SNI == "" && o.ConnectPort == 0 && !o.UseIPv6 && o.MTU == 0 && o.InitialPacketSize == 0 && o.KeepAlivePeriodSeconds == 0 && o.ReconnectDelayMillis == 0
}

// applyOverrides applies non-nil pointer fields from overrides onto s.
func applyOverrides(s *state.Settings, o Overrides) {
	legacyAccessTouched := false
	if o.Backend != nil {
		s.Backend = *o.Backend
	}
	if o.RuntimeFamily != nil {
		s.RuntimeFamily = strings.ToLower(*o.RuntimeFamily)
	}
	if o.Transport != nil {
		s.Transport = strings.ToLower(*o.Transport)
	}
	if o.ListenHost != nil {
		s.ListenHost = *o.ListenHost
		legacyAccessTouched = true
	}
	if o.ListenPort != nil {
		s.ListenPort = *o.ListenPort
		legacyAccessTouched = true
	}
	if o.ProxyMode != nil {
		normalized := strings.ToLower(*o.ProxyMode)
		s.ProxyMode = normalized
		s.Mode = normalized
		legacyAccessTouched = true
	}
	if o.Mode != nil {
		s.Mode = strings.ToLower(*o.Mode)
		legacyAccessTouched = true
	}
	if o.ProxyUsername != nil {
		s.ProxyUsername = *o.ProxyUsername
		legacyAccessTouched = true
	}
	if o.ProxyPassword != nil {
		s.ProxyPassword = *o.ProxyPassword
		legacyAccessTouched = true
	}
	if o.EndpointOverride != nil {
		s.EndpointOverride = *o.EndpointOverride
	}
	if o.LogLevel != nil {
		s.LogLevel = strings.ToLower(*o.LogLevel)
	}
	if legacyAccessTouched {
		syncPrimaryAccessFromLegacy(s)
	}
}
