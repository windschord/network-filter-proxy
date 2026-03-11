package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ProxyPort       string
	APIPort         string
	LogLevel        string
	LogFormat       string
	ShutdownTimeout time.Duration
}

func Load() Config {
	timeout := 30
	if v, err := strconv.Atoi(getEnv("SHUTDOWN_TIMEOUT", "30")); err == nil && v > 0 {
		timeout = v
	}
	return Config{
		ProxyPort:       getEnv("PROXY_PORT", "3128"),
		APIPort:         getEnv("API_PORT", "8080"),
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
