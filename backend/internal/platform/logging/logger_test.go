package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewUsesJSONInProduction(t *testing.T) {
	var output bytes.Buffer
	logger := New("production", &output)

	logger.Info("started", "port", 7666)

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("log is not JSON: %v", err)
	}
	if entry["level"] != "INFO" || entry["msg"] != "started" || entry["port"] != float64(7666) {
		t.Fatalf("unexpected log entry: %#v", entry)
	}
}

func TestNewUsesTextOutsideProduction(t *testing.T) {
	var output bytes.Buffer
	logger := New("development", &output)

	logger.Info("started", "port", 7666)

	line := output.String()
	if !strings.Contains(line, "level=INFO") || !strings.Contains(line, "msg=started") || !strings.Contains(line, "port=7666") {
		t.Fatalf("unexpected log line: %q", line)
	}
}
