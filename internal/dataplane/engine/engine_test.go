package engine_test

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/dataplane/engine"
	"github.com/nexus/cfwarp-cli/internal/transport"
)

type fakeStack struct {
	mu        sync.Mutex
	reads     chan []byte
	writes    [][]byte
	readErr   error
	writeErr  error
	closeErr  error
	closed    bool
	dialFn    func(ctx context.Context, network, addr string) (net.Conn, error)
	resolveFn func(ctx context.Context, host string) ([]net.IP, error)
}

func newFakeStack() *fakeStack {
	return &fakeStack{reads: make(chan []byte, 8)}
}

func (f *fakeStack) ReadPacket(buf []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	pkt, ok := <-f.reads
	if !ok {
		return 0, io.EOF
	}
	n := copy(buf, pkt)
	return n, nil
}

func (f *fakeStack) WritePacket(pkt []byte) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, append([]byte(nil), pkt...))
	return nil
}

func (f *fakeStack) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	close(f.reads)
	return f.closeErr
}

func (f *fakeStack) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if f.dialFn != nil {
		return f.dialFn(ctx, network, addr)
	}
	c1, c2 := net.Pipe()
	go c2.Close()
	return c1, nil
}

func (f *fakeStack) ResolveIP(ctx context.Context, host string) ([]net.IP, error) {
	if f.resolveFn != nil {
		return f.resolveFn(ctx, host)
	}
	return []net.IP{net.ParseIP("127.0.0.1")}, nil
}

func (f *fakeStack) WritesLen() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.writes)
}

func TestEngine_ForwardsPacketsBothWays(t *testing.T) {
	stack := newFakeStack()
	tun := transport.NewFakeTunnel(1280, nil)
	tun.QueueReadPacket([]byte{0xaa, 0xbb})
	stack.reads <- []byte{0x01, 0x02, 0x03}

	e := engine.New(stack, tun)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	deadline := time.After(2 * time.Second)
	for {
		if len(tun.WrittenPackets()) == 1 && stack.WritesLen() == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for bidirectional forwarding; tunnel writes=%v stack writes=%d", tun.WrittenPackets(), stack.WritesLen())
		case <-time.After(10 * time.Millisecond):
		}
	}

	snap := e.Snapshot()
	if snap.ForwarderStats.StackToTunnel.Packets != 1 || snap.ForwarderStats.StackToTunnel.Bytes != 3 {
		t.Fatalf("unexpected stack->tunnel stats: %+v", snap.ForwarderStats.StackToTunnel)
	}
	if snap.ForwarderStats.TunnelToStack.Packets != 1 || snap.ForwarderStats.TunnelToStack.Bytes != 2 {
		t.Fatalf("unexpected tunnel->stack stats: %+v", snap.ForwarderStats.TunnelToStack)
	}
	if snap.ForwarderStats.StackToTunnel.ReadCalls != 1 || snap.ForwarderStats.StackToTunnel.WriteCalls != 1 {
		t.Fatalf("expected stack->tunnel read/write calls to be recorded, got %+v", snap.ForwarderStats.StackToTunnel)
	}
	if snap.ForwarderStats.TunnelToStack.ReadCalls != 1 || snap.ForwarderStats.TunnelToStack.WriteCalls != 1 {
		t.Fatalf("expected tunnel->stack read/write calls to be recorded, got %+v", snap.ForwarderStats.TunnelToStack)
	}
}

func TestEngine_ReportsForwardingErrors(t *testing.T) {
	stack := newFakeStack()
	stack.readErr = errors.New("boom")
	tun := transport.NewFakeTunnel(1280, nil)
	e := engine.New(stack, tun)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Start(ctx)

	select {
	case err := <-e.Errors():
		if err == nil || err.Error() == "" {
			t.Fatal("expected non-empty forwarding error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for engine error")
	}
}

func TestEngine_DialAndResolveDelegateToStack(t *testing.T) {
	stack := newFakeStack()
	stack.resolveFn = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.1")}, nil
	}
	tun := transport.NewFakeTunnel(1280, nil)
	e := engine.New(stack, tun)

	conn, err := e.DialContext(context.Background(), "tcp", "example.com:80")
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	_ = conn.Close()

	ips, err := e.ResolveIP(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("ResolveIP: %v", err)
	}
	if len(ips) != 1 || ips[0].String() != "10.0.0.1" {
		t.Fatalf("unexpected resolved IPs: %v", ips)
	}
}

func TestEngine_SnapshotIncludesObservedEvent(t *testing.T) {
	stack := newFakeStack()
	tun := transport.NewFakeTunnel(1280, nil)
	e := engine.New(stack, tun)
	ev := transport.Event{At: time.Now().UTC(), Level: "info", Type: "connected", Message: "hello"}
	e.ObserveEvent(ev)

	snap := e.Snapshot()
	if snap.RecentEvent == nil || snap.RecentEvent.Type != ev.Type || snap.RecentEvent.Message != ev.Message {
		t.Fatalf("unexpected recent event snapshot: %+v", snap.RecentEvent)
	}
}

func TestEngine_CloseClosesStackAndTunnel(t *testing.T) {
	stack := newFakeStack()
	tun := transport.NewFakeTunnel(1280, nil)
	e := engine.New(stack, tun)
	if err := e.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	stack.mu.Lock()
	closed := stack.closed
	stack.mu.Unlock()
	if !closed {
		t.Fatal("expected stack to be closed")
	}
	if !tun.Closed() {
		t.Fatal("expected tunnel to be closed")
	}
}
