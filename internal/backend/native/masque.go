package native

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/supervisor"
)

type nativeMasqueBackend struct{}

func init() {
	backend.Register(nativeMasqueBackend{})
}

func (nativeMasqueBackend) Name() string { return state.BackendNativeMasque }

func (nativeMasqueBackend) ValidatePrereqs(_ context.Context) error { return nil }

func (nativeMasqueBackend) RenderConfig(input backend.RenderInput) (backend.RenderResult, error) {
	payload := map[string]any{
		"runtime_family": input.Settings.RuntimeFamily,
		"transport":      input.Settings.Transport,
		"mode":           input.Settings.Mode,
		"listen_host":    input.Settings.ListenHost,
		"listen_port":    input.Settings.ListenPort,
		"endpoint":       input.Settings.EndpointOverride,
	}
	if input.Account.Masque == nil {
		payload["masque_state"] = "missing"
	} else {
		payload["masque_state"] = "present"
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return backend.RenderResult{}, fmt.Errorf("marshal native backend config: %w", err)
	}
	return backend.RenderResult{ConfigJSON: data}, nil
}

func (nativeMasqueBackend) Start(ctx context.Context, result backend.RenderResult, dirs state.Dirs, foreground bool) (backend.RuntimeInfo, error) {
	configPath := dirs.BackendConfigFile()
	if err := os.WriteFile(configPath, result.ConfigJSON, 0o600); err != nil {
		return backend.RuntimeInfo{}, fmt.Errorf("write backend config: %w", err)
	}
	bin, err := os.Executable()
	if err != nil {
		return backend.RuntimeInfo{}, fmt.Errorf("locate current executable: %w", err)
	}
	rt, startErr := supervisor.Start(ctx, supervisor.StartConfig{
		Command:    []string{bin, "--state-dir", dirs.Config, "service", "run-native"},
		LogDir:     dirs.LogDir(),
		Backend:    state.BackendNativeMasque,
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

func (nativeMasqueBackend) Stop(_ context.Context, info backend.RuntimeInfo) error {
	return supervisor.Stop(state.RuntimeState{PID: info.PID})
}

func (nativeMasqueBackend) Status(_ context.Context, info backend.RuntimeInfo, settings state.Settings) (backend.BackendStatus, error) {
	return backend.BackendStatus{
		Running:        supervisor.IsRunning(info.PID),
		LocalReachable: health.ProbeLocal(settings.ListenHost, settings.ListenPort, 0),
	}, nil
}
