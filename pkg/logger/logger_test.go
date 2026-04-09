package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mozgovojnikita/delivery-tracker/pkg/logger"
)

func TestNew_Production(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewWithWriter("production", &buf)
	l.Info("test message")

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected non-empty output")
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &m); err != nil {
		t.Fatalf("expected valid JSON output, got: %s, error: %v", output, err)
	}
	if _, ok := m["level"]; !ok {
		t.Errorf("expected 'level' field in JSON output, got: %s", output)
	}
	if _, ok := m["msg"]; !ok {
		t.Errorf("expected 'msg' field in JSON output, got: %s", output)
	}
}

func TestNew_Development(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewWithWriter("development", &buf)
	l.Info("test message")

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected non-empty output")
	}
	// Text handler uses key=value format
	if !strings.Contains(output, "level=") {
		t.Errorf("expected 'level=' in text output, got: %s", output)
	}
}

func TestNewWithService(t *testing.T) {
	var buf bytes.Buffer
	l := logger.NewWithServiceWriter("production", "order-service", &buf)
	l.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "order-service") {
		t.Errorf("expected service name 'order-service' in output, got: %s", output)
	}
}

func TestNew_ReturnsLogger(t *testing.T) {
	l := logger.New("production")
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if _, ok := interface{}(l).(*slog.Logger); !ok {
		t.Fatal("expected *slog.Logger type")
	}
}
