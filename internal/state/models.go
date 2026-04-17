package state

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	CurrentAccountSchemaVersion  = 2
	CurrentSettingsSchemaVersion = 2
	CurrentRuntimeSchemaVersion  = 3

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

// AccessConfig describes how local clients consume the configured uplink.
type AccessConfig struct {
	Type       string `json:"type,omitempty"`
	ListenHost string `json:"listen_host,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	Name       string `json:"name,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
}

// CapCheck defines one built-in capability probe evaluated by the daemon.
type CapCheck struct {
	Probe          string `json:"probe,omitempty"`
	Required       bool   `json:"required,omitempty"`
	RotateOnFail   bool   `json:"rotate_on_fail,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// CapsOptions groups the daemon's capability probe policy.
type CapsOptions struct {
	IntervalSeconds int        `json:"interval_seconds,omitempty"`
	Checks          []CapCheck `json:"checks,omitempty"`
}

// RotationOptions controls how the daemon remediates failed capability probes.
type RotationOptions struct {
	Enabled                bool `json:"enabled,omitempty"`
	MaxAttemptsPerIncident int  `json:"max_attempts_per_incident,omitempty"`
	SettleTimeSeconds      int  `json:"settle_time_seconds,omitempty"`
	CooldownSeconds        int  `json:"cooldown_seconds,omitempty"`
	RestoreLastGood        bool `json:"restore_last_good,omitempty"`
	EnrollMasque           bool `json:"enroll_masque,omitempty"`
}

// DaemonOptions configures the long-running manager process.
type DaemonOptions struct {
	ControlSocket string `json:"control_socket,omitempty"`
}

// Settings holds operator-supplied configuration persisted to settings.json.
type Settings struct {
	SchemaVersion    int              `json:"-"`
	RuntimeFamily    string           `json:"-"`
	Transport        string           `json:"-"`
	Mode             string           `json:"-"`
	ListenHost       string           `json:"-"`
	ListenPort       int              `json:"-"`
	ProxyUsername    string           `json:"-"`
	ProxyPassword    string           `json:"-"`
	EndpointOverride string           `json:"-"`
	StateDir         string           `json:"-"`
	LogLevel         string           `json:"-"`
	MasqueOptions    *MasqueOptions   `json:"-"`
	Access           []AccessConfig   `json:"-"`
	Caps             *CapsOptions     `json:"-"`
	Rotation         *RotationOptions `json:"-"`
	Daemon           *DaemonOptions   `json:"-"`

	// Deprecated in-memory aliases retained for incremental migration.
	Backend   string `json:"-"`
	ProxyMode string `json:"-"`
}

type settingsJSON struct {
	SchemaVersion    int              `json:"schema_version,omitempty"`
	RuntimeFamily    string           `json:"runtime_family,omitempty"`
	Transport        string           `json:"transport,omitempty"`
	Mode             string           `json:"mode,omitempty"`
	ListenHost       string           `json:"listen_host,omitempty"`
	ListenPort       int              `json:"listen_port,omitempty"`
	ProxyUsername    string           `json:"proxy_username,omitempty"`
	ProxyPassword    string           `json:"proxy_password,omitempty"`
	EndpointOverride string           `json:"endpoint_override,omitempty"`
	StateDir         string           `json:"state_dir,omitempty"`
	LogLevel         string           `json:"log_level,omitempty"`
	MasqueOptions    *MasqueOptions   `json:"masque_options,omitempty"`
	Access           []AccessConfig   `json:"access,omitempty"`
	Caps             *CapsOptions     `json:"caps,omitempty"`
	Rotation         *RotationOptions `json:"rotation,omitempty"`
	Daemon           *DaemonOptions   `json:"daemon,omitempty"`
	Backend          string           `json:"backend,omitempty"`
	ProxyMode        string           `json:"proxy_mode,omitempty"`
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
	if s.ProxyMode != "" && s.ProxyMode != s.Mode {
		switch {
		case s.Mode == ModeSocks5 && s.ProxyMode != ModeSocks5:
			s.Mode = s.ProxyMode
		case s.ProxyMode == ModeSocks5 && s.Mode != ModeSocks5:
			// keep explicit mode override
		default:
			s.Mode = s.ProxyMode
		}
	}
	if s.Mode == "" {
		s.Mode = ModeSocks5
	}
	if s.Mode != "" {
		s.ProxyMode = s.Mode
	}
	if derived := DeriveBackendTag(s.RuntimeFamily, s.Transport); derived != "" {
		s.Backend = derived
	}
	if s.Caps != nil {
		if s.Caps.IntervalSeconds == 0 {
			s.Caps.IntervalSeconds = 300
		}
		for i := range s.Caps.Checks {
			s.Caps.Checks[i].Probe = strings.ToLower(strings.TrimSpace(s.Caps.Checks[i].Probe))
			if s.Caps.Checks[i].TimeoutSeconds == 0 {
				s.Caps.Checks[i].TimeoutSeconds = 15
			}
		}
	}
	for i := range s.Access {
		s.Access[i].Type = strings.ToLower(strings.TrimSpace(s.Access[i].Type))
		if s.Access[i].Type != ModeTUN && s.Access[i].ListenHost == "" {
			s.Access[i].ListenHost = "0.0.0.0"
		}
	}
	if len(s.Access) == 0 {
		s.Access = []AccessConfig{{
			Type:       s.Mode,
			ListenHost: defaultString(s.ListenHost, "0.0.0.0"),
			ListenPort: defaultInt(s.ListenPort, 1080),
			Username:   s.ProxyUsername,
			Password:   s.ProxyPassword,
		}}
	}
	primary := s.Access[0]
	if primary.Type == "" {
		primary.Type = s.Mode
	}
	s.Access[0] = primary
	s.Mode = primary.Type
	s.ProxyMode = primary.Type
	s.ListenHost = primary.ListenHost
	s.ListenPort = primary.ListenPort
	s.ProxyUsername = primary.Username
	s.ProxyPassword = primary.Password
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func defaultInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func accessMatchesLegacy(s Settings) bool {
	if len(s.Access) != 1 {
		return false
	}
	access := s.Access[0]
	return access.Type == s.Mode &&
		access.ListenHost == s.ListenHost &&
		access.ListenPort == s.ListenPort &&
		access.Username == s.ProxyUsername &&
		access.Password == s.ProxyPassword &&
		access.Name == "" &&
		access.MTU == 0
}

func zeroRotation(o *RotationOptions) bool {
	return o == nil || (!o.Enabled && o.MaxAttemptsPerIncident == 0 && o.SettleTimeSeconds == 0 && o.CooldownSeconds == 0 && !o.RestoreLastGood && !o.EnrollMasque)
}

func zeroDaemon(o *DaemonOptions) bool {
	return o == nil || o.ControlSocket == ""
}

func (s Settings) MarshalJSON() ([]byte, error) {
	ss := s
	ss.Normalize()
	payload := settingsJSON{
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
		Caps:             ss.Caps,
	}
	if !accessMatchesLegacy(ss) {
		payload.Access = ss.Access
	}
	if !zeroRotation(ss.Rotation) {
		payload.Rotation = ss.Rotation
	}
	if !zeroDaemon(ss.Daemon) {
		payload.Daemon = ss.Daemon
	}
	return json.Marshal(payload)
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
	if raw.Access != nil {
		s.Access = raw.Access
	} else {
		s.Access = nil
	}
	if raw.Caps != nil {
		s.Caps = raw.Caps
	}
	if raw.Rotation != nil {
		s.Rotation = raw.Rotation
	} else {
		s.Rotation = nil
	}
	if raw.Daemon != nil {
		s.Daemon = raw.Daemon
	} else {
		s.Daemon = nil
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

const (
	RuntimePhaseIdle       = "idle"
	RuntimePhaseConnecting = "connecting"
	RuntimePhaseConnected  = "connected"
	RuntimePhaseDegraded   = "degraded"
	RuntimePhaseStopped    = "stopped"
)

// PacketPathStats captures lightweight userspace packet-path counters and call timings.
type PacketPathStats struct {
	Packets      uint64    `json:"packets,omitempty"`
	Bytes        uint64    `json:"bytes,omitempty"`
	ReadCalls    uint64    `json:"read_calls,omitempty"`
	ReadNanos    uint64    `json:"read_nanos,omitempty"`
	WriteCalls   uint64    `json:"write_calls,omitempty"`
	WriteNanos   uint64    `json:"write_nanos,omitempty"`
	LastPacketAt time.Time `json:"last_packet_at,omitempty"`
}

// TransportStatsSnapshot captures the current packet tunnel counters.
type TransportStatsSnapshot struct {
	PacketsRead    uint64    `json:"packets_read,omitempty"`
	PacketsWritten uint64    `json:"packets_written,omitempty"`
	BytesRead      uint64    `json:"bytes_read,omitempty"`
	BytesWritten   uint64    `json:"bytes_written,omitempty"`
	LastActivityAt time.Time `json:"last_activity_at,omitempty"`
}

// RuntimeEventSnapshot stores the most recent transport event observed by the native runtime.
type RuntimeEventSnapshot struct {
	At      time.Time `json:"at,omitempty"`
	Level   string    `json:"level,omitempty"`
	Type    string    `json:"type,omitempty"`
	Message string    `json:"message,omitempty"`
}

// RuntimeDiagnostics captures low-overhead dataplane diagnostics for status and bench inspection.
type RuntimeDiagnostics struct {
	CapturedAt    time.Time              `json:"captured_at,omitempty"`
	LastEvent     *RuntimeEventSnapshot  `json:"last_event,omitempty"`
	Transport     TransportStatsSnapshot `json:"transport,omitempty"`
	StackToTunnel PacketPathStats        `json:"stack_to_tunnel,omitempty"`
	TunnelToStack PacketPathStats        `json:"tunnel_to_stack,omitempty"`
	Netstack      PacketPathStats        `json:"netstack,omitempty"`
}

// RuntimeState holds ephemeral process metadata persisted to runtime.json.
type RuntimeState struct {
	SchemaVersion       int                 `json:"-"`
	PID                 int                 `json:"-"`
	Backend             string              `json:"-"`
	RuntimeFamily       string              `json:"-"`
	Transport           string              `json:"-"`
	Mode                string              `json:"-"`
	Phase               string              `json:"-"`
	ListenHost          string              `json:"-"`
	ListenPort          int                 `json:"-"`
	SelectedEndpoint    string              `json:"-"`
	SelectedAddressFam  string              `json:"-"`
	ServiceSocketPath   string              `json:"-"`
	ConfigPath          string              `json:"-"`
	StdoutLogPath       string              `json:"-"`
	StderrLogPath       string              `json:"-"`
	StartedAt           time.Time           `json:"-"`
	LastReconnectAt     time.Time           `json:"-"`
	LastReconnectReason string              `json:"-"`
	LastTransportError  string              `json:"-"`
	LastError           string              `json:"-"`
	LocalReachable      bool                `json:"-"`
	LastTraceSummary    string              `json:"-"`
	Diagnostics         *RuntimeDiagnostics `json:"-"`
}

type runtimeStateJSON struct {
	SchemaVersion       int                 `json:"schema_version,omitempty"`
	PID                 int                 `json:"pid,omitempty"`
	Backend             string              `json:"backend,omitempty"`
	RuntimeFamily       string              `json:"runtime_family,omitempty"`
	Transport           string              `json:"transport,omitempty"`
	Mode                string              `json:"mode,omitempty"`
	Phase               string              `json:"phase,omitempty"`
	ListenHost          string              `json:"listen_host,omitempty"`
	ListenPort          int                 `json:"listen_port,omitempty"`
	SelectedEndpoint    string              `json:"selected_endpoint,omitempty"`
	SelectedAddressFam  string              `json:"selected_address_family,omitempty"`
	ServiceSocketPath   string              `json:"service_socket_path,omitempty"`
	ConfigPath          string              `json:"config_path,omitempty"`
	StdoutLogPath       string              `json:"stdout_log_path,omitempty"`
	StderrLogPath       string              `json:"stderr_log_path,omitempty"`
	StartedAt           time.Time           `json:"started_at,omitempty"`
	LastReconnectAt     time.Time           `json:"last_reconnect_at,omitempty"`
	LastReconnectReason string              `json:"last_reconnect_reason,omitempty"`
	LastTransportError  string              `json:"last_transport_error,omitempty"`
	LastError           string              `json:"last_error,omitempty"`
	LocalReachable      bool                `json:"local_reachable,omitempty"`
	LastTraceSummary    string              `json:"last_trace_summary,omitempty"`
	Diagnostics         *RuntimeDiagnostics `json:"diagnostics,omitempty"`
}

// Normalize upgrades older runtime state into the richer runtime model.
func (r *RuntimeState) Normalize() {
	if r.SchemaVersion == 0 {
		r.SchemaVersion = CurrentRuntimeSchemaVersion
	}
	r.Backend = strings.ToLower(r.Backend)
	r.RuntimeFamily = strings.ToLower(r.RuntimeFamily)
	r.Transport = strings.ToLower(r.Transport)
	r.Mode = strings.ToLower(r.Mode)
	r.Phase = strings.ToLower(r.Phase)
	if r.RuntimeFamily == "" || r.Transport == "" {
		rf, transport := InferRuntimeSelection(r.Backend)
		if r.RuntimeFamily == "" {
			r.RuntimeFamily = rf
		}
		if r.Transport == "" {
			r.Transport = transport
		}
	}
	if r.Mode == "" {
		r.Mode = ModeSocks5
	}
	if r.Phase == "" {
		switch {
		case r.PID > 0:
			r.Phase = RuntimePhaseConnected
		case r.LastError != "" || r.LastTransportError != "":
			r.Phase = RuntimePhaseStopped
		default:
			r.Phase = RuntimePhaseIdle
		}
	}
}

func (r RuntimeState) MarshalJSON() ([]byte, error) {
	rr := r
	rr.Normalize()
	return json.Marshal(runtimeStateJSON{
		SchemaVersion:       rr.SchemaVersion,
		PID:                 rr.PID,
		Backend:             rr.Backend,
		RuntimeFamily:       rr.RuntimeFamily,
		Transport:           rr.Transport,
		Mode:                rr.Mode,
		Phase:               rr.Phase,
		ListenHost:          rr.ListenHost,
		ListenPort:          rr.ListenPort,
		SelectedEndpoint:    rr.SelectedEndpoint,
		SelectedAddressFam:  rr.SelectedAddressFam,
		ServiceSocketPath:   rr.ServiceSocketPath,
		ConfigPath:          rr.ConfigPath,
		StdoutLogPath:       rr.StdoutLogPath,
		StderrLogPath:       rr.StderrLogPath,
		StartedAt:           rr.StartedAt,
		LastReconnectAt:     rr.LastReconnectAt,
		LastReconnectReason: rr.LastReconnectReason,
		LastTransportError:  rr.LastTransportError,
		LastError:           rr.LastError,
		LocalReachable:      rr.LocalReachable,
		LastTraceSummary:    rr.LastTraceSummary,
		Diagnostics:         rr.Diagnostics,
	})
}

func (r *RuntimeState) UnmarshalJSON(data []byte) error {
	var raw runtimeStateJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = RuntimeState{
		SchemaVersion:       raw.SchemaVersion,
		PID:                 raw.PID,
		Backend:             raw.Backend,
		RuntimeFamily:       raw.RuntimeFamily,
		Transport:           raw.Transport,
		Mode:                raw.Mode,
		Phase:               raw.Phase,
		ListenHost:          raw.ListenHost,
		ListenPort:          raw.ListenPort,
		SelectedEndpoint:    raw.SelectedEndpoint,
		SelectedAddressFam:  raw.SelectedAddressFam,
		ServiceSocketPath:   raw.ServiceSocketPath,
		ConfigPath:          raw.ConfigPath,
		StdoutLogPath:       raw.StdoutLogPath,
		StderrLogPath:       raw.StderrLogPath,
		StartedAt:           raw.StartedAt,
		LastReconnectAt:     raw.LastReconnectAt,
		LastReconnectReason: raw.LastReconnectReason,
		LastTransportError:  raw.LastTransportError,
		LastError:           raw.LastError,
		LocalReachable:      raw.LocalReachable,
		LastTraceSummary:    raw.LastTraceSummary,
		Diagnostics:         raw.Diagnostics,
	}
	r.Normalize()
	return nil
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
