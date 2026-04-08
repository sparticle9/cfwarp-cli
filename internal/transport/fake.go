package transport

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"sync"
	"time"
)

var ErrClosed = errors.New("transport tunnel closed")

// FakeTransport is a deterministic test double for PacketTransport.
type FakeTransport struct {
	NameValue         string
	CapabilitiesValue Capabilities
	Tunnel            PacketTunnel
	StartErr          error

	mu          sync.Mutex
	StartedWith []StartConfig
}

func (f *FakeTransport) Name() string {
	if f.NameValue != "" {
		return f.NameValue
	}
	return "fake"
}

func (f *FakeTransport) Capabilities() Capabilities { return f.CapabilitiesValue }

func (f *FakeTransport) Start(_ context.Context, cfg StartConfig) (PacketTunnel, error) {
	f.mu.Lock()
	f.StartedWith = append(f.StartedWith, cfg)
	f.mu.Unlock()
	if f.StartErr != nil {
		return nil, f.StartErr
	}
	if f.Tunnel == nil {
		return NewFakeTunnel(cfg.MTU, cfg.Addresses), nil
	}
	return f.Tunnel, nil
}

// FakeTunnel is a deterministic in-memory implementation of PacketTunnel.
type FakeTunnel struct {
	mu        sync.Mutex
	mtu       int
	addresses []netip.Prefix
	stats     Stats
	events    chan Event
	readQueue [][]byte
	writes    [][]byte
	readErr   error
	writeErr  error
	closed    bool
}

func NewFakeTunnel(mtu int, addresses []netip.Prefix) *FakeTunnel {
	copied := make([]netip.Prefix, len(addresses))
	copy(copied, addresses)
	return &FakeTunnel{
		mtu:       mtu,
		addresses: copied,
		events:    make(chan Event, 16),
	}
}

func (f *FakeTunnel) MTU() int { return f.mtu }

func (f *FakeTunnel) Addresses() []netip.Prefix {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]netip.Prefix, len(f.addresses))
	copy(out, f.addresses)
	return out
}

func (f *FakeTunnel) ReadPacket(buf []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, ErrClosed
	}
	if f.readErr != nil {
		return 0, f.readErr
	}
	if len(f.readQueue) == 0 {
		return 0, io.EOF
	}
	pkt := f.readQueue[0]
	f.readQueue = f.readQueue[1:]
	n := copy(buf, pkt)
	f.stats.PacketsRead++
	f.stats.BytesRead += uint64(n)
	f.stats.LastActivityAt = time.Now().UTC()
	return n, nil
}

func (f *FakeTunnel) WritePacket(pkt []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return ErrClosed
	}
	if f.writeErr != nil {
		return f.writeErr
	}
	cp := append([]byte(nil), pkt...)
	f.writes = append(f.writes, cp)
	f.stats.PacketsWritten++
	f.stats.BytesWritten += uint64(len(pkt))
	f.stats.LastActivityAt = time.Now().UTC()
	return nil
}

func (f *FakeTunnel) Events() <-chan Event { return f.events }

func (f *FakeTunnel) Stats() Stats {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stats
}

func (f *FakeTunnel) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	close(f.events)
	return nil
}

func (f *FakeTunnel) QueueReadPacket(pkt []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readQueue = append(f.readQueue, append([]byte(nil), pkt...))
}

func (f *FakeTunnel) SetReadError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readErr = err
}

func (f *FakeTunnel) SetWriteError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeErr = err
}

func (f *FakeTunnel) EmitEvent(ev Event) {
	select {
	case f.events <- ev:
	default:
	}
}

func (f *FakeTunnel) WrittenPackets() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.writes))
	for i := range f.writes {
		out[i] = append([]byte(nil), f.writes[i]...)
	}
	return out
}

func (f *FakeTunnel) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}
