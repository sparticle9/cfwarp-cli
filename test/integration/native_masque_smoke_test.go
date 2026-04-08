//go:build integration

package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestSmokeDockerComposeNativeMasqueHTTP verifies the experimental native MASQUE
// HTTP proxy mode starts in a compose deployment and reports warp=on.
// This test is intentionally opt-in because it performs live registration and a
// real remote trace request.
func TestSmokeDockerComposeNativeMasqueHTTP(t *testing.T) {
	if os.Getenv("CFWARP_TEST_LIVE") != "1" || os.Getenv("CFWARP_TEST_NATIVE") != "1" {
		t.Skip("set CFWARP_TEST_LIVE=1 and CFWARP_TEST_NATIVE=1 to enable native MASQUE integration smoke test")
	}
	image := os.Getenv("CFWARP_TEST_IMAGE")
	if image == "" {
		image = "cfwarp-cli:test"
	}
	projectName := "cfwarp-native-smoke-" + fmt.Sprintf("%d", time.Now().Unix())
	proxyPort := freePort(t)

	composeYML := fmt.Sprintf(`
services:
  cfwarp:
    image: %s
    ports:
      - "127.0.0.1:%d:28080"
    environment:
      CFWARP_RUNTIME_FAMILY: "native"
      CFWARP_TRANSPORT: "masque"
      CFWARP_MODE: "http"
      CFWARP_LISTEN_HOST: "0.0.0.0"
      CFWARP_LISTEN_PORT: "28080"
      CFWARP_STATE_DIR: "/home/cfwarp/.local/state/cfwarp-cli"
    volumes:
      - cfwarp-native-smoke-data:/home/cfwarp/.local/state/cfwarp-cli
volumes:
  cfwarp-native-smoke-data:
`, image, proxyPort)

	composeFile := t.TempDir() + "/docker-compose.native-smoke.yml"
	if err := os.WriteFile(composeFile, []byte(composeYML), 0o600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

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

	if !waitForPort(t, "127.0.0.1", proxyPort, 90*time.Second) {
		t.Fatal("native MASQUE HTTP proxy port did not open within 90s")
	}

	out, err := exec.Command("curl", "-fsSL", "-x", fmt.Sprintf("http://127.0.0.1:%d", proxyPort), "https://www.cloudflare.com/cdn-cgi/trace").Output()
	if err != nil {
		t.Fatalf("curl through native MASQUE HTTP proxy failed: %v", err)
	}
	if string(out) == "" || !containsWarpOn(string(out)) {
		t.Fatalf("expected warp=on in trace output, got:\n%s", out)
	}
}

func containsWarpOn(s string) bool {
	return containsLine(s, "warp=on")
}

func containsLine(body, line string) bool {
	for _, candidate := range strings.Split(body, "\n") {
		if candidate == line {
			return true
		}
	}
	return false
}
