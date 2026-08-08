# Atrinik server

This repository is the fresh, clean-room implementation of the authoritative
Atrinik game server in Go.

New source code in this repository is licensed under the MIT License. The
classic GPL server is maintained in the
[`server/` module of `atrinik/classic`](https://github.com/atrinik/classic/tree/main/server).
Do not copy, translate, or adapt its source code.

The implementation roadmap is tracked by
[`atrinik/atrinik#168`](https://github.com/atrinik/atrinik/issues/168).

## Foundation status

M1 supplies the independent build, lifecycle, architecture, provenance, state,
behavior-inventory, and observability contracts. It intentionally does not yet
listen for gameplay traffic. A running M1 shell therefore reports that gameplay
readiness and simulation liveness are false.

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

Run the complete repository contract with `tools/validate.sh`. Create local
release artifacts without overwriting an existing directory with
`tools/package-release.sh build/release 0.1.0`.

## Design records

- [Architecture and concurrency](decisions/0001-architecture.md)
- [Provenance and expression hosts](decisions/0002-provenance-and-expressions.md)
- [Fresh replacement world](decisions/0003-fresh-world.md)
- [Operational observability](decisions/0004-observability.md)
- [Independent implementation policy](CONTRIBUTING.md)
- [Module provenance](PROVENANCE.md)
- [Security and privacy](SECURITY.md)
