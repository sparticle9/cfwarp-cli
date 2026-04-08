package socks_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	enginepkg "github.com/nexus/cfwarp-cli/internal/dataplane/engine"
	"github.com/nexus/cfwarp-cli/internal/dataplane/frontend/socks"
	"golang.org/x/net/proxy"
)

type testStack struct{}

func (testStack) ReadPacket(buf []byte) (int, error) { return 0, io.EOF }
func (testStack) WritePacket(pkt []byte) error       { return nil }
func (testStack) Close() error                       { return nil }
func (testStack) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}
func (testStack) ResolveIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

func startTCPEcho(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestServe_NoAuth(t *testing.T) {
	stack := testStack{}
	server := socks.New(socks.Config{}, stack)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer ln.Close()
	go server.Serve(ln)
	defer server.Close()

	echoAddr := startTCPEcho(t)
	dialer, err := proxy.SOCKS5("tcp", ln.Addr().String(), nil, proxy.Direct)
	if err != nil {
		t.Fatalf("SOCKS5 dialer: %v", err)
	}
	conn, err := dialer.Dial("tcp", echoAddr)
	if err != nil {
		t.Fatalf("dial through socks: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("write through socks: %v", err)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read through socks: %v", err)
	}
	if string(buf) != "hello" {
		t.Fatalf("unexpected echo payload %q", string(buf))
	}
}

func TestServe_WithAuth(t *testing.T) {
	stack := testStack{}
	server := socks.New(socks.Config{Username: "alice", Password: "secret"}, stack)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer ln.Close()
	go server.Serve(ln)
	defer server.Close()

	echoAddr := startTCPEcho(t)
	auth := &proxy.Auth{User: "alice", Password: "secret"}
	dialer, err := proxy.SOCKS5("tcp", ln.Addr().String(), auth, proxy.Direct)
	if err != nil {
		t.Fatalf("SOCKS5 dialer: %v", err)
	}
	conn, err := dialer.Dial("tcp", echoAddr)
	if err != nil {
		t.Fatalf("dial through socks: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("auth")); err != nil {
		t.Fatalf("write through socks: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read through socks: %v", err)
	}
	if string(buf) != "auth" {
		t.Fatalf("unexpected echo payload %q", string(buf))
	}
}

var _ enginepkg.NetworkStack = testStack{}
