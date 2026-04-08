package cmd

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nexus/cfwarp-cli/internal/dataplane/engine"
	httpproxy "github.com/nexus/cfwarp-cli/internal/dataplane/frontend/http"
	"github.com/nexus/cfwarp-cli/internal/dataplane/frontend/socks"
	netstackdp "github.com/nexus/cfwarp-cli/internal/dataplane/netstack"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/transport"
	masquetransport "github.com/nexus/cfwarp-cli/internal/transport/masque"
	"github.com/spf13/cobra"
)

func runNativeRuntime(ctx context.Context, dirs state.Dirs, cmd *cobra.Command) error {
	acc, err := state.LoadAccount(dirs)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	sett, err := resolveSettings(cmd, dirs)
	if err != nil {
		return fmt.Errorf("resolve settings: %w", err)
	}
	if sett.RuntimeFamily != state.RuntimeFamilyNative || sett.Transport != state.TransportMasque {
		return fmt.Errorf("service run-native currently supports only runtime_family=%q transport=%q", state.RuntimeFamilyNative, state.TransportMasque)
	}
	if acc.Masque == nil {
		return fmt.Errorf("account has no MASQUE state; rerun register with --masque or import MASQUE credentials")
	}

	cfg, err := buildMasqueStartConfig(acc, sett)
	if err != nil {
		return err
	}
	stack, err := netstackdp.New(cfg.Addresses, cfg.MTU)
	if err != nil {
		return err
	}
	defer stack.Close()

	var tr masquetransport.Transport
	tun, err := tr.Start(ctx, cfg)
	if err != nil {
		return err
	}
	defer tun.Close()

	eng := engine.New(stack, tun)
	eng.Start(ctx)
	defer eng.Close()

	listenAddr := fmt.Sprintf("%s:%d", sett.ListenHost, sett.ListenPort)
	var closeFn func() error
	serveErrCh := make(chan error, 1)
	switch sett.Mode {
	case state.ModeSocks5:
		socksCfg := socks.Config{ListenAddr: listenAddr, Username: sett.ProxyUsername, Password: sett.ProxyPassword}
		srv := socks.New(socksCfg, stack)
		closeFn = srv.Close
		go func() { serveErrCh <- srv.ListenAndServe(socksCfg) }()
	case state.ModeHTTP:
		srv := httpproxy.New(httpproxy.Config{ListenAddr: listenAddr, Username: sett.ProxyUsername, Password: sett.ProxyPassword}, stack)
		closeFn = srv.Close
		go func() { serveErrCh <- srv.ListenAndServe() }()
	default:
		return fmt.Errorf("native MASQUE mode %q is not yet supported", sett.Mode)
	}
	defer func() {
		if closeFn != nil {
			_ = closeFn()
		}
	}()

	socketPath := orchestrator.ServiceSocketPath(dirs)
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(socketPath, []byte("native-masque"), 0o600); err != nil {
		return err
	}
	defer os.Remove(socketPath)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case sig := <-sigCh:
		return fmt.Errorf("received signal %s", sig)
	case err := <-eng.Errors():
		return err
	case err := <-serveErrCh:
		return err
	}
}

func buildMasqueStartConfig(acc state.AccountState, sett state.Settings) (transport.StartConfig, error) {
	addrs, err := localPrefixes(acc.Masque.IPv4, acc.Masque.IPv6)
	if err != nil {
		return transport.StartConfig{}, err
	}
	mtu := 1280
	if sett.MasqueOptions != nil && sett.MasqueOptions.MTU > 0 {
		mtu = sett.MasqueOptions.MTU
	}
	cfg := transport.StartConfig{
		MTU:              mtu,
		EndpointOverride: sett.EndpointOverride,
		Addresses:        addrs,
		Masque: &transport.MasqueConfig{
			PrivateKeyDERBase64: acc.Masque.PrivateKeyDERBase64,
			EndpointPubKeyPEM:   acc.Masque.EndpointPubKeyPEM,
			EndpointV4:          acc.Masque.EndpointV4,
			EndpointV6:          acc.Masque.EndpointV6,
		},
	}
	if sett.MasqueOptions != nil {
		cfg.Masque.SNI = sett.MasqueOptions.SNI
		cfg.Masque.ConnectPort = sett.MasqueOptions.ConnectPort
		cfg.Masque.UseIPv6 = sett.MasqueOptions.UseIPv6
		cfg.Masque.InitialPacketSize = sett.MasqueOptions.InitialPacketSize
		cfg.Masque.KeepAlivePeriod = durationSeconds(sett.MasqueOptions.KeepAlivePeriodSeconds)
		cfg.Masque.ReconnectDelay = durationMillis(sett.MasqueOptions.ReconnectDelayMillis)
	}
	return cfg, nil
}

func localPrefixes(ipv4, ipv6 string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, 2)
	if ipv4 != "" {
		p, err := parsePrefix(ipv4, 32)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, p)
	}
	if ipv6 != "" {
		p, err := parsePrefix(ipv6, 128)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, p)
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("MASQUE account has no tunnel addresses")
	}
	return prefixes, nil
}

func parsePrefix(raw string, bits int) (netip.Prefix, error) {
	if p, err := netip.ParsePrefix(raw); err == nil {
		return p, nil
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("parse tunnel address %q: %w", raw, err)
	}
	return netip.PrefixFrom(addr, bits), nil
}

func durationSeconds(sec int) time.Duration {
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

func durationMillis(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}
