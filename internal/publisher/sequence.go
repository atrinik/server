// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path"
	"sync"
	"time"
)

const (
	sequenceRecordBytes  = 32
	maximumSequenceBytes = 64 * 1024 * 1024
)

var sequenceMagic = [8]byte{'A', 'T', 'R', 'P', 'S', 'E', 'Q', '1'}

// SequenceStore is an append-only, fsync-before-return high-water ledger.
// A truncated final record is safely discarded on restart; any interior
// corruption fails closed. The wrapper owns single-process state isolation.
type SequenceStore struct {
	mu            sync.Mutex
	file          *os.File
	highWater     uint64
	attempts      []int64
	lastAttemptAt int64
}

// OpenSequenceStore opens or creates a bounded owner-only sequence ledger.
func OpenSequenceStore(root *os.Root, name string) (*SequenceStore, error) {
	if root == nil || name == "" {
		return nil, errors.New("publisher sequence path is empty")
	}
	if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
		return nil, errors.New("create publisher state directory")
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, errors.New("open publisher sequence ledger")
	}
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
		}
	}()
	information, err := file.Stat()
	if err != nil || !information.Mode().IsRegular() ||
		information.Size() < 0 || information.Size() > maximumSequenceBytes ||
		information.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("publisher sequence ledger is unsafe or outside supported bounds")
	}
	completeBytes := information.Size() - information.Size()%sequenceRecordBytes
	if information.Size() != completeBytes {
		if err := file.Truncate(completeBytes); err != nil || file.Sync() != nil {
			return nil, errors.New("repair publisher sequence ledger")
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("read publisher sequence ledger")
	}
	store := &SequenceStore{file: file}
	var record [sequenceRecordBytes]byte
	for offset := int64(0); offset < completeBytes; offset += sequenceRecordBytes {
		if _, err := io.ReadFull(file, record[:]); err != nil {
			return nil, errors.New("read publisher sequence ledger")
		}
		sequence, attemptedAt, valid := parseSequenceRecord(record[:])
		if !valid || sequence == 0 || sequence <= store.highWater {
			return nil, errors.New("publisher sequence ledger is corrupt")
		}
		store.highWater = sequence
		if attemptedAt > 0 {
			if attemptedAt < store.lastAttemptAt {
				return nil, errors.New("publisher sequence ledger clock order is corrupt")
			}
			store.attempts = append(store.attempts, attemptedAt)
			store.lastAttemptAt = attemptedAt
		}
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return nil, errors.New("position publisher sequence ledger")
	}
	failed = false
	return store, nil
}

// Reserve durably consumes and returns the next sequence before network I/O.
// The owner-local rolling budget survives restart; the Worker independently
// enforces the authenticated identity budget.
func (store *SequenceStore) Reserve(now time.Time) (uint64, time.Duration, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.file == nil || store.highWater == math.MaxUint64 || now.Unix() < 1 ||
		now.Unix() < store.lastAttemptAt {
		return 0, 0, errors.New("publisher sequence is exhausted, closed, or has an invalid clock")
	}
	store.pruneAttempts(now.Unix())
	if len(store.attempts) >= maximumAttemptsPerDay {
		retry := time.Unix(store.attempts[0], 0).Add(24 * time.Hour).Sub(now)
		if retry < time.Second {
			retry = time.Second
		}
		return 0, retry, nil
	}
	sequence, err := store.persist(store.highWater+1, now.Unix())
	if err != nil {
		return 0, 0, err
	}
	store.attempts = append(store.attempts, now.Unix())
	store.lastAttemptAt = now.Unix()
	return sequence, 0, nil
}

// AdvanceMinimum durably raises the next reservation to at least minimum.
func (store *SequenceStore) AdvanceMinimum(minimum uint64) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.file == nil || minimum == 0 {
		return errors.New("publisher sequence recovery value is invalid")
	}
	floor := minimum - 1
	if floor <= store.highWater {
		return nil
	}
	_, err := store.persist(floor, 0)
	return err
}

func (store *SequenceStore) persist(sequence uint64, attemptedAt int64) (uint64, error) {
	information, err := store.file.Stat()
	if err != nil || information.Size() > maximumSequenceBytes-sequenceRecordBytes {
		return 0, errors.New("publisher sequence ledger capacity is exhausted")
	}
	record := makeSequenceRecord(sequence, attemptedAt)
	if _, err := store.file.Write(record[:]); err != nil {
		store.failClosed()
		return 0, errors.New("persist publisher sequence")
	}
	if err := store.file.Sync(); err != nil {
		store.failClosed()
		return 0, errors.New("sync publisher sequence")
	}
	store.highWater = sequence
	return sequence, nil
}

func (store *SequenceStore) pruneAttempts(now int64) {
	cutoff := now - int64((24*time.Hour)/time.Second)
	first := 0
	for first < len(store.attempts) && store.attempts[first] <= cutoff {
		first++
	}
	store.attempts = store.attempts[first:]
}

func (store *SequenceStore) failClosed() {
	_ = store.file.Close()
	store.file = nil
}

// HighWater returns the process-local copy of the durable high-water mark.
func (store *SequenceStore) HighWater() uint64 {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.highWater
}

// Close releases the ledger. It is idempotent.
func (store *SequenceStore) Close() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.file == nil {
		return nil
	}
	err := store.file.Close()
	store.file = nil
	return err
}

func makeSequenceRecord(sequence uint64, attemptedAt int64) [sequenceRecordBytes]byte {
	var record [sequenceRecordBytes]byte
	copy(record[:8], sequenceMagic[:])
	binary.BigEndian.PutUint64(record[8:16], sequence)
	binary.BigEndian.PutUint64(record[16:24], uint64(attemptedAt))
	digest := sha256.Sum256(record[:24])
	copy(record[24:], digest[:8])
	return record
}

func parseSequenceRecord(record []byte) (uint64, int64, bool) {
	if len(record) != sequenceRecordBytes || !bytes.Equal(record[:8], sequenceMagic[:]) {
		return 0, 0, false
	}
	digest := sha256.Sum256(record[:24])
	if !bytes.Equal(record[24:], digest[:8]) {
		return 0, 0, false
	}
	timestamp := binary.BigEndian.Uint64(record[16:24])
	if timestamp > math.MaxInt64 {
		return 0, 0, false
	}
	return binary.BigEndian.Uint64(record[8:16]), int64(timestamp), true
}
