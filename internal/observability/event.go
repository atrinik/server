// Package observability owns bounded operational logs, metrics, health, and traces.
package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/atrinik/server/internal/buildinfo"
)

const (
	maxEventName  = 80
	maxMessage    = 512
	maxFields     = 16
	maxFieldKey   = 48
	maxFieldValue = 256
)

var stableName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)

var secretKeys = map[string]struct{}{
	"account": {}, "admin-token": {}, "admin_token": {}, "authorization": {},
	"character": {}, "chat": {}, "credential": {}, "dialog": {}, "password": {},
	"player": {}, "private-key": {}, "private_key": {}, "salt": {}, "token": {},
}

var allowedStringFields = map[string]struct{}{
	"component": {}, "diagnostic-id": {}, "kind": {}, "map-id": {},
	"operation-id": {}, "reason": {}, "result": {}, "session-safe-id": {},
	"source": {}, "stage": {},
}

// Field is one bounded typed log field.
type Field struct {
	key   string
	value any
}

// String constructs a string field. Bounds and redaction are applied at emission.
func String(key, value string) Field { return Field{key: key, value: value} }

// Int64 constructs an integer field.
func Int64(key string, value int64) Field { return Field{key: key, value: value} }

// Bool constructs a boolean field.
func Bool(key string, value bool) Field { return Field{key: key, value: value} }

// Logger emits schema-versioned bounded events through Go slog.
type Logger struct {
	logger  *slog.Logger
	started time.Time
}

// NewLogger creates JSON or human development output.
func NewLogger(output io.Writer, format string) (*Logger, error) {
	if output == nil {
		return nil, errors.New("log output is nil")
	}
	options := &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true, ReplaceAttr: func(_ []string, attribute slog.Attr) slog.Attr {
		if attribute.Key == slog.TimeKey {
			attribute.Value = slog.TimeValue(attribute.Value.Time().UTC())
		}
		return attribute
	}}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(output, options)
	case "human":
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, errors.New("log format must be json or human")
	}
	build := buildinfo.Current()
	base := slog.New(handler).With(
		"process", "atrinik-server",
		"build_version", build.Version,
		"build_revision", build.Revision,
		"protocol_contract", build.ProtocolContract,
	)
	return &Logger{logger: base, started: time.Now()}, nil
}

// Event emits one validated event. Invalid names are replaced with a stable diagnostic.
func (logger *Logger) Event(ctx context.Context, level slog.Level, subsystem, name, message string, fields ...Field) {
	if logger == nil {
		return
	}
	if len(subsystem) > maxEventName || !stableName.MatchString(subsystem) {
		subsystem = "invalid"
	}
	if len(name) > maxEventName || !stableName.MatchString(name) {
		name = "observability.invalid-event-name"
	}
	attributes := []any{
		"schema", 1,
		"subsystem", subsystem,
		"event", name,
		"elapsed_ms", time.Since(logger.started).Milliseconds(),
	}
	if len(fields) > maxFields {
		fields = fields[:maxFields]
	}
	for _, field := range fields {
		key := strings.ToLower(strings.TrimSpace(field.key))
		if _, sensitive := secretKeys[key]; sensitive {
			attributes = append(attributes, "redacted", true)
			continue
		}
		if !stableName.MatchString(key) || len(key) > maxFieldKey {
			continue
		}
		switch value := field.value.(type) {
		case string:
			if _, allowed := allowedStringFields[key]; allowed {
				attributes = append(attributes, key, truncate(value, maxFieldValue))
			}
		case int64, bool:
			attributes = append(attributes, key, value)
		}
	}
	var callers [1]uintptr
	runtime.Callers(2, callers[:])
	record := slog.NewRecord(time.Now().UTC(), level, truncate(message, maxMessage), callers[0])
	record.Add(attributes...)
	_ = logger.logger.Handler().Handle(ctx, record)
}

// Audit emits a separately classified privileged/security event.
func (logger *Logger) Audit(ctx context.Context, name, message string, fields ...Field) {
	logger.Event(ctx, slog.LevelWarn, "security-audit", name, message, fields...)
}

func truncate(value string, maximum int) string {
	if maximum <= 0 {
		return ""
	}
	runes := 0
	for index := range value {
		if runes == maximum {
			return value[:index]
		}
		runes++
	}
	return value
}
