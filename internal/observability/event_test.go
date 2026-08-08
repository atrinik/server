package observability

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestJSONEventsAreBoundedAndRedacted(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger, err := NewLogger(&output, "json")
	if err != nil {
		t.Fatal(err)
	}
	logger.Event(context.Background(), slog.LevelInfo, "network", "session.open", strings.Repeat("m", maxMessage+20),
		String("password", "secret"), String("diagnostic-id", strings.Repeat("x", maxFieldValue+20)))
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON event: %v", err)
	}
	if record["redacted"] != true || strings.Contains(output.String(), "secret") {
		t.Fatal("secret was not redacted")
	}
	if len([]rune(record["msg"].(string))) != maxMessage || len([]rune(record["diagnostic-id"].(string))) != maxFieldValue {
		t.Fatal("event bounds were not applied")
	}
	if record["source"] == nil || record["build_version"] == nil || record["protocol_contract"] == nil {
		t.Fatal("event lacks source or build correlation fields")
	}
}

func TestUnclassifiedAndMalformedStringFieldsAreDropped(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger, err := NewLogger(&output, "json")
	if err != nil {
		t.Fatal(err)
	}
	logger.Event(context.Background(), slog.LevelInfo, "network", "packet.rejected", "rejected",
		String("player-name", "private"), String("reason", "arbitrary prose"), String("diagnostic-id", "contains spaces"), String("result", "invalid-input"))
	if strings.Contains(output.String(), "private") || strings.Contains(output.String(), "arbitrary prose") || strings.Contains(output.String(), "contains spaces") {
		t.Fatal("unclassified or malformed string entered event")
	}
	if !strings.Contains(output.String(), `"result":"invalid-input"`) {
		t.Fatal("bounded enum field was not retained")
	}
}

func FuzzJSONEventNeverEmitsSecret(f *testing.F) {
	f.Add("secret", "ordinary")
	f.Fuzz(func(t *testing.T, secret, message string) {
		if secret == "" {
			return
		}
		var output bytes.Buffer
		logger, err := NewLogger(&output, "json")
		if err != nil {
			t.Fatal(err)
		}
		logger.Event(context.Background(), slog.LevelInfo, "security", "authentication.denied", message, String("token", secret))
		if strings.Contains(output.String(), secret) {
			t.Fatal("secret entered structured output")
		}
		var record map[string]any
		if err := json.Unmarshal(output.Bytes(), &record); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
	})
}

func TestHumanLoggerRemainsAvailable(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger, err := NewLogger(&output, "human")
	if err != nil {
		t.Fatal(err)
	}
	logger.Event(context.Background(), slog.LevelInfo, "lifecycle", "server.ready", "ready")
	if scanner := bufio.NewScanner(&output); !scanner.Scan() || !strings.Contains(scanner.Text(), "event=server.ready") {
		t.Fatal("human event output is missing")
	}
}
