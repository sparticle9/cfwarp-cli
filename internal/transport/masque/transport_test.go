package masque_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/netip"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/transport"
	"github.com/nexus/cfwarp-cli/internal/transport/masque"
)

func TestGenerateECDSAKeypairDER(t *testing.T) {
	privB64, pubB64, err := masque.GenerateECDSAKeypairDER()
	if err != nil {
		t.Fatalf("GenerateECDSAKeypairDER: %v", err)
	}
	if privB64 == "" || pubB64 == "" {
		t.Fatal("expected non-empty DER encodings")
	}
}

func TestPrepareTLSConfig(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{SerialNumber: bigOne(), NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}, &x509.Certificate{}, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cfg, err := masque.PrepareTLSConfig(priv, &priv.PublicKey, [][]byte{certDER}, masque.DefaultSNI)
	if err != nil {
		t.Fatalf("PrepareTLSConfig: %v", err)
	}
	if cfg.ServerName != masque.DefaultSNI {
		t.Fatalf("unexpected SNI %q", cfg.ServerName)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected one client certificate, got %d", len(cfg.Certificates))
	}
}

func TestTransportStart_RequiresMasqueConfig(t *testing.T) {
	var tr masque.Transport
	_, err := tr.Start(context.Background(), transport.StartConfig{})
	if err == nil {
		t.Fatal("expected start config validation error")
	}
}

func TestResolveEndpointDefaultsAndIPv6(t *testing.T) {
	_, pubB64, err := masque.GenerateECDSAKeypairDER()
	if err != nil {
		t.Fatalf("GenerateECDSAKeypairDER: %v", err)
	}
	_ = pubB64
	cfg := transport.StartConfig{
		MTU:       1280,
		Addresses: []netip.Prefix{mustPrefix(t, "172.16.0.2/32")},
		Masque: &transport.MasqueConfig{
			PrivateKeyDERBase64: dummyPrivateKey(t),
			EndpointPubKeyPEM:   dummyPublicKeyPEM(t),
			EndpointV4:          "162.159.198.1:0",
			EndpointV6:          "[::1]:0",
			UseIPv6:             true,
		},
	}
	cfg = withDefaults(cfg)
	if cfg.Masque.ConnectPort != 443 {
		t.Fatalf("expected default connect port, got %d", cfg.Masque.ConnectPort)
	}
}

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
}

func dummyPrivateKey(t *testing.T) string {
	t.Helper()
	priv, _, err := masque.GenerateECDSAKeypairDER()
	if err != nil {
		t.Fatalf("GenerateECDSAKeypairDER: %v", err)
	}
	return priv
}

func dummyPublicKeyPEM(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))
}

func withDefaults(cfg transport.StartConfig) transport.StartConfig {
	mc := *cfg.Masque
	if mc.SNI == "" {
		mc.SNI = masque.DefaultSNI
	}
	if mc.ConnectURI == "" {
		mc.ConnectURI = masque.DefaultConnectURI
	}
	if mc.ConnectPort == 0 {
		mc.ConnectPort = 443
	}
	cfg.Masque = &mc
	return cfg
}

func bigOne() *big.Int { return big.NewInt(1) }

func TestParseHelpersRoundTrip(t *testing.T) {
	priv, pub, err := masque.GenerateECDSAKeypairDER()
	if err != nil {
		t.Fatalf("GenerateECDSAKeypairDER: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(priv); err != nil {
		t.Fatalf("private key is not base64: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(pub); err != nil {
		t.Fatalf("public key is not base64: %v", err)
	}
}
