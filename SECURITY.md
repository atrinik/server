# Security policy

Report vulnerabilities privately through GitHub Security Advisories for
`atrinik/server`. Do not include credentials, private player data, packet
payloads, or production state in a public issue.

The M1 shell binds no gameplay, metrics, diagnostics, or profiling endpoint.
Future adapters must retain the architecture, input bounds, and fail-closed
configuration contracts. Diagnostics require a configured token, constant-time
comparison, and a local console or localhost-only authenticated adapter.

Logs and traces exclude passwords, tokens, salts, private keys, unrestricted
chat/dialog, account/player names, raw network addresses, arbitrary errors, and
unclassified string fields. Metrics admit only registered enum labels. Security
audit events have restricted operator access and retention and are never
gameplay state. Mutable state directories are wrapper-owned; adapters must use
secure rooted file operations and must not follow untrusted links outside that
root.

The replacement launches as a fresh world. It does not read classic databases,
import classic credentials, or mutate classic state. Operational rollback keeps
the two worlds separate.
