// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package publisher

import (
	"os"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestSequenceStorePersistsRecoveryAndRepairsOnlyTrailingPartialRecord(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := OpenSequenceStore(root, "state/sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	for expected := uint64(1); expected <= 2; expected++ {
		if actual, retry, err := store.Reserve(now.Add(time.Duration(expected) * time.Second)); err != nil || retry != 0 || actual != expected {
			t.Fatalf("reserve = %d, %v; want %d", actual, err, expected)
		}
	}
	if err := store.AdvanceMinimum(10); err != nil {
		t.Fatal(err)
	}
	if actual, retry, err := store.Reserve(now.Add(3 * time.Second)); err != nil || retry != 0 || actual != 10 {
		t.Fatalf("recovered reserve = %d, %v", actual, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile("state/sequence.log", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("partial")); err != nil || file.Close() != nil {
		t.Fatal("append partial record")
	}
	reopened, err := OpenSequenceStore(root, "state/sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.HighWater() != 10 {
		t.Fatalf("high water = %d", reopened.HighWater())
	}
	if information, err := root.Stat("state/sequence.log"); err != nil || information.Size()%sequenceRecordBytes != 0 {
		t.Fatalf("partial record was not repaired: %v, %v", information, err)
	}
}

func TestSequenceStoreConcurrentReservationsAreUniqueAndCorruptionFailsClosed(t *testing.T) {
	t.Parallel()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	const workers = 32
	values := make([]uint64, workers)
	errorsByWorker := make([]error, workers)
	retries := make([]time.Duration, workers)
	var wait sync.WaitGroup
	for index := range values {
		wait.Add(1)
		go func() {
			defer wait.Done()
			values[index], retries[index], errorsByWorker[index] = store.Reserve(time.Unix(1_800_000_000, 0))
		}()
	}
	wait.Wait()
	for _, reservationErr := range errorsByWorker {
		if reservationErr != nil {
			t.Fatal(reservationErr)
		}
	}
	for _, retry := range retries {
		if retry != 0 {
			t.Fatalf("unexpected retry delay %s", retry)
		}
	}
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	for index, value := range values {
		if value != uint64(index+1) {
			t.Fatalf("reservation[%d] = %d", index, value)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := root.ReadFile("sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	contents[sequenceRecordBytes+1] ^= 1
	if err := root.WriteFile("sequence.log", contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSequenceStore(root, "sequence.log"); err == nil {
		t.Fatal("corrupt complete record was accepted")
	}
}

func TestSequenceStorePersistsRollingAttemptBudgetAcrossRestart(t *testing.T) {
	t.Parallel()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	for index := 0; index < maximumAttemptsPerDay; index++ {
		if _, retry, err := store.Reserve(now); err != nil || retry != 0 {
			t.Fatalf("reservation %d = retry %s, error %v", index, retry, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if sequence, retry, err := reopened.Reserve(now); err != nil || sequence != 0 || retry != 24*time.Hour {
		t.Fatalf("budget result = %d, %s, %v", sequence, retry, err)
	}
	if sequence, retry, err := reopened.Reserve(now.Add(24*time.Hour + time.Second)); err != nil || retry != 0 || sequence != maximumAttemptsPerDay+1 {
		t.Fatalf("refilled result = %d, %s, %v", sequence, retry, err)
	}
}

func TestSequenceStoreFailsClosedOnClockRollback(t *testing.T) {
	t.Parallel()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	store, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	if _, retry, err := store.Reserve(now); err != nil || retry != 0 {
		t.Fatalf("initial reservation = %s, %v", retry, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenSequenceStore(root, "sequence.log")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if sequence, retry, err := reopened.Reserve(now.Add(-time.Second)); err == nil || sequence != 0 || retry != 0 {
		t.Fatalf("rollback reservation = %d, %s, %v", sequence, retry, err)
	}
}

func FuzzParseSequenceRecord(f *testing.F) {
	seed := makeSequenceRecord(1, 1)
	f.Add(seed[:])
	f.Add([]byte("not a sequence record"))
	f.Fuzz(func(t *testing.T, input []byte) {
		sequence, attemptedAt, valid := parseSequenceRecord(input)
		if !valid {
			return
		}
		if sequence == 0 || attemptedAt < 0 {
			t.Fatalf("accepted invalid record values: %d, %d", sequence, attemptedAt)
		}
		reencoded := makeSequenceRecord(sequence, attemptedAt)
		if string(input) != string(reencoded[:]) {
			t.Fatal("accepted a non-canonical sequence record")
		}
	})
}
