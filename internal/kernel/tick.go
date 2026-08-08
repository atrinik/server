package kernel

import (
	"context"
	"errors"
)

const maxCommandsPerTick = 4_096

// RunTick claims and applies at most limit commands on the calling simulation owner.
// Cancellation stops before claiming the next command; a claimed command is always
// applied or returned as an explicit error and is never silently requeued.
func RunTick(ctx context.Context, world *World, commands *Queue[Command], limit int) (int, error) {
	if world == nil || commands == nil || limit < 1 || limit > maxCommandsPerTick {
		return 0, errors.New("tick inputs are invalid or outside supported bounds")
	}
	processed := 0
	for processed < limit {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		command, found := commands.Pop()
		if !found {
			return processed, nil
		}
		if err := world.Apply(command); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}
