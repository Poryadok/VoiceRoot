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

### Exact terminal receipt repair

`RepairReportTerminalReceipts` accepts a separately injected shared private-repair
acquire function and a batch limit from 1 to 1,000. It does not use effects/P5
admission. A candidate is an existing exact report UUID/slot with either a
positive `USER_ERASURE_FACT` redaction or a journal subject present in the actual
local permanent Auth terminal union. Missing owner history, generic errors,
expired/stale DELETED observations and an Auth-terminal receipt without that
owning terminal union cannot authorize deletion.

The batch observes raw tuples and journal receipts, then locks all stored raw
accounts in sorted order, all known subjects, all report rows, and finally their
journal slots. It rechecks the full observed raw tuple and receipt set before
mutation. Changed observations retry with a new complete lock set; known subject
divergence rejects. No newly discovered account is acquired after a subject or
report lock. An old NULL raw binding can be cleaned by its exact current receipt
and positive terminal authority without reconstructing an account mapping.

Auth terminalization is recorded atomically before the existing raw-report purge
participant. Positive User erasure evidence retains its original fact identity
and version. The transaction checks durable journal generation and its captured
private-repair check before commit. Failure rolls back the complete batch. It
does not create Auth terminal fences, guess other reports for the same profile,
revoke sanctions or assert other participants have been scrubbed. Existing
minimal sanction/appeal snapshot handling is preserved.

`Removed` and `More` describe one bounded local observation, never a complete
source-tail or ready certificate. The coordinator must complete actual terminal
imports, all owning participant repairs and the final checker under its supported
writer fence/restore mode. Nonterminal legacy provenance and missing canonical
User facts still need their dedicated authoritative recovery path.

### Bounded journal byte format

`internal/reportrecovery` defines protocol/schema 2 canonical UTF-8 JSON with one LF per
line. A canonical JSON manifest names every artifact; chunks contain one header
followed by canonical tagged record lines, ordered by report UUID, family and slot. Each
`report_journal_chunk_000001` (and subsequent ordinal) is independently named in
the trusted complete inventory. Header, manifest and expected cut must agree on
the operation, store incarnation, journal generation, snapshot and bounded view.
The expected cut includes the manifest SHA-256 and each chunk's exact byte count,
record count and SHA-256. These identities prove bytes, not source currency.

Protocol 2 has separately validated `SUBJECT_PRIVACY` and
`USER_TARGET_REJECTION` families. A rejection encodes only report UUID, typed
decision/provenance and its record schema version; no subject/evidence fields
or zero UUID placeholders appear. Counts and hashes cover both families in
one RR capture. Protocol 1 artifacts are historical evidence and are rejected
as complete current recovery input; missing rejection history cannot be silently
accepted. The importer locks all subject-journal records before any rejection
record, unions both atomically and does not minimize raw reports itself.

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

### Journal-aware final checker

`RequirePrivacyRecovery` remains read-only. It requires durable nonzero journal
generation, valid opaque receipt identities and the local permanent terminal
union for every `AUTH_TERMINAL` receipt. Every retained reporter must have its
exact subject in a `BOUND` receipt and the evidence kind corresponding to its
explicit provenance class.
For `AUTH_SUBJECT`, the evidence UUID is the opaque subject UUID itself: Auth
returns a binding/revision, not a separate binding-fact UUID. The store writer,
snapshot codec and final checker reject a different UUID, including orphan
journal records. `USER_OWNER_FACT` retains its distinct canonical owner-fact UUID.
The same correspondence holds for a VALID User target or an existing
User-target receipt restored alongside an older raw row. Redacted raw residue,
unknown classes, missing/mismatched receipts and unresolved retained bindings
reject. A recovered legacy class requires its User owner fact and confers no
new submission, vote or validation authority.

After migration 9, retained DELETED bindings also require a known nonzero Auth
deletion cycle; LIVE bindings require a NULL cycle. This applies to reporters,
validated targets and retained target-subject history after plain rejection.
A legacy NULL cycle with a future deadline is unresolved, never fabricated or
promoted by this read-only checker. Fresh owning-authority repair must preserve
the exact cycle and its immutable deadline before completion.
The report-created outbox applies the same known-cycle/retained-deadline rule
under its account/subject/report locks before invoking publication. A legacy
NULL cycle produces no send, ACK or invented terminal marker. A fresh retained
cycle can still publish within its exact database deadline; this is not a new
submission or target-admission authority.

A fresh opaque pending target without a target receipt still has no inferred
owner or eligibility; it does not require synchronous target lookup at intake or
this checker. Known current receipts cannot be discarded to make an older row
look like that case. The checker creates no events or ownership, performs no
purge and cannot establish fresh remote owner state or physical deadline
completion. The shared coordinator still needs actual current lifecycle/terminal
authority, participant cleanup and restore continuity before admitting effects.

The initial source slice implements explicit provenance, transactional opaque
journal writes at submission/validation and terminal account/subject/late-result
cleanup. It also implements bounded journal byte verification, finite SQL capture
and authorized journal-only import as enabling inputs for the shared coordinator.

### Verified-pair journal backfill

`BackfillVerifiedReportJournal` handles only missing receipts for stored
`VERIFIED_AT_SUBMISSION` reporters and `VERIFIED_AT_VALIDATION` User targets.
Its bounded limit is 1–100 reports; `Journaled` counts slots, `Purged` counts
reports and `More` describes the observed local batch, never a current source
cut or readiness. Legacy and `OWNER_RECOVERED_LEGACY` rows are not promoted.

The worker closes its repeatable-read candidate transaction before protected
Auth calls. Each distinct stored account/subject pair needs fresh exact Auth
binding with a revision at least as new as the stored pair. A positive local
permanent terminal fence instead selects terminal receipt repair and raw report
purge; transport failures, error text and elapsed deletion deadlines cannot do
that. Retained DELETED bindings must pass the existing typed Auth validator and
database-clock deadline checks.

The write transaction acquires all sorted account locks before subject and
report locks, compares the complete raw/journal observations and checks the
captured private-repair generation before commit. Concurrent canonical owner
facts survive unchanged. A deadline crossing or generation closure rolls back
both the new local binding and journal receipt. Successful backfill leaves
report provenance, validation state, events and threshold hooks unchanged and
does not provision a lifecycle producer epoch or cursor.

Real Linux PostgreSQL race tests cover retained LIVE/DELETED pairs, authority
failures, newer local revisions, both captured-generation boundaries, deadline
crossing behind a held report lock, concurrent User owner facts, bounded
deduplicated calls, legacy exclusion and positive-terminal-only cleanup.
The shared lifecycle cycle projection and coordinator invocation remain owned
by BE-045; this helper does not establish their assembled acceptance.

### User target rejection (BE-137 c51/c52)

MISSING/DELETED is the final decision for one intake, not identity-erasure
authority. A User rejection retains the dismissed report/reporter lifecycle,
sets the internal target UUID and target owner/subject tuple to SQL NULL, clears
body/evidence and pending work, and keeps only its safe resolution code. Public
input still requires a real UUID and synchronous acceptance remains opaque.
Other target types retain their existing representation and cleanup contracts.

The independent immutable `report_target_rejections` receipt contains only the
existing report UUID, MISSING/DELETED decision, OWNER_VALIDATION or
LEGACY_RECORDED_DECISION provenance and schema/diagnostic sequence metadata.
It has no raw target, subject/account, content, hash or report-cascading FK.
Same decision is idempotent; opposite decisions are divergence. A fresh report
UUID may refer to the same now-LIVE target; the old intake never reopens.

`RepairReportTargetRejections` implements finite private legacy/minimized replay
without an owner lookup. It accepts limit 1–1000, returns local Minimized/More
observations and compares complete raw/journal tuples under the shared gate.
An existing receipt can minimize only its restored PENDING or matching rejected
intake; VALID, another target type or an opposite decision is divergence.
Ordinary completion rechecks its actual lease against the database clock after
the report lock and before commit, including lock waits crossing expiry.
Nullable store/DTO readers preserve NULL without a fabricated UUID. Repair uses
the shared private gate and
account→subject→report→subject-journal→rejection lock order. Existing subject
receipts retain their separate lifecycle/terminal obligations. A plain rejection
never becomes AUTH_TERMINAL, USER_ERASURE_FACT, a subject fence or account purge.
The final rejection advisory lock also protects an absent receipt; ordinary
activation checks it only after finishing every subject-journal write and before
threshold hooks/events. A current receipt rolls back attempted late activation.
Import takes no account/report locks after this family, and terminal cleanup
retains the minimal rejection receipt without acquiring it as subject evidence.
Keep receipts while a supported backup/replay can restore their raw intake;
this is purpose-limited pseudonymous metadata, not an anonymity or physical
deletion claim.
The J1–J14 matrix in BE-137 c52 governs the new behavior; earlier slice proofs do
not certify these new nullability or artifact changes.

It does **not** complete legacy owner recovery or authoritative boot repair,
the protected User recovery RPC or assembled
P0-P5 activation. Those remain required before complete BE-137 integration and
acceptance. Publication or a scoped source review of this slice grants none of
those runtime claims.
