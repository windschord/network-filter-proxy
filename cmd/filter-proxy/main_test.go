package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func extractPort(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to extract port: %v", err)
	}
	return port
}

func TestRunHealthcheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()

	t.Setenv("API_PORT", port)
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1", code)
	}
}

func TestRunHealthcheck_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Setenv("API_PORT", "")
	t.Setenv("API_BIND_ADDR", "")

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1 (default port, no server)", code)
	}
}

func TestRunHealthcheck_WildcardBindAddr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("API_PORT", extractPort(t, srv))
	t.Setenv("API_BIND_ADDR", "0.0.0.0")

	code := runHealthcheck()
	if code != 0 {
		t.Errorf("runHealthcheck() = %d, want 0 (0.0.0.0 should resolve to 127.0.0.1)", code)
	}
}
