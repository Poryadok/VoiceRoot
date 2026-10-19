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

The initial source slice implements explicit provenance, transactional opaque
journal writes at submission/validation and terminal account/subject/late-result
cleanup. It is an enabling input for BE046 and the shared coordinator.

It does **not** complete legacy owner recovery, finite snapshot export/import,
journal-aware boot repair/checking, the protected User recovery RPC or assembled
P0-P5 activation. Those remain required before complete BE-137 integration and
acceptance. Publication or a scoped source review of this slice grants none of
those runtime claims.
