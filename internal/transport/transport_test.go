package transport_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/transport"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("ParsePrefix(%q): %v", s, err)
	}
	return p
}

func TestFakeTransport_StartCapturesConfigAndReturnsTunnel(t *testing.T) {
	tun := transport.NewFakeTunnel(1280, []netip.Prefix{mustPrefix(t, "172.16.0.2/32")})
	tr := &transport.FakeTransport{
		NameValue:         "fake-masque",
		CapabilitiesValue: transport.Capabilities{SupportsSocks5: true, SupportsHTTP: true},
		Tunnel:            tun,
	}

	cfg := transport.StartConfig{
		MTU:              1280,
		EndpointOverride: "162.159.192.1:443",
		Addresses:        []netip.Prefix{mustPrefix(t, "172.16.0.2/32")},
	}
	got, err := tr.Start(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got != tun {
		t.Fatal("expected fake transport to return the configured tunnel")
	}
	if tr.Name() != "fake-masque" {
		t.Fatalf("unexpected transport name %q", tr.Name())
	}
	if len(tr.StartedWith) != 1 {
		t.Fatalf("expected 1 captured start config, got %d", len(tr.StartedWith))
	}
	if tr.StartedWith[0].EndpointOverride != cfg.EndpointOverride || tr.StartedWith[0].MTU != 1280 {
		t.Fatalf("unexpected captured config: %+v", tr.StartedWith[0])
	}
}

func TestFakeTransport_StartError(t *testing.T) {
	tr := &transport.FakeTransport{StartErr: errors.New("boom")}
	if _, err := tr.Start(context.Background(), transport.StartConfig{}); err == nil {
		t.Fatal("expected start error")
	}
}

func TestFakeTunnel_ReadWriteStatsAndClose(t *testing.T) {
	tun := transport.NewFakeTunnel(1280, []netip.Prefix{mustPrefix(t, "172.16.0.2/32"), mustPrefix(t, "fd01::2/128")})
	tun.QueueReadPacket([]byte{0x01, 0x02, 0x03, 0x04})

	buf := make([]byte, 32)
	n, err := tun.ReadPacket(buf)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if n != 4 {
		t.Fatalf("expected to read 4 bytes, got %d", n)
	}
	if err := tun.WritePacket([]byte{0xaa, 0xbb}); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	stats := tun.Stats()
	if stats.PacketsRead != 1 || stats.PacketsWritten != 1 {
		t.Fatalf("unexpected packet counts: %+v", stats)
	}
	if stats.BytesRead != 4 || stats.BytesWritten != 2 {
		t.Fatalf("unexpected byte counts: %+v", stats)
	}
	if stats.LastActivityAt.IsZero() {
		t.Fatal("expected LastActivityAt to be populated")
	}
	if got := tun.WrittenPackets(); len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("unexpected written packets snapshot: %v", got)
	}
	if tun.MTU() != 1280 {
		t.Fatalf("unexpected MTU %d", tun.MTU())
	}
	if addrs := tun.Addresses(); len(addrs) != 2 {
		t.Fatalf("unexpected addresses: %v", addrs)
	}

	if err := tun.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !tun.Closed() {
		t.Fatal("expected tunnel to report closed")
	}
	if _, err := tun.ReadPacket(buf); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("expected ErrClosed after close, got %v", err)
	}
	if err := tun.WritePacket([]byte{0x01}); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("expected ErrClosed on write after close, got %v", err)
	}
}

func TestFakeTunnel_Events(t *testing.T) {
	tun := transport.NewFakeTunnel(1400, nil)
	ev := transport.Event{At: time.Now().UTC(), Level: "info", Type: "connected", Message: "hello"}
	tun.EmitEvent(ev)

	select {
	case got := <-tun.Events():
		if got.Type != ev.Type || got.Message != ev.Message {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}
