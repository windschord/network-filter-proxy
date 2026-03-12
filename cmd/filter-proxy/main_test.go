package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newIPv4TestServer creates a test server bound to 127.0.0.1 (IPv4 only)
// to match runHealthcheck's 127.0.0.1 target and avoid IPv6 environment issues.
func newIPv4TestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on IPv4 loopback: %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	return srv
}

func extractPort(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to extract port: %v", err)
	}
	return port
}

// --- Address resolution tests (no network I/O) ---

func TestHealthcheckAddr_Default(t *testing.T) {
	t.Setenv("API_PORT", "")

	addr := healthcheckAddr()
	if addr != "127.0.0.1:8080" {
		t.Errorf("healthcheckAddr() = %q, want %q", addr, "127.0.0.1:8080")
	}
}

func TestHealthcheckAddr_CustomPort(t *testing.T) {
	t.Setenv("API_PORT", "9090")

	addr := healthcheckAddr()
	if addr != "127.0.0.1:9090" {
		t.Errorf("healthcheckAddr() = %q, want %q", addr, "127.0.0.1:9090")
	}
}

func TestHealthcheckAddr_IgnoresBindAddr(t *testing.T) {
	// healthcheckAddr always returns 127.0.0.1 regardless of API_BIND_ADDR.
	for _, bindAddr := range []string{"0.0.0.0", "::", "172.20.0.2", "::1"} {
		t.Run(bindAddr, func(t *testing.T) {
			t.Setenv("API_BIND_ADDR", bindAddr)
			t.Setenv("API_PORT", "8080")

			addr := healthcheckAddr()
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				t.Fatalf("failed to parse addr %q: %v", addr, err)
			}
			if host != "127.0.0.1" {
				t.Errorf("healthcheckAddr() host = %q, want %q", host, "127.0.0.1")
			}
		})
	}
}

// --- Integration tests (with real server) ---

func TestRunHealthcheck_Success(t *testing.T) {
	srv := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	t.Setenv("API_PORT", extractPort(t, srv))
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 0 {
		t.Errorf("runHealthcheck() = %d, want 0", code)
	}
}

func TestRunHealthcheck_Non200(t *testing.T) {
	srv := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	t.Setenv("API_PORT", extractPort(t, srv))
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1", code)
	}
}

func TestRunHealthcheck_ServerDown(t *testing.T) {
	srv := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	port := extractPort(t, srv)
	srv.Close() // close immediately so nothing is listening

	t.Setenv("API_PORT", port)
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1", code)
	}
}
