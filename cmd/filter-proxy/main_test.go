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

func TestRunHealthcheck_ServerDown(t *testing.T) {
	// Acquire and immediately release a port to ensure nothing is listening.
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := extractPort(t, &httptest.Server{Listener: listener})
	listener.Close()

	t.Setenv("API_PORT", port)
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1", code)
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

func TestRunHealthcheck_DefaultPort(t *testing.T) {
	// We leave API_PORT empty so the default value (8080) is used. Since we do not
	// start any server on port 8080, the healthcheck should fail to connect.
	t.Setenv("API_PORT", "")
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1 (no server on default port 8080)", code)
	}
}

func TestRunHealthcheck_WildcardBindAddr(t *testing.T) {
	srv := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("API_PORT", extractPort(t, srv))
	t.Setenv("API_BIND_ADDR", "0.0.0.0")

	code := runHealthcheck()
	if code != 0 {
		t.Errorf("runHealthcheck() = %d, want 0 (0.0.0.0 should still check 127.0.0.1)", code)
	}
}

func TestRunHealthcheck_IPv6WildcardBindAddr(t *testing.T) {
	srv := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("API_PORT", extractPort(t, srv))
	t.Setenv("API_BIND_ADDR", "::")

	code := runHealthcheck()
	if code != 0 {
		t.Errorf("runHealthcheck() = %d, want 0 (:: should still check 127.0.0.1)", code)
	}
}
