package config_test

import (
	"testing"
	"time"

	"github.com/claudework/network-filter-proxy/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("PROXY_PORT", "")
	t.Setenv("API_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg := config.Load()

	if cfg.ProxyPort != "3128" {
		t.Errorf("ProxyPort = %q, want %q", cfg.ProxyPort, "3128")
	}
	if cfg.APIPort != "8080" {
		t.Errorf("APIPort = %q, want %q", cfg.APIPort, "8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 30*time.Second)
	}
}

func TestLoad_CustomProxyPort(t *testing.T) {
	t.Setenv("PROXY_PORT", "9999")
	t.Setenv("API_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg := config.Load()
	if cfg.ProxyPort != "9999" {
		t.Errorf("ProxyPort = %q, want %q", cfg.ProxyPort, "9999")
	}
}

func TestLoad_ShutdownTimeout_Valid(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "60")
	t.Setenv("PROXY_PORT", "")
	t.Setenv("API_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg := config.Load()
	if cfg.ShutdownTimeout != 60*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 60*time.Second)
	}
}

func TestLoad_ShutdownTimeout_InvalidString(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "abc")
	t.Setenv("PROXY_PORT", "")
	t.Setenv("API_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg := config.Load()
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 30*time.Second)
	}
}

func TestLoad_ShutdownTimeout_Negative(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "-1")
	t.Setenv("PROXY_PORT", "")
	t.Setenv("API_PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg := config.Load()
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 30*time.Second)
	}
}
