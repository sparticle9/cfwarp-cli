package singbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
)

type singboxBackend struct{}

func init() {
	backend.Register(singboxBackend{})
}

func (singboxBackend) Name() string { return state.BackendSingboxWireGuard }

func (singboxBackend) ValidatePrereqs(ctx context.Context) error {
	return ValidatePrereqs(ctx)
}

func (singboxBackend) RenderConfig(input backend.RenderInput) (backend.RenderResult, error) {
	data, err := Render(input)
	if err != nil {
		return backend.RenderResult{}, err
	}
	return backend.RenderResult{ConfigJSON: data}, nil
}

func (singboxBackend) Start(ctx context.Context, result backend.RenderResult, dirs state.Dirs, foreground bool) (backend.RuntimeInfo, error) {
	configPath := result.ConfigPath
	if configPath == "" {
		configPath = dirs.BackendConfigFile()
	}
	if err := os.WriteFile(configPath, result.ConfigJSON, 0o600); err != nil {
		return backend.RuntimeInfo{}, fmt.Errorf("write backend config: %w", err)
	}

	singboxBin, err := exec.LookPath(binaryName)
	if err != nil {
		return backend.RuntimeInfo{}, fmt.Errorf("find %s binary: %w", binaryName, err)
	}

	rt, startErr := supervisor.Start(ctx, supervisor.StartConfig{
		Command:    []string{singboxBin, "run", "-c", configPath},
		LogDir:     dirs.LogDir(),
		Backend:    state.BackendSingboxWireGuard,
		Foreground: foreground,
	})
	return backend.RuntimeInfo{
		PID:           rt.PID,
		ConfigPath:    configPath,
		StdoutLogPath: rt.StdoutLogPath,
		StderrLogPath: rt.StderrLogPath,
		StartedAt:     rt.StartedAt,
		LastError:     rt.LastError,
	}, startErr
}

func (singboxBackend) Stop(_ context.Context, info backend.RuntimeInfo) error {
	return supervisor.Stop(state.RuntimeState{PID: info.PID})
}

func (singboxBackend) Status(_ context.Context, info backend.RuntimeInfo, settings state.Settings) (backend.BackendStatus, error) {
	return backend.BackendStatus{
		Running:        supervisor.IsRunning(info.PID),
		LocalReachable: health.ProbeLocal(settings.ListenHost, settings.ListenPort, 0),
	}, nil
}
