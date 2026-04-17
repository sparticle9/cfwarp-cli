package cmd

import (
	"fmt"
	"net"

	"github.com/nexus/cfwarp-cli/internal/caps"
	"github.com/nexus/cfwarp-cli/internal/state"
)

func firstProxyAccess(sett state.Settings) (state.AccessConfig, error) {
	sett.Normalize()
	for _, access := range sett.Access {
		switch access.Type {
		case state.ModeSocks5, state.ModeHTTP:
			return access, nil
		}
	}
	return state.AccessConfig{}, fmt.Errorf("no proxy access configured; at least one socks5 or http access entry is required")
}

func probeTargetFromSettings(sett state.Settings) (caps.ProbeTarget, error) {
	access, err := firstProxyAccess(sett)
	if err != nil {
		return caps.ProbeTarget{}, err
	}
	return caps.ProbeTarget{
		Type:     access.Type,
		Address:  net.JoinHostPort(access.ListenHost, fmt.Sprintf("%d", access.ListenPort)),
		Username: access.Username,
		Password: access.Password,
	}, nil
}
