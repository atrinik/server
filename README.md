# Atrinik server

This repository contains the authoritative Go server for Atrinik's agentic
next-generation reimplementation.

## Development model

The current replacement server foundation is fresh, independently authored
MIT-licensed Go code developed
primarily through Codex workflows under maintainer direction, review,
provenance controls, tests, and repository validation. It reimplements and
improves Classic behavior and project direction; it is not a mechanical C or
Python translation or a source port.

Direct human-written code contributions are welcome. “Agentic” describes the
project's primary current software-development workflow, not a requirement that
every line, commit, or contributor use an agent. See the
[replacement roadmap](https://github.com/atrinik/atrinik/issues/168) and the
[canonical project authorship statement](https://github.com/atrinik/.github/blob/main/profile/README.md#the-project).

Atrinik's art, maps, quests, lore, audio, and other game content remain
external, human-created work. They retain their exact authors, upstream credit,
licenses, and notices in their source repositories and are not covered by this
server's MIT license.

New source code in this repository is licensed under the MIT License. The
classic GPL server is maintained in the
[`server/` module of `atrinik/classic`](https://github.com/atrinik/classic/tree/main/server).
Its checked-in distribution remains GPL-2.0-or-later. Exact historical source
reuse is possible only under the temporal, sole-authorship, separability, and
destination-record requirements in the [provenance policy](PROVENANCE.md).
That source-reuse route does not permit GPL dependencies or bundles; MIT
permission applies only to the exact selected destination material recorded by
the review.

## Foundation status

M1 supplies the independent build, lifecycle, architecture, provenance, state,
behavior-inventory, and observability contracts. It intentionally does not yet
listen for gameplay traffic. A running M1 shell therefore reports that gameplay
readiness and simulation liveness are false.

The optional Game Protocol 1 publisher is implemented as a separate adapter. It
does not make the gameplay foundation production-ready: it publishes only when
an operator explicitly configures the canonical HTTPS origin and the same
P-256 certificate/private-key identity that the future QUIC listener will use.

```text
transport/storage/content/telemetry adapters
                    |
                    v
        application/domain contracts
                    |
                    v
     deterministic single-owner kernel
```

Dependencies point inward. Network, database, generated protocol, and telemetry
types cannot enter `internal/domain` or `internal/kernel`; an architecture test
enforces that rule. Mutable world state has one owner. Bounded queues carry
commands inward and immutable snapshots and committed events travel outward.

## Build and inspect

Go 1.26.5 is required.

```sh
go build ./cmd/atrinik-server
go test ./...
go run ./cmd/atrinik-server version
go run ./cmd/atrinik-server config
```

`config` prints only redacted settings. The privileged diagnostics token is
accepted exclusively through `ATRINIK_ADMIN_TOKEN`, never a command-line flag.
M1 exposes an authenticated, bounded immutable diagnostics snapshot API for the
future local console/sidecar. It does not expose a remote HTTP or profiling
endpoint.

## Game Protocol 1 publication

Publication is disabled when `-publish-origin` is empty, which is the default.
The production origin is `https://publish.meta.atrinik.org`. When enabled, the
server reads `-identity-certificate` and `-identity-private-key` relative to
the wrapper-owned state directory, derives the server ID from the exact leaf
DER certificate, and sends the protocol-owned one-request signed body. HTTP
redirects are forbidden and ordinary Go HTTPS certificate verification remains
mandatory.

The non-secret publish sequence is durably reserved before every request in
`state/metaserver/publish-sequence-v1.log`. Ambiguous attempts consume their
sequence. A valid replay response raises the local high-water mark before one
bounded retry. The append-only ledger is owner-only, checksummed, bounded to
64 MiB, and belongs in state backup/restore with the identity. Restoring an old
copy is safe only through the authenticated `minimumNextSequence` recovery
contract; sequence exhaustion requires operator recovery and never wraps.
On POSIX systems owner-only means no group or other mode bits. On Windows the
ledger is created with, and both the ledger and private key are validated
against, a protected DACL containing exactly one full-control entry for the
current user; inherited or additional grants fail closed.

Startup publishes once. Visible updates supplied to the adapter coalesce behind
the configured debounce, and unchanged liveness uses a jittered multi-hour
heartbeat. The configured interval is at most three and a half hours; its
15-minute jitter therefore stays strictly inside the four-hour production
listing lifetime. Transient failures use bounded exponential backoff,
`Retry-After` is honored, and the process attempts at most 47 publications in a
rolling 24-hour window. Permanent errors suspend publication until explicit
resume or a new complete snapshot. The response rendezvous token is discarded
and cleared until Game Protocol 1 rendezvous has a separately reviewed consumer.

Only an explicit canonical DNS hostname and port are serialized. Request-source
and discovered numeric addresses are never publisher input. Logs contain only
the closed trigger/result classes; certificates, keys, signatures, nonces,
sequences, tokens, response bodies, addresses, and server metadata are absent.
Use `go run ./cmd/atrinik-server config -publish-origin=https://publish.meta.atrinik.org`
to inspect the complete non-secret configuration without contacting the
service.

Run the complete repository contract with `tools/validate.sh`. Create local
release artifacts without overwriting an existing directory with
`tools/package-release.sh build/release 0.1.0`.

## Design records

- [Architecture and concurrency](decisions/0001-architecture.md)
- [Provenance and expression hosts](decisions/0002-provenance-and-expressions.md)
- [Fresh replacement world](decisions/0003-fresh-world.md)
- [Operational observability](decisions/0004-observability.md)
- [Certificate-bound Game Protocol 1 publication](decisions/0005-game-publisher.md)
- [Independent implementation policy](CONTRIBUTING.md)
- [Module provenance](PROVENANCE.md)
- [Security and privacy](SECURITY.md)
