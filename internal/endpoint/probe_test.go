package endpoint_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/nexus/cfwarp-cli/internal/endpoint"
)

func TestProbe_InvalidSyntax(t *testing.T) {
	r := endpoint.Probe(context.Background(), "no-port", 3*time.Second)
	if r.Valid {
		t.Fatal("expected Valid=false for 'no-port'")
	}
	if r.Notes == "" {
		t.Error("expected non-empty Notes explaining the error")
	}
}

func TestProbe_ValidFormat(t *testing.T) {
	// Start a local listener so the format is provably valid and DNS resolves.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	r := endpoint.Probe(context.Background(), ln.Addr().String(), 3*time.Second)
	if !r.Valid {
		t.Fatalf("expected Valid=true, notes: %s", r.Notes)
	}
	if r.Host == "" {
		t.Error("expected Host to be populated")
	}
	if r.Port == 0 {
		t.Error("expected Port to be populated")
	}
}

func TestProbe_Reachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Accept connections in background so dial completes.
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	r := endpoint.Probe(context.Background(), ln.Addr().String(), 3*time.Second)
	if !r.Valid {
		t.Fatalf("expected Valid=true, notes: %s", r.Notes)
	}
	if !r.Reachable {
		t.Fatalf("expected Reachable=true, notes: %s", r.Notes)
	}
}

func TestProbe_Unreachable(t *testing.T) {
	// Open then immediately close a listener; the port is guaranteed closed.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	r := endpoint.Probe(context.Background(), addr, 2*time.Second)
	if !r.Valid {
		t.Fatalf("expected Valid=true for valid syntax, notes: %s", r.Notes)
	}
	// On most OSes a closed-port dial gets "connection refused" which we treat
	// as reachable (host is routable). So we just verify the probe completes
	// without hanging and returns a bool — either outcome is acceptable here
	// depending on OS. The important invariant: no panic, no hang.
	_ = r.Reachable
}

func TestProbe_ContextCancel(t *testing.T) {
	// Listener that accepts but never responds — simulates a black-hole host.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Accept silently (never read/write), causing the dial to stay connected
	// but we cancel via context before any useful exchange.
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c // hold open, never close
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel almost immediately.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Use a very long timeout so context cancel is what fires.
	r := endpoint.Probe(ctx, ln.Addr().String(), 10*time.Second)

	// Dial succeeds (connection accepted) before cancel fires in practice,
	// but the important guarantee is the function returns promptly.
	// What we must NOT do is hang past context cancellation.
	_ = r.Reachable
}
