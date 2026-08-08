# Atrinik Go server repository guide

These instructions apply to the entire `atrinik/server` repository.

## Purpose and ownership

- This repository is the fresh MIT-licensed authoritative Atrinik game server
  implemented in Go. It is an independent implementation, not a port of
  the [`server/` module of `atrinik/classic`](https://github.com/atrinik/classic/tree/main/server).
- Own simulation, authorization, sessions, persistence, native gameplay
  services, compiled-content consumption, bounded expression evaluation, and
  production server operation here.
- Preserve the gameplay and content decisions in their owning issues. A
  technical replacement does not authorize redesigning progression, combat,
  quests, maps, dialogue, economy, or presentation behavior.
- Use [atrinik/atrinik#168](https://github.com/atrinik/atrinik/issues/168) as
  the cross-repository roadmap and the issue's owning milestone as the phase
  gate. Keep cross-repository dependencies explicit in issues and pull
  requests.

## Repository boundaries

- `atrinik/protocol` owns Game Protocol 1 Protobuf schemas, Buf policy,
  transport/framing specifications, conformance fixtures, and generated Go and
  Rust contracts. Consume a released or pinned generated Go package; do not
  duplicate schemas, hand-edit generated code, or make internal Go structs the
  wire contract.
- `atrinik/content-toolkit` owns authored-content parsing, catalogs, validation,
  transactions, and compilation. Load only versioned, bounded compiled content
  artifacts. Do not add legacy source-format parsers or execute source scripts
  in the server.
- Content, resources, and sound keep their exact per-file licenses. Compiled
  inputs are data, not MIT server source, and their required notices must
  survive packaging.
- `atrinik/client`, `atrinik/editor`, and `atrinik/renderer` own presentation
  and authoring. Do not add UI, rendering, editor, or client prediction logic
  here.
- `atrinik/atrinik` owns checkout composition, profiles, isolated runtime
  state, process supervision, and canonical/classic coexistence. Do not
  recreate wrapper paths, locks, topology state, or launch orchestration.
- `atrinik/metaserver-worker` owns registration and discovery service
  operation; it never receives gameplay authority.
- Do not introduce Go-to-Rust FFI, Git submodules, or a second owner for a
  contract. Add a new parser, scheduler, persistence model, expression host,
  or schema owner only through an explicit architecture issue.

## Go architecture and deterministic simulation

- Follow the architecture and package-dependency decision in
  [server#3](https://github.com/atrinik/server/issues/3). Until it is approved,
  keep bootstrap choices minimal and reversible rather than establishing a
  competing layout.
- Use a deterministic single-owner simulation. Mutable world state belongs to
  the simulation owner; transport, storage, and observability workers exchange
  bounded typed commands/results and never retain mutable world objects.
- Use bounded region or service queues with defined overload, cancellation,
  shutdown, and backpressure behavior. Do not start a goroutine per entity,
  map object, timer, effect, quest, or connection-owned gameplay object.
- Inject clocks and seeded random sources. Define stable ordering for commands,
  events, timers, iteration, and persistence. Never depend on Go map iteration,
  wall-clock timing, scheduler order, pointer identity, or process-local enum
  order for gameplay results.
- Use stable generational identities and explicit ownership. A stale identity
  must fail without targeting a replacement object.
- Model effects as validate/preflight, commit, and cancel or rollback. Invalid,
  cancelled, duplicate, stale, timed-out, or persistence-failed work must not
  expose partial gameplay state.
- Keep domain rules independent of QUIC, generated Protobuf types, SQLite row
  layouts, CEL implementation details, logging, and filesystem paths. Translate
  at narrow adapters and keep package dependencies acyclic.
- Keep every queue, collection, payload, recursion/fan-out path, timer horizon,
  and per-tick workload bounded. Limits and overload behavior are part of the
  design and tests, not deployment folklore.

## Protocol, authorization, and content

- Decode Game Protocol 1 into temporary validated inputs, enforce protocol and
  domain bounds plus current authorization/state, and only then submit an
  internal command. A malformed or incomplete message cannot mutate live
  state.
- Produce viewer-authorized projections from committed state. Never disclose a
  hidden field and ask the client to hide it, accept client-supplied authority,
  or infer identity from display text or filesystem paths.
- Obey the protocol repository's QUIC framing, stream roles, rate, queue,
  deadline, close, and compatibility rules. Slow or hostile peers must consume
  bounded resources and receive structured failures without secrets.
- Load a compiled content package into an immutable catalog identified by its
  schema/version/digest. Persist mutable world state separately, retain the
  associated content digest, and reject unsupported or corrupt combinations
  before simulation starts.
- Treat content reload/version transitions as explicit transactions. Runtime
  code cannot mutate authored catalogs or silently reinterpret saved state.

## Persistence and runtime behavior

- SQLite WAL is the initial persistence owner. Use one coordinated writer,
  explicit transactional schema migrations, typed repositories, checkpoints,
  backups, restore validation, content-version association, and failure
  injection. Do not persist generated Protobuf messages or implementation
  pointers as the domain model.
- Keep mutable state outside the source checkout and let the wrapper own its
  isolation and locking. Never share one state directory between concurrent
  servers or canonical/classic topologies.
- Implement quests, dialogue/interfaces, achievements, commands, commerce,
  property/community systems, triggers, metrics, and gameplay behavior as
  native Go services plus compiled typed data.
- There is no runtime Python compatibility layer or CPython plugin ABI. Do not
  add Python execution, `.py` dispatch, embedded interpreters, or Python as a
  production runtime dependency.
- CEL is allowed only through versioned, typed, deterministic, pure, and
  resource-bounded environments over immutable inputs. CEL may compute an
  approved condition or value; it cannot perform effects, access mutable Go
  objects, filesystem/network/environment/process state, ambient time, or
  ambient randomness.
- Starlark is not a baseline dependency. Do not add it before the evidence-based
  go/no-go in [server#60](https://github.com/atrinik/server/issues/60). If that
  issue approves a host, expose only immutable snapshots and typed command
  results with explicit execution, memory, recursion, output, time, and
  capability limits.

## Testing, security, and validation

- Make deterministic unit and replay tests the normal path. Add property tests
  for invariants and transactional boundaries, race tests for concurrency,
  fuzz tests for every untrusted decoder/evaluator/importer, and failure
  injection for storage, cancellation, queue pressure, and shutdown.
- Cross-language behavior uses protocol-owned golden and negative fixtures.
  Independently authored gameplay scenarios must cover success, rejection,
  retry, duplicate, stale generation/revision, reconnect, save/reload, and
  rollback without deriving GPL tests.
- Treat protocol frames, compiled content, CEL/Starlark input, configuration,
  state import, and operator commands as untrusted boundaries. Reject malformed,
  oversized, deeply nested, out-of-order, or impossible data before effects.
- Keep credentials, tokens, account data, network identities, and private
  player state out of logs, metrics labels, traces, fixtures, panic output, and
  snapshots. Bound observability cardinality.
- Once [server#1](https://github.com/atrinik/server/issues/1) bootstraps the Go
  module and component scripts, the documented aggregate server validation must
  include formatting, static analysis/vetting, all Go tests, race tests,
  selected fuzz smoke tests, generated-contract drift where applicable,
  dependency/license review, and `git diff --check`. CI's eventual required
  aggregate check is `Server validation`.
- Today this seed repository has no Go module, build script, or server binary.
  For guidance-only changes, inspect the changed Markdown and run
  `git diff --check`; do not claim `go test`, wrapper builds, or runtime
  topologies succeeded. Add exact component commands to the README/CI in the
  bootstrap issue before requiring them here.
- When wrapper integration exists, handoffs use the thin wrapper's exact
  profile/build/scenario/topology commands and a distinct named state. Until
  then, report that wrapper/runtime validation is unavailable rather than
  reconstructing component internals.

## Milestone priorities

- **M1 — Clean-room foundations:** complete repository/provenance policy,
  architecture and behavior/state decisions, the Go bootstrap, and structured
  observability. Keep later gameplay work behind stable foundations.
- **M2 — Contracts, content, and headless world:** build the deterministic
  kernel, QUIC/session adapter, compiled catalog, entities/maps, SQLite,
  accounts, actions/events/metrics, native quest/dialogue, bounded CEL,
  commands, and deterministic test harness against protocol/toolkit contracts.
- **M3 — First playable replacement:** implement the smallest integrated
  account-to-world slice with movement, inventory, combat/AI, social behavior,
  native services, lighting, one compiled content chain, save/reconnect, and
  zero Python or Starlark.
- **M4 — Shared editor and scalable presentation:** this repository has no
  independent rendering/editor implementation lane. Supply only explicit
  authoritative contracts or performance fixes required by M4 owners.
- **M5 — Gameplay and world migration:** migrate preserved feature designs and
  the complete behavior manifest in parallel domain batches, decide Starlark
  only at its formal gate, and prove zero runtime Python.
- **M6 — Production hardening and cutover:** implement only the approved state
  policy/importer, reproducible operations, security/load/soak/fault gates, and
  rehearsed cutover/rollback. Do not archive or bypass the classic fallback
  before the program exit gate.

Milestones are dependency gates, not permission to ignore an issue's explicit
prerequisites. Prototypes may begin early, but a phase closes only when its
cross-repository exit criteria pass.

## Licensing, provenance, and delivery

- New server source, tests, generators, and repository infrastructure are MIT.
  Do not copy, adapt, or mechanically translate GPL legacy source or tests.
- Historical reuse is allowed only under the exhaustive approved-grantor
  registry and proof rules in the current `atrinik/atrinik` root `AGENTS.md`.
  A complete, non-shallow history audit must follow renames and moves, prove
  sole original authorship by an approved grantor, resolve historical
  identities, and exclude embedded third-party or conflicting material.
  Mixed, incomplete, or uncertain evidence fails closed.
- Record eligible reuse in the destination pull request or a committed
  provenance manifest: exact source repository/path/revision and full history,
  identity evidence, destination, transformation, third-party review,
  applicable grantor, and the exact wrapper commit containing the registry
  entry. A grant never blanket-relicenses a repository, file, content pack, or
  generated output.
- Keep one owning issue and milestone for material work. Use Conventional
  Commits for commits and pull-request titles, update affected specifications
  and fixtures with contract changes, and name cross-repository producer and
  consumer dependencies.
- Update the wrapper supply-chain inventory whenever dependencies, toolchains,
  package sources, Actions, images, licenses, or validation paths change. Pin
  reproducible inputs and do not commit secrets, local state, generated runtime
  data, or confidential/unreleased project information.
