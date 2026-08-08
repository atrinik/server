# Server provenance

This is the module-level source-of-design record required by the independent-
implementation policy. All M1 Go code and synthetic tests were authored anew
for the linked public issues. No historical MIT provenance grant is used.

| Destination | Source of design | Original material and fixtures | Owner |
| --- | --- | --- | --- |
| `cmd`, `internal/buildinfo`, `internal/config`, lifecycle shell | `atrinik/server#1` | New Go implementation and synthetic configuration/lifecycle tests | Server maintainers |
| `internal/domain`, `internal/kernel`, architecture test | `atrinik/server#3` | New contracts, queue, deterministic state harness, synthetic commands | Server maintainers |
| `migration/classic-behavior.json` | Canonical independently reviewed inventory at `atrinik/content@60a3a53c4e3e64293cdba21c91caf0c7a618764d` | Aggregate counts and digest only; no runtime script implementation or test content | Server/content maintainers |
| Fresh-state decision | `atrinik/server#5` | Product/operations decision; no classic records or credentials | Server maintainers |
| `internal/observability` | `atrinik/server#21` and Go/OpenTelemetry/OpenMetrics public APIs | New implementation and synthetic bounded-input tests | Server maintainers |

Generated protocol bindings are not present. When introduced, only MIT schemas
from `atrinik/protocol` and its pinned generator/drift contract are permitted.
The external content inventory is evidence for replacement ownership and
burn-down, never an implementation specification. See `CONTRIBUTING.md` for the
required record when a future module or fixture is added.
