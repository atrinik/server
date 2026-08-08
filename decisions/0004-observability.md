# ADR 0004: bounded operational observability

Status: accepted for M1.

Go slog owns JSON Lines and human development logs. Stable subsystem/event
names and typed bounded fields are contracts; prose is not. Passwords, tokens,
salts, private keys, credentials, unrestricted chat/dialog, player names, map
paths, and arbitrary error text are forbidden or redacted. Security/admin
events use the separate `security-audit` classification and restricted
retention.

The central registry admits only declared OpenMetrics names, units, bounded
enum labels, and histogram buckets. Counters saturate, snapshots have byte
budgets, and gameplay never reads operational metrics. Tick, connection,
authentication, packet, queue, map, save, compiled-handler, active-object,
handshake, and operation health have registered coordinates.

OpenTelemetry spans use the process provider and no exporter by default.
Operators may install a bounded exporter outside the simulation owner. Routine
successes may be sampled; errors/slow summaries remain bounded. Optional pprof
is not exposed by this bootstrap and, if enabled later, must be localhost-only
and separately authorized.

Readiness means immutable startup resources and listeners are usable. Liveness
requires a recent simulation heartbeat and does not confuse ordinary game
inactivity with a stall. Remote scraping/TLS/retention belongs to a sidecar;
the simulation loop never serves arbitrary HTTP work.
