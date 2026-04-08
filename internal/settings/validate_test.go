package settings_test

import (
	"strings"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/settings"
	"github.com/nexus/cfwarp-cli/internal/state"
)

func validSettings() state.Settings {
	return state.DefaultSettings()
}

// --- Validate ---

func TestValidate_Valid(t *testing.T) {
	if err := settings.Validate(validSettings()); err != nil {
		t.Errorf("expected valid settings to pass, got: %v", err)
	}
}

func TestValidate_InvalidRuntimeFamily(t *testing.T) {
	s := validSettings()
	s.RuntimeFamily = "daemonless"
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "runtime_family") {
		t.Errorf("expected runtime_family error, got: %v", err)
	}
}

func TestValidate_InvalidTransport(t *testing.T) {
	s := validSettings()
	s.Transport = "quic"
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "transport") {
		t.Errorf("expected transport error, got: %v", err)
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	s := validSettings()
	s.Mode = "ftp"
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Errorf("expected mode error, got: %v", err)
	}
}

func TestValidate_InvalidRuntimeCombination_LegacyMasque(t *testing.T) {
	s := validSettings()
	s.Transport = state.TransportMasque
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "runtime_family") && !strings.Contains(err.Error(), "transport") {
		t.Errorf("expected runtime/transport combination error, got: %v", err)
	}
}

func TestValidate_RuntimeFamilyNativeMasqueHTTP(t *testing.T) {
	s := validSettings()
	s.RuntimeFamily = state.RuntimeFamilyNative
	s.Transport = state.TransportMasque
	s.Mode = state.ModeHTTP
	if err := settings.Validate(s); err != nil {
		t.Errorf("expected native MASQUE HTTP settings to be valid, got: %v", err)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	s := validSettings()
	s.LogLevel = "verbose"
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "log_level") {
		t.Errorf("expected log_level error, got: %v", err)
	}
}

func TestValidate_PortZero(t *testing.T) {
	s := validSettings()
	s.ListenPort = 0
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "listen_port") {
		t.Errorf("expected listen_port error, got: %v", err)
	}
}

func TestValidate_PortTooHigh(t *testing.T) {
	s := validSettings()
	s.ListenPort = 99999
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "listen_port") {
		t.Errorf("expected listen_port error, got: %v", err)
	}
}

func TestValidate_AuthUsernameOnly(t *testing.T) {
	s := validSettings()
	s.ProxyUsername = "user"
	s.ProxyPassword = ""
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Errorf("expected auth consistency error, got: %v", err)
	}
}

func TestValidate_AuthPasswordOnly(t *testing.T) {
	s := validSettings()
	s.ProxyUsername = ""
	s.ProxyPassword = "pass"
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Errorf("expected auth consistency error, got: %v", err)
	}
}

func TestValidate_AuthBothSet(t *testing.T) {
	s := validSettings()
	s.ProxyUsername = "user"
	s.ProxyPassword = "pass"
	if err := settings.Validate(s); err != nil {
		t.Errorf("both auth fields set should be valid, got: %v", err)
	}
}

func TestValidate_ValidEndpointOverride(t *testing.T) {
	s := validSettings()
	s.EndpointOverride = "162.159.192.1:4500"
	if err := settings.Validate(s); err != nil {
		t.Errorf("valid endpoint should pass, got: %v", err)
	}
}

func TestValidate_InvalidEndpointOverride(t *testing.T) {
	s := validSettings()
	s.EndpointOverride = "not-valid"
	err := settings.Validate(s)
	if err == nil || !strings.Contains(err.Error(), "endpoint_override") {
		t.Errorf("expected endpoint_override error, got: %v", err)
	}
}

// --- ValidateEndpoint ---

func TestValidateEndpoint_Valid(t *testing.T) {
	cases := []string{
		"162.159.192.1:2408",
		"162.159.192.1:4500",
		"engage.cloudflareclient.com:2408",
		"[2606:4700:d0::1]:2408",
	}
	for _, ep := range cases {
		if err := settings.ValidateEndpoint(ep); err != nil {
			t.Errorf("expected %q to be valid, got: %v", ep, err)
		}
	}
}

func TestValidateEndpoint_Invalid(t *testing.T) {
	cases := []struct {
		ep      string
		errFrag string
	}{
		{"no-port", "host:port"},
		{"host:", "not numeric"},
		{":2408", "empty"},
		{"host:0", "out of range"},
		{"host:65536", "out of range"},
		{"host:abc", "not numeric"},
	}
	for _, tc := range cases {
		err := settings.ValidateEndpoint(tc.ep)
		if err == nil {
			t.Errorf("expected error for %q", tc.ep)
			continue
		}
		if !strings.Contains(err.Error(), tc.errFrag) {
			t.Errorf("for %q: expected error containing %q, got: %v", tc.ep, tc.errFrag, err)
		}
	}
}
