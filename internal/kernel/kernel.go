// Package kernel owns deterministic simulation state and mutation contracts.
package kernel

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"sort"
)

// Handle prevents an identifier reused at a later generation from accepting stale work.
type Handle struct {
	ID         uint64
	Generation uint32
}

// CommandKind is the closed M1 mutation vocabulary used by the harness.
type CommandKind uint8

const (
	Spawn CommandKind = iota + 1
	Adjust
	Remove
)

// Command enters simulation in one explicit sequence.
type Command struct {
	Sequence uint64
	Kind     CommandKind
	Entity   Handle
	Delta    int64
}

type entity struct {
	generation uint32
	value      int64
}

// World has exactly one mutable owner. Snapshots are copies.
type World struct {
	revision     uint64
	lastSequence uint64
	entities     map[uint64]entity
	generations  map[uint64]uint32
}

// Snapshot is an immutable deterministic projection.
type Snapshot struct {
	Revision     uint64
	LastSequence uint64
	KnownHandles []Handle
	Entities     []EntitySnapshot
	Digest       [32]byte
}

// EntitySnapshot is ordered by ID.
type EntitySnapshot struct {
	Handle Handle
	Value  int64
}

// NewWorld constructs empty state.
func NewWorld() *World {
	return &World{entities: make(map[uint64]entity), generations: make(map[uint64]uint32)}
}

// Apply preflights a command and commits it atomically or leaves state unchanged.
func (world *World) Apply(command Command) error {
	if world.lastSequence == math.MaxUint64 || world.revision == math.MaxUint64 || command.Sequence != world.lastSequence+1 {
		return errors.New("command sequence is stale, duplicate, or has a gap")
	}
	current, found := world.entities[command.Entity.ID]
	switch command.Kind {
	case Spawn:
		if found || command.Entity.ID == 0 || command.Entity.Generation == 0 || command.Entity.Generation <= world.generations[command.Entity.ID] {
			return errors.New("spawn handle is invalid or already active")
		}
	case Adjust, Remove:
		if !found || current.generation != command.Entity.Generation {
			return errors.New("entity handle is stale")
		}
	default:
		return errors.New("command kind is invalid")
	}
	if command.Kind == Adjust && ((command.Delta > 0 && current.value > math.MaxInt64-command.Delta) ||
		(command.Delta < 0 && current.value < math.MinInt64-command.Delta)) {
		return errors.New("adjustment overflows entity value")
	}

	switch command.Kind {
	case Spawn:
		if world.entities == nil {
			world.entities = make(map[uint64]entity)
		}
		if world.generations == nil {
			world.generations = make(map[uint64]uint32)
		}
		world.entities[command.Entity.ID] = entity{generation: command.Entity.Generation, value: command.Delta}
		world.generations[command.Entity.ID] = command.Entity.Generation
	case Adjust:
		current.value += command.Delta
		world.entities[command.Entity.ID] = current
	case Remove:
		delete(world.entities, command.Entity.ID)
	}
	world.lastSequence = command.Sequence
	world.revision++
	return nil
}

// Snapshot copies state in stable order and hashes only explicit portable fields.
func (world *World) Snapshot() Snapshot {
	knownHandles := make([]Handle, 0, len(world.generations))
	for id, generation := range world.generations {
		knownHandles = append(knownHandles, Handle{ID: id, Generation: generation})
	}
	sort.Slice(knownHandles, func(left, right int) bool { return knownHandles[left].ID < knownHandles[right].ID })
	entities := make([]EntitySnapshot, 0, len(world.entities))
	for id, current := range world.entities {
		entities = append(entities, EntitySnapshot{Handle: Handle{ID: id, Generation: current.generation}, Value: current.value})
	}
	sort.Slice(entities, func(left, right int) bool { return entities[left].Handle.ID < entities[right].Handle.ID })
	hash := sha256.New()
	var encoded [20]byte
	hash.Write([]byte("ATRSNAP1"))
	binary.LittleEndian.PutUint64(encoded[0:8], world.revision)
	binary.LittleEndian.PutUint64(encoded[8:16], world.lastSequence)
	hash.Write(encoded[:16])
	binary.LittleEndian.PutUint64(encoded[0:8], uint64(len(knownHandles)))
	hash.Write(encoded[:8])
	for _, handle := range knownHandles {
		binary.LittleEndian.PutUint64(encoded[0:8], handle.ID)
		binary.LittleEndian.PutUint32(encoded[8:12], handle.Generation)
		hash.Write(encoded[:12])
	}
	binary.LittleEndian.PutUint64(encoded[0:8], uint64(len(entities)))
	hash.Write(encoded[:8])
	for _, current := range entities {
		binary.LittleEndian.PutUint64(encoded[0:8], current.Handle.ID)
		binary.LittleEndian.PutUint32(encoded[8:12], current.Handle.Generation)
		binary.LittleEndian.PutUint64(encoded[12:20], uint64(current.Value))
		hash.Write(encoded[:])
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return Snapshot{Revision: world.revision, LastSequence: world.lastSequence, KnownHandles: knownHandles, Entities: entities, Digest: digest}
}
