package proxy_test

import (
	"io"
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

func TestProxyHandler_UnregisteredIP_HTTP(t *testing.T) {
	store := rule.NewStore()
	h := newTestHandler(t, store)

	// Create a proxy server
	proxyServer := httptest.NewServer(h)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	// Target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	proxyServer := httptest.NewServer(h)
	defer proxyServer.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	}))
	defer target.Close()

	targetURL, _ := url.Parse(target.URL)
	// Register 127.0.0.1 with allowed host
	store.Set("127.0.0.1", []rule.Entry{{Host: targetURL.Hostname(), Port: 0}})

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

	proxyServer := httptest.NewServer(h)
	defer proxyServer.Close()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Register rule for a different host
	store.Set("127.0.0.1", []rule.Entry{{Host: "other.example.com", Port: 443}})

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
