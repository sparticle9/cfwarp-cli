package http_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	enginepkg "github.com/nexus/cfwarp-cli/internal/dataplane/engine"
	httpproxy "github.com/nexus/cfwarp-cli/internal/dataplane/frontend/http"
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

func startEcho(t *testing.T) string {
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

func TestServe_HTTPForward(t *testing.T) {
	upstream := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	stack := testStack{}
	server := httpproxy.New(httpproxy.Config{ListenAddr: "127.0.0.1:0"}, stack)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer ln.Close()
	go server.Serve(ln)
	defer server.Close()

	client := &nethttp.Client{Transport: &nethttp.Transport{Proxy: func(*nethttp.Request) (*url.URL, error) {
		return url.Parse("http://" + ln.Addr().String())
	}}}
	resp, err := client.Get(upstream.URL)
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("unexpected body %q", string(body))
	}
}

func TestServe_AuthRequired(t *testing.T) {
	stack := testStack{}
	server := httpproxy.New(httpproxy.Config{ListenAddr: "127.0.0.1:0", Username: "alice", Password: "secret"}, stack)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer ln.Close()
	go server.Serve(ln)
	defer server.Close()

	resp, err := nethttp.Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatalf("GET proxy root: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != nethttp.StatusProxyAuthRequired {
		t.Fatalf("expected 407, got %d", resp.StatusCode)
	}
}

func TestServe_CONNECT(t *testing.T) {
	echoAddr := startEcho(t)
	stack := testStack{}
	server := httpproxy.New(httpproxy.Config{ListenAddr: "127.0.0.1:0"}, stack)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen proxy: %v", err)
	}
	defer ln.Close()
	go server.Serve(ln)
	defer server.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", echoAddr, echoAddr); err != nil {
		t.Fatalf("write connect request: %v", err)
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read connect response: %v", err)
	}
	if line[:12] != "HTTP/1.1 200" {
		t.Fatalf("unexpected CONNECT response line %q", line)
	}
	for {
		l, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("drain headers: %v", err)
		}
		if l == "\r\n" {
			break
		}
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write tunneled payload: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(reader, buf); err != nil {
		t.Fatalf("read tunneled payload: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("unexpected tunneled payload %q", string(buf))
	}
}

var _ enginepkg.NetworkStack = testStack{}
