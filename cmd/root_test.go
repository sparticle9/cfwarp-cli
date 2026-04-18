package cmd

import (
	"bytes"
	"os"
	"path/filepath"
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
	for _, cmd := range []string{"register", "import", "registration", "transport", "mode", "stats", "render", "up", "down", "connect", "disconnect", "status", "endpoint", "completion", "version"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected help to list command %q", cmd)
		}
	}
}

func TestCompletionCommand(t *testing.T) {
	out, err := executeRoot("completion", "bash")
	if err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}
	if len(out) < 100 {
		t.Fatalf("expected non-trivial bash completion output, got %q", out)
	}

	out, err = executeRoot("completion", "zsh")
	if err != nil {
		t.Fatalf("completion zsh failed: %v", err)
	}
	if len(out) < 100 {
		t.Fatalf("expected non-trivial zsh completion output, got %q", out)
	}

	if _, err = executeRoot("completion"); err == nil {
		t.Fatalf("completion without args should fail")
	}

	if _, err = executeRoot("completion", "fish"); err == nil {
		t.Fatalf("completion for unsupported shell should fail")
	}
}

func TestCompletionInitFileOption(t *testing.T) {
	initFile := filepath.Join(t.TempDir(), ".zshrc.local")
	out, err := executeRoot("completion", "zsh", "--init-file", initFile)
	if err != nil {
		t.Fatalf("completion zsh with init file failed: %v", err)
	}
	if !strings.Contains(out, "Appended zsh completion") {
		t.Fatalf("expected confirmation output, got: %s", out)
	}

	contents, err := os.ReadFile(initFile)
	if err != nil {
		t.Fatalf("failed reading init file %s: %v", initFile, err)
	}
	if !strings.Contains(string(contents), "# cfwarp-cli completion (zsh)") {
		t.Fatalf("expected init file marker in completion content, got: %s", string(contents))
	}
	if !strings.Contains(string(contents), "#compdef cfwarp-cli") {
		t.Fatalf("expected zsh completion function registration in init file, got: %s", string(contents))
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
