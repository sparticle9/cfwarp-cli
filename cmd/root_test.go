package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/nexus/cfwarp-cli/internal/state"
)

// executeRoot runs the root command with the given args and returns combined output.
// It resets the cobra command tree's output between calls.
func executeRoot(args ...string) (string, error) {
	buf := &bytes.Buffer{}
	// SetOut/SetErr propagates to all child commands via OutOrStdout()/ErrOrStderr().
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	// Reset to defaults so subsequent calls don't share state.
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return buf.String(), err
}

func TestVersion(t *testing.T) {
	out, err := executeRoot("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(out, "cfwarp-cli") {
		t.Errorf("expected version output to contain 'cfwarp-cli', got: %s", out)
	}
	if !strings.Contains(out, runtime.GOOS) {
		t.Errorf("expected version output to contain OS %q, got: %s", runtime.GOOS, out)
	}
}

func TestHelp(t *testing.T) {
	out, err := executeRoot("--help")
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	for _, cmd := range []string{"register", "import", "registration", "transport", "mode", "stats", "render", "up", "down", "connect", "disconnect", "status", "endpoint", "version"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected help to list command %q", cmd)
		}
	}
}

func TestPlatformCheck(t *testing.T) {
	err := platformCheck()
	if runtime.GOOS == "linux" || (runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") {
		if err != nil {
			t.Errorf("platformCheck should pass on supported platforms, got: %v", err)
		}
	} else {
		if err == nil {
			t.Error("platformCheck should fail on unsupported platforms")
		}
		if !strings.Contains(err.Error(), "unsupported platform") {
			t.Errorf("expected 'unsupported platform' in error, got: %v", err)
		}
	}
}

func TestPlatformCheckSettingsSupportMatrix(t *testing.T) {
	legacy := state.DefaultSettings()
	legacy.RuntimeFamily = state.RuntimeFamilyLegacy
	legacy.Transport = state.TransportWireGuard

	native := state.DefaultSettings()
	native.RuntimeFamily = state.RuntimeFamilyNative
	native.Transport = state.TransportMasque

	if runtime.GOOS == "linux" {
		if err := platformCheckSettings(legacy); err != nil {
			t.Fatalf("linux should allow legacy wireguard: %v", err)
		}
		if err := platformCheckSettings(native); err != nil {
			t.Fatalf("linux should allow native masque: %v", err)
		}
		return
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		if err := platformCheckSettings(legacy); err != nil {
			t.Fatalf("darwin arm64 should allow legacy wireguard: %v", err)
		}
		if err := platformCheckSettings(native); err == nil {
			t.Fatal("darwin arm64 should reject native masque runtime")
		}
		return
	}
	if err := platformCheckSettings(legacy); err == nil {
		t.Fatal("unsupported platforms should fail runtime matrix check")
	}
}

func TestImportRequiresFile(t *testing.T) {
	_, err := executeRoot("import")
	if err == nil {
		t.Error("import without --file should return an error")
	}
}

func TestEndpointTestRequiresArgs(t *testing.T) {
	_, err := executeRoot("endpoint", "test")
	if err == nil {
		t.Error("endpoint test without args should return an error")
	}
}

func TestStatusNoArgs(t *testing.T) {
	// status runs on all platforms (no platformCheck) and reports state.
	out, err := executeRoot("status")
	if err != nil {
		t.Fatalf("status failed unexpectedly: %v", err)
	}
	// With no state dir configured, it should report account not configured.
	if !strings.Contains(out, "configured") {
		t.Errorf("expected configured/not configured in output, got: %s", out)
	}
}
