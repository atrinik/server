package buildinfo

import (
	"runtime"
	"testing"
)

func TestCurrentReportsAllCompatibilityCoordinates(t *testing.T) {
	t.Parallel()
	current := Current()
	if current.Version != version || current.Revision != revision || current.GoVersion != runtime.Version() ||
		current.ProtocolContract != "game-protocol-1" || current.ContentFormatRange != ">=1.0.0 <2.0.0" || !current.CleanRoomFoundation {
		t.Fatalf("incomplete build coordinates: %+v", current)
	}
}
