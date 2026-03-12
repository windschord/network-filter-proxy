package proxy_test

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/claudework/network-filter-proxy/internal/logger"
	"github.com/claudework/network-filter-proxy/internal/proxy"
	"github.com/claudework/network-filter-proxy/internal/rule"
)

func newTestHandler(t *testing.T, store *rule.Store) *proxy.Handler {
	t.Helper()
	log := logger.New("json", "debug")
	return proxy.NewHandler(store, log)
}

// newIPv4Server creates a test server bound to 127.0.0.1 (IPv4 only).
func newIPv4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	return srv
}

// newIPv4TLSServer creates a TLS test server bound to 127.0.0.1 (IPv4 only).
func newIPv4TLSServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp4: %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener.Close()
	srv.Listener = l
	srv.StartTLS()
	return srv
}

// loopbackIP extracts the loopback IP from a server URL (handles both IPv4 and IPv6).
func loopbackIP(t *testing.T, serverURL string) string {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return u.Hostname()
}

func TestProxyHandler_UnregisteredIP_HTTP(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	target := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if reason := resp.Header.Get("X-Filter-Reason"); reason != "no-rules" {
		t.Errorf("X-Filter-Reason = %q, want %q", reason, "no-rules")
	}
}

func TestProxyHandler_AllowedHost_HTTP(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	target := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	}))
	defer target.Close()

	srcIP := loopbackIP(t, proxyServer.URL)
	targetURL, _ := url.Parse(target.URL)
	store.Set(srcIP, []rule.Entry{{Host: targetURL.Hostname(), Port: 0}})

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestProxyHandler_DeniedHost_HTTP(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	target := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	srcIP := loopbackIP(t, proxyServer.URL)
	store.Set(srcIP, []rule.Entry{{Host: "other.example.com", Port: 443}})

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if reason := resp.Header.Get("X-Filter-Reason"); reason != "denied" {
		t.Errorf("X-Filter-Reason = %q, want %q", reason, "denied")
	}
}

func TestProxyHandler_ActiveConn_Initial(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)
	if h.ActiveConnections() != 0 {
		t.Errorf("ActiveConnections = %d, want 0", h.ActiveConnections())
	}
}

// sendCONNECT sends a raw CONNECT request to the proxy and returns the response.
func sendCONNECT(t *testing.T, proxyAddr, targetHost string) *http.Response {
	t.Helper()
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()

	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetHost, targetHost)
	if err != nil {
		t.Fatalf("write CONNECT: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	return resp
}

// CONNECT tests via HTTPS proxy
func TestProxyHandler_CONNECT_UnregisteredIP(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)

	// No rules set -> CONNECT should be rejected with 403
	resp := sendCONNECT(t, proxyURL.Host, "example.com:443")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestProxyHandler_CONNECT_AllowedHost(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	target := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "CONNECT OK")
	}))
	defer target.Close()

	srcIP := loopbackIP(t, proxyServer.URL)
	targetURL, _ := url.Parse(target.URL)
	store.Set(srcIP, []rule.Entry{{Host: targetURL.Hostname(), Port: 0}})

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "CONNECT OK" {
		t.Errorf("body = %q, want %q", string(body), "CONNECT OK")
	}
}

func TestProxyHandler_CONNECT_DeniedHost(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	srcIP := loopbackIP(t, proxyServer.URL)
	// Set rules for a different host
	store.Set(srcIP, []rule.Entry{{Host: "other.example.com", Port: 443}})

	proxyURL, _ := url.Parse(proxyServer.URL)

	// CONNECT to a host not in rules -> should be rejected with 403
	resp := sendCONNECT(t, proxyURL.Host, "blocked.example.com:443")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestProxyHandler_CONNECT_BadGateway(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	proxyServer := newIPv4Server(t, h)
	defer proxyServer.Close()

	srcIP := loopbackIP(t, proxyServer.URL)
	// Allow CONNECT to an unreachable host (port that nothing listens on)
	store.Set(srcIP, []rule.Entry{{Host: "127.0.0.1", Port: 0}})

	proxyURL, _ := url.Parse(proxyServer.URL)

	// CONNECT to an unreachable port -> should get 502 Bad Gateway
	resp := sendCONNECT(t, proxyURL.Host, "127.0.0.1:1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}
