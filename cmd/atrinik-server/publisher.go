// Copyright 2026 The Atrinik Project
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/hex"
	"flag"
	"log/slog"
	"os"

	metaserverv1 "github.com/atrinik/protocol/gen/go/atrinik/metaserver/v1"
	"github.com/atrinik/server/internal/config"
	"github.com/atrinik/server/internal/observability"
	"github.com/atrinik/server/internal/publisher"
)

type publisherRuntime struct {
	service  *publisher.Service
	sequence *publisher.SequenceStore
	root     *os.Root
	initial  publisher.Snapshot
}

func configurePublisherFlags(flags *flag.FlagSet, configuration *config.PublisherConfig) {
	flags.StringVar(&configuration.Origin, "publish-origin", configuration.Origin, "HTTPS Game Protocol 1 publisher origin; empty disables publishing")
	flags.StringVar(&configuration.CertificatePath, "identity-certificate", configuration.CertificatePath, "leaf certificate path relative to state")
	flags.StringVar(&configuration.PrivateKeyPath, "identity-private-key", configuration.PrivateKeyPath, "private key path relative to state")
	flags.StringVar(&configuration.Name, "server-name", configuration.Name, "public directory server name")
	flags.StringVar(&configuration.Description, "server-description", configuration.Description, "public directory server description")
	flags.StringVar(&configuration.Region, "server-region", configuration.Region, "optional public deployment region")
	flags.UintVar(&configuration.ProtocolMinor, "protocol-minor", configuration.ProtocolMinor, "Game Protocol 1 compatible minor version")
	flags.StringVar(&configuration.ContentID, "content-id", configuration.ContentID, "versioned compiled-content identity")
	flags.StringVar(&configuration.ContentRevisionSHA256, "content-revision-sha256", configuration.ContentRevisionSHA256, "compiled-content revision digest")
	flags.UintVar(&configuration.PlayersCapacity, "players-capacity", configuration.PlayersCapacity, "public player capacity")
	flags.BoolVar(&configuration.Public, "server-public", configuration.Public, "publish this server in the public directory")
	flags.BoolVar(&configuration.PasswordRequired, "password-required", configuration.PasswordRequired, "require independent game join authentication")
	flags.StringVar(&configuration.DirectHostname, "direct-hostname", configuration.DirectHostname, "optional explicit public DNS fallback")
	flags.UintVar(&configuration.DirectPort, "direct-port", configuration.DirectPort, "explicit public DNS fallback UDP port")
	flags.DurationVar(&configuration.HeartbeatInterval, "publish-heartbeat", configuration.HeartbeatInterval, "slow liveness publication interval")
	flags.DurationVar(&configuration.ChangeDebounce, "publish-debounce", configuration.ChangeDebounce, "visible-change coalescing delay")
}

func newPublisherRuntime(configuration config.Config, logger *observability.Logger) (*publisherRuntime, error) {
	if !configuration.Publisher.Enabled() {
		return nil, nil
	}
	if err := os.MkdirAll(configuration.StateDirectory, 0o700); err != nil {
		return nil, errInvalidPublisherConfiguration
	}
	root, err := os.OpenRoot(configuration.StateDirectory)
	if err != nil {
		return nil, errInvalidPublisherConfiguration
	}
	identity, err := publisher.LoadIdentity(
		root,
		configuration.Publisher.CertificatePath,
		configuration.Publisher.PrivateKeyPath,
	)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	sequence, err := publisher.OpenSequenceStore(root, "metaserver/publish-sequence-v1.log")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	client, err := publisher.NewClient(configuration.Publisher.Origin, identity, sequence, nil)
	if err != nil {
		_ = sequence.Close()
		_ = root.Close()
		return nil, err
	}
	snapshot, err := configuredSnapshot(configuration.Publisher)
	if err != nil || client.ValidateSnapshot(snapshot) != nil {
		_ = sequence.Close()
		_ = root.Close()
		return nil, errInvalidPublisherConfiguration
	}
	serviceConfiguration := publisher.DefaultServiceConfig()
	serviceConfiguration.HeartbeatInterval = configuration.Publisher.HeartbeatInterval
	serviceConfiguration.Debounce = configuration.Publisher.ChangeDebounce
	serviceConfiguration.Observe = func(event publisher.ServiceEvent) {
		logger.Event(context.Background(), slog.LevelInfo, "metaserver", "metaserver.publish", "metaserver publication attempt completed",
			observability.String("kind", event.Trigger), observability.String("result", event.Result.String()))
	}
	service, err := publisher.NewService(client, serviceConfiguration)
	if err != nil {
		_ = sequence.Close()
		_ = root.Close()
		return nil, err
	}
	return &publisherRuntime{service: service, sequence: sequence, root: root, initial: snapshot}, nil
}

func configuredSnapshot(configuration config.PublisherConfig) (publisher.Snapshot, error) {
	revision, err := hex.DecodeString(configuration.ContentRevisionSHA256)
	if err != nil || len(revision) != 32 {
		return publisher.Snapshot{}, errInvalidPublisherConfiguration
	}
	var digest [32]byte
	copy(digest[:], revision)
	snapshot := publisher.Snapshot{
		Name: configuration.Name, Description: configuration.Description,
		ProtocolMinor: uint32(configuration.ProtocolMinor), ContentID: configuration.ContentID,
		ContentRevisionSHA256: digest, PlayersCapacity: uint32(configuration.PlayersCapacity),
		Status: metaserverv1.DirectoryServerStatus_DIRECTORY_SERVER_STATUS_ONLINE,
		Public: configuration.Public, PasswordRequired: configuration.PasswordRequired,
	}
	if configuration.Region != "" {
		snapshot.Region = &configuration.Region
	}
	if configuration.DirectHostname != "" {
		snapshot.Endpoint = &metaserverv1.DirectEndpoint{Hostname: configuration.DirectHostname, Port: uint32(configuration.DirectPort)}
	}
	return snapshot, nil
}

func (runtime *publisherRuntime) start(ctx context.Context) error {
	if runtime == nil {
		return nil
	}
	return runtime.service.Start(ctx, runtime.initial)
}

func (runtime *publisherRuntime) close(ctx context.Context) error {
	if runtime == nil {
		return nil
	}
	serviceErr := runtime.service.Close(ctx)
	sequenceErr := runtime.sequence.Close()
	rootErr := runtime.root.Close()
	if serviceErr != nil {
		return serviceErr
	}
	if sequenceErr != nil {
		return sequenceErr
	}
	return rootErr
}
