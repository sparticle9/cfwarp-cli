// Package state manages persistent and runtime state paths and file I/O.
package state

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "cfwarp-cli"

// isContainer reports whether we are running inside a container.
// Heuristic: /run/.containerenv (Podman) or /.dockerenv (Docker).
func isContainer() bool {
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}

// Dirs holds the resolved config and runtime root directories.
type Dirs struct {
	// Config is the durable state root (account, settings, backend config).
	Config string
	// Runtime is the ephemeral runtime root (PID file, rendered config, logs).
	Runtime string
}

// Resolve returns the Dirs to use, honouring overrides and container detection.
// If configOverride or runtimeOverride are non-empty they take full precedence.
func Resolve(configOverride, runtimeOverride string) Dirs {
	var d Dirs

	if configOverride != "" {
		d.Config = configOverride
	} else if isContainer() || runtime.GOOS != "linux" {
		d.Config = "/var/lib/" + appName
	} else {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(homeDir(), ".config")
		}
		d.Config = filepath.Join(base, appName)
	}

	if runtimeOverride != "" {
		d.Runtime = runtimeOverride
	} else if configOverride != "" {
		// When --state-dir is given, co-locate runtime under the same root
		// so callers don't need a separate --runtime-dir flag.
		d.Runtime = filepath.Join(configOverride, "run")
	} else if isContainer() || runtime.GOOS != "linux" {
		d.Runtime = "/run/" + appName
	} else {
		base := os.Getenv("XDG_STATE_HOME")
		if base == "" {
			base = filepath.Join(homeDir(), ".local", "state")
		}
		d.Runtime = filepath.Join(base, appName)
	}

	return d
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "/root"
}

// AccountFile returns the path to account.json inside the config root.
func (d Dirs) AccountFile() string { return filepath.Join(d.Config, "account.json") }

// SettingsFile returns the path to settings.json inside the config root.
func (d Dirs) SettingsFile() string { return filepath.Join(d.Config, "settings.json") }

// LastGoodAccountFile returns the path to the daemon's last-known-good account snapshot.
func (d Dirs) LastGoodAccountFile() string { return filepath.Join(d.Config, "last-good-account.json") }

// RuntimeFile returns the path to runtime.json inside the runtime root.
func (d Dirs) RuntimeFile() string { return filepath.Join(d.Runtime, "runtime.json") }

// DaemonSocketFile returns the default daemon control socket path.
func (d Dirs) DaemonSocketFile() string { return filepath.Join(d.Runtime, "daemon.sock") }

// BackendConfigFile returns the path for the rendered backend config.
func (d Dirs) BackendConfigFile() string { return filepath.Join(d.Runtime, "backend.json") }

// LogDir returns the directory for backend stdout/stderr logs.
func (d Dirs) LogDir() string { return filepath.Join(d.Config, "logs") }

// MkdirAll creates both the config and runtime directories.
func (d Dirs) MkdirAll() error {
	for _, dir := range []string{d.Config, d.Runtime, d.LogDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}
