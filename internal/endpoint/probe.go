package endpoint

import (
	"context"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/nexus/cfwarp-cli/internal/settings"
)

// Result holds the outcome of probing a single endpoint candidate.
type Result struct {
	Candidate   string   `json:"candidate"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Valid        bool     `json:"valid"`
	Reachable   bool     `json:"reachable"`
	ResolvedIPs []string `json:"resolved_ips,omitempty"`
	Notes       string   `json:"notes,omitempty"`
}

const probeNote = "TCP probe used for WireGuard (UDP) endpoint; " +
	"both successful connect and 'connection refused' are treated as reachable " +
	"because the host is routable. Only timeout or no-route means unreachable."

// Probe validates candidate syntax, resolves DNS, and performs a TCP dial.
func Probe(ctx context.Context, candidate string, timeout time.Duration) Result {
	// Step 1: validate syntax.
	if err := settings.ValidateEndpoint(candidate); err != nil {
		return Result{Candidate: candidate, Valid: false, Notes: err.Error()}
	}

	// Step 2: split host/port.
	host, portStr, _ := net.SplitHostPort(candidate)
	port, _ := strconv.Atoi(portStr)

	r := Result{
		Candidate: candidate,
		Host:      host,
		Port:      port,
		Valid:     true,
		Notes:     probeNote,
	}

	// Step 3: DNS lookup.
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		r.Notes = "DNS lookup failed: " + err.Error()
		return r
	}
	r.ResolvedIPs = addrs

	// Use the first resolved IP for the dial.
	dialAddr := net.JoinHostPort(addrs[0], portStr)

	// Step 4: TCP dial with timeout.
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", dialAddr)
	if dialErr == nil {
		conn.Close()
		r.Reachable = true
		return r
	}

	// "connection refused" means the host is routable — treat as reachable.
	var opErr *net.OpError
	if errors.As(dialErr, &opErr) {
		if isSyscallError(opErr, "connection refused") || isRefused(opErr) {
			r.Reachable = true
			return r
		}
	}

	// Timeout, no-route, or context cancel → not reachable.
	r.Reachable = false
	return r
}

// isRefused checks whether the OpError wraps a connection-refused syscall error.
func isRefused(opErr *net.OpError) bool {
	if opErr == nil {
		return false
	}
	// syscall.ECONNREFUSED message differs by OS; check the string.
	msg := opErr.Error()
	return containsAny(msg, "connection refused", "ECONNREFUSED")
}

func isSyscallError(opErr *net.OpError, substr string) bool {
	if opErr.Err == nil {
		return false
	}
	return containsAny(opErr.Err.Error(), substr)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
