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

	code := runHealthcheck()
	if code != 0 {
		t.Errorf("runHealthcheck() = %d, want 0", code)
	}
}

func TestRunHealthcheck_ServerDown(t *testing.T) {
	// Use a port that's unlikely to be in use
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	listener.Close() // Close immediately so the port is free but nothing is listening

	t.Setenv("API_PORT", port)

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

	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1", code)
	}
}

func TestRunHealthcheck_DefaultPort(t *testing.T) {
	t.Setenv("API_PORT", "")

	// This will fail to connect (nothing on 8080 in test) but should not panic
	code := runHealthcheck()
	if code != 1 {
		t.Errorf("runHealthcheck() = %d, want 1 (default port, no server)", code)
	}
}
