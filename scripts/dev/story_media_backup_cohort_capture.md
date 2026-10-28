# Guarded selected-cohort dump producer

`Producer(binding, databases, expected, admin_role, guard, drain, journal,
runner=None)` supplies `producer(member, output)` to the cohort exporter. The
selected set and engine observations must first be authenticated and admitted
by the central transaction; this producer does not prove completeness. Its
`expected` has the fixed Kubernetes and PostgreSQL identity fields required
by `story_media_backup_capture.Producer`. It must match the cohort's frozen
operation, snapshot and engine identity. Only the admitted fixed database names
and `roles` are accepted. The connection role is the separately pinned dump
administrator, not a claim about each application's runtime database role.

Before and after each dump the producer checks the existing Kubernetes source
observations, guard and drain, plus the actual session database, database OID,
administrator role and PostgreSQL server-start instant. These are observations
under the trusted Kubernetes boundary, not atomic container or physical storage
attestations. The caller must maintain the real fence and drain throughout the
capture and independently establish application-to-engine mapping.

The mandatory journal implements `begin(member) -> attempt_nonce`,
`complete(member, receipt)` and `uncertain(member)`. Begin must verify
authority, persist the operation/member pending state and fence the attempt
before returning a unique 32-character lowercase hexadecimal nonce. Complete
must verify and durably bind the exact terminal-only receipt to that nonce and
the protected ledger. Any unknown execution or failed postcondition invokes
uncertain, retains the operation lock and requires reconciliation; no retry is
authorized by this producer. Exceptions and database diagnostics are not
returned as receipts. No key, SQL data or plaintext is placed in a receipt.

The local Linux fixture uses a real populated PostgreSQL but synthetic
Kubernetes metadata and authority callbacks. It is not evidence of an actual
DEV capture, a complete cohort, off-node custody or restore admission.
