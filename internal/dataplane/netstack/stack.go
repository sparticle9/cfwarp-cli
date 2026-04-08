package netstack

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
	wgnetstack "golang.zx2c4.com/wireguard/tun/netstack"
)

var defaultDNSServers = []netip.Addr{
	netip.MustParseAddr("1.1.1.1"),
	netip.MustParseAddr("1.0.0.1"),
	netip.MustParseAddr("2606:4700:4700::1111"),
	netip.MustParseAddr("2606:4700:4700::1001"),
}

// Stack adapts wireguard-go's userspace netstack to the shared data-plane interfaces.
type Stack struct {
	dev tun.Device
	net *wgnetstack.Net

	readBufPool sync.Pool
	readSzPool  sync.Pool
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
	_, err := s.dev.Read(*bufsPtr, *sizesPtr, 0)
	if err != nil {
		return 0, err
	}
	return (*sizesPtr)[0], nil
}

func (s *Stack) WritePacket(pkt []byte) error {
	_, err := s.dev.Write([][]byte{pkt}, 0)
	return err
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
