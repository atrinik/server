// Package config owns typed, bounded startup configuration.
package config

import (
	"errors"
	"net"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxQueueCapacity   = 65_536
	maxCommandsPerTick = 4_096
)

// Config is validated in full before any service starts.
type Config struct {
	ListenAddress   string        `json:"listen_address"`
	StateDirectory  string        `json:"state_directory"`
	LogFormat       string        `json:"log_format"`
	QueueCapacity   int           `json:"queue_capacity"`
	CommandsPerTick int           `json:"commands_per_tick"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	AdminToken      string        `json:"-"`
}

// RedactedConfig is safe for logs and operator output.
type RedactedConfig struct {
	ListenAddress   string `json:"listen_address"`
	StateDirectory  string `json:"state_directory"`
	LogFormat       string `json:"log_format"`
	QueueCapacity   int    `json:"queue_capacity"`
	CommandsPerTick int    `json:"commands_per_tick"`
	ShutdownTimeout string `json:"shutdown_timeout"`
	AdminToken      string `json:"admin_token"`
}

// Default returns conservative development defaults. The wrapper overrides state.
func Default() Config {
	return Config{
		ListenAddress:   "127.0.0.1:13327",
		StateDirectory:  "state",
		LogFormat:       "json",
		QueueCapacity:   1_024,
		CommandsPerTick: 256,
		ShutdownTimeout: 10 * time.Second,
	}
}

// Validate checks every field without touching the filesystem or network.
func (configuration Config) Validate() error {
	host, port, err := net.SplitHostPort(configuration.ListenAddress)
	if err != nil || host == "" || port == "" {
		return errors.New("listen address must contain a host and port")
	}
	if configuration.StateDirectory == "" || len(configuration.StateDirectory) > 4_096 || strings.ContainsRune(configuration.StateDirectory, 0) || filepath.IsAbs(configuration.StateDirectory) || hasParentSegment(configuration.StateDirectory) {
		return errors.New("state directory must be a relative wrapper-owned path without traversal")
	}
	if configuration.LogFormat != "json" && configuration.LogFormat != "human" {
		return errors.New("log format must be json or human")
	}
	if configuration.QueueCapacity < 1 || configuration.QueueCapacity > maxQueueCapacity {
		return errors.New("queue capacity is outside supported bounds")
	}
	if configuration.CommandsPerTick < 1 || configuration.CommandsPerTick > maxCommandsPerTick || configuration.CommandsPerTick > configuration.QueueCapacity {
		return errors.New("commands per tick is outside supported bounds")
	}
	if configuration.ShutdownTimeout <= 0 || configuration.ShutdownTimeout > time.Minute {
		return errors.New("shutdown timeout is outside supported bounds")
	}
	if configuration.AdminToken != "" && (len(configuration.AdminToken) < 32 || len(configuration.AdminToken) > 4_096) {
		return errors.New("admin token is outside supported bounds")
	}
	return nil
}

func hasParentSegment(path string) bool {
	for _, segment := range strings.FieldsFunc(path, func(character rune) bool {
		return character == '/' || character == '\\'
	}) {
		if segment == ".." {
			return true
		}
	}
	return false
}

// Redacted returns a representation which never exposes credential material.
func (configuration Config) Redacted() RedactedConfig {
	redacted := "unset"
	if configuration.AdminToken != "" {
		redacted = "[redacted]"
	}
	return RedactedConfig{
		ListenAddress: configuration.ListenAddress, StateDirectory: configuration.StateDirectory,
		LogFormat: configuration.LogFormat, QueueCapacity: configuration.QueueCapacity,
		CommandsPerTick: configuration.CommandsPerTick, ShutdownTimeout: configuration.ShutdownTimeout.String(),
		AdminToken: redacted,
	}
}
