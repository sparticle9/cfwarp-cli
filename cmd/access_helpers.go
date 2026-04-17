package cmd

import (
	"fmt"
	"net"
	"strings"

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
	host := access.ListenHost
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::", "[::]":
		host = "::1"
	}
	return caps.ProbeTarget{
		Type:     access.Type,
		Address:  net.JoinHostPort(host, fmt.Sprintf("%d", access.ListenPort)),
		Username: access.Username,
		Password: access.Password,
	}, nil
}
