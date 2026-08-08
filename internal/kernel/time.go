package kernel

import (
	"math/rand/v2"
	"time"
)

// Clock separates wall and monotonic/deadline ownership from gameplay.
type Clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

// Random is an injected deterministic random stream.
type Random interface {
	Uint64N(uint64) uint64
}

// SeededRandom returns an isolated reproducible PCG stream.
func SeededRandom(seedHigh, seedLow uint64) Random {
	return rand.New(rand.NewPCG(seedHigh, seedLow))
}
