// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	metaserverv1 "github.com/atrinik/protocol/gen/go/atrinik/metaserver/v1"
)

const (
	DefaultHeartbeatInterval = 150 * time.Minute
	DefaultHeartbeatJitter   = 15 * time.Minute
	MaximumHeartbeatInterval = 210 * time.Minute
	DefaultDebounce          = 5 * time.Second
	DefaultMinimumBackoff    = 5 * time.Second
	DefaultMaximumBackoff    = 5 * time.Minute
	maximumAttemptsPerDay    = 47
)

// AttemptPublisher is implemented by Client and deterministic test adapters.
type AttemptPublisher interface {
	ValidateSnapshot(Snapshot) error
	Publish(context.Context, Snapshot) (Result, error)
}

// ServiceEvent contains only closed, low-cardinality operational state.
type ServiceEvent struct {
	Trigger string
	Result  ResultKind
}

// ServiceConfig owns bounded publication cadence and hooks.
type ServiceConfig struct {
	HeartbeatInterval time.Duration
	HeartbeatJitter   time.Duration
	Debounce          time.Duration
	MinimumBackoff    time.Duration
	MaximumBackoff    time.Duration
	Entropy           io.Reader
	Now               func() time.Time
	Observe           func(ServiceEvent)
}

// DefaultServiceConfig returns the reviewed production cadence.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		HeartbeatInterval: DefaultHeartbeatInterval,
		HeartbeatJitter:   DefaultHeartbeatJitter,
		Debounce:          DefaultDebounce,
		MinimumBackoff:    DefaultMinimumBackoff,
		MaximumBackoff:    DefaultMaximumBackoff,
		Entropy:           rand.Reader,
		Now:               time.Now,
	}
}

// Service serializes startup, dirty, heartbeat, and retry publication work.
type Service struct {
	publisher AttemptPublisher
	config    ServiceConfig

	mu      sync.Mutex
	latest  Snapshot
	started bool
	stopped bool
	updates chan struct{}
	resume  chan struct{}
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewService validates a bounded scheduler configuration.
func NewService(client AttemptPublisher, configuration ServiceConfig) (*Service, error) {
	if client == nil || configuration.HeartbeatInterval < time.Hour ||
		configuration.HeartbeatInterval > MaximumHeartbeatInterval ||
		configuration.HeartbeatJitter < 0 ||
		configuration.HeartbeatJitter >= configuration.HeartbeatInterval ||
		configuration.HeartbeatInterval+configuration.HeartbeatJitter >= 4*time.Hour ||
		configuration.Debounce <= 0 || configuration.Debounce > time.Minute ||
		configuration.MinimumBackoff <= 0 ||
		configuration.MaximumBackoff < configuration.MinimumBackoff ||
		configuration.MaximumBackoff > time.Hour ||
		configuration.Entropy == nil || configuration.Now == nil {
		return nil, errors.New("publisher service configuration is invalid")
	}
	return &Service{
		publisher: client,
		config:    configuration,
		updates:   make(chan struct{}, 1),
		resume:    make(chan struct{}, 1),
		done:      make(chan struct{}),
	}, nil
}

// Start validates the initial snapshot and starts one serialized scheduler.
func (service *Service) Start(ctx context.Context, initial Snapshot) error {
	if err := validateSnapshot(initial); err != nil {
		return err
	}
	if err := service.publisher.ValidateSnapshot(initial); err != nil {
		return err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.started || service.stopped {
		return errors.New("publisher service cannot be started")
	}
	runContext, cancel := context.WithCancel(ctx)
	service.latest = cloneSnapshot(initial)
	service.cancel = cancel
	service.started = true
	go service.run(runContext)
	return nil
}

// Update coalesces the latest complete snapshot behind one debounce timer.
func (service *Service) Update(snapshot Snapshot) error {
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	if err := service.publisher.ValidateSnapshot(snapshot); err != nil {
		return err
	}
	service.mu.Lock()
	if !service.started || service.stopped {
		service.mu.Unlock()
		return errors.New("publisher service is not running")
	}
	if snapshotsEqual(service.latest, snapshot) {
		service.mu.Unlock()
		return nil
	}
	service.latest = cloneSnapshot(snapshot)
	service.mu.Unlock()
	select {
	case service.updates <- struct{}{}:
	default:
	}
	return nil
}

// Resume explicitly retries after a permanent response without changing state.
func (service *Service) Resume() error {
	service.mu.Lock()
	running := service.started && !service.stopped
	service.mu.Unlock()
	if !running {
		return errors.New("publisher service is not running")
	}
	select {
	case service.resume <- struct{}{}:
	default:
	}
	return nil
}

// Close stops scheduling and waits for the in-flight request context to end.
func (service *Service) Close(ctx context.Context) error {
	service.mu.Lock()
	if service.stopped {
		service.mu.Unlock()
		return nil
	}
	service.stopped = true
	if !service.started {
		service.mu.Unlock()
		return nil
	}
	service.cancel()
	done := service.done
	service.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (service *Service) run(ctx context.Context) {
	defer close(service.done)
	timer := time.NewTimer(0)
	defer timer.Stop()
	trigger := "startup"
	backoff := service.config.MinimumBackoff
	replayRetried := false
	suspended := false
	var attempts []time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-service.updates:
			suspended = false
			replayRetried = false
			backoff = service.config.MinimumBackoff
			trigger = "change"
			resetTimer(timer, service.config.Debounce)
			continue
		case <-service.resume:
			suspended = false
			replayRetried = false
			backoff = service.config.MinimumBackoff
			trigger = "resume"
			resetTimer(timer, 0)
			continue
		case <-timer.C:
		}

		if suspended {
			continue
		}
		now := service.config.Now().UTC()
		attempts = pruneAttempts(attempts, now)
		if len(attempts) >= maximumAttemptsPerDay {
			trigger = "budget"
			resetTimer(timer, attempts[0].Add(24*time.Hour).Sub(now))
			continue
		}
		attempts = append(attempts, now)
		result, err := service.publisher.Publish(ctx, service.snapshot())
		if err != nil {
			service.observe(ServiceEvent{Trigger: trigger, Result: ResultPermanent})
			suspended = true
			continue
		}
		if result.validate() != nil {
			service.observe(ServiceEvent{Trigger: trigger, Result: ResultPermanent})
			suspended = true
			continue
		}
		service.observe(ServiceEvent{Trigger: trigger, Result: result.Kind})
		switch result.Kind {
		case ResultAccepted:
			clear(result.RendezvousToken[:])
			backoff = service.config.MinimumBackoff
			replayRetried = false
			trigger = "heartbeat"
			resetTimer(timer, service.heartbeatDelay())
		case ResultReplay:
			if replayRetried {
				suspended = true
				continue
			}
			replayRetried = true
			trigger = "replay-recovery"
			resetTimer(timer, 0)
		case ResultRateLimited:
			trigger = "rate-limit-retry"
			resetTimer(timer, result.RetryAfter)
		case ResultTransient:
			trigger = "transient-retry"
			if result.RetryAfter > 0 {
				resetTimer(timer, result.RetryAfter)
			} else {
				resetTimer(timer, service.backoffDelay(backoff))
				backoff = min(backoff*2, service.config.MaximumBackoff)
			}
		case ResultPermanent:
			suspended = true
		}
	}
}

func (service *Service) snapshot() Snapshot {
	service.mu.Lock()
	defer service.mu.Unlock()
	return cloneSnapshot(service.latest)
}

func (service *Service) heartbeatDelay() time.Duration {
	return jitter(service.config.HeartbeatInterval, service.config.HeartbeatJitter, service.config.Entropy)
}

func (service *Service) backoffDelay(base time.Duration) time.Duration {
	return jitter(base, base/4, service.config.Entropy)
}

func (service *Service) observe(event ServiceEvent) {
	if service.config.Observe != nil {
		service.config.Observe(event)
	}
}

func validateSnapshot(snapshot Snapshot) error {
	if snapshot.PlayersCapacity == 0 || snapshot.PlayersOnline > snapshot.PlayersCapacity ||
		snapshot.Status == metaserverv1.DirectoryServerStatus_DIRECTORY_SERVER_STATUS_UNSPECIFIED {
		return errors.New("publisher snapshot is invalid")
	}
	return nil
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Region = cloneString(snapshot.Region)
	snapshot.Endpoint = cloneEndpoint(snapshot.Endpoint)
	return snapshot
}

func snapshotsEqual(left, right Snapshot) bool {
	return left.Name == right.Name && left.Description == right.Description &&
		optionalStringsEqual(left.Region, right.Region) && left.ProtocolMinor == right.ProtocolMinor &&
		left.ContentID == right.ContentID && left.ContentRevisionSHA256 == right.ContentRevisionSHA256 &&
		left.PlayersOnline == right.PlayersOnline && left.PlayersCapacity == right.PlayersCapacity &&
		left.Status == right.Status && left.Public == right.Public &&
		left.PasswordRequired == right.PasswordRequired && endpointsEqual(left.Endpoint, right.Endpoint)
}

func optionalStringsEqual(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func endpointsEqual(left, right *metaserverv1.DirectEndpoint) bool {
	return left == nil && right == nil || left != nil && right != nil &&
		left.Hostname == right.Hostname && left.Port == right.Port
}

func resetTimer(timer *time.Timer, delay time.Duration) {
	if delay < 0 {
		delay = 0
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
}

func jitter(base, spread time.Duration, entropy io.Reader) time.Duration {
	if spread <= 0 {
		return base
	}
	var encoded [8]byte
	if _, err := io.ReadFull(entropy, encoded[:]); err != nil {
		return base
	}
	width := uint64(spread)*2 + 1
	offset := time.Duration(binary.BigEndian.Uint64(encoded[:])%width) - spread
	return base + offset
}

func pruneAttempts(attempts []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-24 * time.Hour)
	first := 0
	for first < len(attempts) && !attempts[first].After(cutoff) {
		first++
	}
	return attempts[first:]
}
