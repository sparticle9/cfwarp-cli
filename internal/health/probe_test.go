package health_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/health"
)

// --- ProbeLocal ---

func TestProbeLocal_Open(t *testing.T) {
	// Start a TCP listener to represent an open proxy port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	if !health.ProbeLocal(host, port, time.Second) {
		t.Error("ProbeLocal should return true for an open port")
	}
}

func TestProbeLocal_Closed(t *testing.T) {
	// Use a port that is definitely not listening.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close() // close immediately so the port is free

	host, portStr, _ := net.SplitHostPort(addr)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	if health.ProbeLocal(host, port, 200*time.Millisecond) {
		t.Error("ProbeLocal should return false for a closed port")
	}
}

// --- ProbeTrace (HTTP proxy mode against a mock server) ---

func traceBody(warpOn bool) string {
	warpVal := "off"
	if warpOn {
		warpVal = "on"
	}
	return fmt.Sprintf("fl=abc\nip=1.2.3.4\nts=1234567890\nvisit_scheme=https\nuagent=Go\ncolo=SJC\nsliver=none\ncountry=US\nwarp=%s\ngwarp=off\n", warpVal)
}

// mockHTTPProxy starts an HTTP proxy that forwards requests to the given backend.
// For simplicity, it only handles CONNECT and plain HTTP to our test server.
func mockHTTPProxy(t *testing.T, backend *httptest.Server) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple forwarding: strip the scheme and re-issue against backend.
		backendURL := backend.URL + r.URL.Path
		resp, err := http.Get(backendURL) //nolint:noctx
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			fmt.Fprintln(w, scanner.Text())
		}
	}))
}

func TestProbeTrace_WARPOn(t *testing.T) {
	// Backend that simulates cdn-cgi/trace with warp=on.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, traceBody(true))
	}))
	defer backend.Close()

	proxy := mockHTTPProxy(t, backend)
	defer proxy.Close()

	// Override traceURL via the http-proxy path; the proxy will forward /cdn-cgi/trace.
	result, err := health.ProbeTraceURL(context.Background(), "http", proxyAddr(proxy), "", "", backend.URL+"/cdn-cgi/trace")
	if err != nil {
		t.Fatalf("ProbeTrace: %v", err)
	}
	if !result.WARPOn {
		t.Error("expected WARPOn=true")
	}
	if result.Fields["ip"] != "1.2.3.4" {
		t.Errorf("expected fields parsed, got %v", result.Fields)
	}
}

func TestProbeTrace_WARPOff(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, traceBody(false))
	}))
	defer backend.Close()

	proxy := mockHTTPProxy(t, backend)
	defer proxy.Close()

	result, err := health.ProbeTraceURL(context.Background(), "http", proxyAddr(proxy), "", "", backend.URL+"/cdn-cgi/trace")
	if err != nil {
		t.Fatalf("ProbeTrace: %v", err)
	}
	if result.WARPOn {
		t.Error("expected WARPOn=false")
	}
}

func TestProbeTrace_ProxyUnreachable(t *testing.T) {
	// Use a closed port.
	_, err := health.ProbeTraceURL(context.Background(), "http", "127.0.0.1:1", "", "", "http://example.com/trace")
	if err == nil {
		t.Error("expected error when proxy is unreachable")
	}
}

func TestProbeTrace_ContextCancel(t *testing.T) {
	// Server that never responds.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn
		}
	}()
	defer ln.Close()

	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer proxy.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := health.ProbeTraceURL(ctx, "http", proxyAddr(proxy), "", "", "http://"+ln.Addr().String())
	if err == nil {
		t.Error("expected error on context cancellation")
	}
}

func TestProbeTrace_InvalidMode(t *testing.T) {
	_, err := health.ProbeTraceURL(context.Background(), "vmess", "127.0.0.1:1080", "", "", "http://x.com")
	if err == nil || !strings.Contains(err.Error(), "unsupported proxy mode") {
		t.Errorf("expected unsupported proxy mode error, got: %v", err)
	}
}

func proxyAddr(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "http://")
}
