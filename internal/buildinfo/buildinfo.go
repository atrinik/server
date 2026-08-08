// Package buildinfo exposes immutable build and compatibility coordinates.
package buildinfo

import "runtime"

var (
	version  = "0.1.0-dev"
	revision = "unknown"
)

// Info is the stable machine-readable version response.
type Info struct {
	Version             string `json:"version"`
	Revision            string `json:"revision"`
	GoVersion           string `json:"go_version"`
	Target              string `json:"target"`
	ProtocolContract    string `json:"protocol_contract"`
	ContentFormatRange  string `json:"content_format_range"`
	CleanRoomFoundation bool   `json:"clean_room_foundation"`
}

// Current returns the current build coordinates without initializing runtime services.
func Current() Info {
	return Info{
		Version:             version,
		Revision:            revision,
		GoVersion:           runtime.Version(),
		Target:              runtime.GOOS + "/" + runtime.GOARCH,
		ProtocolContract:    "game-protocol-1",
		ContentFormatRange:  ">=1.0.0 <2.0.0",
		CleanRoomFoundation: true,
	}
}
