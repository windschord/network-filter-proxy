package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/claudework/network-filter-proxy/internal/rule"
	"github.com/elazarl/goproxy"
)

type Handler struct {
	store        *rule.Store
	logger       *slog.Logger
	activeConn   atomic.Int64
	proxy        *goproxy.ProxyHttpServer
	tunnelMu     sync.Mutex
	tunnels      map[net.Conn]struct{}
	shuttingDown bool
}

func NewHandler(store *rule.Store, logger *slog.Logger) *Handler {
	h := &Handler{store: store, logger: logger, tunnels: make(map[net.Conn]struct{})}
	p := goproxy.NewProxyHttpServer()

	p.OnRequest().HandleConnect(goproxy.FuncHttpsHandler(
		func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			srcIP := extractIP(ctx.Req.RemoteAddr)
			dstHost, dstPort := splitHostPort(host, 443)

			rs, ok := h.store.Get(srcIP)
			if !ok {
				h.logger.Info("proxy request",
					"action", "deny", "src_ip", srcIP,
					"dst_host", dstHost, "dst_port", dstPort,
					"reason", "no-rules")
				ctx.Resp = &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"X-Filter-Reason": {"no-rules"}},
					Body:       http.NoBody,
				}
				return goproxy.RejectConnect, host
			}

			for _, entry := range rs.Entries {
				if rule.Matches(entry, dstHost, dstPort) {
					h.logger.Info("proxy request",
						"action", "allow", "src_ip", srcIP,
						"dst_host", dstHost, "dst_port", dstPort)
					return &goproxy.ConnectAction{
						Action: goproxy.ConnectHijack,
						Hijack: h.hijackTunnel,
					}, host
				}
			}

			h.logger.Info("proxy request",
				"action", "deny", "src_ip", srcIP,
				"dst_host", dstHost, "dst_port", dstPort,
				"reason", "denied")
			ctx.Resp = &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     http.Header{"X-Filter-Reason": {"denied"}},
				Body:       http.NoBody,
			}
			return goproxy.RejectConnect, host
		},
	))

	p.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			srcIP := extractIP(r.RemoteAddr)
			defaultPort := 80
			if r.URL != nil && r.URL.Scheme == "https" {
				defaultPort = 443
			}
			hostHeader := r.Host
			if r.URL != nil && r.URL.Host != "" {
				hostHeader = r.URL.Host
			}
			dstHost, dstPort := splitHostPort(hostHeader, defaultPort)

			rs, ok := h.store.Get(srcIP)
			if !ok {
				h.logger.Info("proxy request",
					"action", "deny", "src_ip", srcIP,
					"dst_host", dstHost, "dst_port", dstPort,
					"reason", "no-rules")
				return r, &http.Response{
					StatusCode: http.StatusForbidden,
					Header:     http.Header{"X-Filter-Reason": {"no-rules"}},
					Body:       http.NoBody,
					Request:    r,
				}
			}

			for _, entry := range rs.Entries {
				if rule.Matches(entry, dstHost, dstPort) {
					h.logger.Info("proxy request",
						"action", "allow", "src_ip", srcIP,
						"dst_host", dstHost, "dst_port", dstPort)
					return r, nil
				}
			}

			h.logger.Info("proxy request",
				"action", "deny", "src_ip", srcIP,
				"dst_host", dstHost, "dst_port", dstPort,
				"reason", "denied")
			return r, &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     http.Header{"X-Filter-Reason": {"denied"}},
				Body:       http.NoBody,
				Request:    r,
			}
		},
	)

	h.proxy = p
	return h
}

func (h *Handler) hijackTunnel(req *http.Request, client net.Conn, _ *goproxy.ProxyCtx) {
	host := req.URL.Host
	remote, err := net.DialTimeout("tcp", host, 30*time.Second)
	if err != nil {
		h.logger.Error("tunnel dial failed", "host", host, "err", err)
		_, _ = fmt.Fprintf(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		_ = client.Close()
		return
	}

	h.activeConn.Add(1)
	if !h.trackTunnel(client, remote) {
		_ = remote.Close()
		_ = client.Close()
		h.activeConn.Add(-1)
		return
	}
	_, _ = fmt.Fprintf(client, "HTTP/1.1 200 Connection Established\r\n\r\n")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, client)
		_ = remote.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(client, remote)
		_ = client.Close()
	}()
	wg.Wait()
	h.untrackTunnel(client, remote)
	h.activeConn.Add(-1)
}

func (h *Handler) trackTunnel(conns ...net.Conn) bool {
	h.tunnelMu.Lock()
	defer h.tunnelMu.Unlock()
	if h.shuttingDown {
		return false
	}
	for _, c := range conns {
		h.tunnels[c] = struct{}{}
	}
	return true
}

func (h *Handler) untrackTunnel(conns ...net.Conn) {
	h.tunnelMu.Lock()
	defer h.tunnelMu.Unlock()
	for _, c := range conns {
		delete(h.tunnels, c)
	}
}

// Shutdown closes all tracked tunnel connections.
func (h *Handler) Shutdown() {
	h.tunnelMu.Lock()
	defer h.tunnelMu.Unlock()
	h.shuttingDown = true
	for c := range h.tunnels {
		_ = c.Close()
	}
	h.tunnels = nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) ActiveConnections() int64 {
	return h.activeConn.Load()
}

func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func splitHostPort(hostport string, defaultPort int) (string, int) {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, defaultPort
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, defaultPort
	}
	return host, port
}
