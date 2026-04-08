package masque

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	connectip "github.com/Diniboy1123/connect-ip-go"
	"github.com/nexus/cfwarp-cli/internal/transport"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/yosida95/uritemplate/v3"
)

const (
	DefaultSNI            = "consumer-masque.cloudflareclient.com"
	DefaultConnectURI     = "https://cloudflareaccess.com"
	defaultConnectPort    = 443
	defaultInitialPktSize = 1242
	defaultKeepAlive      = 30 * time.Second
	defaultReconnectDelay = time.Second
	cfConnectProtocol     = "cf-connect-ip"
	settingsH3Datagram00  = 0x276
)

// packetSession abstracts the session operations needed by the tunnel.
type packetSession interface {
	ReadPacket(b []byte, nonBlocking bool) (int, error)
	WritePacket(b []byte) ([]byte, error)
	Close() error
}

type sessionBundle struct {
	udpConn *net.UDPConn
	h3      *http3.Transport
	conn    packetSession
}

func (s *sessionBundle) Close() error {
	if s == nil {
		return nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.h3 != nil {
		_ = s.h3.Close()
	}
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}
	return nil
}

// Transport implements Cloudflare MASQUE as a PacketTransport.
type Transport struct{}

func (Transport) Name() string { return "masque" }

func (Transport) Capabilities() transport.Capabilities {
	return transport.Capabilities{SupportsSocks5: true, SupportsHTTP: true, SupportsTUN: true, SupportsIPv6: true}
}

func (Transport) Start(ctx context.Context, cfg transport.StartConfig) (transport.PacketTunnel, error) {
	if cfg.Masque == nil {
		return nil, fmt.Errorf("masque start config is required")
	}
	cfg = withDefaults(cfg)
	tun := &tunnel{
		ctx:          ctx,
		cfg:          cfg,
		pendingReads: make(chan []byte, 8),
		events:       make(chan transport.Event, 16),
	}
	bund, err := tun.dial(ctx)
	if err != nil {
		return nil, err
	}
	tun.bundle = bund
	tun.emit("info", "connected", "MASQUE tunnel established")
	return tun, nil
}

func withDefaults(cfg transport.StartConfig) transport.StartConfig {
	mc := *cfg.Masque
	if mc.SNI == "" {
		mc.SNI = DefaultSNI
	}
	if mc.ConnectURI == "" {
		mc.ConnectURI = DefaultConnectURI
	}
	if mc.ConnectPort == 0 {
		mc.ConnectPort = defaultConnectPort
	}
	if mc.InitialPacketSize == 0 {
		mc.InitialPacketSize = defaultInitialPktSize
	}
	if mc.KeepAlivePeriod == 0 {
		mc.KeepAlivePeriod = defaultKeepAlive
	}
	if mc.ReconnectDelay == 0 {
		mc.ReconnectDelay = defaultReconnectDelay
	}
	cfg.Masque = &mc
	return cfg
}

type tunnel struct {
	ctx context.Context
	cfg transport.StartConfig

	mu     sync.RWMutex
	bundle *sessionBundle

	stats  transport.Stats
	events chan transport.Event

	pendingReads chan []byte
}

func (t *tunnel) MTU() int { return t.cfg.MTU }

func (t *tunnel) Addresses() []netip.Prefix {
	out := make([]netip.Prefix, len(t.cfg.Addresses))
	copy(out, t.cfg.Addresses)
	return out
}

func (t *tunnel) Events() <-chan transport.Event { return t.events }

func (t *tunnel) Stats() transport.Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats
}

func (t *tunnel) ReadPacket(buf []byte) (int, error) {
	select {
	case pkt := <-t.pendingReads:
		n := copy(buf, pkt)
		t.recordRead(n)
		return n, nil
	default:
	}

	bundle := t.current()
	n, err := bundle.conn.ReadPacket(buf, true)
	if err != nil {
		if recErr := t.reconnect(err); recErr != nil {
			return 0, recErr
		}
		bundle = t.current()
		n, err = bundle.conn.ReadPacket(buf, true)
		if err != nil {
			return 0, err
		}
	}
	t.recordRead(n)
	return n, nil
}

func (t *tunnel) WritePacket(pkt []byte) error {
	bundle := t.current()
	icmp, err := bundle.conn.WritePacket(pkt)
	if err != nil {
		if recErr := t.reconnect(err); recErr != nil {
			return recErr
		}
		bundle = t.current()
		icmp, err = bundle.conn.WritePacket(pkt)
		if err != nil {
			return err
		}
	}
	if len(icmp) > 0 {
		select {
		case t.pendingReads <- append([]byte(nil), icmp...):
		default:
		}
	}
	t.recordWrite(len(pkt))
	return nil
}

func (t *tunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.bundle != nil {
		_ = t.bundle.Close()
		t.bundle = nil
	}
	select {
	case <-t.events:
	default:
	}
	close(t.events)
	return nil
}

func (t *tunnel) current() *sessionBundle {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.bundle
}

func (t *tunnel) setBundle(b *sessionBundle) {
	t.mu.Lock()
	old := t.bundle
	t.bundle = b
	t.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
}

func (t *tunnel) reconnect(cause error) error {
	t.emit("warn", "reconnect", cause.Error())
	t.mu.RLock()
	delay := t.cfg.Masque.ReconnectDelay
	t.mu.RUnlock()
	select {
	case <-time.After(delay):
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
	bund, err := t.dial(t.ctx)
	if err != nil {
		return fmt.Errorf("reconnect MASQUE tunnel: %w", err)
	}
	t.setBundle(bund)
	t.emit("info", "reconnected", "MASQUE tunnel re-established")
	return nil
}

func (t *tunnel) dial(ctx context.Context) (*sessionBundle, error) {
	privKey, err := parsePrivateKey(t.cfg.Masque.PrivateKeyDERBase64)
	if err != nil {
		return nil, err
	}
	peerPubKey, err := parseEndpointPublicKey(t.cfg.Masque.EndpointPubKeyPEM)
	if err != nil {
		return nil, err
	}
	certDER, err := generateSelfSignedCert(privKey)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := PrepareTLSConfig(privKey, peerPubKey, certDER, t.cfg.Masque.SNI)
	if err != nil {
		return nil, err
	}
	quicConfig := &quic.Config{
		EnableDatagrams:   true,
		InitialPacketSize: t.cfg.Masque.InitialPacketSize,
		KeepAlivePeriod:   t.cfg.Masque.KeepAlivePeriod,
	}
	endpoint, err := resolveEndpoint(*t.cfg.Masque, t.cfg.EndpointOverride)
	if err != nil {
		return nil, err
	}
	udpConn, h3t, ipConn, err := connectTunnel(ctx, tlsConfig, quicConfig, t.cfg.Masque.ConnectURI, endpoint)
	if err != nil {
		return nil, err
	}
	return &sessionBundle{udpConn: udpConn, h3: h3t, conn: ipConn}, nil
}

func (t *tunnel) emit(level, typ, msg string) {
	select {
	case t.events <- transport.Event{At: time.Now().UTC(), Level: level, Type: typ, Message: msg}:
	default:
	}
}

func (t *tunnel) recordRead(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.PacketsRead++
	t.stats.BytesRead += uint64(n)
	t.stats.LastActivityAt = time.Now().UTC()
}

func (t *tunnel) recordWrite(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.PacketsWritten++
	t.stats.BytesWritten += uint64(n)
	t.stats.LastActivityAt = time.Now().UTC()
}

// PrepareTLSConfig creates a TLS config for MASQUE client-auth and endpoint pinning.
func PrepareTLSConfig(privKey *ecdsa.PrivateKey, peerPubKey *ecdsa.PublicKey, certDER [][]byte, sni string) (*tls.Config, error) {
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: certDER, PrivateKey: privKey}},
		ServerName:   sni,
		NextProtos:   []string{http3.NextProtoH3},
		// Cloudflare MASQUE endpoints use SNI separate from the endpoint IP.
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return nil
			}
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			remote, ok := cert.PublicKey.(*ecdsa.PublicKey)
			if !ok {
				return x509.ErrUnsupportedAlgorithm
			}
			if !remote.Equal(peerPubKey) {
				return fmt.Errorf("remote endpoint public key mismatch")
			}
			return nil
		},
	}, nil
}

func parsePrivateKey(b64 string) (*ecdsa.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode MASQUE private key: %w", err)
	}
	priv, err := x509.ParseECPrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("parse MASQUE private key: %w", err)
	}
	return priv, nil
}

func parseEndpointPublicKey(pemText string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, fmt.Errorf("decode endpoint public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint public key: %w", err)
	}
	ec, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("endpoint public key is not ECDSA")
	}
	return ec, nil
}

func generateSelfSignedCert(privKey *ecdsa.PrivateKey) ([][]byte, error) {
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}, &x509.Certificate{}, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, fmt.Errorf("generate self-signed cert: %w", err)
	}
	return [][]byte{der}, nil
}

func resolveEndpoint(cfg transport.MasqueConfig, override string) (*net.UDPAddr, error) {
	raw := override
	if raw == "" {
		if cfg.UseIPv6 {
			raw = cfg.EndpointV6
		} else {
			raw = cfg.EndpointV4
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("MASQUE endpoint is required")
	}
	host := raw
	port := cfg.ConnectPort
	if h, p, err := net.SplitHostPort(raw); err == nil {
		host = h
		if portNum, err := net.LookupPort("udp", p); err == nil {
			port = portNum
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		resolved, err := net.ResolveIPAddr("ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve MASQUE endpoint %q: %w", host, err)
		}
		ip = resolved.IP
	}
	return &net.UDPAddr{IP: ip, Port: port}, nil
}

func connectTunnel(ctx context.Context, tlsConfig *tls.Config, quicConfig *quic.Config, connectURI string, endpoint *net.UDPAddr) (*net.UDPConn, *http3.Transport, packetSession, error) {
	var udpConn *net.UDPConn
	var err error
	if endpoint.IP.To4() == nil {
		udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6zero, Port: 0})
	} else {
		udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	}
	if err != nil {
		return nil, nil, nil, err
	}
	conn, err := quic.Dial(ctx, udpConn, endpoint, tlsConfig, quicConfig)
	if err != nil {
		udpConn.Close()
		return nil, nil, nil, err
	}
	tr := &http3.Transport{
		EnableDatagrams: true,
		AdditionalSettings: map[uint64]uint64{
			settingsH3Datagram00: 1,
		},
		DisableCompression: true,
	}
	clientConn := tr.NewClientConn(conn)
	additionalHeaders := http.Header{"User-Agent": []string{""}}
	tmpl := uritemplate.MustNew(connectURI)
	ipConn, rsp, err := connectip.Dial(ctx, clientConn, tmpl, cfConnectProtocol, additionalHeaders, true)
	if err != nil {
		tr.Close()
		udpConn.Close()
		return nil, nil, nil, fmt.Errorf("dial connect-ip: %w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		ipConn.Close()
		tr.Close()
		udpConn.Close()
		return nil, nil, nil, fmt.Errorf("connect-ip handshake returned %s", rsp.Status)
	}
	return udpConn, tr, ipConn, nil
}

// GenerateECDSAKeypairDER returns base64 DER private/public key encodings suitable for MASQUE enrollment.
func GenerateECDSAKeypairDER() (privateKeyDERBase64, publicKeyDERBase64 string, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ECDSA keypair: %w", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("marshal ECDSA private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal ECDSA public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(privDER), base64.StdEncoding.EncodeToString(pubDER), nil
}
