# ADR 0002: provenance, generated code, and expression hosts

Status: accepted for M1.

Replacement implementation uses public specifications/issues, observable
behavior, interoperability facts, independently licensed data, synthetic
fixtures, and permissive dependencies. It does not copy or structurally
translate classic GPL source, runtime Python, tests, or fixtures.

Protocol-generated Go code is permitted only from versioned MIT protocol
schemas through the protocol repository's pinned generator and drift check; it
does not make a server domain type into a wire contract. No generated protocol
code is present in this bootstrap.

CEL may be added only as a separately versioned typed pure environment over
immutable inputs, with explicit cost, recursion, output, and capability bounds.
It cannot access effects, filesystem, network, environment, time, randomness,
or mutable objects. Starlark is not approved by M1. The 32 residual behavior
rows stay blocked behind the dedicated go/no-go issue and are not evidence for
adding an interpreter.

Mixed-license content is loaded as external compiled data and keeps its exact
notices. It is not MIT by association with this server.
