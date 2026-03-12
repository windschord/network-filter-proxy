package config

import (
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ProxyPort       string
	APIPort         string
	APIBindAddr     string
	LogLevel        string
	LogFormat       string
	ShutdownTimeout time.Duration
}

func Load() Config {
	timeout := 30
	if v, err := strconv.Atoi(getEnv("SHUTDOWN_TIMEOUT", "30")); err == nil && v > 0 {
		timeout = v
	}
	apiBindAddr := getEnv("API_BIND_ADDR", "127.0.0.1")
	if net.ParseIP(apiBindAddr) == nil {
		apiBindAddr = "127.0.0.1"
	}
	return Config{
		ProxyPort:       getEnv("PROXY_PORT", "3128"),
		APIPort:         getEnv("API_PORT", "8080"),
		APIBindAddr:     apiBindAddr,
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
