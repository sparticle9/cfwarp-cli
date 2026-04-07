package cmd

import (
	"github.com/nexus/cfwarp-cli/internal/backend"
	"github.com/nexus/cfwarp-cli/internal/state"
)

func configuredBackend(sett state.Settings) (backend.Backend, error) {
	sett.Normalize()
	return backend.Lookup(sett.Backend)
}

func runtimeBackend(rt state.RuntimeState) (backend.Backend, error) {
	return backend.Lookup(rt.Backend)
}

func runtimeInfo(rt state.RuntimeState) backend.RuntimeInfo {
	return backend.RuntimeInfo{
		PID:           rt.PID,
		ConfigPath:    rt.ConfigPath,
		StdoutLogPath: rt.StdoutLogPath,
		StderrLogPath: rt.StderrLogPath,
	}
}
