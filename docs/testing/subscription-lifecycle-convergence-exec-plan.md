# ExecPlan: Subscription lifecycle convergence

## Purpose

Make the provider-independent Premium and Space Pro lifecycle implementable and
testable without live merchant credentials. A committed billing transition must
survive process crashes, publish one stable state snapshot, and converge every
consumer after duplicate or out-of-order delivery. Personal downgrade must
preserve data and support an explicit two-profile selection; renewal must undo
only subscription-owned freezes. File retention and Space Pro grace must follow
the same authoritative entitlement revision.

This document is the RED contract for the accepted target. It does not claim
that the runtime below is implemented.

## Context

- Product behavior: [subscription.md](../features/subscription.md),
  [file-storage.md](../features/file-storage.md),
  [multi-profile.md](../features/multi-profile.md), and
  [spaces.md](../features/spaces.md).
- Service boundaries: [subscription-service.md](../microservices/subscription-service.md),
  [file-service.md](../microservices/file-service.md),
  [user-service.md](../microservices/user-service.md),
  [space-service.md](../microservices/space-service.md),
  [voice-service.md](../microservices/voice-service.md), and
  [notification-service.md](../microservices/notification-service.md).
- Event ownership: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md),
  [DATA_MODEL.md](../DATA_MODEL.md), [DATA_STORES.md](../DATA_STORES.md), and
  [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md).
- Current producer: `src/backend/subscription/internal/subscriptionevents/`,
  `internal/grpcsvc/subscription.go`, `internal/store/`, and
  `internal/sweeper/`.
- Current consumers: Auth `NatsSubscriptionTierStore`, Space
  `internal/subscriptionconsume`, Notification
  `subscription_events_consumer.go`, and Analytics `internal/consumer`. File
  currently ACKs legacy lifecycle into an unwired in-memory cache with no
  retention effect; User has no personal lifecycle consumer.
- Current event envelope already has `event_id` and `occurred_at`, but the
  publisher creates those after the billing transaction. The personal sweeper
  publishes before marking reminders sent; publisher and consumer retries are
  therefore not crash-safe.

## Scope

In scope:

- Premium and Space Pro `ACTIVE`, `GRACE_PERIOD`, and `INACTIVE` entitlement
  state, including manual cancel-at-period-end, resume, renewal, failed payment,
  recovery, grace expiry, account delete/restore, and irreversible purge.
- A stable `subscription.entitlement_changed` snapshot event, transactional
  producer outbox, leased dispatch, and consumer inbox/revision rules.
- D1/D3/D7 reminder scheduling and delivery deduplication.
- Auth tier projection, User profile selection/freeze/unfreeze, File retention
  recalculation, Space Pro and Voice enforcement projections, Notification
  delivery, and Analytics consumption.
- Deterministic fake-provider, database integration, NATS redelivery, and
  Compose/Flutter acceptance evidence.

Out of scope:

- Live Paddle or CloudPayments credentials, merchant/legal activation, refunds,
  pricing changes, and provider-specific checkout UI.
- Benefits whose own feature implementations remain open. This contract moves
  their entitlement state; it does not silently declare every benefit shipped.
- Federation.
- R23 Space deletion/provider-fence semantics, which remain governed by the
  already accepted P3 contract.

## Accepted decisions

### One authoritative state snapshot

Add an additive `EntitlementChanged` arm to `SubscriptionStreamEvent` and publish
it on `subscription.entitlement_changed`. The envelope carries
`protocol_version=1`, `aggregate_kind=PERSONAL|SPACE`, canonical `aggregate_id`,
and positive `aggregate_revision`. The aggregate key is `(PERSONAL, account_id)`
or `(SPACE, space_id)` and survives cancellation followed by a new purchase
until irreversible account purge replaces raw identifiers with the deletion HMAC
tombstone described below.

The payload is a complete snapshot: Subscription-owned entitlement ID, plan,
personal `account_id` or Space `space_id` plus optional `purchaser_account_id`
and `purchaser_deleted`, opaque `deletion_fence_id`, optional
`deletion_cycle_id`/`purge_at`, state,
`cancel_at_period_end`, optional `downgrade_cycle_id`, `current_period_end`,
optional `grace_period_end`, and a closed reason enum plus `effective_at` and
`entitled_until` (`STARTED`, `RENEWED`, `PAYMENT_FAILED`,
`PAYMENT_RECOVERED`, `CANCEL_SCHEDULED`, `CANCEL_RESUMED`, `PERIOD_ENDED`,
`GRACE_EXPIRED`, `ACCOUNT_DELETE_SCHEDULED`, `ACCOUNT_RESTORED`,
`PURCHASER_PURGED` (Space metadata only)). `ACCOUNT_PURGED` is the terminal
deletion-fence state, not a raw entitlement snapshot.
`ACTIVE` and `GRACE_PERIOD` both grant benefits;
`INACTIVE` does not. Even an active/grace snapshot grants only while consumer
database time is strictly before `entitled_until`; equality is inactive and
triggers reconciliation, so a lost renewal/expiry event cannot grant forever.
Consumers derive entitlement only from this snapshot, not from arrival order or
legacy event names.

`ACTIVE.entitled_until=current_period_end`,
`GRACE_PERIOD.entitled_until=grace_period_end`, and
`INACTIVE.entitled_until=effective_at`. Provider `paused` is not a fourth state:
a verified snapshot that preserves service to the paid boundary maps to
`ACTIVE(cancel_at_period_end=true)` followed by `INACTIVE`; ambiguous pause time
is reconciled/quarantined rather than guessed.

Every committed state transition locks the aggregate revision, stores the new
state, and inserts immutable outbox bytes in one PostgreSQL transaction. The
same logical transition never allocates another event ID. Legacy
`plan_started`, `plan_cancelled`, `plan_expired`, `downgrade`, payment, and
Space-Pro subjects may be dual-published during migration, but are not a second
entitlement authority. Each compatibility child has its own stable deterministic
ID derived from the canonical event ID plus payload kind and carries that parent
as correlation; one `Nats-Msg-Id` is never reused for different bytes.

### Provider replay and ordering

Each adapter normalizes a verified webhook into provider, exact provider event
ID, provider subscription ID, product/target, provider event time, billing
period, authoritative provider version or snapshot, and semantic event type.
Arrival order is never treated as provider order. When the provider supplies no
monotonic version, the adapter must reconcile the provider subscription before
committing a state regression; an ambiguous fact is retryable/quarantined, not
guessed. The deterministic fake provider supplies this version and exercises the
same state machine.

Exact replay of `(provider, provider_event_id)` returns the saved outcome and
creates no transition or outbox row. Reuse with different retained request bytes
is a contract mismatch while billing detail exists. After R23 purge, the
permanent purpose-scoped HMAC fence remains the only permitted evidence and
returns its stored terminal outcome without resurrecting billing data.

### Producer outbox and reminders

`subscription_event_outbox` stores `event_id`, aggregate key/revision, event
kind, subject, deterministic protobuf bytes plus SHA-256, `available_at`,
attempt/error state, fenced `lease_token`/`lease_until`, and `delivered_at`.
PubAck stream/sequence is stored for exact account-purge deletion evidence.
Workers claim bounded ready/expired leases with `FOR UPDATE SKIP LOCKED`, publish
the stored bytes with `Nats-Msg-Id=event_id`, require JetStream PubAck, and
complete only their own lease. A crash after PubAck may resend the same bytes and
ID; it may not recreate the event. Known and ambiguous failures retry with
bounded exponential backoff and lag alerts. Undelivered rows are never expired.

Entering grace atomically creates three scheduled reminder outbox rows with a
unique logical key `(aggregate_kind, aggregate_id, grace_revision, day)` for
days 1, 3, and 7. D1 is due at grace start, D3 at `+P2D`, and D7 at `+P6D`;
each expires at the next reminder boundary or grace end. Claim rechecks the
current aggregate under lock. Recovery, expiry, or a newer revision cancels
pending reminders; a missed old window is not delivered as a burst. Reminder
events reference the grace revision and include personal `account_id`, or Space
`space_id` plus the purchaser account.

### Consumer inbox, replay, and reconciliation

Auth, User, File, Space, Voice, and Notification each own a PostgreSQL inbox and
projection in their own database. Analytics uses service-native durable
ClickHouse ingest/dedupe keyed by canonical event ID; it is not an enforcement
projection and does not acquire a new PostgreSQL database. Enforcement consumer
processing is one local transaction:

1. Validate the complete envelope and hash exact bytes.
2. Insert `(consumer_name, event_id, payload_hash)` and lock the aggregate
   projection.
3. Exact event replay is a no-op. Same event ID with different bytes, or the
   same aggregate/revision with a different snapshot, is `CONTRACT_MISMATCH`, is
   not applied, and pages/quarantines rather than ACKing as success.
4. A lower revision is a recorded stale no-op. The same revision and snapshot is
   an exact no-op. A higher revision atomically replaces the projection and
   applies the domain effect.
5. ACK only after commit. Parse, database, dependency, dispatch, unknown payload
   arm, and unsupported protocol failures are contract mismatches and are not
   ACKed as success; they enter bounded retry/quarantine/DLQ with an alert and no
   state mutation.

Because the event is a full state snapshot, revision `N+2` may safely apply
before `N+1`; the later `N+1` is stale. Every enforcement consumer first creates
its durable NATS consumer, then opens a protected S2S snapshot session. A session
uses one repeatable-read owner watermark, signed opaque page cursors, a bounded
expiry, and returns every non-purged permanent aggregate—including `INACTIVE`
tombstones—exactly once plus `complete=true` on the final page. It never treats absence or a
failed page as deletion. Pages apply the same revision rule, after which the
consumer drains messages queued since durable creation. Periodic reconciliation
repeats this sequence and repairs missing/outdated projections. Only allowlisted
workload identities may read target-scoped pages; cursors cannot change scope.
A seven-day NATS history or an in-memory cache is not bootstrap authority.

Analytics does not reconstruct lifecycle reason history from current state. It
replays Subscription's immutable provider-neutral lifecycle journal through a
separate protected cursor using the same canonical event IDs, then drains NATS.
At the recorded canonical cutover watermark Analytics stops mapping legacy
plan/Space/downgrade subjects into lifecycle metrics; payment subjects remain
billing-attempt telemetry only. Every enforcement projection persists
`entitled_until` and fails closed/reconciles at that boundary instead of waiting
indefinitely for another event.

Auth access claims add `subscription_revision` and
`subscription_entitled_until` beside tier. A consumer treats paid benefits as
expired at equality even if the access token itself remains valid. When the
claim boundary is reached, Voice must call Subscription-owned protected
`ResolveEntitlementAtBoundary` with aggregate key, observed deadline and
`minimum_revision=claim.subscription_revision`. The response is one complete
current snapshot at revision `>= minimum_revision`; a lower/missing ambiguous
result is unavailable, never free by inference. The call budget is
`min(500ms, remaining request deadline)` with no retry past that budget. Timeout,
auth failure or unavailable response denies the paid quality/cap for this
decision and triggers reconciliation; it does not invalidate the base/free path.
This keeps grace discoverable while preventing an already issued Premium JWT
from granting beyond its entitlement.

### Account deletion and restore

The account-deletion coordinator sends a durable idempotent command to
Subscription. At delete scheduling, personal entitlement becomes immediate
`INACTIVE(reason=ACCOUNT_DELETE_SCHEDULED)`, pending reminders are suppressed,
and the same transaction creates a leased provider-operation outbox to cancel
auto-renewal. Provider failure retries/alerts but never restores local access.
During the canonical `P30D` restore window, explicit account restore makes
Subscription reconcile the provider. If paid time still remains it emits
`ACTIVE(reason=ACCOUNT_RESTORED, cancel_at_period_end=true)` to that verified
boundary; recurring billing is not resumed without a separate explicit user
action. Otherwise it stays `INACTIVE`. This `ACTIVE` clears the old downgrade
cycle and subscription-freeze overlay like a new purchase.

Space Pro paid by the deleting account is not transferred. Its auto-renewal is
cancelled through the same outbox, reminders to the deleted purchaser are
suppressed, and the Space keeps already-paid Pro entitlement only through its
verified `current_period_end`, then becomes `INACTIVE`. Ownership-transfer
billing remains the separate H1 decision.

At delete scheduling, the final raw personal `INACTIVE` snapshot carries an
opaque permanent `deletion_fence_id`, one-use `deletion_cycle_id`, and
`purge_at=scheduled_at+P30D`. Every consumer persists a local DB-time purge job;
restore cancels only that exact cycle. At `purge_at`, Subscription and every
consumer erase raw state independently—neither another participant's receipt nor
a provider receipt is a gate. Receipts are asynchronous evidence/SLO. An offline
consumer must apply the protected permanent purge-fence snapshot by fence ID
before it may serve any restored projection, then emit its late receipt.

A consumer that missed delete scheduling and restored only an old raw aggregate
key uses Subscription-owned protected batch
`ResolvePurgeFenceByLegacyAggregate` before serving. Subscription computes the
purpose HMAC against retained key versions inside its trust boundary and returns
only `PURGED`, fence ID/cycle and `purged_at`; a non-match is explicit
`NOT_FOUND`, not entitlement. Raw request IDs are never logged/traced/retained,
only allowlisted participant workloads may call it, and audit records use the
resulting fence ID. The caller purges a match locally before serving.

The external provider receipt also never
blocks the `P30D` privacy deadline: if still pending, Subscription atomically
detaches only provider + opaque subscription/cancel handle into a random-ID,
cancellation-only escrow with no raw account/purchaser link or payment detail.
Only the cancellation worker identity may read it; it retries until terminal
provider receipt, then crypto-shreds the opaque handle while the HMAC fence keeps
the outcome. Missing receipt pages from `P25D`; merchant activation is forbidden
unless provider API and manual runbook can service this escrow. Subscription then erases/crypto-shreds raw
`account_id`/`purchaser_account_id` from current rows, journal/outbox payloads,
quarantine, Analytics and provider detail, including rows otherwise inside
`P400D`, and deletes every recorded retained JetStream sequence carrying those
raw IDs. The purge receipt records those stream deletions. Restore tooling
reapplies permanent purge tombstones before any restored backup may serve.
Subscription retains only a permanent purpose-scoped HMAC tombstone containing
deletion fence ID/cycle, aggregate/provider replay fences, last revision, event
IDs/hashes and terminal `ACCOUNT_PURGED` outcome. Purged tombstones are excluded
from raw entitlement snapshots and included in the separate protected permanent
purge-fence snapshot. Provider replay returns the fenced outcome and can never
recreate billing detail or entitlement.

A Space aggregate is not removed with its purchaser. Purge clears its raw
`purchaser_account_id`, sets `purchaser_deleted=true`, stores the payer/provider
link only in the non-reversible HMAC fence (the unlinked cancellation escrow may
still hold its opaque provider handle), increments the Space revision and
emits `PURCHASER_PURGED` without changing already-paid state/deadline. Subsequent
snapshots and the period-end `INACTIVE` event remain complete with `space_id` and
the deleted marker; purchaser reminders/routing stay suppressed.

### Personal User and File effects

While manual cancellation is scheduled or the account is in grace, Subscription
assigns a stable `downgrade_cycle_id`; it survives the direct successor
`INACTIVE` transition and any intervening failed-payment grace, but is cleared by
resume, renewal, payment recovery, or a new `STARTED` activation that prevents or
reverses downgrade. A later schedule
creates a new cycle.
User records a pending selection against that cycle when more than two
non-deleted profiles exist but does not freeze them yet. The primary ID always
reserves one slot; when an eligible secondary exists the picker requires the
exact pair of primary plus one distinct owned, non-deleted profile not disabled
or frozen for another reason. On the first `INACTIVE` snapshot for the same
cycle User applies that stored pair atomically; if it is missing or invalid, the
deterministic fallback is primary plus the earliest-created eligible secondary.
If no secondary is eligible, only the primary slot is selected; User never
revives unrelated disabled state merely to reach two. Every other non-deleted
profile receives the independent subscription-freeze overlay, even if another
disable reason already exists, so removing that other reason later cannot exceed
the free limit. This closes the maximum even when no client is open. A selection
from a cleared or different cycle is rejected without changes.
Retaining the primary in the subscription selection never clears a moderation,
deletion, or other independent disabled reason on that profile.

Subscription freezes are separately attributable (reason/revision), so every
`ACTIVE` result—new `STARTED`, renewal, resume, or payment recovery—unfreezes
every and only profile frozen by subscription and clears the pending cycle and
selection. Deleted, moderation-frozen, or otherwise disabled profiles are never
revived. Grace does not freeze profiles. The client uses the
authoritative pending-selection state, not `tier=free && profiles.length>2`, and
refreshes its token/profile context if its current profile is not one of the
selected two.

Each exact durable File reference stores immutable `retention_account_id`,
verified from the authenticated account that acquires that reference, alongside
the actor/uploader profile where relevant. Forward/reuse acquires a new exact
reference for the forwarding account; binary dedup never transfers retention
ownership between references. Derived subresources inherit their parent
reference account. Legacy references are resolved from their domain owner via
protected paginated APIs before activation; zero/ambiguous ownership blocks the
row instead of guessing from uploader convenience fields. The User S2S resolver
allowlist must add the authenticated File workload for this backfill; it cannot
reuse a `messaging|chat` identity. A free non-E2E
reference starts with `expires_at=created_at+P90D`; `ACTIVE` or `GRACE_PERIOD`
clears that deadline. On `INACTIVE`, every still-live non-E2E reference gets a
new deadline `downgrade_effective_at+P90D`, so an old Premium file is not deleted
at the transition. E2E references stay `created_at+P90D` in every tier.
Downgrade recalculation, inbox, and entitlement projection commit together and
serialize with upload/expiry claims. Renewal clears an entitlement-created
deadline only while the reference is still logically live. It never resurrects
an already expired reference or a blob handed to zero-reference GC. Shared
binary deletion remains reference-counted by File authority.

### Space Pro and reminder delivery

Space Pro uses the identical failed-payment lifecycle. `GRACE_PERIOD` preserves
all Pro caps for seven days. Recovery returns to `ACTIVE` and cancels remaining
reminders. Grace expiry or paid-period end produces `INACTIVE`; existing members,
tree nodes, and voice participants are not deleted or kicked, while new growth is
denied until the relevant count is below the free cap. Space stores the source
revision with its projection; transitional synchronous sync may remain only if it
carries the exact same revision and snapshot.

Voice stores the Space entitlement revision/deadline in `voice_db`; Space Pro
room admission uses that projection for the 32/128 cap and fails
closed/reconciles at `entitled_until`. Personal stream quality uses Auth's
durable personal entitlement carried in a short-lived trusted claim; Voice must
not infer Premium from an unversioned client value. Grace preserves paid caps;
stale failure/expiry cannot regress a recovered projection.

Notification consumes entitlement snapshots before reminders. A reminder ahead
of its state projection is retried; an older or non-grace reminder is a stale
no-op. Its local transaction creates a unique channel-dispatch outbox row keyed
by `(aggregate_kind, aggregate_id, grace_revision, day, channel)`; a leased
worker retries with one stable logical notification/client ID. In-app insertion
is exactly-once by that ID; FCM/APNs dispatch is at-least-once because provider
acceptance and the local commit are not atomic, so clients collapse repeats by
the same ID. Personal reminders go once logically to the account's
authoritative primary profile; Space reminders go once to the purchaser
account's primary profile and identify the Space. Immediately before external
dispatch the worker compares the current Subscription revision/state through a
protected `ResolveEntitlementAtBoundary` call with
`minimum_revision=grace_revision`: recovery committed before that comparison
suppresses the row. Timeout/unavailable retries only while the reminder window
remains open, then stores `SUPPRESSED`; it never sends on an assumed grace state.
A provider request already sent may race a later recovery
and cannot be recalled. They use push plus in-app routing and respect normal
settings/quiet hours. Email remains auth-only and is not added by this contract.

## Milestones and ownership

- [ ] S1 — protobuf and migration seam: `SubscriptionStreamEvent`, Subscription
  aggregate revisions/outbox/reminder schedule, and generated stubs.
- [ ] S2 — Subscription state machine: fake-provider normalization, webhook
  replay/order, personal and Space Pro transitions, leased dispatcher, snapshot
  API, account delete/restore/purge receipts, and reconciliation metadata.
- [ ] S3 — Auth/User consumers: durable tier projection, downgrade pending/default
  selection, eligible-pair override, token/profile recovery, and renewal unfreeze.
- [ ] S4 — File consumer: per-reference retention-account binding/backfill, retention projection,
  downgrade recalculation, renewal race, and expiry/GC fencing.
- [ ] S5 — Space/Voice/Notification/Analytics consumers: revisioned Space and
  Voice enforcement caches, grace reminders/channel outbox, complete analytics
  mapping.
- [ ] S6 — Gateway/Flutter/fake-provider vertical and feature/Compose E2E.

Parallel write ownership must remain disjoint: S1/S2 own Subscription/protos;
S3 owns Auth/User/Flutter profile selection; S4 owns File; S5 is split by
Space, Voice, Notification, and Analytics modules. S6 integrates only after those PRs
land. Each code slice follows documentation-first RED/GREEN and merges through a
separate PR-owner.

## RED acceptance matrix

| ID | Required failing proof before implementation | Expected GREEN evidence |
|---|---|---|
| SUB-R01 | Crash after lifecycle DB commit but before publish. | Restart publishes the one stored event with the original UUID/bytes. |
| SUB-R02 | PubAck succeeds, worker crashes before outbox completion. | Retry uses the same `Nats-Msg-Id`; each consumer effect occurs once. |
| SUB-R03 | Two dispatchers claim one row and the first lease expires. | Only the current token completes; stale completion cannot erase retry state. |
| SUB-R04 | Duplicate webhook and same ID with changed retained body. | Exact replay returns saved outcome; changed body is quarantined and has no mutation. |
| SUB-R05 | Renewal is delivered before an older failed-payment event. | Projection remains the higher `ACTIVE` revision in every consumer. |
| SUB-R06 | New consumer starts after JetStream history expired; snapshot spans multiple pages while newer events arrive. | One-watermark pages including inactive tombstones plus queued events build current state; missing/failed page never deletes local state. |
| SUB-R07 | Concurrent grace sweepers at D1/D3/D7 and crash around PubAck. | One reminder/in-app ID per day; any push retry carries that same client/collapse ID. |
| SUB-R08 | Renewal races a claimed D3/D7 reminder. | Recovery committed before the dispatcher's final authoritative compare suppresses the row; a provider request already sent is explicitly outside recall guarantees. |
| SUB-R09 | Premium expires with three to five profiles and no client online. | User enforces primary + deterministic eligible secondary and persists the cycle-bound selection outcome. |
| SUB-R10 | User submits another exact primary+eligible-secondary pair, repeats it, then renewal arrives. | Selection is idempotent for one downgrade cycle; renewal clears the cycle and removes only the subscription-freeze overlay. |
| SUB-R11 | Selection from a cleared/different cycle arrives after renewal. | It is rejected/no-op and cannot refreeze profiles. |
| SUB-R12 | Personal downgrade covers young, older-than-P90D, E2E, expired, and shared references. | Live non-E2E deadlines start at downgrade + P90D; E2E remains upload + P90D; renewal rescues only live references; shared blob GC waits for zero references. |
| SUB-R13 | Upload races downgrade/renewal projection. | The committed file deadline matches the locked latest entitlement revision. |
| SUB-R14 | Space Pro payment fails, recovers, then a stale failure redelivers. | Pro remains through grace/recovery; stale failure cannot lower caps. |
| SUB-R15 | Space Pro grace expires above each free cap. | Existing state remains; only growth is denied and one inactive snapshot is applied. |
| SUB-R16 | Notification receives reminder before/after matching state and redelivery after restart. | Ahead retries, stale drops, matching logical/in-app delivery is durable and unique; push remains at-least-once. |
| SUB-R17 | Consumer DB error or dependency outage. | Message is not ACKed; restart/retry converges from inbox/projection. |
| SUB-R18 | Same event/revision is delivered with conflicting bytes/snapshot. | No domain mutation; contract mismatch is observable and paged. |
| SUB-R19 | Voice sees Space Pro grace/recovery/expiry out of order and a personal quality claim reaches its deadline. | 128 cap survives grace/recovery, stale expiry cannot lower it, and expired paid quality fails closed until reconciliation. |
| SUB-R20 | Notification commits channel work, crashes before/after provider acceptance, then restarts. | In-app row remains unique; push retries with one logical/client ID and may be provider-delivered at least once, with duplicate collapse rather than an impossible exactly-once claim. |
| SUB-R21 | Snapshot page fails/is replayed, includes inactive tombstones, and receives a scope-tampered cursor. | Failure preserves local state, replay is idempotent, stale active converges to inactive, and tampered/cross-target cursor is denied. |
| SUB-R22 | Analytics starts beyond stream MaxAge across the canonical cutover. | Journal replay restores canonical reason history once; legacy entitlement events after the watermark do not double-count it. |
| SUB-R23 | An inactive account with subscription-frozen profiles purchases Premium again. | `STARTED` clears the old downgrade cycle/selection and removes every subscription-freeze overlay without reviving independent disabled state. |
| SUB-R24 | Premium account schedules deletion, restores inside P30D, then is deleted while provider cancellation remains unavailable past purge. | Delete emits immediate inactive; restore exposes only verified paid time without resuming billing; P30D erases raw IDs into HMAC fence while an unlinked cancel escrow retries, then crypto-shreds its handle on receipt. |
| SUB-R25 | A deleting account pays for annual Space Pro used by a live Space beyond P30D. | Renewal/reminders stop; purge emits `PURCHASER_PURGED`, clears raw payer ID, and paid Space caps plus later period-end inactive continue by `space_id` without identity transfer. |
| SUB-R26 | Consumer misses delete scheduling, stream raw event is purged, and a pre-delete backup restores an active projection with only account ID. | Protected legacy-key lookup resolves the permanent HMAC fence, local raw state is purged before serving, and no entitlement is exposed. |

## Validation

- [ ] `buf lint`, `buf format -d --exit-code`, breaking check, and generated Go,
  Java, and Dart stubs for the additive contract.
- [ ] Full Subscription database integration suite with injected DB clock and
  deterministic fake-provider versions.
- [ ] Focused Auth Maven, User Go, File Go, Space Go, Notification Go, and
  Voice Go and Analytics Go tests including each service's inbox/restart cases.
- [ ] Embedded JetStream redelivery tests for stable IDs, PubAck loss, durable
  consumer restart, stale revisions, and conflicting bytes.
- [ ] Compose billing proof: personal failed payment → D1/D3/D7 behavior →
  profile/File downgrade → renewal/unfreeze, plus Space Pro failure → grace →
  recovery/expiry and cap behavior, plus account delete → restore/purge with
  provider-operation and consumer receipts.
- [ ] Flutter test for authoritative pending picker, exact eligible-pair selection,
  current-profile recovery, stale submission, and renewal disappearance.
- [ ] CI must be green on the exact head SHA before each implementation PR merge.

## Progress

### S1 producer durability slice

The first implementation slice adds the additive snapshot envelope and an
internal, source-disabled PostgreSQL producer seam. It does not enable a
provider adapter, change entitlement authority, or activate consumers. Its
acceptance is SUB-R01–R03 for stored event bytes and fenced dispatch, plus
transaction rollback, revision compare-and-set and invalid snapshot rejection.
S2 remains responsible for normalized provider replay/order and protected
snapshot APIs; S3–S6 remain required before authoritative cutover.

Implementation order: delegate contract/storage RED tests; review the tests;
add protobuf/generated stubs and migration `000004_entitlement_outbox`; add
`internal/entitlementoutbox` transactional append, lease claim and completion;
add stable-byte JetStream dispatch; run focused and full Subscription tests,
Go lint and protobuf/generated parity; obtain independent security review.
All production changes follow failing tests. PostgreSQL owns lease time and
aggregate serialization; callers provide an expected revision within their
billing transaction. No fixture or live handler calls the seam by default.

S1's recorded PubAck sequence is evidence of the acknowledged delivery only.
Before activation S2 purge must also discover every retained copy after an
ambiguous publish/retry beyond the JetStream deduplication window; one stored
sequence cannot prove erasure of an earlier unacknowledged copy.

- [x] Snapshot contract and validation RED/GREEN (including delete/purchaser privacy fields).
- [ ] Atomic revision/snapshot/outbox storage RED/GREEN.
- [ ] Stored-byte dispatch and stale-lease fencing RED/GREEN.
- [ ] Security review, affected checks and CI on PR head.

Local evidence: Subscription `go test -short ./...` and `golangci-lint run
./...` pass; embedded JetStream proves stable-ID byte replay and corruption
rejection. `buf lint`, format, breaking and Go/Dart generation pass; Auth
`mvn -B generate-sources` succeeds. PostgreSQL tests require hosted CI because
the local Docker Desktop engine pipe is absent. Storage tests remain open until
that full, non-short run succeeds; short skips are not database evidence.

- [x] Canon and current producer/consumer gaps audited.
- [x] Provider-independent lifecycle, event, outbox, inbox, reminder, User,
  File, Space, Voice, Notification, and Analytics decisions accepted in docs.
- [ ] Runtime milestones S1–S6 remain open and must not be inferred shipped from
  this decision PR.

## Risks and follow-ups

- The current `SubscriptionStreamEvent` payloads are too sparse for revisioned
  convergence; consumers must not emulate revision from timestamps or NATS
  stream sequence.
- The existing direct post-commit publishers and unversioned Space sync are
  migration paths only. Dual publishing must be bounded and observed before the
  authoritative event cutover.
- Space ownership transfer does not define whether the old purchaser remains the
  payer or renewal transfers/cancels. No adapter may infer that behavior or move
  payment credentials; it needs a narrow H1 product decision before that path is
  enabled. The lifecycle/event durability work is independent.
- Live provider activation still requires merchant setup, environment-separated
  secrets/URLs, negative cross-environment signature tests, and the G2 go/no-go.

Outbox retention terms are distinct: authoritative entitlement rows are never
discarded while undelivered. Reminder rows have a delivery window, but reaching
it stores terminal `SUPPRESSED`/`EXPIRED` state; it does not silently delete the
row. Authoritative journal/outbox, inbox/dedupe/quarantine, and logical reminder
outcomes retain `P400D`; verbose provider attempt bodies may be redacted after
`P30D`. Billing/provider records retain their separate legal/privacy policy; see
[DATA_STORES.md](../DATA_STORES.md).
