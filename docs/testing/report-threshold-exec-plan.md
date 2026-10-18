# BE-046: authoritative report threshold

## Purpose and sources

Replace the environment-based automatic report sanction with an authoritative,
privacy-safe account audience and one durable review case. This implements the
accepted autonomous decision BE-046 contract v1 (TT BE-046 c3, attachment
`606443f7c49928644c46eccc35d0e169`) under the user's delegated routine decisions.
The explicit human ID-only, non-disclosing report intake choice is unchanged.

Sources: [reports](../features/reports.md), [User](../microservices/user-service.md),
[Auth](../microservices/auth-service.md), [Moderation](../microservices/moderation-service.md),
[data model](../DATA_MODEL.md), [stores](../DATA_STORES.md),
[architecture requirements](../ARCHITECTURE_REQUIREMENTS.md), and
[testing](../TESTING.md). Consume BE-137 opaque intake and BE-045 subject/lock/
erasure contracts; do not create parallel intake, subject or restriction writers.

Starting immutable source is `c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c`.
The old implementation uses an environment denominator, duplicate report rows,
a profile-as-account fallback and separately committed log/sanction/event.

## Accepted behavior

Only `target_type=user` can create an automatic account shadow ban. Message,
Space and Story reports remain manual; shadow ban never suspends Auth status.
For `N > 0`, the threshold is `max(10, ceil(N/100))`, calculated with safe
integer division/remainder. Authoritative `N=0` is manual-only; unknown,
partial, invalid or stale authority never becomes zero or a static fallback.

`POTENTIAL_PUBLIC_PROFILE_ACCOUNTS_V1` counts distinct other nondeleted Auth
accounts having at least one nondeleted User profile. Include guests and frozen
profiles once. Exclude the target's entire account and accounts with no live
User profile. Blocks, presence, suspensions and field settings do not shrink
this potential audience; this metric does not change profile read privacy.
This conservative autonomous definition avoids threshold manipulation through
personal restrictions. It makes no Sybil-resistance claim.

User owns the consistent candidate/binding snapshot. Auth positively classifies
every requested candidate as LIVE/DELETED/MISSING in bounded batches of at most
1000, without returning subjects/content or omitting unknown IDs as live.
User rechecks target binding/revision after classification. The complete
observation vector and commit must fit 30 seconds from its first observation;
work exhaustion, partial results and expired contexts cause persisted retry.
This is bounded observation across owners, not an atomic cross-database snapshot.

The numerator counts distinct verified submission accounts on the exact target
profile in `(t-24h,t]` at Moderation database time. Count only VALID unresolved
reports bound to the current target account/lifetime and newer than its closed
case cutoff. Duplicate receipts/categories/profiles count once; target-owner
accounts and unverified legacy rows count zero. Intake receipts do not change.
Fresh rich authority can establish compatible binding for modern verified
reports; unknown or changed historical bindings remain manual-only.

Fetch external authority outside SQL transactions, then use BE-045's shared
account lock/order to recheck lifecycle, validation, freshness and cutoff.
Commit a unique active account case, machine-issued shadow sanction, log,
qualifying report associations, evaluation result and immutable event outbox
atomically. Existing shadow sanctions suppress duplicates without changing
issuer/reason. Stable outbox IDs give at-least-once delivery, not network
exactly-once. Failed/ambiguous commits never cause speculative effects.

Staff revoke/appeal/expiry closure records a database-time account cutoff;
complaints received before or during the episode cannot immediately reban it.
Fresh later complaints may qualify. Report resolution alone never revokes a
sanction. Existing bans and restrictions remain effective. Erasure and permanent
terminal fences follow BE-045/deletion canon; no raw identity resurrection or
new retention exception is permitted.

## Ownership and integration seams

- Author158 owns this vertical and its delivery until real integrator121 accepts.
- BE-137 author144 owns `reports.go`, migration `000005_report_intake`, intake,
  validation and its worker. Its agreed `ReportValidated(ctx, pgx.Tx,
  ValidationFact) error` hook runs inside LIVE activation and enqueues user-only
  threshold work transactionally. The fact carries report/profile IDs, nullable
  verified reporter account, nullable validated target account, target type/ID
  and original creation time. No revision/subject is guessed.
- BE-045 author156 owns subject/lifecycle, restriction projection, shared account
  lock and existing sanction/expiry writers. Consume its dedicated
  `voice.automod.v1.AuthModerationAuthority`, with no new subject namespace or
  duplicate Auth account-status writer.
- Use separate BE-046 store/worker files and reserved Moderation migration
  `000010_report_threshold`. No edits to authors144/156's mutable shared files
  before agreeing exact hunk ownership and immutable integration order.

## Producer contracts and files

Add protected `UserService.ResolveReportThresholdContext` for exact
`service:moderation`, and `AuthService.ClassifyReportAudienceAccounts` for exact
`service:user`. Bind the entire deterministic protobuf, RPC, audience and unique
request ID using Phase-0 RS256 credentials, TTL at most 30 seconds, HTTPS JWKS,
mTLS and cross-instance replay guards. No ordinary listener/Gateway exposure,
raw metadata identity, delegated user, or generic content grant is accepted.

User request: evaluation UUID, target profile, optional expected account and
positive profile revision. Response: LIVE/MISSING/DELETED/EMPTY_AUDIENCE,
exact profile/account/revision, metric/version/count, start/completion/expiry
and opaque snapshot ID. Moderation combines this with its own fresh BE-045
subject lookup; this avoids adding a new User caller to subject authority.

Auth request: evaluation UUID and a unique canonical nonnil UUID batch of at
most 1000 accounts. Response: matching evaluation, exactly one disposition per
ID, bounded observation metadata and opaque snapshot ID. Suspended but existing
nondeleted accounts are LIVE for this metric. Inconsistent or incomplete owner
facts are unavailable, never a smaller audience.

Expected changes: owning docs; additive Auth/User proto and generated Go/Dart/
Java sync; User `internal/reportaudience` snapshot/classifier client and private
context listener; Auth `reportaudience` classifier/private listener; dedicated
Moderation threshold policy/store/worker/outbox and migration; final agreed
activation hooks/config/render checks. Prefer independent packages over changes
to live Auth repositories/servers owned by156.

## Milestones and TDD sequence

- [x] Read accepted decision and establish concrete source ownership with144/156.
- [ ] User snapshot RED/GREEN: distinct account candidates, live/deleted/frozen/
  guest and exact target revision; real PostgreSQL, no private data exposure.
- [ ] Auth classifier RED/GREEN: complete positive dispositions, missing/deleted/
  suspended accounts, batch/input validation and consistent read metadata.
- [ ] Protected producer transport: exact caller/RPC/body, no legacy exposure,
  stale/replayed/wrong metadata/JWKS/TLS negatives before store access.
- [ ] User complete classification/recheck/freshness, including multi-batch and
  unknown/partial/timeout tests; no partial denominator.
- [ ] Moderation integer/window/dedup/cutoff/case transaction and concurrency
  oracles; consume agreed immutable prerequisite heads.
- [ ] Activate the single VALID hook and threshold worker, remove the legacy
  synchronous environment writer in the agreed integration. A disabled-only
  producer foundation is not completion of BE-046.
- [ ] Independent review, affected Go/Java/proto checks and portable proof.
- [ ] Real accepted integrator121 handoff, assembled exact-version CI/delivery.

Tests and implementation stay with one author under the ordinary scoped TDD
workflow. Start new APIs with documented compile-red where necessary; distinguish
that from a reproduced runtime regression. Do not invent a RED result.

## Verification matrix

T1: N 0/1/999/1000/1001/2000/int64 max, invalid/partial/stale authority.
T2: duplicate reporter/categories/profiles, same owner, separate target profiles.
T3: distinct User candidates and positive Auth classifications; no audience list
leaves User or is exposed publicly.
T4: exact rolling-window edges, server creation time, delayed validation/erasure.
T5: missing/mismatched binding/subject/revision, target deletion and stale context.
T6: exact principal/caller/request binding and ordinary-listener/TLS/JWKS/replay.
T7: concurrent validators/profiles, rollback at every write, commit/outbox crash.
T8: revoke race/cutoff, unrelated restrictions, no shadow-to-Auth suspension.
T9: authority outage leaves receipt/manual queue intact; restart/backoff/age-out.
T10: complete fresh denominator drift, immutable committed evidence.
T11: erasure, restore, stale backups and delayed effects respect terminal fences.
T12: non-user reports remain manual; profile/account IDs are never interchangeable.

Run focused Go tests, PostgreSQL/transport/concurrency checks in owned Linux
Docker runners, affected Go short/vet/lint, Auth Maven focused/full checks under
the repository's risk rules, buf lint/format/breaking/generated parity and
applicable configuration render checks. Never run Windows test executables or
start new host test listeners. Shared DEV/STAGING requires its separate current
TT resource window; this plan does not authorize a rollout or acceptance.

## Progress, decisions and risks

2026-10-18 implementation checkpoint: User complete-audience assembler,
repeatable-read candidate snapshot, post-classification binding recheck,
private RPC and configured mTLS runtime are authored. Auth's dedicated exact
User-only classifier supplies an explicit LIVE/DELETED/MISSING record for every
requested account. Canonical proto, Maven copy and Go/Dart stubs are generated.
These are enabling sources; the complete BE-046 runtime is not finished.

Owned Linux Go 1.26 checks passed: audience unit negatives (omission, extra,
duplicate, unknown, expired/future/wrong evaluation/outage/changed binding),
1001-candidate batching, empty/terminal outcomes, real PostgreSQL canonical User
migrations and binding revision, plus private RPC principal/ordinary-method
negatives. A filtered User module run compiled all packages and passed the
selected tests; it is not the full User test suite. No runtime RED was claimed.

Owned Linux Maven 3.9.11/Java 25 offline compile passed. Surefire records show
two cryptographic exact-principal/replay tests, one real PostgreSQL/Flyway 26
classifier test, and one actual mTLS/HTTPS-JWKS/Redis-replay factory test passing
with zero skips. The factory checks successful classification, request tamper,
replay, missing certificate, plaintext, wrong host/trust and replay outage.
Logs/Surefire copies are retained under external `tmp/swarm/rudeus158-be046-*`;
TT checkpoints e4/e5 record actual execution. Native Maven cache was copied
from a read-only seed into an owned volume, never shared-writable.

BE-137 immutable input `aca25d5ffe9cef3d0ac3edcf5ddb3f535eaccdf9` is ready.
BE-045 contract `31d8d2358` pins the shared machine issuer, subject binding,
account lock and closure callback. Atomic count/case/sanction/outbox, subject
purge, worker activation, full risk checks and independent review remain open.
Moderation migration versions are coordinated: intake 000005, automod 000006,
threshold 000010. Do not deliver the enabling producer alone as task completion.

The candidate scan must be complete within the deadline; large populations may
need a proven owner projection later, not a silently capped count. Dedicated
producer configuration fails closed. Stale/unknown authority preserves manual
processing. Activation and source integration must reconcile all three authors'
new contracts; elapsed time is not authority to overwrite another writer.
