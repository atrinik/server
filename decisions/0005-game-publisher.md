# ADR 0005: certificate-bound Game Protocol 1 publication

Status: accepted for the metaserver publisher foundation.

The replacement server publishes presence through the released Game Protocol 1
contract. The publisher is an outbound adapter: generated protocol types stop
there and do not enter the domain or kernel. Its server ID is the SHA-256
fingerprint of the exact P-256 leaf certificate used by the future QUIC
listener. The matching private key remains in wrapper-owned state and is never
copied into configuration, logs, metrics, or directory output.

Each request reserves a monotonically increasing sequence in an append-only,
checksummed, owner-only ledger before nonce generation or network I/O. Complete
record corruption, sequence exhaustion, unsafe file state, and wall-clock
rollback fail closed. A truncated final record may be removed during startup.
An authenticated replay response can only raise the durable high-water mark;
it never lowers or reuses a sequence. The same ledger retains the rolling local
attempt ceiling across process restart and belongs in the identity's backup and
restore set.

The scheduler has one goroutine and one in-flight request. Startup, immutable
snapshot changes, explicit resume, transient retry, and liveness heartbeat are
serialized. Equal updates are no-ops, changes coalesce behind a bounded
debounce, and heartbeat jitter stays strictly within the production listing
lifetime. Permanent contract or authentication failures suspend until a new
snapshot or explicit resume. Transient failures use bounded jittered backoff;
rate responses use their validated bounded `Retry-After`.

Only a canonical operator-configured DNS hostname and port can enter the
published model. No source, listener, mapped, discovered, or candidate address
is inferred. HTTP redirects are forbidden; request duration, response headers,
and response bodies are bounded. The success credential is parsed to validate
the exact response contract, then cleared and discarded because Game Protocol
1 rendezvous has no reviewed consumer yet.

Publication remains disabled by default. Code validation proves request bytes,
signature binding, replay recovery, persistence, cadence, redaction, and
failure classification. Production enablement and live canary/rollback remain
deployment gates owned with the broader server operations and metaserver
cutover.
