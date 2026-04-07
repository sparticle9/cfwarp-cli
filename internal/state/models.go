package state

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	CurrentAccountSchemaVersion  = 2
	CurrentSettingsSchemaVersion = 2

	RuntimeFamilyLegacy = "legacy"
	RuntimeFamilyNative = "native"

	TransportWireGuard = "wireguard"
	TransportMasque    = "masque"

	ModeSocks5 = "socks5"
	ModeHTTP   = "http"
	ModeTUN    = "tun"

	BackendSingboxWireGuard = "singbox-wireguard"
	BackendNativeMasque     = "native-masque"
	BackendNativeWireGuard  = "native-wireguard"
)

// WireGuardState stores transport-specific account material for the WireGuard path.
type WireGuardState struct {
	PrivateKey   string `json:"private_key"`
	PeerPubKey   string `json:"peer_public_key"`
	IPv4         string `json:"ipv4"`
	IPv6         string `json:"ipv6"`
	Reserved     [3]int `json:"reserved"`
	PeerEndpoint string `json:"peer_endpoint,omitempty"`
}

// MasqueState stores transport-specific account material for the MASQUE path.
type MasqueState struct {
	PrivateKeyDERBase64 string `json:"private_key_der_base64"`
	EndpointPubKeyPEM   string `json:"endpoint_pub_key_pem"`
	EndpointV4          string `json:"endpoint_v4,omitempty"`
	EndpointV6          string `json:"endpoint_v6,omitempty"`
	IPv4                string `json:"ipv4,omitempty"`
	IPv6                string `json:"ipv6,omitempty"`
}

// AccountState holds the WARP device registration data persisted to account.json.
type AccountState struct {
	SchemaVersion int             `json:"-"`
	AccountID     string          `json:"-"`
	Token         string          `json:"-"`
	License       string          `json:"-"`
	ClientID      string          `json:"-"`
	WireGuard     *WireGuardState `json:"-"`
	Masque        *MasqueState    `json:"-"`
	CreatedAt     time.Time       `json:"-"`
	Source        string          `json:"-"` // "register" | "import"

	// Deprecated in-memory aliases retained so the current code can migrate incrementally.
	WARPPrivateKey   string `json:"-"`
	WARPPeerPubKey   string `json:"-"`
	WARPIPV4         string `json:"-"`
	WARPIPV6         string `json:"-"`
	WARPReserved     [3]int `json:"-"`
	WARPPeerEndpoint string `json:"-"`
}

type accountStateJSON struct {
	SchemaVersion    int             `json:"schema_version,omitempty"`
	AccountID        string          `json:"account_id"`
	Token            string          `json:"token"`
	License          string          `json:"license,omitempty"`
	ClientID         string          `json:"client_id,omitempty"`
	WireGuard        *WireGuardState `json:"wireguard,omitempty"`
	Masque           *MasqueState    `json:"masque,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	Source           string          `json:"source"`
	WARPPrivateKey   string          `json:"warp_private_key,omitempty"`
	WARPPeerPubKey   string          `json:"warp_peer_public_key,omitempty"`
	WARPIPV4         string          `json:"warp_ipv4,omitempty"`
	WARPIPV6         string          `json:"warp_ipv6,omitempty"`
	WARPReserved     [3]int          `json:"warp_reserved,omitempty"`
	WARPPeerEndpoint string          `json:"warp_peer_endpoint,omitempty"`
}

// Normalize upgrades legacy in-memory fields into the transport-aware shape and
// back-fills compatibility aliases for older call sites.
func (a *AccountState) Normalize() {
	if a.SchemaVersion == 0 {
		a.SchemaVersion = CurrentAccountSchemaVersion
	}
	if a.WireGuard == nil && hasLegacyWireGuardFields(*a) {
		a.WireGuard = &WireGuardState{
			PrivateKey:   a.WARPPrivateKey,
			PeerPubKey:   a.WARPPeerPubKey,
			IPv4:         a.WARPIPV4,
			IPv6:         a.WARPIPV6,
			Reserved:     a.WARPReserved,
			PeerEndpoint: a.WARPPeerEndpoint,
		}
	}
	if a.WireGuard != nil {
		a.WARPPrivateKey = a.WireGuard.PrivateKey
		a.WARPPeerPubKey = a.WireGuard.PeerPubKey
		a.WARPIPV4 = a.WireGuard.IPv4
		a.WARPIPV6 = a.WireGuard.IPv6
		a.WARPReserved = a.WireGuard.Reserved
		a.WARPPeerEndpoint = a.WireGuard.PeerEndpoint
	}
}

func hasLegacyWireGuardFields(a AccountState) bool {
	return a.WARPPrivateKey != "" || a.WARPPeerPubKey != "" || a.WARPIPV4 != "" ||
		a.WARPIPV6 != "" || a.WARPPeerEndpoint != "" || a.WARPReserved != [3]int{}
}

func (a AccountState) MarshalJSON() ([]byte, error) {
	aa := a
	aa.Normalize()
	return json.Marshal(accountStateJSON{
		SchemaVersion: aa.SchemaVersion,
		AccountID:     aa.AccountID,
		Token:         aa.Token,
		License:       aa.License,
		ClientID:      aa.ClientID,
		WireGuard:     aa.WireGuard,
		Masque:        aa.Masque,
		CreatedAt:     aa.CreatedAt,
		Source:        aa.Source,
	})
}

func (a *AccountState) UnmarshalJSON(data []byte) error {
	var raw accountStateJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*a = AccountState{
		SchemaVersion:    raw.SchemaVersion,
		AccountID:        raw.AccountID,
		Token:            raw.Token,
		License:          raw.License,
		ClientID:         raw.ClientID,
		WireGuard:        raw.WireGuard,
		Masque:           raw.Masque,
		CreatedAt:        raw.CreatedAt,
		Source:           raw.Source,
		WARPPrivateKey:   raw.WARPPrivateKey,
		WARPPeerPubKey:   raw.WARPPeerPubKey,
		WARPIPV4:         raw.WARPIPV4,
		WARPIPV6:         raw.WARPIPV6,
		WARPReserved:     raw.WARPReserved,
		WARPPeerEndpoint: raw.WARPPeerEndpoint,
	}
	a.Normalize()
	return nil
}

// MasqueOptions stores MASQUE-specific runtime knobs separately from shared settings.
type MasqueOptions struct {
	SNI                    string `json:"sni,omitempty"`
	ConnectPort            int    `json:"connect_port,omitempty"`
	UseIPv6                bool   `json:"use_ipv6,omitempty"`
	MTU                    int    `json:"mtu,omitempty"`
	InitialPacketSize      uint16 `json:"initial_packet_size,omitempty"`
	KeepAlivePeriodSeconds int    `json:"keepalive_period_seconds,omitempty"`
	ReconnectDelayMillis   int    `json:"reconnect_delay_millis,omitempty"`
}

// Settings holds operator-supplied configuration persisted to settings.json.
type Settings struct {
	SchemaVersion    int            `json:"-"`
	RuntimeFamily    string         `json:"-"`
	Transport        string         `json:"-"`
	Mode             string         `json:"-"`
	ListenHost       string         `json:"-"`
	ListenPort       int            `json:"-"`
	ProxyUsername    string         `json:"-"`
	ProxyPassword    string         `json:"-"`
	EndpointOverride string         `json:"-"`
	StateDir         string         `json:"-"`
	LogLevel         string         `json:"-"`
	MasqueOptions    *MasqueOptions `json:"-"`

	// Deprecated in-memory aliases retained for incremental migration.
	Backend   string `json:"-"`
	ProxyMode string `json:"-"`
}

type settingsJSON struct {
	SchemaVersion    int            `json:"schema_version,omitempty"`
	RuntimeFamily    string         `json:"runtime_family,omitempty"`
	Transport        string         `json:"transport,omitempty"`
	Mode             string         `json:"mode,omitempty"`
	ListenHost       string         `json:"listen_host,omitempty"`
	ListenPort       int            `json:"listen_port,omitempty"`
	ProxyUsername    string         `json:"proxy_username,omitempty"`
	ProxyPassword    string         `json:"proxy_password,omitempty"`
	EndpointOverride string         `json:"endpoint_override,omitempty"`
	StateDir         string         `json:"state_dir,omitempty"`
	LogLevel         string         `json:"log_level,omitempty"`
	MasqueOptions    *MasqueOptions `json:"masque_options,omitempty"`
	Backend          string         `json:"backend,omitempty"`
	ProxyMode        string         `json:"proxy_mode,omitempty"`
}

// DefaultSettings returns a Settings with sane defaults.
func DefaultSettings() Settings {
	s := Settings{
		SchemaVersion: CurrentSettingsSchemaVersion,
		RuntimeFamily: RuntimeFamilyLegacy,
		Transport:     TransportWireGuard,
		Mode:          ModeSocks5,
		ListenHost:    "0.0.0.0",
		ListenPort:    1080,
		LogLevel:      "info",
		Backend:       BackendSingboxWireGuard,
		ProxyMode:     ModeSocks5,
	}
	s.Normalize()
	return s
}

// Normalize upgrades legacy settings into the runtime-family/transport/mode
// model while keeping compatibility aliases populated for older call sites.
func (s *Settings) Normalize() {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = CurrentSettingsSchemaVersion
	}
	s.RuntimeFamily = strings.ToLower(s.RuntimeFamily)
	s.Transport = strings.ToLower(s.Transport)
	s.Mode = strings.ToLower(s.Mode)
	s.Backend = strings.ToLower(s.Backend)
	s.ProxyMode = strings.ToLower(s.ProxyMode)
	s.LogLevel = strings.ToLower(s.LogLevel)

	if s.RuntimeFamily == "" || s.Transport == "" {
		runtimeFamily, transport := InferRuntimeSelection(s.Backend)
		if s.RuntimeFamily == "" {
			s.RuntimeFamily = runtimeFamily
		}
		if s.Transport == "" {
			s.Transport = transport
		}
	}
	if s.Mode == "" {
		s.Mode = s.ProxyMode
	}
	if s.Mode != "" {
		s.ProxyMode = s.Mode
	}
	if derived := DeriveBackendTag(s.RuntimeFamily, s.Transport); derived != "" {
		s.Backend = derived
	}
}

func (s Settings) MarshalJSON() ([]byte, error) {
	ss := s
	ss.Normalize()
	return json.Marshal(settingsJSON{
		SchemaVersion:    ss.SchemaVersion,
		RuntimeFamily:    ss.RuntimeFamily,
		Transport:        ss.Transport,
		Mode:             ss.Mode,
		ListenHost:       ss.ListenHost,
		ListenPort:       ss.ListenPort,
		ProxyUsername:    ss.ProxyUsername,
		ProxyPassword:    ss.ProxyPassword,
		EndpointOverride: ss.EndpointOverride,
		StateDir:         ss.StateDir,
		LogLevel:         ss.LogLevel,
		MasqueOptions:    ss.MasqueOptions,
	})
}

func (s *Settings) UnmarshalJSON(data []byte) error {
	var raw settingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = DefaultSettings()
	if raw.SchemaVersion != 0 {
		s.SchemaVersion = raw.SchemaVersion
	}
	if raw.RuntimeFamily != "" {
		s.RuntimeFamily = raw.RuntimeFamily
	}
	if raw.Transport != "" {
		s.Transport = raw.Transport
	}
	if raw.Mode != "" {
		s.Mode = raw.Mode
		s.ProxyMode = raw.Mode
	}
	if raw.ListenHost != "" {
		s.ListenHost = raw.ListenHost
	}
	if raw.ListenPort != 0 {
		s.ListenPort = raw.ListenPort
	}
	if raw.ProxyUsername != "" {
		s.ProxyUsername = raw.ProxyUsername
	}
	if raw.ProxyPassword != "" {
		s.ProxyPassword = raw.ProxyPassword
	}
	if raw.EndpointOverride != "" {
		s.EndpointOverride = raw.EndpointOverride
	}
	if raw.StateDir != "" {
		s.StateDir = raw.StateDir
	}
	if raw.LogLevel != "" {
		s.LogLevel = raw.LogLevel
	}
	if raw.MasqueOptions != nil {
		s.MasqueOptions = raw.MasqueOptions
	}
	if raw.Backend != "" {
		s.Backend = raw.Backend
	}
	if raw.ProxyMode != "" {
		s.ProxyMode = raw.ProxyMode
		if raw.Mode == "" {
			s.Mode = raw.ProxyMode
		}
	}
	s.Normalize()
	return nil
}

// DeriveBackendTag converts the new runtime-family/transport selection into the
// legacy backend tag used by current code paths.
func DeriveBackendTag(runtimeFamily, transport string) string {
	switch {
	case runtimeFamily == RuntimeFamilyLegacy && transport == TransportWireGuard:
		return BackendSingboxWireGuard
	case runtimeFamily == RuntimeFamilyNative && transport == TransportMasque:
		return BackendNativeMasque
	case runtimeFamily == RuntimeFamilyNative && transport == TransportWireGuard:
		return BackendNativeWireGuard
	default:
		return ""
	}
}

// InferRuntimeSelection maps a legacy backend tag into runtime-family and transport.
func InferRuntimeSelection(backend string) (runtimeFamily, transport string) {
	switch strings.ToLower(backend) {
	case BackendSingboxWireGuard:
		return RuntimeFamilyLegacy, TransportWireGuard
	case BackendNativeMasque:
		return RuntimeFamilyNative, TransportMasque
	case BackendNativeWireGuard:
		return RuntimeFamilyNative, TransportWireGuard
	default:
		return "", ""
	}
}

// RuntimeState holds ephemeral process metadata persisted to runtime.json.
type RuntimeState struct {
	PID              int       `json:"pid"`
	Backend          string    `json:"backend"`
	ConfigPath       string    `json:"config_path"`
	StdoutLogPath    string    `json:"stdout_log_path"`
	StderrLogPath    string    `json:"stderr_log_path"`
	StartedAt        time.Time `json:"started_at"`
	LastError        string    `json:"last_error,omitempty"`
	LocalReachable   bool      `json:"local_reachable"`
	LastTraceSummary string    `json:"last_trace_summary,omitempty"`
}

// EndpointCandidate describes a WireGuard peer endpoint under consideration.
type EndpointCandidate struct {
	Value        string     `json:"value"`  // "host:port"
	Source       string     `json:"source"` // "manual" | "default" | "env"
	Valid        bool       `json:"valid"`
	LastTestedAt *time.Time `json:"last_tested_at,omitempty"`
	LastResult   string     `json:"last_result,omitempty"` // "success" | "failure"
	Notes        string     `json:"notes,omitempty"`
}
