# ADR 0003: launch with a fresh replacement world

Status: accepted for replacement launch; revisit only through a new importer
issue with independently approved domains.

The replacement starts with new accounts, credentials, characters, inventory,
quests, achievements, property/community state, roles, bans, and mutable world
state. M1 creates no classic-state importer and makes no credential-
compatibility promise. Authored maps/content remain versioned external inputs,
not mutable classic state.

Classic storage is never opened or changed by the replacement. Operators retain
the classic service and its backups under their existing access/retention
policy during the cutover window. Users receive advance notice that replacement
accounts are new and classic credentials must not be reused.

Rollback before replacement writes are accepted simply restores the separately
operated classic endpoint. After replacement users create state, rollback may
restore service availability but never merges that state into classic; the two
worlds remain independent. The production go/no-go requires a rehearsal,
verified backups, a named operator, data-retention approval, and thresholds for
error rate, state durability, and authentication readiness.

No credential, account, network identity, or personal record enters logs,
metrics, traces, fixtures, issue artifacts, or migration reports. This decision
lets the SQLite domain model proceed without inherited identifiers or password
formats and closes importer work for M1/M6 unless product policy explicitly
changes.
