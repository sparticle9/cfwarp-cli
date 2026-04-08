package http

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	nethttp "net/http"
	"strings"
	"sync"
	"time"

	enginepkg "github.com/nexus/cfwarp-cli/internal/dataplane/engine"
)

// Config configures the HTTP proxy frontend listener.
type Config struct {
	ListenAddr string
	Username   string
	Password   string
}

// Server wraps an HTTP/CONNECT proxy exposed by the shared data-plane engine.
type Server struct {
	stack enginepkg.NetworkStack
	cfg   Config
	srv   *nethttp.Server
	ln    net.Listener
	once  sync.Once
}

func New(cfg Config, stack enginepkg.NetworkStack) *Server {
	s := &Server{stack: stack, cfg: cfg}
	s.srv = &nethttp.Server{
		Addr:              cfg.ListenAddr,
		Handler:           nethttp.HandlerFunc(s.handle),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) Serve(l net.Listener) error {
	s.ln = l
	return s.srv.Serve(l)
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) Close() error {
	var err error
	s.once.Do(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = s.srv.Shutdown(shutdownCtx)
		if s.ln != nil {
			_ = s.ln.Close()
		}
	})
	return err
}

func (s *Server) handle(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !s.authenticate(r) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="Proxy"`)
		nethttp.Error(w, "Proxy authentication required", nethttp.StatusProxyAuthRequired)
		return
	}
	if r.Method == nethttp.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleForward(w, r)
}

func (s *Server) authenticate(r *nethttp.Request) bool {
	if s.cfg.Username == "" && s.cfg.Password == "" {
		return true
	}
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(s.cfg.Username+":"+s.cfg.Password))
	return r.Header.Get("Proxy-Authorization") == expected
}

func (s *Server) handleConnect(w nethttp.ResponseWriter, r *nethttp.Request) {
	destConn, err := s.stack.DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusBadGateway)
		return
	}

	hj, ok := w.(nethttp.Hijacker)
	if !ok {
		destConn.Close()
		nethttp.Error(w, "proxy does not support hijacking", nethttp.StatusInternalServerError)
		return
	}
	clientConn, bufrw, err := hj.Hijack()
	if err != nil {
		destConn.Close()
		nethttp.Error(w, err.Error(), nethttp.StatusBadGateway)
		return
	}
	_, _ = bufrw.WriteString("HTTP/1.1 200 Connection established\r\n\r\n")
	_ = bufrw.Flush()
	go proxyCopy(destConn, clientConn)
	go proxyCopy(clientConn, destConn)
}

func (s *Server) handleForward(w nethttp.ResponseWriter, r *nethttp.Request) {
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	if outReq.URL == nil || outReq.URL.Host == "" {
		nethttp.Error(w, "absolute URI required", nethttp.StatusBadRequest)
		return
	}
	transport := &nethttp.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return s.stack.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2: false,
	}
	defer transport.CloseIdleConnections()

	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func copyHeader(dst, src nethttp.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func proxyCopy(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	_, _ = io.Copy(dst, src)
}

// ParseBasicAuthHeader is a small helper used in tests and diagnostics.
func ParseBasicAuthHeader(v string) (string, string, bool) {
	if !strings.HasPrefix(v, "Basic ") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(v, "Basic "))
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ReadConnectResponse reads the first HTTP response line from a CONNECT tunnel.
func ReadConnectResponse(conn net.Conn) (string, error) {
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func BasicAuthHeader(username, password string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}
