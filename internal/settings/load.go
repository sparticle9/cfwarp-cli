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
	ListenHost       *string
	ListenPort       *int
	ProxyMode        *string
	ProxyUsername    *string
	ProxyPassword    *string
	EndpointOverride *string
	LogLevel         *string
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

	// 2. Overlay persisted file.
	persisted, err := state.LoadSettings(dirs)
	if err != nil && !errors.Is(err, state.ErrNotFound) {
		return s, fmt.Errorf("load persisted settings: %w", err)
	}
	if err == nil {
		applyPersisted(&s, persisted)
	}

	// 3. Overlay env vars.
	applyEnv(&s)

	// 4. Overlay explicit overrides.
	applyOverrides(&s, overrides)

	// 5. Validate.
	if err := Validate(s); err != nil {
		return s, err
	}
	return s, nil
}

// applyPersisted copies non-zero persisted fields onto s.
func applyPersisted(s *state.Settings, p state.Settings) {
	if p.Backend != "" {
		s.Backend = p.Backend
	}
	if p.ListenHost != "" {
		s.ListenHost = p.ListenHost
	}
	if p.ListenPort != 0 {
		s.ListenPort = p.ListenPort
	}
	if p.ProxyMode != "" {
		s.ProxyMode = p.ProxyMode
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
	if p.LogLevel != "" {
		s.LogLevel = p.LogLevel
	}
}

// applyEnv reads CFWARP_* environment variables.
func applyEnv(s *state.Settings) {
	if v := os.Getenv("CFWARP_BACKEND"); v != "" {
		s.Backend = v
	}
	if v := os.Getenv("CFWARP_LISTEN_HOST"); v != "" {
		s.ListenHost = v
	}
	if v := os.Getenv("CFWARP_LISTEN_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			s.ListenPort = p
		}
	}
	if v := os.Getenv("CFWARP_PROXY_MODE"); v != "" {
		s.ProxyMode = strings.ToLower(v)
	}
	if v := os.Getenv("CFWARP_PROXY_USERNAME"); v != "" {
		s.ProxyUsername = v
	}
	if v := os.Getenv("CFWARP_PROXY_PASSWORD"); v != "" {
		s.ProxyPassword = v
	}
	if v := os.Getenv("CFWARP_ENDPOINT_OVERRIDE"); v != "" {
		s.EndpointOverride = v
	}
	if v := os.Getenv("CFWARP_LOG_LEVEL"); v != "" {
		s.LogLevel = strings.ToLower(v)
	}
}

// applyOverrides applies non-nil pointer fields from overrides onto s.
func applyOverrides(s *state.Settings, o Overrides) {
	if o.Backend != nil {
		s.Backend = *o.Backend
	}
	if o.ListenHost != nil {
		s.ListenHost = *o.ListenHost
	}
	if o.ListenPort != nil {
		s.ListenPort = *o.ListenPort
	}
	if o.ProxyMode != nil {
		s.ProxyMode = strings.ToLower(*o.ProxyMode)
	}
	if o.ProxyUsername != nil {
		s.ProxyUsername = *o.ProxyUsername
	}
	if o.ProxyPassword != nil {
		s.ProxyPassword = *o.ProxyPassword
	}
	if o.EndpointOverride != nil {
		s.EndpointOverride = *o.EndpointOverride
	}
	if o.LogLevel != nil {
		s.LogLevel = strings.ToLower(*o.LogLevel)
	}
}


