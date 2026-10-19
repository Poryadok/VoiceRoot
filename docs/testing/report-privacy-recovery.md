# Report privacy recovery

Accepted authority: BE-137 c18/c19 and c23/c24. These refine c11/c12 without
changing public opaque receipt acceptance or adding target lookups to ingress.
Implementation and assembled activation are separate gates.

## Provenance and authority

`VERIFIED_AT_SUBMISSION` records protected Gateway identity and fresh Auth subject
authority in the submission transaction. `OWNER_RECOVERED_LEGACY` records only
privacy ownership recovered later; it cannot create a vote, activate an old
pending report or publish events. Target privacy ownership is likewise distinct
from `VERIFIED_AT_VALIDATION`. Existing historical records and effective sanction
or appeal snapshots are preserved under their owning policies.

The separate Moderation-only User recovery authority returns a retained owner,
positive exact erasure proof, or `INCOMPLETE`. Retained soft-deleted mappings may
support privacy cleanup, never new eligibility. Missing history, account inactivity,
an elapsed inferred deadline and Auth errors are not irreversible proof. No
ordinary raw-header owner resolver, Role selector, key or journal is reused.

Resolve fresh User/Auth authority outside SQL. Under sorted account locks, then
subject and report locks, recheck the exact claim/binding and retained deadline
before commit. Terminal authority dominates late callbacks; stale expiry cannot
override a newer LIVE lifecycle revision. All recovery uses the shared private
repair generation after P2; public effects still require the shared P5 gate.

## Durable opaque journal

Migration11 adds an explicit provenance classification and a journal keyed by an
existing report UUID and `REPORTER` or `USER_TARGET` slot. It has no FK to deletable
raw reports. An entry contains only a binding to opaque Auth subject or terminal
report-redaction disposition and opaque typed evidence reference/version. Raw
profile/account identifiers, their hashes and report contents are forbidden.

Report mutation and journal mutation commit together. Subject disagreement is
divergence. Terminal dispositions never regress, and remain after report payload
deletion for as long as supported backups/replay can resurrect that report.
Capture participant dispositions before raw rows disappear, including the mapped
account callback that runs before the subject-only callback. A positive terminal
receipt can clean an old NULL-provenance backup without inventing a raw owner.
A nonterminal opaque receipt cannot reconstruct an account mapping.

Migration11 may classify preexisting complete migration8 submission subject tuples
as verified because their sole supported writer already verified Gateway and
fresh Auth atomically. Raw account alone is never sufficient. Old writers must
be fenced; unsupported direct writers or unknown provenance remain unverified.

## Finite export and startup modes

`LIVE_CURRENT` trusts the established current local store after migrations and
integrity checks. Its finite local journal census requires neither an external
sink nor a fabricated restore operation or export acknowledgment.

`DECLARED_RESTORE` cannot fall back to live mode when evidence is missing. The
protected offline operation supplies the expected identities, generation and
current final cut. Capture follows writer fencing/drain, or actual independently
retained native backup/WAL history is replayed and verified through that cut.
Periodic dumps, seals, high-water values or mutually consistent old stores alone
cannot establish a lost post-backup tail.

Export uses a finite repeatable-read snapshot, strict opaque records and an exact
artifact inventory/count/hash. Individual record parsing is bounded; unknown or
duplicate fields, invalid UUIDs/enums, duplicate/conflicting slots and mismatched
store/cut identities reject. Publish only complete immutable artifacts and an
atomically published manifest after readback. Diagnostic sequence gaps are not
missing-record evidence; sequence MAX is not a completeness proof.

The shared verifier156 binds a complete required domain cut vector and trusted
operation evidence to this boot/generation. Report artifact validation certifies
only its own bytes and identities. It cannot certify Auth, sanctions, User or
Space history, or open readiness. Import is a monotonic union followed by exact
participant cleanup and the read-only `RequirePrivacyRecovery` checker. Crash
retries are idempotent; unresolved historical ownership remains `INCOMPLETE`.

## Required finite checks

Retain the accepted c19 L1-L13 and c24 F1-F12 matrices. In particular: rollback
atomicity, restart persistence, subject conflict and terminal dominance; private
caller/request binding; retained versus erased versus incomplete User authority;
no eligibility promotion; purge/backfill and generation races; old NULL backup
cleanup; strict corrupt/duplicate/mixed snapshot rejection; current live boot
without a remote endpoint; declared restore missing complete history; and genuine
fenced final capture or verified native replay. Source/unit proof is never a
claim that an operational restore or production deployment occurred.

## Implementation checkpoint

### Bounded journal byte format

`internal/reportrecovery` defines schema 1 canonical UTF-8 JSON with one LF per
line. A canonical JSON manifest names every artifact; chunks contain one header
followed by canonical record lines, ordered by report UUID and slot. Each
`report_journal_chunk_000001` (and subsequent ordinal) is independently named in
the trusted complete inventory. Header, manifest and expected cut must agree on
the operation, store incarnation, journal generation, snapshot and bounded view.
The expected cut includes the manifest SHA-256 and each chunk's exact byte count,
record count and SHA-256. These identities prove bytes, not source currency.

The finite limits are 8,192 records per chunk, 128 chunks, 8 MiB per chunk,
64 MiB for the complete domain snapshot including its manifest, 64 KiB for the
manifest and 1,024 bytes per header/record line. The shared bundle's own total
limit still applies across domains. Capacity exhaustion returns an error before
publication; it never truncates the journal. Empty journals have one header-only
chunk. Diagnostic high-water values may have gaps and do not prove completeness.

Unknown/duplicate JSON fields, alternate encodings, missing/extra/mixed chunks,
duplicate report slots (including conflicting subjects), noncanonical order,
invalid evidence tuples and changed expected identities reject. Parsed accessors
return copies. There is no ready flag, final-source assertion or external sink
in this codec. The actual repeatable-read export, durable file publication,
verified restore import and journal-aware cleanup/checker remain separate work.

### SQL capture and import boundary

`ReportStore.ExportReportRecoverySnapshot` reads journal generation, the hashed
PostgreSQL snapshot view, diagnostic high-water/count and ordered records in one
read-only `REPEATABLE READ` transaction. It returns only complete bounded codec
bytes after commit. Store incarnation and capture operation come from the trusted
offline caller; passing IDs does not prove a final drain or restore continuity.
An uncommitted report mutation cannot appear in this observation.

`ImportReportRecoverySnapshot` requires a parsed immutable snapshot and a nonnil
`ReportRecoveryAuthorize` callback. The shared coordinator must authorize the
exact cut and return a captured private-repair boot/generation check. The check
runs before the transaction and immediately before commit. This is distinct from
effects/P5 authorization. The complete opaque journal union commits atomically,
never regresses a redacted slot and rejects conflicting known subjects. It takes
only canonical journal locks, never report/account locks after them. It neither
restores raw payload/mappings nor fabricates owning Auth terminal fences.
Terminal union import, raw participant cleanup and the final read-only checker
remain mandatory before readiness. Durable offline file publication and an
actual complete restore operation are not supplied by these SQL methods.

### Offline artifact command

Run `cmd/report-journal-export` from the Moderation Go module in a trusted offline
capture operation. Supply `DATABASE_URL` through the environment, and required
`--store-incarnation`, `--operation` and `--output-parent` arguments. The timeout
defaults to five minutes and must be between one second and thirty minutes.
The command has no listener, no DSN command-line flag and no FINAL/ready option.
It emits the final directory and exact `Cut` as JSON; database errors never print
the connection string. Preserve that metadata with the independently selected
whole recovery bundle. Its own output is not proof of current source continuity.

The output parent must already be a private operator-owned directory (0700 on
Linux) with no unrelated concurrent writer. `PublishDirectory` writes 0600 files
in its own 0700 staging directory, synchronizes them, verifies complete readback,
synchronizes the staging directory, atomically renames the complete nonempty
directory and synchronizes the parent. It verifies an existing identical snapshot
instead of replacing it. Changed bytes under the same snapshot UUID reject.
An identical-output retry also synchronizes the parent before reporting success:
the prior attempt may have renamed its directory but failed before that sync.
An interrupted private staging directory is not a published snapshot and is not
accepted by `ReadDirectory`; retries can publish a new complete staging set.
Cleanup affects only the current call's own staging directory. Operational crash
orphan cleanup is a separate fenced operator action, not a readiness shortcut.
Missing, extra, corrupt or symlinked files and open permissions reject.

Source tests exercise Linux filesystem synchronization/readback and an actual
private PostgreSQL capture through file publication. A temporary filesystem
fixture and a successful command do not demonstrate durable off-node backup,
FINAL writer drain, other domain cuts or a real restore operation.

The initial source slice implements explicit provenance, transactional opaque
journal writes at submission/validation and terminal account/subject/late-result
cleanup. It also implements bounded journal byte verification, finite SQL capture
and authorized journal-only import as enabling inputs for the shared coordinator.

It does **not** complete legacy owner recovery, journal-aware boot repair/checking,
the protected User recovery RPC or assembled
P0-P5 activation. Those remain required before complete BE-137 integration and
acceptance. Publication or a scoped source review of this slice grants none of
those runtime claims.
