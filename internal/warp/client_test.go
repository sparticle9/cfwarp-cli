package warp_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/warp"
)

// successResponse returns a minimal valid WARP registration JSON body.
func successResponse() map[string]any {
	return map[string]any{
		"id":    "acct-test-123",
		"token": "tok-test-abc",
		"account": map[string]any{
			"license": "lic-xyz",
		},
		"config": map[string]any{
			"client_id": "cid-001",
			"peers": []any{
				map[string]any{"public_key": "peerPubKey=="},
			},
			"interface": map[string]any{
				"addresses": map[string]any{
					"v4": "172.16.0.2",
					"v6": "fd01::2",
				},
			},
		},
	}
}

func newMockServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func testClient(baseURL string) *warp.Client {
	return warp.NewClientWithBase(baseURL, &http.Client{Timeout: 5 * time.Second})
}

// --- keygen ---

func TestGenerateKeypair(t *testing.T) {
	kp, err := warp.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if kp.PrivateKey == "" || kp.PublicKey == "" {
		t.Error("expected non-empty private and public keys")
	}
	if kp.PrivateKey == kp.PublicKey {
		t.Error("private and public keys must differ")
	}
	// Keys should be distinct between calls.
	kp2, _ := warp.GenerateKeypair()
	if kp.PrivateKey == kp2.PrivateKey {
		t.Error("two successive keypairs should differ")
	}
}

// --- registration success ---

func TestRegister_Success(t *testing.T) {
	srv := newMockServer(t, http.StatusOK, successResponse())
	defer srv.Close()

	result, err := testClient(srv.URL).Register(context.Background(), "myPubKey==")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.AccountID != "acct-test-123" {
		t.Errorf("expected AccountID acct-test-123, got %s", result.AccountID)
	}
	if result.Token != "tok-test-abc" {
		t.Errorf("expected Token tok-test-abc, got %s", result.Token)
	}
	if result.PeerPublicKey != "peerPubKey==" {
		t.Errorf("expected PeerPublicKey peerPubKey==, got %s", result.PeerPublicKey)
	}
	if result.IPv4 != "172.16.0.2" {
		t.Errorf("expected IPv4 172.16.0.2, got %s", result.IPv4)
	}
	if result.License != "lic-xyz" {
		t.Errorf("expected License lic-xyz, got %s", result.License)
	}
}

func TestRegister_PrefersUsablePeerEndpoint(t *testing.T) {
	resp := successResponse()
	resp["config"].(map[string]any)["peers"] = []any{
		map[string]any{
			"public_key": "peerPubKey==",
			"endpoint": map[string]any{
				"v4":    "162.159.192.7:0",
				"v6":    "[2606:4700:d0::a29f:c007]:0",
				"host":  "engage.cloudflareclient.com:2408",
				"ports": []any{2408, 500, 1701, 4500},
			},
		},
	}
	srv := newMockServer(t, http.StatusOK, resp)
	defer srv.Close()

	result, err := testClient(srv.URL).Register(context.Background(), "myPubKey==")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if result.PeerEndpoint != "engage.cloudflareclient.com:2408" {
		t.Fatalf("expected usable peer endpoint, got %q", result.PeerEndpoint)
	}
}

func TestRegister_201Created(t *testing.T) {
	srv := newMockServer(t, http.StatusCreated, successResponse())
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err != nil {
		t.Fatalf("201 should be treated as success, got: %v", err)
	}
}

// --- registration errors ---

func TestRegister_Non200(t *testing.T) {
	srv := newMockServer(t, http.StatusTooManyRequests, map[string]any{"error": "rate limited"})
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestRegister_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json {{{"))
	}))
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestRegister_MissingAccountID(t *testing.T) {
	resp := successResponse()
	delete(resp, "id")
	srv := newMockServer(t, http.StatusOK, resp)
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err == nil {
		t.Fatal("expected error when account id missing")
	}
}

func TestRegister_MissingToken(t *testing.T) {
	resp := successResponse()
	delete(resp, "token")
	srv := newMockServer(t, http.StatusOK, resp)
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err == nil {
		t.Fatal("expected error when token missing")
	}
}

func TestRegister_MissingPeerKey(t *testing.T) {
	resp := successResponse()
	resp["config"].(map[string]any)["peers"] = []any{}
	srv := newMockServer(t, http.StatusOK, resp)
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err == nil {
		t.Fatal("expected error when peer public key missing")
	}
}

func TestRegister_MissingAddresses(t *testing.T) {
	resp := successResponse()
	resp["config"].(map[string]any)["interface"] = map[string]any{
		"addresses": map[string]any{"v4": "", "v6": ""},
	}
	srv := newMockServer(t, http.StatusOK, resp)
	defer srv.Close()

	_, err := testClient(srv.URL).Register(context.Background(), "pubkey==")
	if err == nil {
		t.Fatal("expected error when addresses missing")
	}
}

func TestRegister_Timeout(t *testing.T) {
	// Use a raw listener that accepts connections but never sends HTTP responses.
	// This avoids httptest.Server.Close() blocking on the stuck handler.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			_ = conn // hold open, never respond
		}
	}()
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = testClient("http://"+ln.Addr().String()).Register(ctx, "pubkey==")
	if err == nil {
		t.Fatal("expected error on timeout/context cancellation")
	}
}
