package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/cloudflare"
)

func newClient(ts *httptest.Server) *cloudflare.Client {
	return cloudflare.NewClientWithBase(ts.URL, &http.Client{Timeout: 5 * time.Second})
}

func TestEnrollMasqueKey_Success(t *testing.T) {
	var sawAuth string
	var sawPatchBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&sawPatchBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"config": map[string]any{
				"peers": []any{map[string]any{
					"public_key": "-----BEGIN PUBLIC KEY-----\nABC\n-----END PUBLIC KEY-----\n",
					"endpoint":   map[string]any{"v4": "162.159.198.1", "v6": "2606:4700:100::1"},
				}},
				"interface": map[string]any{"addresses": map[string]any{"v4": "172.16.0.2", "v6": "fd01::2"}},
			},
		})
	}))
	defer ts.Close()

	result, err := newClient(ts).EnrollMasqueKey(context.Background(), "acct-1", "tok-1", "pubkey==", "edge-box")
	if err != nil {
		t.Fatalf("EnrollMasqueKey: %v", err)
	}
	if sawAuth != "Bearer tok-1" {
		t.Fatalf("expected bearer auth header, got %q", sawAuth)
	}
	if sawPatchBody["key_type"] != "secp256r1" || sawPatchBody["tunnel_type"] != "masque" {
		t.Fatalf("unexpected patch body: %#v", sawPatchBody)
	}
	if result.EndpointV4 != "162.159.198.1" || result.IPv4 != "172.16.0.2" {
		t.Fatalf("unexpected enrollment result: %+v", result)
	}
}

func TestEnrollMasqueKey_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer ts.Close()

	_, err := newClient(ts).EnrollMasqueKey(context.Background(), "acct-1", "tok-1", "pubkey==", "")
	if err == nil {
		t.Fatal("expected enrollment error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

func TestEnrollMasqueKey_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer ts.Close()

	_, err := newClient(ts).EnrollMasqueKey(context.Background(), "acct-1", "tok-1", "pubkey==", "")
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

func TestEnrollMasqueKey_MissingFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"config": map[string]any{"peers": []any{}}})
	}))
	defer ts.Close()

	_, err := newClient(ts).EnrollMasqueKey(context.Background(), "acct-1", "tok-1", "pubkey==", "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBuildMasqueState(t *testing.T) {
	state := cloudflare.BuildMasqueState("priv==", cloudflare.MasqueEnrollmentResult{
		EndpointPubKeyPEM: "pem",
		EndpointV4:        "162.159.198.1",
		EndpointV6:        "2606:4700:100::1",
		IPv4:              "172.16.0.2",
		IPv6:              "fd01::2",
	})
	if state.PrivateKeyDERBase64 != "priv==" || state.EndpointPubKeyPEM != "pem" {
		t.Fatalf("unexpected masque state: %+v", state)
	}
}
