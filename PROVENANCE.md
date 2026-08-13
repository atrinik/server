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
| `internal/publisher`, publisher configuration/lifecycle | `atrinik/server#68`, `atrinik/protocol@v1.3.0`, RFC 9421, and RFC 9530 | New Go HTTP/state scheduler implementation; protocol-owned MIT types, validators, and golden values | Server/protocol maintainers |

The metaserver publisher consumes only the released MIT protocol package; it
does not copy or hand-edit generated bindings. Future gameplay bindings remain
subject to the same pinned generator/drift contract.
The external content inventory is evidence for replacement ownership and
burn-down, never an implementation specification. See `CONTRIBUTING.md` for the
required record when a future module or fixture is added.

The independent M1 history above does not impose a categorical ban on later
reuse of eligible historical Classic source. Independent implementation
remains the default when exact historical reuse cannot be proven. Under the
canonical
[`atrinik/atrinik` registry](https://github.com/atrinik/atrinik/blob/main/docs/PROVENANCE.md),
each selected, independently separable contribution must itself fit one
applicable row's “original past Atrinik contributions solely authored” scope.
Historical rows cannot be combined to cover a jointly authored contribution,
agent-generated output, or inseparable mixed work; later or agent-generated
material needs its own contemporaneous compatible rights. Complete,
rename-aware history, identity, embedded-material, separability,
transformation, reviewer, and destination-record evidence is required. Tests,
fixtures, generated output, assets, and dependency code receive no presumption
of coverage, and this source-reuse route does not permit GPL/AGPL dependencies
or bundles. The checked-in Classic distribution remains GPL-2.0-or-later; MIT
permission applies only to the exact selected destination material recorded by
the review.

`provenance/identity-reference.synthetic.json` demonstrates the canonical
privacy-preserving identity reference workflow for issue #84. It is
reviewer-signed synthetic evidence only: it grants no permission for real
material and copies neither the coordinator registry nor identity aliases.
`tools/check-foundations.sh` always validates the local record shape. With an
explicit coordinator checkout it also performs bounded offline verification:

```sh
ATRINIK_COORDINATOR=/path/to/atrinik tools/check-foundations.sh
```

Before coordinator PR #381 merges, audit its pushed branch without treating
the result as approval:

```sh
ATRINIK_COORDINATOR=/path/to/atrinik \
ATRINIK_COORDINATOR_TRUSTED_REF=origin/feat/privacy-preserving-provenance-registry \
tools/check-provenance-identity-reference.sh
```

The record's `evidence_reference.url` is the immutable online permalink.
