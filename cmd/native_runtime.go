package cmd

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nexus/cfwarp-cli/internal/dataplane/engine"
	httpproxy "github.com/nexus/cfwarp-cli/internal/dataplane/frontend/http"
	"github.com/nexus/cfwarp-cli/internal/dataplane/frontend/socks"
	netstackdp "github.com/nexus/cfwarp-cli/internal/dataplane/netstack"
	"github.com/nexus/cfwarp-cli/internal/health"
	"github.com/nexus/cfwarp-cli/internal/orchestrator"
	"github.com/nexus/cfwarp-cli/internal/state"
	"github.com/nexus/cfwarp-cli/internal/transport"
	masquetransport "github.com/nexus/cfwarp-cli/internal/transport/masque"
	"github.com/spf13/cobra"
)

const runtimeSnapshotInterval = 5 * time.Second

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
	tracker := newRuntimeTracker(dirs, sett)
	tracker.persistSnapshot(eng, stack)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-tun.Events():
				if !ok {
					return
				}
				eng.ObserveEvent(ev)
				tracker.recordEvent(ev)
				log.Printf("transport event level=%s type=%s msg=%s", ev.Level, ev.Type, ev.Message)
			}
		}
	}()

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

	tracker.setServiceSocket(socketPath)

	go func() {
		ticker := time.NewTicker(runtimeSnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tracker.persistSnapshot(eng, stack)
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var exitErr error
	select {
	case <-ctx.Done():
		exitErr = ctx.Err()
	case sig := <-sigCh:
		exitErr = fmt.Errorf("received signal %s", sig)
	case err := <-eng.Errors():
		exitErr = err
	case err := <-serveErrCh:
		exitErr = err
	}

	tracker.stop(exitErr)
	return exitErr
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

type runtimeTracker struct {
	dirs state.Dirs
	mu   sync.Mutex
	rt   state.RuntimeState
}

func newRuntimeTracker(dirs state.Dirs, sett state.Settings) *runtimeTracker {
	rt, err := state.LoadRuntime(dirs)
	if err != nil {
		rt = state.RuntimeState{}
	}
	rt.Normalize()
	rt.Backend = sett.Backend
	rt.RuntimeFamily = sett.RuntimeFamily
	rt.Transport = sett.Transport
	rt.Mode = sett.Mode
	rt.ListenHost = sett.ListenHost
	rt.ListenPort = sett.ListenPort
	if rt.Phase == "" || rt.Phase == state.RuntimePhaseIdle {
		rt.Phase = state.RuntimePhaseConnecting
	}
	return &runtimeTracker{dirs: dirs, rt: rt}
}

func (t *runtimeTracker) setServiceSocket(path string) {
	t.update(func(rt *state.RuntimeState) {
		rt.ServiceSocketPath = path
	})
}

func (t *runtimeTracker) recordEvent(ev transport.Event) {
	t.update(func(rt *state.RuntimeState) {
		if rt.Diagnostics == nil {
			rt.Diagnostics = &state.RuntimeDiagnostics{}
		}
		rt.Diagnostics.LastEvent = &state.RuntimeEventSnapshot{
			At:      ev.At,
			Level:   ev.Level,
			Type:    ev.Type,
			Message: ev.Message,
		}
		switch ev.Type {
		case "endpoint_selected":
			rt.SelectedAddressFam = parseEventField(ev.Message, "family")
			rt.SelectedEndpoint = parseEventField(ev.Message, "addr")
		case "retry", "reconnect":
			orchestrator.MarkTransportError(rt, ev.Message, ev.At)
		case "connected", "reconnected":
			rt.LastTransportError = ""
			if rt.LocalReachable {
				rt.Phase = state.RuntimePhaseConnected
			} else {
				rt.Phase = state.RuntimePhaseConnecting
			}
		}
	})
}

func (t *runtimeTracker) persistSnapshot(eng *engine.Engine, stack *netstackdp.Stack) {
	snap := eng.Snapshot()
	netStats := stack.Stats()
	t.update(func(rt *state.RuntimeState) {
		if rt.Diagnostics == nil {
			rt.Diagnostics = &state.RuntimeDiagnostics{}
		}
		rt.LocalReachable = health.ProbeLocal(rt.ListenHost, rt.ListenPort, 0)
		if rt.LastTransportError != "" {
			rt.Phase = state.RuntimePhaseDegraded
		} else if rt.LocalReachable {
			rt.Phase = state.RuntimePhaseConnected
		} else if rt.Phase == "" || rt.Phase == state.RuntimePhaseIdle {
			rt.Phase = state.RuntimePhaseConnecting
		}
		rt.Diagnostics.CapturedAt = time.Now().UTC()
		rt.Diagnostics.Transport = state.TransportStatsSnapshot{
			PacketsRead:    snap.TransportStats.PacketsRead,
			PacketsWritten: snap.TransportStats.PacketsWritten,
			BytesRead:      snap.TransportStats.BytesRead,
			BytesWritten:   snap.TransportStats.BytesWritten,
			LastActivityAt: snap.TransportStats.LastActivityAt,
		}
		rt.Diagnostics.StackToTunnel = state.PacketPathStats{
			Packets:      snap.ForwarderStats.StackToTunnel.Packets,
			Bytes:        snap.ForwarderStats.StackToTunnel.Bytes,
			ReadCalls:    snap.ForwarderStats.StackToTunnel.ReadCalls,
			ReadNanos:    snap.ForwarderStats.StackToTunnel.ReadNanos,
			WriteCalls:   snap.ForwarderStats.StackToTunnel.WriteCalls,
			WriteNanos:   snap.ForwarderStats.StackToTunnel.WriteNanos,
			LastPacketAt: snap.ForwarderStats.StackToTunnel.LastPacketAt,
		}
		rt.Diagnostics.TunnelToStack = state.PacketPathStats{
			Packets:      snap.ForwarderStats.TunnelToStack.Packets,
			Bytes:        snap.ForwarderStats.TunnelToStack.Bytes,
			ReadCalls:    snap.ForwarderStats.TunnelToStack.ReadCalls,
			ReadNanos:    snap.ForwarderStats.TunnelToStack.ReadNanos,
			WriteCalls:   snap.ForwarderStats.TunnelToStack.WriteCalls,
			WriteNanos:   snap.ForwarderStats.TunnelToStack.WriteNanos,
			LastPacketAt: snap.ForwarderStats.TunnelToStack.LastPacketAt,
		}
		rt.Diagnostics.Netstack = state.PacketPathStats{
			Packets:      netStats.Packets,
			Bytes:        netStats.Bytes,
			ReadCalls:    netStats.ReadCalls,
			ReadNanos:    netStats.ReadNanos,
			WriteCalls:   netStats.WriteCalls,
			WriteNanos:   netStats.WriteNanos,
			LastPacketAt: netStats.LastPacketAt,
		}
		if snap.RecentEvent != nil {
			rt.Diagnostics.LastEvent = &state.RuntimeEventSnapshot{
				At:      snap.RecentEvent.At,
				Level:   snap.RecentEvent.Level,
				Type:    snap.RecentEvent.Type,
				Message: snap.RecentEvent.Message,
			}
		}
	})
}

func (t *runtimeTracker) stop(err error) {
	t.update(func(rt *state.RuntimeState) {
		orchestrator.MarkStopped(rt, errorString(err))
	})
}

func (t *runtimeTracker) update(fn func(*state.RuntimeState)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(&t.rt)
	t.rt.Normalize()
	if err := state.SaveRuntime(t.dirs, t.rt); err != nil {
		log.Printf("warning: persist runtime state: %v", err)
	}
}

func parseEventField(msg, key string) string {
	prefix := key + "="
	for _, part := range strings.Fields(msg) {
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
