# Atrinik Go server repository guide

## Ownership and boundaries

- This repository owns the fresh MIT authoritative Go server: deterministic
  simulation, authorization, sessions, persistence, native gameplay services,
  compiled-content consumption, bounded expression evaluation, and production
  operation. It is an independent implementation, not a classic port.
- Preserve product behavior owned by issues; a technical replacement does not
  authorize redesigning progression, combat, quests, maps, dialogue, economy,
  or presentation.
- `atrinik/protocol` owns Game Protocol 1 schemas, framing specifications,
  generated Go/Rust contracts, and conformance fixtures. Consume released
  generated packages; never duplicate schemas or hand-edit generated code.
- `atrinik/content-toolkit` owns authored parsing/catalogs/compilation. Load
  only bounded versioned artifacts; do not parse classic source formats or
  execute source scripts here. Content/media keep their individual licenses.
- Client/editor/renderer own presentation and authoring. The wrapper owns
  checkout composition, runtime state, locks, supervision, and classic/
  replacement coexistence. Do not recreate those concerns or add Go-to-Rust
  FFI, submodules, or a second contract owner.

## Deterministic simulation and state

- Keep mutable world state under one simulation owner. Workers exchange bounded
  typed commands/results and never retain mutable world objects. Do not spawn a
  goroutine per entity, object, timer, effect, or gameplay action.
- Inject clocks and seeded randomness. Define stable ordering for commands,
  events, timers, iteration, and persistence; never depend on Go map order,
  scheduler timing, pointers, or process-local enum positions.
- Use stable generational identities. Model effects as validate/preflight,
  commit, and cancel/rollback so invalid, stale, duplicate, cancelled, or
  persistence-failed work cannot expose partial state.
- Keep domain rules independent of QUIC, Protobuf, SQLite rows, expression
  engines, logging, and filesystem paths. Bound queues, payloads, collections,
  recursion/fan-out, timer horizons, and per-tick work with tested overload and
  shutdown behavior.
- SQLite WAL is the initial persistence owner: one coordinated writer,
  transactional migrations, typed repositories, checkpoints/backups, restore
  validation, content-version association, and failure injection. Never persist
  wire messages or implementation pointers as the domain model.
- Keep mutable state outside source and under wrapper isolation. A content
  transition is explicit and transactional; never mutate authored catalogs or
  reinterpret saved state silently.

## Protocol, authorization, and security

- Decode into temporary bounded inputs, validate protocol/domain rules and
  current authorization, then submit an internal command. Produce only
  viewer-authorized projections; never send hidden facts and ask a client to
  hide them.
- Treat frames, compiled content, expressions, configuration, state import, and
  operator commands as untrusted. Reject malformed, oversized, deeply nested,
  out-of-order, or impossible data before effects. Bound observability
  cardinality and keep credentials, account/private state, tokens, and network
  identities out of logs, traces, fixtures, panics, and snapshots.
- There is no Python runtime or CPython plugin ABI. CEL may run only in a
  versioned, typed, pure, deterministic, resource-bounded environment over
  immutable inputs. Starlark is not a baseline dependency and requires the
  explicit decision in server issue #60.

## Licensing, roadmap, and validation

- New server work is MIT. Do not copy/adapt GPL implementation or tests.
  Historical reuse follows local `PROVENANCE.md` and the canonical
  `atrinik/atrinik/docs/PROVENANCE.md` registry; incomplete, mixed, or uncertain
  evidence fails closed.
- `atrinik/atrinik#168` is the cross-repository roadmap; local issue acceptance
  criteria and milestones own delivery. Do not duplicate the M1-M6 plan here.
- Run the real aggregate contract now present in this repository:

  ```sh
  tools/validate.sh
  git diff --check
  ```

  `Server validation` owns formatting/static analysis, tests/race coverage,
  selected fuzz smoke tests, dependency/license checks, and other documented
  gates. Add deterministic, property, replay, boundary, failure-injection, and
  fuzz coverage appropriate to the changed trust boundary.
- Wrapper replacement build/runtime adapters are not available yet. Use
  repository validation for this Go server and do not route it through classic
  C code. Commits and pull-request titles use Conventional Commits;
  semantic-release owns releases and tags.
