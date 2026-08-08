# ADR 0001: deterministic single-owner server architecture

Status: accepted for M1.

One application owner composes transport, simulation, persistence, immutable
compiled content, observability, and operator adapters. Each mutable world
partition has exactly one simulation owner. Other workers exchange bounded
typed commands, immutable snapshots, and committed events; they never retain
or mutate world entities.

The dependency direction is adapters → application contracts → domain/kernel.
Generated protocol messages end at a transport adapter. SQLite rows end at a
storage adapter. OpenTelemetry, slog, and OpenMetrics remain observability
adapters. Core packages contain none of those types, and the architecture test
rejects forbidden imports and any classic/client/editor/renderer/toolkit or
Python dependency.

Stable `{ID, generation}` handles reject reuse. Commands have explicit
monotonic sequence, preflight completely, then commit atomically. Clocks,
deadlines, and seeded PCG streams are injected. State digests sort identities
and encode portable fixed-width values, never Go map order or pointers.

Queues are allocated to a configured maximum, reject overload without
blocking, and stop admission before drain. No goroutine is created per entity,
timer, effect, or gameplay object. Per-tick work is bounded independently from
queue capacity. Later region partitioning must retain these contracts.

Shutdown order is fixed: stop admission, drain connections, quiesce simulation,
checkpoint persistence, then flush telemetry. Each stage owns its failure;
errors are joined after all safe cleanup is attempted.

Failure ownership is explicit:

| Failure | Owner and bounded response |
| --- | --- |
| Malformed or unauthorized client input | Transport validates before enqueue, rejects the request/connection, and records only a safe category. |
| Invalid compiled content | Content adapter rejects the immutable catalog before readiness; active state never observes a partial catalog. |
| Persistence conflict or I/O failure | Storage transaction rolls back; the application applies a bounded retry/service-degradation policy before acknowledging durable work. |
| Internal invariant violation or panic | The owning partition/lifecycle supervisor contains the failure, stops admission, preserves diagnostics, and initiates safe shutdown; it never continues partially mutated work. |
