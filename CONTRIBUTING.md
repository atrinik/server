# Contributing to the replacement server

All implementation currently in this repository was independently written
under MIT; no historical grant was used for the server foundation. The Classic
server and its runtime Python remain GPL in their source repository. This
historical implementation record is not a categorical ban on later reuse of
eligible historical source under the historical-grant process below.

Direct human-written code contributions are welcome; contributors do not need
to use an agent. “Agentic” describes the project's primary current
software-development workflow. Human- and agent-assisted contributions follow
the same review, provenance, testing, and repository-validation requirements
below.

## Permitted design inputs

You may use public issues and specifications, black-box observable behavior,
interoperability facts, independently licensed content/data, synthetic or
clean-room fixtures, and permissively licensed dependencies. Preserve a public
gameplay rule in a new design: for example, a rule saying an exhausted player
cannot attack may become a table-driven Go precondition. Do not preserve the
classic function decomposition, control flow, names, comments, or test cases
that happened to implement that rule unless the exact material is admitted
through the historical-reuse process below.

Independent implementation remains the default when exact historical reuse
cannot be proven. Eligible, independently separable Classic material may be
inspected as source reference, copied, ported or translated, adapted, or
relicensed for an MIT destination only after the historical-reuse review below.
Unadmitted GPL source, tests, or fixtures must not be copied, transliterated,
mechanically translated, or structurally rewritten. This source-reuse route
does not permit adding GPL/AGPL dependencies, bundled GPL runtime Python, or
other GPL packages. A black-box comparison records inputs and externally
observable results without consulting implementation source; source-informed
work is recorded as historical reuse rather than described as independent
implementation.

## Provenance records

Every new module or material behavior group adds or updates `PROVENANCE.md` with
its source of design, authorship, independently created fixtures, dependencies,
and owning issue. Generated code records its input revision, generator version,
and deterministic drift check. A pull request must identify copied fixtures or
assets explicitly; silence is not a license record.

The organization’s canonical
[`atrinik/atrinik` provenance-grantor registry](https://github.com/atrinik/atrinik/blob/main/docs/PROVENANCE.md)
is exhaustive. A grant may be used only after complete non-shallow,
rename-aware history proves that each selected, independently separable
contribution fits one applicable row's “original past Atrinik contributions
solely authored” scope. Historical rows cannot be combined to cover a jointly
authored contribution, agent-generated output, or inseparable mixed work; later
or agent-generated material needs its own contemporaneous compatible rights.
Record exact source and destination paths and revisions, history and identity
evidence, transformation, embedded-material review, grantor, reviewer,
destination license/notices, and the exact registry revision. Tests, fixtures,
generated output, assets, and dependency code receive no presumption of
coverage. The checked-in Classic distribution remains GPL-2.0-or-later; MIT
permission applies only to the exact selected destination material recorded by
the review. Uncertainty fails closed. No grant is used by the M1 server
foundation.

Mixed-license maps, archetypes, audio, graphics, and other content remain
external data under their own notices. Loading them does not make them MIT and
does not change the server’s independent MIT license.

## Pull-request checklist

- [ ] The design inputs and original authorship are recorded in `PROVENANCE.md`.
- [ ] Classic/GPL implementation, test, fixture, or runtime Python was either
      not used or every selected contribution has the complete historical-grant
      or separate-rights destination record above; no GPL dependency or bundle
      was added.
- [ ] Generated files record inputs/tools and pass a deterministic drift check,
      or the pull request states that none are present.
- [ ] Every copied fixture, asset, and data input has an explicit license and
      provenance record, or the pull request states that none are present.
- [ ] Dependency policy, notices, SBOM inputs, and tests are updated.
- [ ] `tools/validate.sh` passes and the change has bounded failure behavior.
