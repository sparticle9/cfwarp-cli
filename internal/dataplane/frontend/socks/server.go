package socks

import (
	"context"
	"net"

	enginepkg "github.com/nexus/cfwarp-cli/internal/dataplane/engine"
	"github.com/things-go/go-socks5"
)

// Config configures the SOCKS5 frontend listener.
type Config struct {
	ListenAddr string
	Username   string
	Password   string
}

// Server wraps the SOCKS5 listener exposed by the shared data-plane engine.
type Server struct {
	srv *socks5.Server
	ln  net.Listener
}

type resolver struct{ stack enginepkg.NetworkStack }

func (r resolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	ips, err := r.stack.ResolveIP(ctx, name)
	if err != nil {
		return ctx, nil, err
	}
	if len(ips) == 0 {
		return ctx, nil, net.InvalidAddrError("no IPs resolved")
	}
	return ctx, ips[0], nil
}

func New(cfg Config, stack enginepkg.NetworkStack) *Server {
	opts := []socks5.Option{
		socks5.WithResolver(resolver{stack: stack}),
		socks5.WithDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return stack.DialContext(ctx, network, addr)
		}),
	}
	if cfg.Username != "" || cfg.Password != "" {
		opts = append(opts, socks5.WithCredential(socks5.StaticCredentials{cfg.Username: cfg.Password}))
	}
	return &Server{srv: socks5.NewServer(opts...)}
}

func (s *Server) Serve(l net.Listener) error {
	s.ln = l
	return s.srv.Serve(l)
}

func (s *Server) ListenAndServe(cfg Config) error {
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) Close() error {
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}
