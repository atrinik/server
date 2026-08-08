package observability

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const maxSpanAttributes = 12

// Tracer starts bounded OpenTelemetry spans without choosing an exporter.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer uses the process-global provider, which is a safe no-op until an operator configures export.
func NewTracer() Tracer {
	return Tracer{tracer: otel.Tracer("github.com/atrinik/server", trace.WithInstrumentationVersion("1"))}
}

// Start begins a stable operation span and returns its context and idempotent end function.
func (tracer Tracer) Start(ctx context.Context, name string, fields ...Field) (context.Context, func(error)) {
	if len(name) > maxEventName || !stableName.MatchString(name) {
		name = "observability.invalid-span-name"
	}
	if len(fields) > maxSpanAttributes {
		fields = fields[:maxSpanAttributes]
	}
	started := time.Now()
	attributes := spanAttributes(fields)
	spanContext, span := tracer.tracer.Start(ctx, name, trace.WithAttributes(attributes...))
	return spanContext, func(err error) {
		span.SetAttributes(attribute.Int64("atrinik.elapsed_ms", time.Since(started).Milliseconds()))
		if err != nil {
			span.SetAttributes(attribute.Bool("atrinik.error", true))
		}
		span.End()
	}
}

func spanAttributes(fields []Field) []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, 0, len(fields))
	for _, field := range fields {
		key := strings.ToLower(strings.TrimSpace(field.key))
		if _, sensitive := secretKeys[key]; sensitive {
			attributes = append(attributes, attribute.Bool("atrinik.redacted", true))
			continue
		}
		if !stableName.MatchString(key) || len(key) > maxFieldKey {
			continue
		}
		switch value := field.value.(type) {
		case string:
			if _, allowed := allowedStringFields[key]; allowed {
				attributes = append(attributes, attribute.String("atrinik."+key, truncate(value, maxFieldValue)))
			}
		case int64:
			attributes = append(attributes, attribute.Int64("atrinik."+key, value))
		case bool:
			attributes = append(attributes, attribute.Bool("atrinik."+key, value))
		}
	}
	if len(attributes) == 0 {
		return nil
	}
	return attributes
}
