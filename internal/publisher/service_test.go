// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestServicePublishesStartupCoalescedChangeAndStops(t *testing.T) {
	t.Parallel()
	attempts := make(chan Snapshot, 4)
	fake := attemptPublisherFunc(func(_ context.Context, snapshot Snapshot) (Result, error) {
		attempts <- snapshot
		return Result{Kind: ResultAccepted}, nil
	})
	configuration := testServiceConfig()
	service, err := NewService(fake, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), testSnapshot()); err != nil {
		t.Fatal(err)
	}
	first := receiveSnapshot(t, attempts)
	if first.PlayersOnline != 1 {
		t.Fatalf("startup players = %d", first.PlayersOnline)
	}
	changed := testSnapshot()
	changed.PlayersOnline = 2
	if err := service.Update(changed); err != nil {
		t.Fatal(err)
	}
	changed.PlayersOnline = 3
	if err := service.Update(changed); err != nil {
		t.Fatal(err)
	}
	second := receiveSnapshot(t, attempts)
	if second.PlayersOnline != 3 {
		t.Fatalf("coalesced players = %d", second.PlayersOnline)
	}
	assertNoAttempt(t, attempts, 20*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServiceDoesNotPublishAnIdenticalSnapshot(t *testing.T) {
	t.Parallel()
	attempts := make(chan Snapshot, 2)
	fake := attemptPublisherFunc(func(_ context.Context, snapshot Snapshot) (Result, error) {
		attempts <- snapshot
		return Result{Kind: ResultAccepted}, nil
	})
	service, err := NewService(fake, testServiceConfig())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshot()
	if err := service.Start(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	receiveSnapshot(t, attempts)
	if err := service.Update(snapshot); err != nil {
		t.Fatal(err)
	}
	assertNoAttempt(t, attempts, 20*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServiceRetriesReplayOnceAndRequiresResumeAfterPermanentResult(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	results := []Result{
		{Kind: ResultReplay, MinimumNextSequence: 2},
		{Kind: ResultReplay, MinimumNextSequence: 3},
		{Kind: ResultAccepted},
	}
	attempted := make(chan struct{}, 4)
	fake := attemptPublisherFunc(func(context.Context, Snapshot) (Result, error) {
		mu.Lock()
		defer mu.Unlock()
		attempted <- struct{}{}
		result := results[0]
		results = results[1:]
		return result, nil
	})
	service, err := NewService(fake, testServiceConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), testSnapshot()); err != nil {
		t.Fatal(err)
	}
	receiveAttempt(t, attempted)
	receiveAttempt(t, attempted)
	assertNoAttempt(t, attempted, 20*time.Millisecond)
	if err := service.Resume(); err != nil {
		t.Fatal(err)
	}
	receiveAttempt(t, attempted)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServiceSuspendsAnInternallyInvalidRetryDecision(t *testing.T) {
	t.Parallel()
	attempted := make(chan struct{}, 2)
	fake := attemptPublisherFunc(func(context.Context, Snapshot) (Result, error) {
		attempted <- struct{}{}
		return Result{Kind: ResultRateLimited}, nil
	})
	service, err := NewService(fake, testServiceConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), testSnapshot()); err != nil {
		t.Fatal(err)
	}
	receiveAttempt(t, attempted)
	assertNoAttempt(t, attempted, 20*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestServiceRejectsHeartbeatJitterAtThePresenceLifetime(t *testing.T) {
	t.Parallel()
	configuration := testServiceConfig()
	configuration.HeartbeatInterval = MaximumHeartbeatInterval
	configuration.HeartbeatJitter = 30 * time.Minute
	if _, err := NewService(attemptPublisherFunc(func(context.Context, Snapshot) (Result, error) {
		return Result{Kind: ResultAccepted}, nil
	}), configuration); err == nil {
		t.Fatal("heartbeat delay reaching the presence lifetime was accepted")
	}
}

func TestServiceCannotStartAfterClose(t *testing.T) {
	t.Parallel()
	service, err := NewService(attemptPublisherFunc(func(context.Context, Snapshot) (Result, error) {
		return Result{Kind: ResultAccepted}, nil
	}), testServiceConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), testSnapshot()); err == nil {
		t.Fatal("closed publisher service started")
	}
}

func TestServiceHonorsRetryAfterAndHardDailyAttemptCeiling(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	count := 0
	attempted := make(chan time.Time, maximumAttemptsPerDay+2)
	fake := attemptPublisherFunc(func(context.Context, Snapshot) (Result, error) {
		mu.Lock()
		defer mu.Unlock()
		count++
		attempted <- time.Now()
		if count == 1 {
			return Result{Kind: ResultRateLimited, RetryAfter: 15 * time.Millisecond}, nil
		}
		return Result{Kind: ResultTransient}, nil
	})
	configuration := testServiceConfig()
	configuration.MinimumBackoff = time.Millisecond
	configuration.MaximumBackoff = time.Millisecond
	service, err := NewService(fake, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Start(context.Background(), testSnapshot()); err != nil {
		t.Fatal(err)
	}
	first := receiveTime(t, attempted)
	second := receiveTime(t, attempted)
	if second.Sub(first) < 12*time.Millisecond {
		t.Fatalf("Retry-After was not honored: %s", second.Sub(first))
	}
	for index := 2; index < maximumAttemptsPerDay; index++ {
		receiveTime(t, attempted)
	}
	assertNoAttempt(t, attempted, 20*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func testServiceConfig() ServiceConfig {
	configuration := DefaultServiceConfig()
	configuration.HeartbeatInterval = time.Hour
	configuration.HeartbeatJitter = 0
	configuration.Debounce = time.Millisecond
	configuration.MinimumBackoff = time.Millisecond
	configuration.MaximumBackoff = 2 * time.Millisecond
	configuration.Entropy = zeroReader{}
	return configuration
}

type attemptPublisherFunc func(context.Context, Snapshot) (Result, error)

func (attemptPublisherFunc) ValidateSnapshot(snapshot Snapshot) error {
	return validateSnapshot(snapshot)
}

func (function attemptPublisherFunc) Publish(ctx context.Context, snapshot Snapshot) (Result, error) {
	return function(ctx, snapshot)
}

type zeroReader struct{}

func (zeroReader) Read(value []byte) (int, error) {
	clear(value)
	return len(value), nil
}

func receiveSnapshot(t *testing.T, channel <-chan Snapshot) Snapshot {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(time.Second):
		t.Fatal("publisher attempt timed out")
		return Snapshot{}
	}
}

func receiveAttempt(t *testing.T, channel <-chan struct{}) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(time.Second):
		t.Fatal("publisher attempt timed out")
	}
}

func receiveTime(t *testing.T, channel <-chan time.Time) time.Time {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(time.Second):
		t.Fatal("publisher attempt timed out")
		return time.Time{}
	}
}

func assertNoAttempt[T any](t *testing.T, channel <-chan T, duration time.Duration) {
	t.Helper()
	select {
	case <-channel:
		t.Fatal("unexpected publisher attempt")
	case <-time.After(duration):
	}
}
