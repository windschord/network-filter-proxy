package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newIPv4TestServer creates a test server bound to 127.0.0.1 (IPv4 only)
// to match the default healthcheck target and avoid IPv6 environment issues.
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
	t.Setenv("API_BIND_ADDR", "")

	addr := healthcheckAddr()
	if addr != "127.0.0.1:8080" {
		t.Errorf("healthcheckAddr() = %q, want %q", addr, "127.0.0.1:8080")
	}
}

func TestHealthcheckAddr_CustomPort(t *testing.T) {
	t.Setenv("API_PORT", "9090")
	t.Setenv("API_BIND_ADDR", "")

	addr := healthcheckAddr()
	if addr != "127.0.0.1:9090" {
		t.Errorf("healthcheckAddr() = %q, want %q", addr, "127.0.0.1:9090")
	}
}

func TestHealthcheckAddr_WildcardBindAddr(t *testing.T) {
	// Wildcard addresses (0.0.0.0, ::) should resolve to 127.0.0.1
	// since the server listens on all interfaces including loopback.
	for _, bindAddr := range []string{"0.0.0.0", "::", "127.0.0.1", ""} {
		t.Run(bindAddr, func(t *testing.T) {
			t.Setenv("API_BIND_ADDR", bindAddr)
			t.Setenv("API_PORT", "8080")

			addr := healthcheckAddr()
			if addr != "127.0.0.1:8080" {
				t.Errorf("healthcheckAddr() = %q, want %q", addr, "127.0.0.1:8080")
			}
		})
	}
}

func TestHealthcheckAddr_SpecificBindAddr(t *testing.T) {
	// Specific non-loopback addresses should be used as-is
	// so the healthcheck reaches the actual API bind address.
	tests := []struct {
		bindAddr string
		wantHost string
	}{
		{"172.20.0.2", "172.20.0.2"},
		{"::1", "::1"},
		{"10.0.0.1", "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.bindAddr, func(t *testing.T) {
			t.Setenv("API_BIND_ADDR", tt.bindAddr)
			t.Setenv("API_PORT", "8080")

			addr := healthcheckAddr()
			want := net.JoinHostPort(tt.wantHost, "8080")
			if addr != want {
				t.Errorf("healthcheckAddr() = %q, want %q", addr, want)
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
