package masque

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/transport"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type stubPacketSession struct{}

func (stubPacketSession) ReadPacket([]byte, bool) (int, error) {
	return 0, errors.New("not implemented")
}
func (stubPacketSession) WritePacket([]byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (stubPacketSession) Close() error { return nil }

func TestDialWithRetry_SucceedsAfterTransientFailure(t *testing.T) {
	orig := connectTunnelFunc
	defer func() { connectTunnelFunc = orig }()

	attempts := 0
	connectTunnelFunc = func(ctx context.Context, tlsConfig *tls.Config, quicConfig *quic.Config, connectURI string, endpoint *net.UDPAddr) (*net.UDPConn, *http3.Transport, packetSession, error) {
		attempts++
		if attempts < 3 {
			return nil, nil, nil, errors.New("transient connect-ip failure")
		}
		return nil, nil, stubPacketSession{}, nil
	}

	tun := &tunnel{ctx: context.Background(), cfg: retryTestConfig(t), events: make(chan transport.Event, 16)}
	bund, err := tun.dialWithRetry(context.Background(), 3)
	if err != nil {
		t.Fatalf("dialWithRetry: %v", err)
	}
	if bund == nil || bund.conn == nil {
		t.Fatal("expected session bundle with packet session")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDialWithRetry_ReturnsLastError(t *testing.T) {
	orig := connectTunnelFunc
	defer func() { connectTunnelFunc = orig }()

	attempts := 0
	connectTunnelFunc = func(ctx context.Context, tlsConfig *tls.Config, quicConfig *quic.Config, connectURI string, endpoint *net.UDPAddr) (*net.UDPConn, *http3.Transport, packetSession, error) {
		attempts++
		return nil, nil, nil, errors.New("still failing")
	}

	tun := &tunnel{ctx: context.Background(), cfg: retryTestConfig(t), events: make(chan transport.Event, 16)}
	_, err := tun.dialWithRetry(context.Background(), 2)
	if err == nil || err.Error() != "still failing" {
		t.Fatalf("expected last error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestStartupBackoff_FloorAndCap(t *testing.T) {
	tun := &tunnel{cfg: retryTestConfig(t)}
	if got := tun.startupBackoff(1); got != 2*time.Second {
		t.Fatalf("attempt 1 backoff = %s, want 2s", got)
	}
	if got := tun.startupBackoff(3); got != maxStartupBackoff {
		t.Fatalf("attempt 3 backoff = %s, want %s", got, maxStartupBackoff)
	}
}

func retryTestConfig(t *testing.T) transport.StartConfig {
	t.Helper()
	privB64, pubPEM := retryTestKeys(t)
	return withDefaults(transport.StartConfig{
		MTU:       1280,
		Addresses: []netip.Prefix{netip.MustParsePrefix("172.16.0.2/32")},
		Masque: &transport.MasqueConfig{
			PrivateKeyDERBase64: privB64,
			EndpointPubKeyPEM:   pubPEM,
			EndpointV4:          "162.159.198.1:443",
		},
	})
}

func retryTestKeys(t *testing.T) (string, string) {
	t.Helper()
	privB64, _, err := GenerateECDSAKeypairDER()
	if err != nil {
		t.Fatalf("GenerateECDSAKeypairDER: %v", err)
	}
	peerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&peerKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	return privB64, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
}
