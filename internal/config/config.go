// Package config owns typed, bounded startup configuration.
package config

import (
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	metaserverv1 "github.com/atrinik/protocol/gen/go/atrinik/metaserver/v1"
	protocolmeta "github.com/atrinik/protocol/metaserver"
)

const (
	maxQueueCapacity                  = 65_536
	maxCommandsPerTick                = 4_096
	maximumPublisherHeartbeatInterval = 210 * time.Minute
)

// Config is validated in full before any service starts.
type Config struct {
	ListenAddress   string          `json:"listen_address"`
	StateDirectory  string          `json:"state_directory"`
	LogFormat       string          `json:"log_format"`
	QueueCapacity   int             `json:"queue_capacity"`
	CommandsPerTick int             `json:"commands_per_tick"`
	ShutdownTimeout time.Duration   `json:"shutdown_timeout"`
	AdminToken      string          `json:"-"`
	Publisher       PublisherConfig `json:"publisher"`
}

// PublisherConfig is the non-secret Game Protocol 1 publication policy. An
// empty origin disables publication; key material remains in referenced files.
type PublisherConfig struct {
	Origin                string        `json:"origin"`
	CertificatePath       string        `json:"certificate_path"`
	PrivateKeyPath        string        `json:"private_key_path"`
	Name                  string        `json:"name"`
	Description           string        `json:"description"`
	Region                string        `json:"region"`
	ProtocolMinor         uint          `json:"protocol_minor"`
	ContentID             string        `json:"content_id"`
	ContentRevisionSHA256 string        `json:"content_revision_sha256"`
	PlayersCapacity       uint          `json:"players_capacity"`
	Public                bool          `json:"public"`
	PasswordRequired      bool          `json:"password_required"`
	DirectHostname        string        `json:"direct_hostname"`
	DirectPort            uint          `json:"direct_port"`
	HeartbeatInterval     time.Duration `json:"heartbeat_interval"`
	ChangeDebounce        time.Duration `json:"change_debounce"`
}

// RedactedConfig is safe for logs and operator output.
type RedactedConfig struct {
	ListenAddress   string                  `json:"listen_address"`
	StateDirectory  string                  `json:"state_directory"`
	LogFormat       string                  `json:"log_format"`
	QueueCapacity   int                     `json:"queue_capacity"`
	CommandsPerTick int                     `json:"commands_per_tick"`
	ShutdownTimeout string                  `json:"shutdown_timeout"`
	AdminToken      string                  `json:"admin_token"`
	Publisher       RedactedPublisherConfig `json:"publisher"`
}

// RedactedPublisherConfig contains no private key or response credential.
type RedactedPublisherConfig struct {
	Enabled               bool   `json:"enabled"`
	Origin                string `json:"origin"`
	CertificatePath       string `json:"certificate_path"`
	PrivateKeyPath        string `json:"private_key_path"`
	Name                  string `json:"name"`
	Description           string `json:"description"`
	Region                string `json:"region"`
	ProtocolMinor         uint   `json:"protocol_minor"`
	ContentID             string `json:"content_id"`
	ContentRevisionSHA256 string `json:"content_revision_sha256"`
	PlayersCapacity       uint   `json:"players_capacity"`
	Public                bool   `json:"public"`
	PasswordRequired      bool   `json:"password_required"`
	DirectHostname        string `json:"direct_hostname"`
	DirectPort            uint   `json:"direct_port"`
	HeartbeatInterval     string `json:"heartbeat_interval"`
	ChangeDebounce        string `json:"change_debounce"`
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
		Publisher: PublisherConfig{
			CertificatePath:       "identity/certificate.pem",
			PrivateKeyPath:        "identity/private-key.pem",
			Name:                  "Atrinik Server",
			ContentID:             "atrinik-main",
			ContentRevisionSHA256: strings.Repeat("0", 64),
			PlayersCapacity:       100,
			HeartbeatInterval:     150 * time.Minute,
			ChangeDebounce:        5 * time.Second,
		},
	}
}

// Validate checks every field without touching the filesystem or network.
func (configuration Config) Validate() error {
	host, port, err := net.SplitHostPort(configuration.ListenAddress)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || host == "" || strings.TrimSpace(host) != host || portErr != nil || portNumber < 1 || portNumber > 65_535 {
		return errors.New("listen address must contain a host and port")
	}
	if configuration.StateDirectory == "" || len(configuration.StateDirectory) > 4_096 || strings.ContainsRune(configuration.StateDirectory, 0) || isPortableAbsolute(configuration.StateDirectory) || hasParentSegment(configuration.StateDirectory) {
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
	if err := configuration.Publisher.Validate(); err != nil {
		return err
	}
	return nil
}

// Enabled reports whether the operator selected authenticated publication.
func (configuration PublisherConfig) Enabled() bool { return configuration.Origin != "" }

// Validate checks the complete non-secret publisher policy without I/O.
func (configuration PublisherConfig) Validate() error {
	if !configuration.Enabled() {
		return nil
	}
	origin, err := url.Parse(configuration.Origin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.Host != origin.Hostname() ||
		origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" ||
		(origin.EscapedPath() != "" && origin.EscapedPath() != "/") ||
		configuration.Origin != strings.ToLower(configuration.Origin) {
		return errors.New("publisher origin must be one lowercase HTTPS hostname")
	}
	if !safeRelativePath(configuration.CertificatePath) || !safeRelativePath(configuration.PrivateKeyPath) {
		return errors.New("publisher identity paths must be relative without traversal")
	}
	if configuration.ProtocolMinor > 65_535 || configuration.PlayersCapacity < 1 || configuration.PlayersCapacity > protocolmeta.MaximumDirectoryPlayers ||
		configuration.HeartbeatInterval < time.Hour || configuration.HeartbeatInterval > maximumPublisherHeartbeatInterval ||
		configuration.ChangeDebounce <= 0 || configuration.ChangeDebounce > time.Minute ||
		(configuration.DirectHostname == "") != (configuration.DirectPort == 0) || configuration.DirectPort > 65_535 {
		return errors.New("publisher bounds are invalid")
	}
	contentRevision, err := hex.DecodeString(configuration.ContentRevisionSHA256)
	if err != nil || len(contentRevision) != 32 || configuration.ContentRevisionSHA256 != strings.ToLower(configuration.ContentRevisionSHA256) {
		return errors.New("publisher content revision must be lowercase SHA-256")
	}
	serverID := make([]byte, 32)
	server := &metaserverv1.DirectoryServer{
		ServerId: serverID, CertificateSha256: append([]byte(nil), serverID...),
		Name: configuration.Name, Description: configuration.Description,
		ProtocolMajor: 1, ProtocolMinor: uint32(configuration.ProtocolMinor),
		ContentId: configuration.ContentID, ContentRevisionSha256: contentRevision,
		PlayersCapacity:  uint32(configuration.PlayersCapacity),
		Status:           metaserverv1.DirectoryServerStatus_DIRECTORY_SERVER_STATUS_ONLINE,
		PasswordRequired: configuration.PasswordRequired,
	}
	if configuration.Region != "" {
		server.Region = &configuration.Region
	}
	if configuration.DirectHostname != "" {
		server.Endpoint = &metaserverv1.DirectEndpoint{Hostname: configuration.DirectHostname, Port: uint32(configuration.DirectPort)}
	}
	if !protocolmeta.DirectoryServerCompatible(server, 1, uint32(configuration.ProtocolMinor), configuration.ContentID, contentRevision) {
		return errors.New("publisher directory metadata is invalid")
	}
	return nil
}

func isPortableAbsolute(path string) bool {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
		return true
	}
	return len(path) >= 2 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':'
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

func safeRelativePath(path string) bool {
	return path != "" && len(path) <= 4_096 && !strings.ContainsRune(path, 0) &&
		!strings.Contains(path, `\`) && !isPortableAbsolute(path) && !hasParentSegment(path)
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
		AdminToken: redacted, Publisher: configuration.Publisher.Redacted(),
	}
}

// Redacted returns the bounded non-secret operator view.
func (configuration PublisherConfig) Redacted() RedactedPublisherConfig {
	return RedactedPublisherConfig{
		Enabled: configuration.Enabled(), Origin: configuration.Origin,
		CertificatePath: configuration.CertificatePath, PrivateKeyPath: configuration.PrivateKeyPath,
		Name: configuration.Name, Description: configuration.Description, Region: configuration.Region,
		ProtocolMinor: configuration.ProtocolMinor, ContentID: configuration.ContentID,
		ContentRevisionSHA256: configuration.ContentRevisionSHA256,
		PlayersCapacity:       configuration.PlayersCapacity, Public: configuration.Public,
		PasswordRequired: configuration.PasswordRequired, DirectHostname: configuration.DirectHostname,
		DirectPort: configuration.DirectPort, HeartbeatInterval: configuration.HeartbeatInterval.String(),
		ChangeDebounce: configuration.ChangeDebounce.String(),
	}
}
