package transport

import (
	"context"
	"net/netip"
	"time"
)

// Capabilities describes which service modes and features a transport can support.
type Capabilities struct {
	SupportsSocks5 bool
	SupportsHTTP   bool
	SupportsTUN    bool
	SupportsIPv6   bool
}

// Event is a transport/runtime signal suitable for higher-level status reporting.
type Event struct {
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
}

// Stats is a minimal packet-oriented transport statistics snapshot.
type Stats struct {
	PacketsRead    uint64    `json:"packets_read"`
	PacketsWritten uint64    `json:"packets_written"`
	BytesRead      uint64    `json:"bytes_read"`
	BytesWritten   uint64    `json:"bytes_written"`
	LastActivityAt time.Time `json:"last_activity_at,omitempty"`
}

// MasqueConfig contains MASQUE-specific startup inputs.
type MasqueConfig struct {
	PrivateKeyDERBase64 string
	EndpointPubKeyPEM   string
	EndpointV4          string
	EndpointV6          string
	ConnectURI          string
	SNI                 string
	ConnectPort         int
	UseIPv6             bool
	InitialPacketSize   uint16
	KeepAlivePeriod     time.Duration
	ReconnectDelay      time.Duration
}

// StartConfig contains transport-agnostic startup inputs. Concrete transports
// may interpret the fields differently, but the packet tunnel contract remains shared.
type StartConfig struct {
	MTU              int
	EndpointOverride string
	Addresses        []netip.Prefix
	Masque           *MasqueConfig
}

// PacketTunnel is the shared packet-oriented seam between transports and the data plane.
type PacketTunnel interface {
	MTU() int
	Addresses() []netip.Prefix
	ReadPacket(buf []byte) (int, error)
	WritePacket(pkt []byte) error
	Events() <-chan Event
	Stats() Stats
	Close() error
}

// PacketTransport starts a packet tunnel for a specific transport implementation.
type PacketTransport interface {
	Name() string
	Capabilities() Capabilities
	Start(ctx context.Context, cfg StartConfig) (PacketTunnel, error)
}
