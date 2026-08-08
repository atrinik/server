// Package app composes service lifecycle without owning domain implementations.
package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/atrinik/server/internal/config"
	"github.com/atrinik/server/internal/observability"
)

// Stage names a shutdown phase in its required order.
type Stage string

const (
	StageAdmissionStop         Stage = "admission-stop"
	StageConnectionDrain       Stage = "connection-drain"
	StageSimulationQuiesce     Stage = "simulation-quiesce"
	StagePersistenceCheckpoint Stage = "persistence-checkpoint"
	StageTelemetryFlush        Stage = "telemetry-flush"
)

var shutdownOrder = [...]Stage{
	StageAdmissionStop,
	StageConnectionDrain,
	StageSimulationQuiesce,
	StagePersistenceCheckpoint,
	StageTelemetryFlush,
}

// Hook performs one idempotent lifecycle stage.
type Hook interface {
	Run(context.Context) error
}

// HookFunc adapts a function to Hook.
type HookFunc func(context.Context) error

// Run executes hook.
func (hook HookFunc) Run(ctx context.Context) error { return hook(ctx) }

// App is the single owner of application startup and ordered shutdown.
type App struct {
	configuration config.Config
	logger        *observability.Logger
	mu            sync.Mutex
	started       bool
	stopped       bool
	hooks         map[Stage][]Hook
	health        *observability.Health
	metrics       *observability.Registry
}

// New constructs an unstarted application.
func New(configuration config.Config, logger *observability.Logger) *App {
	return &App{
		configuration: configuration,
		logger:        logger,
		hooks:         make(map[Stage][]Hook),
		health:        &observability.Health{},
		metrics:       observability.DefaultRegistry(),
	}
}

// Register adds an ordered shutdown hook before startup.
func (application *App) Register(stage Stage, hook Hook) error {
	application.mu.Lock()
	defer application.mu.Unlock()
	if application.started || hook == nil {
		return errors.New("shutdown hooks must be non-nil and registered before startup")
	}
	if !validStage(stage) {
		return errors.New("unknown shutdown stage")
	}
	application.hooks[stage] = append(application.hooks[stage], hook)
	return nil
}

// Start validates configuration and transitions the application once.
func (application *App) Start(ctx context.Context) error {
	application.mu.Lock()
	defer application.mu.Unlock()
	if err := application.configuration.Validate(); err != nil {
		return err
	}
	if application.started {
		return errors.New("application already started")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	application.started = true
	application.health.SetReady(false, "gameplay resources and listeners are not implemented")
	application.logger.Event(ctx, slog.LevelInfo, "lifecycle", "server.start", "application lifecycle started")
	return nil
}

// Shutdown runs all registered hooks in fixed stage order and joins failures.
func (application *App) Shutdown(ctx context.Context) error {
	application.mu.Lock()
	if !application.started || application.stopped {
		application.mu.Unlock()
		return errors.New("application is not running")
	}
	application.stopped = true
	hooks := make(map[Stage][]Hook, len(application.hooks))
	for stage, registered := range application.hooks {
		hooks[stage] = append([]Hook(nil), registered...)
	}
	application.mu.Unlock()

	var failures []error
	for _, stage := range shutdownOrder {
		for _, hook := range hooks[stage] {
			if err := runHook(ctx, hook); err != nil {
				failures = append(failures, err)
			}
		}
		application.logger.Event(ctx, slog.LevelInfo, "lifecycle", "server.shutdown.stage", "shutdown stage complete", observability.String("stage", string(stage)))
	}
	return errors.Join(failures...)
}

func runHook(ctx context.Context, hook Hook) (failure error) {
	defer func() {
		if recover() != nil {
			failure = errors.New("shutdown hook panicked")
		}
	}()
	return hook.Run(ctx)
}

// DiagnosticSnapshot is a bounded immutable response for a local console or sidecar.
type DiagnosticSnapshot struct {
	Health  observability.HealthSnapshot `json:"health"`
	Metrics string                       `json:"openmetrics"`
}

// Diagnostics authorizes with the configured token and copies current health and metrics.
func (application *App) Diagnostics(token string, now time.Time) (DiagnosticSnapshot, error) {
	configured := application.configuration.AdminToken
	if configured == "" || len(token) > 4_096 {
		return DiagnosticSnapshot{}, errors.New("diagnostics are disabled or authorization failed")
	}
	configuredDigest := sha256.Sum256([]byte(configured))
	candidateDigest := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(candidateDigest[:], configuredDigest[:]) != 1 {
		return DiagnosticSnapshot{}, errors.New("diagnostics are disabled or authorization failed")
	}
	metrics, err := application.metrics.OpenMetrics(256 * 1024)
	if err != nil {
		return DiagnosticSnapshot{}, err
	}
	return DiagnosticSnapshot{
		Health:  application.health.Snapshot(now, 5*time.Second),
		Metrics: string(metrics),
	}, nil
}

func validStage(candidate Stage) bool {
	for _, stage := range shutdownOrder {
		if candidate == stage {
			return true
		}
	}
	return false
}
