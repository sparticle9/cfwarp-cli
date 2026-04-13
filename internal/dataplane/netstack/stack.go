package netstack

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"golang.zx2c4.com/wireguard/tun"
	wgnetstack "golang.zx2c4.com/wireguard/tun/netstack"
)

var defaultDNSServers = []netip.Addr{
	netip.MustParseAddr("1.1.1.1"),
	netip.MustParseAddr("1.0.0.1"),
	netip.MustParseAddr("2606:4700:4700::1111"),
	netip.MustParseAddr("2606:4700:4700::1001"),
}

// Stats captures low-overhead packet-device counters and call timings for the userspace netstack edge.
type Stats struct {
	Packets      uint64
	Bytes        uint64
	ReadCalls    uint64
	ReadNanos    uint64
	WriteCalls   uint64
	WriteNanos   uint64
	LastPacketAt time.Time
}

// Stack adapts wireguard-go's userspace netstack to the shared data-plane interfaces.
type Stack struct {
	dev tun.Device
	net *wgnetstack.Net

	readBufPool sync.Pool
	readSzPool  sync.Pool

	packets    atomic.Uint64
	bytes      atomic.Uint64
	readCalls  atomic.Uint64
	readNanos  atomic.Uint64
	writeCalls atomic.Uint64
	writeNanos atomic.Uint64
	lastPacket atomic.Int64
}

func New(addresses []netip.Prefix, mtu int) (*Stack, error) {
	localAddrs := make([]netip.Addr, 0, len(addresses))
	for _, p := range addresses {
		localAddrs = append(localAddrs, p.Addr())
	}
	dev, network, err := wgnetstack.CreateNetTUN(localAddrs, defaultDNSServers, mtu)
	if err != nil {
		return nil, fmt.Errorf("create netstack TUN: %w", err)
	}
	return &Stack{
		dev: dev,
		net: network,
		readBufPool: sync.Pool{New: func() any {
			bufs := make([][]byte, 1)
			return &bufs
		}},
		readSzPool: sync.Pool{New: func() any {
			sizes := make([]int, 1)
			return &sizes
		}},
	}, nil
}

func (s *Stack) ReadPacket(buf []byte) (int, error) {
	bufsPtr := s.readBufPool.Get().(*[][]byte)
	sizesPtr := s.readSzPool.Get().(*[]int)
	defer func() {
		(*bufsPtr)[0] = nil
		s.readBufPool.Put(bufsPtr)
		s.readSzPool.Put(sizesPtr)
	}()
	(*bufsPtr)[0] = buf
	started := time.Now()
	_, err := s.dev.Read(*bufsPtr, *sizesPtr, 0)
	if err != nil {
		return 0, err
	}
	n := (*sizesPtr)[0]
	s.readCalls.Add(1)
	s.readNanos.Add(uint64(time.Since(started)))
	s.packets.Add(1)
	s.bytes.Add(uint64(n))
	s.lastPacket.Store(time.Now().UTC().UnixNano())
	return n, nil
}

func (s *Stack) WritePacket(pkt []byte) error {
	started := time.Now()
	_, err := s.dev.Write([][]byte{pkt}, 0)
	if err != nil {
		return err
	}
	s.writeCalls.Add(1)
	s.writeNanos.Add(uint64(time.Since(started)))
	s.packets.Add(1)
	s.bytes.Add(uint64(len(pkt)))
	s.lastPacket.Store(time.Now().UTC().UnixNano())
	return nil
}

func (s *Stack) Close() error { return s.dev.Close() }

func (s *Stack) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return s.net.DialContext(ctx, network, addr)
}

func (s *Stack) ResolveIP(_ context.Context, host string) ([]net.IP, error) {
	addrs, err := s.net.LookupHost(host)
	if err != nil {
		return nil, err
	}
	out := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil {
			out = append(out, ip)
		}
	}
	return out, nil
}

func (s *Stack) Stats() Stats {
	stats := Stats{
		Packets:    s.packets.Load(),
		Bytes:      s.bytes.Load(),
		ReadCalls:  s.readCalls.Load(),
		ReadNanos:  s.readNanos.Load(),
		WriteCalls: s.writeCalls.Load(),
		WriteNanos: s.writeNanos.Load(),
	}
	if ts := s.lastPacket.Load(); ts > 0 {
		stats.LastPacketAt = time.Unix(0, ts).UTC()
	}
	return stats
}
