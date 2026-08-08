// Package domain defines inward-facing contracts with no adapter types.
package domain

import (
	"context"
	"errors"
	"time"
)

// ContentCoordinate binds every immutable catalog snapshot.
type ContentCoordinate struct {
	FormatVersion string
	Revision      string
	SHA256        [32]byte
}

// Catalog exposes only validated immutable compiled content.
type Catalog interface {
	Coordinate() ContentCoordinate
	Lookup(kind, stableID string) (Record, bool)
}

// Record is a bounded compiled content projection, not source syntax.
type Record struct {
	StableID string
	Kind     string
	payload  []byte
}

// NewRecord validates identity and copies compiled bytes into immutable ownership.
func NewRecord(stableID, kind string, payload []byte) (Record, error) {
	record := Record{StableID: stableID, Kind: kind, payload: payload}
	if err := ValidateRecord(record); err != nil {
		return Record{}, err
	}
	record.payload = append([]byte(nil), payload...)
	return record, nil
}

// Payload returns a defensive copy of compiled bytes.
func (record Record) Payload() []byte { return append([]byte(nil), record.payload...) }

// StorageTransaction is a coordinated mutable-state transaction.
type StorageTransaction interface {
	Put(context.Context, string, []byte) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

// Storage begins explicit transactions and owns checkpoint/restore validation.
type Storage interface {
	Begin(context.Context) (StorageTransaction, error)
	Checkpoint(context.Context) error
}

// Event is a committed bounded domain event.
type Event struct {
	Revision uint64
	Kind     string
	payload  []byte
}

// NewEvent bounds identity and copies committed event bytes.
func NewEvent(revision uint64, kind string, payload []byte) (Event, error) {
	if revision == 0 || kind == "" || len(kind) > 80 || len(payload) > 1<<20 {
		return Event{}, errors.New("event is invalid or exceeds its bound")
	}
	return Event{Revision: revision, Kind: kind, payload: append([]byte(nil), payload...)}, nil
}

// Payload returns a defensive copy of committed bytes.
func (event Event) Payload() []byte { return append([]byte(nil), event.payload...) }

// Publisher accepts only committed events and owns backpressure.
type Publisher interface {
	Publish(context.Context, Event) error
}

// Queue submits typed commands with explicit overload/cancellation.
type Queue[T any] interface {
	Submit(context.Context, T) error
}

// Mutation makes validate/commit/cancel ownership explicit.
type Mutation interface {
	Preflight(context.Context) error
	Commit(context.Context) ([]Event, error)
	Cancel(context.Context) error
}

// Deadline distinguishes monotonic timeout policy from wall-clock display time.
type Deadline struct {
	At time.Time
}

// ValidateRecord prevents adapters from handing unbounded content into domains.
func ValidateRecord(record Record) error {
	if record.StableID == "" || len(record.StableID) > 160 || record.Kind == "" || len(record.Kind) > 80 {
		return errors.New("compiled record identity is invalid")
	}
	if len(record.payload) > 1<<20 {
		return errors.New("compiled record payload exceeds bound")
	}
	return nil
}
