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

func TestLoad_APIBindAddr_Default(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "")

	cfg := config.Load()
	if cfg.APIBindAddr != "127.0.0.1" {
		t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "127.0.0.1")
	}
}

func TestLoad_APIBindAddr_AllInterfaces(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "0.0.0.0")

	cfg := config.Load()
	if cfg.APIBindAddr != "0.0.0.0" {
		t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "0.0.0.0")
	}
}

func TestLoad_APIBindAddr_SpecificIP(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "172.20.0.2")

	cfg := config.Load()
	if cfg.APIBindAddr != "172.20.0.2" {
		t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "172.20.0.2")
	}
}

func TestLoad_APIBindAddr_InvalidString(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "abc")

	cfg := config.Load()
	if cfg.APIBindAddr != "127.0.0.1" {
		t.Errorf("APIBindAddr = %q, want %q (fallback)", cfg.APIBindAddr, "127.0.0.1")
	}
}

func TestLoad_APIBindAddr_InvalidIP(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "999.999.999.999")

	cfg := config.Load()
	if cfg.APIBindAddr != "127.0.0.1" {
		t.Errorf("APIBindAddr = %q, want %q (fallback)", cfg.APIBindAddr, "127.0.0.1")
	}
}

func TestLoad_APIBindAddr_IPv6(t *testing.T) {
	t.Setenv("API_BIND_ADDR", "::1")

	cfg := config.Load()
	if cfg.APIBindAddr != "::1" {
		t.Errorf("APIBindAddr = %q, want %q", cfg.APIBindAddr, "::1")
	}
}
