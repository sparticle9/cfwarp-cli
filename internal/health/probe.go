// Package health provides lightweight probes for the proxy's local and remote status.
package health

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	localProbeTimeout = 2 * time.Second
	traceProbeTimeout = 15 * time.Second
	traceURL          = "https://www.cloudflare.com/cdn-cgi/trace"
)

// ProbeLocal attempts a TCP connection to host:port.
// Returns true if the port accepts connections within the timeout.
func ProbeLocal(host string, port int, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = localProbeTimeout
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// TraceResult holds the parsed outcome of a cdn-cgi/trace probe.
type TraceResult struct {
	WARPOn bool
	// Raw key=value pairs from the trace response (for diagnostics).
	Fields map[string]string
}

// ProbeTrace fetches https://www.cloudflare.com/cdn-cgi/trace through the
// configured proxy and reports whether warp=on appears in the response.
//
// proxyMode must be "socks5" or "http".
// proxyAddr must be "host:port".
// username/password may be empty for no-auth proxies.
func ProbeTrace(ctx context.Context, proxyMode, proxyAddr, username, password string) (TraceResult, error) {
	return ProbeTraceURL(ctx, proxyMode, proxyAddr, username, password, traceURL)
}

// ProbeTraceURL is like ProbeTrace but fetches a custom URL.
// Used in tests to avoid real network calls.
func ProbeTraceURL(ctx context.Context, proxyMode, proxyAddr, username, password, targetURL string) (TraceResult, error) {
	transport, err := buildTransport(proxyMode, proxyAddr, username, password)
	if err != nil {
		return TraceResult{}, fmt.Errorf("build proxy transport: %w", err)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   traceProbeTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return TraceResult{}, fmt.Errorf("build trace request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return TraceResult{}, fmt.Errorf("trace request failed: %w", err)
	}
	defer resp.Body.Close()

	fields, err := parseTraceBody(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return TraceResult{}, fmt.Errorf("parse trace response: %w", err)
	}

	return TraceResult{
		WARPOn: fields["warp"] == "on",
		Fields: fields,
	}, nil
}

// buildTransport creates an *http.Transport that routes through the proxy.
func buildTransport(mode, addr, username, password string) (*http.Transport, error) {
	switch mode {
	case "socks5":
		var auth *proxy.Auth
		if username != "" {
			auth = &proxy.Auth{User: username, Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("create SOCKS5 dialer: %w", err)
		}
		dc, ok := dialer.(proxy.ContextDialer)
		if ok {
			return &http.Transport{DialContext: dc.DialContext}, nil
		}
		return &http.Transport{
			DialContext: func(ctx context.Context, network, a string) (net.Conn, error) {
				return dialer.Dial(network, a)
			},
		}, nil

	case "http":
		u := &url.URL{Scheme: "http", Host: addr}
		if username != "" {
			u.User = url.UserPassword(username, password)
		}
		return &http.Transport{Proxy: http.ProxyURL(u)}, nil

	default:
		return nil, fmt.Errorf("unsupported proxy mode %q for trace probe", mode)
	}
}

// parseTraceBody reads the cdn-cgi/trace key=value response into a map.
func parseTraceBody(r io.Reader) (map[string]string, error) {
	fields := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		if k != "" {
			fields[k] = v
		}
	}
	return fields, scanner.Err()
}
