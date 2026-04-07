// Package backend defines the interface all proxy backends must satisfy.
package backend

import (
	"context"
	"time"

	"github.com/nexus/cfwarp-cli/internal/state"
)

// RenderInput is the data the renderer needs to produce a backend config.
type RenderInput struct {
	Account  state.AccountState
	Settings state.Settings
}

// RenderResult holds the output of a successful config render.
type RenderResult struct {
	// ConfigJSON is the rendered backend configuration as a JSON byte slice.
	ConfigJSON []byte
	// ConfigPath is where the config was written (empty for dry-run / stdout).
	ConfigPath string
}

// RuntimeInfo captures what is needed to manage a running backend process.
type RuntimeInfo struct {
	PID           int
	ConfigPath    string
	StdoutLogPath string
	StderrLogPath string
	StartedAt     time.Time
	LastError     string
}

// BackendStatus is the health snapshot reported by Status.
type BackendStatus struct {
	Running        bool
	LocalReachable bool
	LastError      string
}

// Backend is the interface every proxy backend must implement.
type Backend interface {
	// Name returns the unique backend identifier (e.g. "singbox-wireguard").
	Name() string
	// ValidatePrereqs checks that required binaries and permissions are present.
	ValidatePrereqs(ctx context.Context) error
	// RenderConfig produces the backend configuration from account + settings.
	RenderConfig(input RenderInput) (RenderResult, error)
	// Start launches the backend process and returns runtime metadata.
	Start(ctx context.Context, result RenderResult, dirs state.Dirs, foreground bool) (RuntimeInfo, error)
	// Stop terminates the backend process identified by info.
	Stop(ctx context.Context, info RuntimeInfo) error
	// Status reports the current health of the backend.
	Status(ctx context.Context, info RuntimeInfo, settings state.Settings) (BackendStatus, error)
}
