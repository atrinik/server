package kernel

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrQueueFull reports explicit overload without blocking the submitter.
	ErrQueueFull = errors.New("command queue is full")
	// ErrQueueClosed reports lifecycle rejection.
	ErrQueueClosed = errors.New("command queue is closed")
)

// Queue is a bounded FIFO. It owns no worker or mutable world state.
type Queue[T any] struct {
	mu     sync.Mutex
	items  []T
	head   int
	size   int
	closed bool
}

// NewQueue allocates exactly capacity slots.
func NewQueue[T any](capacity int) (*Queue[T], error) {
	if capacity < 1 || capacity > 65_536 {
		return nil, errors.New("queue capacity is outside supported bounds")
	}
	return &Queue[T]{items: make([]T, capacity)}, nil
}

// Push submits without waiting.
func (queue *Queue[T]) Push(item T) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.closed {
		return ErrQueueClosed
	}
	if queue.size == len(queue.items) {
		return ErrQueueFull
	}
	index := (queue.head + queue.size) % len(queue.items)
	queue.items[index] = item
	queue.size++
	return nil
}

// PushContext rejects work whose caller has already canceled. It never waits.
func (queue *Queue[T]) PushContext(ctx context.Context, item T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return queue.Push(item)
}

// Pop removes the oldest item without waiting.
func (queue *Queue[T]) Pop() (T, bool) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	var zero T
	if queue.size == 0 {
		return zero, false
	}
	item := queue.items[queue.head]
	queue.items[queue.head] = zero
	queue.head = (queue.head + 1) % len(queue.items)
	queue.size--
	return item, true
}

// Close stops admission. Already accepted items remain drainable.
func (queue *Queue[T]) Close() {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.closed = true
}

// Len returns a synchronized depth snapshot.
func (queue *Queue[T]) Len() int {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.size
}
