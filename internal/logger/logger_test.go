package logger_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/claudework/network-filter-proxy/internal/logger"
)

func TestNew_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter("json", "info", &buf)
	log.Info("test message")

	output := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Errorf("expected JSON output starting with '{', got %q", output)
	}
	if !strings.Contains(output, `"timestamp"`) {
		t.Errorf("expected 'timestamp' key in JSON output, got %q", output)
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter("text", "debug", &buf)
	log.Debug("test message")

	output := buf.String()
	if !strings.Contains(output, "timestamp=") {
		t.Errorf("expected 'timestamp=' in text output, got %q", output)
	}
}

func TestNew_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter("json", "warn", &buf)
	log.Info("should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output for info level when level is warn, got %q", buf.String())
	}
}

func TestNew_DefaultOutput(t *testing.T) {
	// New() should return a valid logger (writing to os.Stdout)
	log := logger.New("json", "info")
	if log == nil {
		t.Error("expected non-nil logger")
	}
}

func TestNew_InvalidFormatDefaultsToJSON(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter("invalid", "info", &buf)
	log.Info("test")

	output := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Errorf("expected JSON output for invalid format, got %q", output)
	}
}

func TestNew_InvalidLevelDefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	log := logger.NewWithWriter("json", "invalid", &buf)

	// Debug should not appear when level defaults to info
	log.Debug("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output for debug level when level defaults to info, got %q", buf.String())
	}

	log.Log(nil, slog.LevelInfo, "should appear")
	if buf.Len() == 0 {
		t.Error("expected output for info level")
	}
}
