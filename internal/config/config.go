package config

import (
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ProxyPort            string
	APIPort              string
	APIBindAddr          string
	APIBindAddrFallback  bool // true if API_BIND_ADDR was invalid and fell back to 127.0.0.1
	LogLevel             string
	LogFormat            string
	ShutdownTimeout      time.Duration
}

func Load() Config {
	timeout := 30
	if v, err := strconv.Atoi(getEnv("SHUTDOWN_TIMEOUT", "30")); err == nil && v > 0 {
		timeout = v
	}
	apiBindAddrRaw := getEnv("API_BIND_ADDR", "127.0.0.1")
	apiBindAddr := apiBindAddrRaw
	apiBindAddrFallback := false
	if net.ParseIP(apiBindAddr) == nil {
		apiBindAddr = "127.0.0.1"
		apiBindAddrFallback = true
	}
	return Config{
		ProxyPort:           getEnv("PROXY_PORT", "3128"),
		APIPort:             getEnv("API_PORT", "8080"),
		APIBindAddr:         apiBindAddr,
		APIBindAddrFallback: apiBindAddrFallback,
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		LogFormat:       getEnv("LOG_FORMAT", "json"),
		ShutdownTimeout: time.Duration(timeout) * time.Second,
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
