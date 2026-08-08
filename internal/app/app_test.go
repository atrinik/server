package app

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/atrinik/server/internal/config"
	"github.com/atrinik/server/internal/observability"
)

func TestShutdownOrderIsFixed(t *testing.T) {
	t.Parallel()
	logger, err := observability.NewLogger(&bytes.Buffer{}, "json")
	if err != nil {
		t.Fatal(err)
	}
	application := New(config.Default(), logger)
	var actual []Stage
	for _, stage := range shutdownOrder {
		stage := stage
		if err := application.Register(stage, HookFunc(func(context.Context) error {
			actual = append(actual, stage)
			return nil
		})); err != nil {
			t.Fatal(err)
		}
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, shutdownOrder[:]) {
		t.Fatalf("shutdown order = %v, want %v", actual, shutdownOrder)
	}
}

func TestShutdownContainsPanicAndContinues(t *testing.T) {
	t.Parallel()
	logger, err := observability.NewLogger(&bytes.Buffer{}, "json")
	if err != nil {
		t.Fatal(err)
	}
	application := New(config.Default(), logger)
	continued := false
	if err := application.Register(StageAdmissionStop, HookFunc(func(context.Context) error { panic("synthetic") })); err != nil {
		t.Fatal(err)
	}
	if err := application.Register(StageTelemetryFlush, HookFunc(func(context.Context) error { continued = true; return nil })); err != nil {
		t.Fatal(err)
	}
	if err := application.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := application.Shutdown(context.Background()); err == nil || !continued {
		t.Fatalf("shutdown error = %v, continued = %v", err, continued)
	}
}

func TestDiagnosticsRequireConfiguredTokenAndStayBounded(t *testing.T) {
	t.Parallel()
	logger, err := observability.NewLogger(&bytes.Buffer{}, "json")
	if err != nil {
		t.Fatal(err)
	}
	configuration := config.Default()
	application := New(configuration, logger)
	if _, err := application.Diagnostics("", time.Unix(1, 0)); err == nil {
		t.Fatal("diagnostics were enabled without a configured token")
	}
	configuration.AdminToken = "diagnostic-secret-that-is-32-bytes"
	application = New(configuration, logger)
	if _, err := application.Diagnostics("wrong-secret", time.Unix(1, 0)); err == nil {
		t.Fatal("diagnostics accepted the wrong token")
	}
	snapshot, err := application.Diagnostics("diagnostic-secret-that-is-32-bytes", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Metrics) > 256*1024 || snapshot.Health.Ready || snapshot.Health.Live {
		t.Fatal("diagnostics violated bounds or foundation readiness semantics")
	}
}
