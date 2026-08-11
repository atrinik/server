# Contributing to the replacement server

All implementation in this repository is independently written under MIT. The
classic server and its runtime Python are GPL history, not source material for
this repository.

Direct human-written code contributions are welcome; contributors do not need
to use an agent. “Agentic” describes the project's primary current
software-development workflow. Human- and agent-assisted contributions follow
the same clean-room, review, provenance, testing, and repository-validation
requirements below.

## Permitted design inputs

You may use public issues and specifications, black-box observable behavior,
interoperability facts, independently licensed content/data, synthetic or
clean-room fixtures, and permissively licensed dependencies. Preserve a public
gameplay rule in a new design: for example, a rule saying an exhausted player
cannot attack may become a table-driven Go precondition. Do not preserve the
classic function decomposition, control flow, names, comments, or test cases
that happened to implement that rule.

Do not copy, transliterate, mechanically translate, or structurally rewrite GPL
source. Do not copy GPL tests or fixtures, or add GPL/AGPL dependencies. A
black-box comparison records inputs and externally observable results without
consulting implementation source; translation derives the new code structure
from that source and is prohibited.

## Provenance records

Every new module or material behavior group adds or updates `PROVENANCE.md` with
its source of design, authorship, independently created fixtures, dependencies,
and owning issue. Generated code records its input revision, generator version,
and deterministic drift check. A pull request must identify copied fixtures or
assets explicitly; silence is not a license record.

The organization’s approved provenance-grantor registry is exhaustive. A grant
may be used only after the complete non-shallow history proves sole original
authorship, identities, separability, and absence of embedded third-party work.
Record exact source and destination paths and revisions, the history evidence,
transformation, third-party review, grantor, and exact wrapper-repository
revision containing the registry. Uncertainty fails closed. No grant is used by
the M1 server foundation.

Mixed-license maps, archetypes, audio, graphics, and other content remain
external data under their own notices. Loading them does not make them MIT and
does not change the server’s independent MIT license.

## Pull-request checklist

- [ ] The design inputs and original authorship are recorded in `PROVENANCE.md`.
- [ ] No classic/GPL implementation, test, fixture, or runtime Python was copied
      or used as a structural specification.
- [ ] Generated files record inputs/tools and pass a deterministic drift check,
      or the pull request states that none are present.
- [ ] Every copied fixture, asset, and data input has an explicit license and
      provenance record, or the pull request states that none are present.
- [ ] Dependency policy, notices, SBOM inputs, and tests are updated.
- [ ] `tools/validate.sh` passes and the change has bounded failure behavior.
