package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
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
	for _, cmd := range []string{"register", "import", "render", "up", "down", "status", "endpoint", "version"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected help to list command %q", cmd)
		}
	}
}

func TestPlatformCheck(t *testing.T) {
	err := platformCheck()
	if runtime.GOOS == "linux" {
		if err != nil {
			t.Errorf("platformCheck should pass on linux, got: %v", err)
		}
	} else {
		if err == nil {
			t.Error("platformCheck should fail on non-linux")
		}
		if !strings.Contains(err.Error(), "unsupported platform") {
			t.Errorf("expected 'unsupported platform' in error, got: %v", err)
		}
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
