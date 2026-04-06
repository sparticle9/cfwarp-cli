//go:build integration

package integration_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestSmokeDockerCompose verifies the single-container deployment starts
// and the proxy port accepts connections.
// Requires: docker compose, a built image named by CFWARP_TEST_IMAGE env var
// (defaults to cfwarp-cli:test).
func TestSmokeDockerCompose(t *testing.T) {
	image := os.Getenv("CFWARP_TEST_IMAGE")
	if image == "" {
		image = "cfwarp-cli:test"
	}
	projectName := "cfwarp-smoke-" + fmt.Sprintf("%d", time.Now().Unix())
	proxyPort := freePort(t)

	// Write a minimal compose override for the smoke test
	composeYML := fmt.Sprintf(`
services:
  cfwarp:
    image: %s
    ports:
      - "127.0.0.1:%d:1080"
    environment:
      CFWARP_LISTEN_HOST: "0.0.0.0"
      CFWARP_LISTEN_PORT: "1080"
      CFWARP_PROXY_MODE: "socks5"
    volumes:
      - cfwarp-smoke-data:/var/lib/cfwarp-cli
volumes:
  cfwarp-smoke-data:
`, image, proxyPort)

	composeFile := t.TempDir() + "/docker-compose.smoke.yml"
	if err := os.WriteFile(composeFile, []byte(composeYML), 0o600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	// Start compose
	up := exec.Command("docker", "compose", "-p", projectName, "-f", composeFile, "up", "-d")
	up.Stdout = os.Stdout
	up.Stderr = os.Stderr
	if err := up.Run(); err != nil {
		t.Fatalf("docker compose up: %v", err)
	}
	t.Cleanup(func() {
		down := exec.Command("docker", "compose", "-p", projectName, "-f", composeFile, "down", "-v")
		down.Stdout = os.Stdout
		down.Stderr = os.Stderr
		_ = down.Run()
	})

	// Wait for proxy port to open (up to 60s — registration takes time)
	t.Log("Waiting for proxy port to open…")
	if !waitForPort(t, "127.0.0.1", proxyPort, 60*time.Second) {
		t.Fatal("proxy port did not open within 60s")
	}
	t.Logf("Proxy port %d is open ✓", proxyPort)

	// Optional live trace check (only if CFWARP_TEST_LIVE=1)
	if os.Getenv("CFWARP_TEST_LIVE") == "1" {
		t.Log("Running live cdn-cgi/trace check…")
		checkTrace(t, proxyPort)
	}
}

func waitForPort(t *testing.T, host string, port int, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func checkTrace(t *testing.T, proxyPort int) {
	t.Helper()
	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", proxyPort)
	transport := &http.Transport{
		Proxy: func(r *http.Request) (*http.URL, error) {
			// Note: SOCKS5 support needs golang.org/x/net — use HTTP CONNECT style for this check
			return nil, nil // skip if complex; just verify port is reachable
		},
	}
	_ = transport
	// For the live check, just verify warp=on appears via socks5 proxy.
	// This requires curl or golang.org/x/net; use curl subprocess for simplicity.
	out, err := exec.Command("curl", "-fsSL", "--proxy", fmt.Sprintf("socks5h://127.0.0.1:%d", proxyPort),
		"https://www.cloudflare.com/cdn-cgi/trace").Output()
	if err != nil {
		t.Errorf("curl through proxy failed: %v", err)
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if scanner.Text() == "warp=on" {
			t.Log("warp=on confirmed ✓")
			return
		}
	}
	t.Errorf("warp=on not found in trace output:\n%s", out)
}
