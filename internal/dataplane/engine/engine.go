package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nexus/cfwarp-cli/internal/transport"
)

// PacketDevice is the packet-oriented side of a userspace network stack.
type PacketDevice interface {
	ReadPacket(buf []byte) (int, error)
	WritePacket(pkt []byte) error
	Close() error
}

// NetworkStack is the shared data-plane contract required by the frontends.
type NetworkStack interface {
	PacketDevice
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	ResolveIP(ctx context.Context, host string) ([]net.IP, error)
}

// BufferPool amortizes packet buffer allocations on the forwarding paths.
type BufferPool struct {
	size int
	pool sync.Pool
}

func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		size: size,
		pool: sync.Pool{New: func() any {
			b := make([]byte, size)
			return &b
		}},
	}
}

func (p *BufferPool) Get() []byte {
	return *(p.pool.Get().(*[]byte))
}

func (p *BufferPool) Put(buf []byte) {
	if cap(buf) != p.size {
		return
	}
	p.pool.Put(&buf)
}

// Engine wires a packet transport to a shared userspace network stack and
// exposes dial/resolve helpers to service-mode frontends.
type Engine struct {
	stack  NetworkStack
	tunnel transport.PacketTunnel
	pool   *BufferPool

	ctx    context.Context
	cancel context.CancelFunc
	errCh  chan error
	once   sync.Once
}

func New(stack NetworkStack, tunnel transport.PacketTunnel) *Engine {
	mtu := tunnel.MTU()
	if mtu <= 0 {
		mtu = 1500
	}
	return &Engine{
		stack:  stack,
		tunnel: tunnel,
		pool:   NewBufferPool(mtu),
		errCh:  make(chan error, 2),
	}
}

func (e *Engine) Start(ctx context.Context) {
	e.ctx, e.cancel = context.WithCancel(ctx)
	go e.forwardStackToTunnel()
	go e.forwardTunnelToStack()
}

func (e *Engine) Errors() <-chan error { return e.errCh }

func (e *Engine) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return e.stack.DialContext(ctx, network, addr)
}

func (e *Engine) ResolveIP(ctx context.Context, host string) ([]net.IP, error) {
	return e.stack.ResolveIP(ctx, host)
}

func (e *Engine) MTU() int { return e.tunnel.MTU() }

func (e *Engine) Close() error {
	var err error
	e.once.Do(func() {
		if e.cancel != nil {
			e.cancel()
		}
		if closeErr := e.stack.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			err = closeErr
		}
		if closeErr := e.tunnel.Close(); closeErr != nil && !errors.Is(closeErr, transport.ErrClosed) {
			if err == nil {
				err = closeErr
			}
		}
	})
	return err
}

func (e *Engine) forwardStackToTunnel() {
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}

		buf := e.pool.Get()
		n, err := e.stack.ReadPacket(buf)
		if err != nil {
			e.pool.Put(buf)
			e.reportError(fmt.Errorf("read from stack: %w", err))
			return
		}
		if err := e.tunnel.WritePacket(buf[:n]); err != nil {
			e.pool.Put(buf)
			e.reportError(fmt.Errorf("write to tunnel: %w", err))
			return
		}
		e.pool.Put(buf)
	}
}

func (e *Engine) forwardTunnelToStack() {
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}

		buf := e.pool.Get()
		n, err := e.tunnel.ReadPacket(buf)
		if err != nil {
			e.pool.Put(buf)
			e.reportError(fmt.Errorf("read from tunnel: %w", err))
			return
		}
		if err := e.stack.WritePacket(buf[:n]); err != nil {
			e.pool.Put(buf)
			e.reportError(fmt.Errorf("write to stack: %w", err))
			return
		}
		e.pool.Put(buf)
	}
}

func (e *Engine) reportError(err error) {
	select {
	case e.errCh <- err:
	default:
	}
}

// Snapshot returns a lightweight packet+event view suitable for status reporting.
type Snapshot struct {
	TransportStats transport.Stats  `json:"transport_stats"`
	RecentEvent    *transport.Event `json:"recent_event,omitempty"`
	CapturedAt     time.Time        `json:"captured_at"`
}

func (e *Engine) Snapshot() Snapshot {
	var recent *transport.Event
	select {
	case ev := <-e.tunnel.Events():
		recent = &ev
	default:
	}
	return Snapshot{
		TransportStats: e.tunnel.Stats(),
		RecentEvent:    recent,
		CapturedAt:     time.Now().UTC(),
	}
}
