# ExecPlan: ordinary Space event delivery

## Purpose

Accepted target `BE038-ORDINARY-v1`: a successful ordinary Space mutation leaves
a durable publication obligation, and Search converges from current Space
authority after duplicate, reordered or interrupted delivery. This document is
an implementation plan, not evidence that the target is shipped.

## Context

The owning contracts are [Space](../microservices/space-service.md),
[Search](../microservices/search-service.md), [CONTRACT_MATRIX](../CONTRACT_MATRIX.md),
[DATA_MODEL](../DATA_MODEL.md), [DATA_STORES](../DATA_STORES.md), and
[TESTING](../TESTING.md). Ordinary events are immutable committed facts used as
invalidation hints, not an event-sourced state, grants, commands or history API.

Current ordinary handlers publish after store mutations. The existing ownership
outbox, audit ledger/outbox, media authority invalidation and P3 lifecycle each
have separate identities, visibility and acknowledgement contracts. Their
existence does not imply ordinary mutations are durable.

## Scope and accepted wire rules

Keep logical `chat.events`, physical `chat_events`, `ChatStreamEvent`, existing
tags and subjects. Add only unused protobuf tags. The covered facts are:

| Committed transition | Subject and payload |
|---|---|
| Space creation | `space.created`: `space_id`, `owner_profile_id`; no bootstrap-owner joined event |
| Ordinary properties change, including MM configuration | `space.updated`: `space_id`, sorted unique `changed_fields` |
| New membership / actual removal | `space.member_joined` / `space.member_left`: `space_id`, `profile_id` |
| New or changed effective ban | `space.member_banned`: `space_id`, `account_id`, `banned_by_profile_id` |
| Existing tree upsert/remove/pin/unpin | `space.tree_changed`: existing `SpaceTreeChanged` fields and `change=upserted|removed` |
| Invite creation | `space.invite_created`: `space_id`, `invite_code`, privileged internal material |
| Tree reorder/category/room metadata not represented by an existing node fact | one `space.updated` with `tree`, `categories` or `voice_rooms` respectively |
| Actual invite revocation / unban | one `space.updated` with `invites` / `bans` respectively |

Property names are exactly `name`, `description`, `icon_url`, `banner_url`,
`visibility`, `entry_requirement`, `entry_questions_json`, `mm_config_json`,
`allow_guests`. Compare persisted normalized values, including JSON; a request
mask or an incidental `updated_at` write is not a change. Equal input, existing
membership, absent removal and empty effective reorder create no extra fact.
A ban also saves a distinct left fact for each membership actually removed;
this does not change which memberships BanMember may remove. Include neither
reason nor other account profiles nor authentication material in the ban event.

Do not add a generic update for a typed fact's member-count/tree side effect.
Composite operations retain their existing complete-outcome boundary and cannot
publish contradictory intermediate success facts. Do not change Role join
compensation, Chat/tree orchestration, timeouts or cross-service sagas.

Empty legacy `changed_fields` and unknown field names mean full invalidation.
Ownership's saved `space.updated` bytes remain unchanged: no reserialization or
second ordinary ownership-completion event. Future ownership field additions
belong to that owner. Logical `tree_node_upserted/removed` catalog labels do not
introduce new subjects or dual publication.

`voice.room_created/deleted` remain reserved and un-emitted. No new generic Space
WS op, push, automatic subscription, recipient index or Realtime filter widening
is authorized. Ordinary mutations never emit `space.deleted`: only converged P3
purge may do that. Dedicated access invalidation, audit IDs, ownership terminal
barriers and P3 schedule/restore/delete producers remain separate.

## Transaction, delivery and recovery contract

Each covered enabled mutation commits its domain change and all required
ordinary outbox rows in the same `space_db` transaction. Existing required local
audit/media effects remain atomic there too, with separate records and IDs.
Failure/rollback creates no ordinary fact; encode/insert failure rolls back the
domain mutation. A successful API call means domain outcome plus publication
obligation are durable, without waiting for NATS, Search or a client. A NATS
outage after commit cannot turn the committed result into a business rollback.
Ambiguous database outcomes retain the API's existing retry semantics; transport
dedupe does not invent request idempotency.

Generate one nonzero canonical UUID and one valid UTC timestamp at event
creation. Persist the exact subject and deterministic complete envelope bytes
before commit, with timestamp precision that round-trips exactly. Retry/restart/
upgrade republishes those bytes and `Nats-Msg-Id=event_id`; never regenerate the
ID/time or hydrate historical payloads from mutable entities. No secret headers.

Claim bounded batches with expiring database-time leases and fresh opaque
tokens; commit the claim before network I/O. Acknowledgement and failure updates
compare event ID plus current lease token. Only a `chat_events` PubAck with
positive sequence marks delivery; duplicate PubAck is valid. Missing/wrong ACK,
timeout, transport or local mark failure never silently discards a row. Crash
after publish replays the same bytes beyond the broker dedupe window.

Persist attempts and next-attempt time. Retry transient failure indefinitely
with bounded jitter/backoff from one second to at most five minutes and finite
network deadlines. Known failure releases only the matching token; uncertain
local updates recover by lease expiry. Corrupt payloads remain visible and
recoverable with an alert and bounded retry; unrelated rows continue. No TTL,
max-attempt drop or silent poison-row loss. Delivered ordinary rows may be
cleaned after confirmed PubAck; they are not the audit/replay journal. Pending
facts survive child-entity deletion.

Exception: terminal P3 purge supersedes remaining ordinary notifications only
after its participant barriers. Its terminal local transaction erases ordinary
pending/delivered rows with Space-owned data without marking them delivered or
expanding the minimal tombstone. P3 lifecycle delivery remains intact.

Production cannot select a dropping NoopPublisher. Durable persistence is always
enabled for covered mutations; unavailable persistence fails mutation readiness,
while unavailable NATS accumulates obligations. Expose pending count/oldest age,
attempts/failures, invalid-row count and worker liveness. Logs carry event/type/
Space/error class, never raw payload, invite code or ban identity.

## Consumer convergence and audience

There is no global or per-Space delivery ordering guarantee. UUID, timestamp,
arrival/scan order and WS sequence are not Space revisions. Scan order is only
fairness. Do not introduce an aggregate revision or journal for this slice.

Search refreshes authoritative catalog state for created, updated, joined, left
and banned facts; initially every update key refreshes, including empty/unknown.
It never computes member count arithmetically from event counts. Serialize each
Space refresh across workers/replicas from before the authoritative fetch through
projection commit, or prove an equivalent source-version fence. An old slow
fetch cannot overwrite a newer refresh. Use a supported service API, never
cross-database reads or forged user metadata. Timeout, permission and unavailable
results retry; only authoritative NotFound removes a stale projection. Preserve
the canonical catalog visibility predicate; public-to-private removes the public
projection, and all result reads retain caller visibility checks.

Search ACK follows durable commit or a deliberate valid no-op. Failed fetch/
store retries under its durable consumer policy, with explicit observable poison
handling. Idempotent refresh needs no permanent ID inbox; any new non-idempotent
effect must dedupe the ID atomically beyond the broker dedupe window. Reusing an
ID with different canonical bytes is a contract violation.

Check P3 FROZEN/PURGED fences in the projection write transaction: delayed facts
and racing refreshes cannot resurrect the Space. Ordinary NotFound handling does
not substitute for P3 purge evidence or tombstones.

Only the existing internal stream and relevant service consumers are authorized.
Search never persists invite codes or ban actor/account identities in its public
catalog. Analytics/Notification retain their separately documented purposes; an
event creates no automatic public notification. Never broadcast the internal
invite/ban envelope or copy it into audit details. Realtime retains `chat.>`.
A later UI vertical needs authoritative current audience/ACL resolution and
minimal invalidation; unavailable policy suppresses delivery.

Restart scans pending/expired leases; consumers resume durable backlog and reread
current state. Broker persistence is bounded by broker retention/failure policy,
not infinite recovery or client receipt. Lost history requires an explicit
supported source rebuild, never fabricated domain history. Client recovery uses
ACL-checked authoritative snapshots; transient failure preserves cache, explicit
access loss removes inaccessible data. No WS journal/global message catch-up.

## Milestones and detailed steps

- [x] Accept and canonicalize `BE038-ORDINARY-v1` and this verification matrix.
- [ ] Add compatible payloads and ordinary outbox schema/writer/worker plumbing.
- [ ] Replace covered post-commit publication with atomic store obligations.
- [ ] Wire serialized authoritative Search refresh and P3 fence checks.
- [ ] Verify T1–T12, independent review, required CI and rollout compatibility.

Implement compatible consumers/additive schema before enabling new producer
arms. Old rolling publishers remain outside the durable guarantee until cutover;
do not invent historical events. Record actual cutover/reconciliation evidence.
Implementation paths: `protos/voice/events/v1`, Space `internal/store`,
`internal/spaceevents`, ordinary delivery worker/startup, and Search
`internal/indexer`, `internal/store`, supported Space hydration dependency.
Use a distinct additive migration and preserve separately owned foundations.

## Validation

The following tests are required; unchecked rows are planned, not PASS evidence.

| Case | Required proof |
|---|---|
| T1 | PostgreSQL faults before/after outbox insertion roll back domain and all ordinary/audit/invalidation effects; commit retains every intended effect |
| T2 | Denied/no-op/retried membership, equal update and empty reorder add no fact; ban plus actual eviction saves distinct banned/left facts, no phantom removal |
| T3 | Exact golden ID/time/subject/header/bytes survive retry/restart/upgrade; additive payload round-trip, legacy empty update and unknown-field preservation |
| T4 | Fault after domain/claim commits, before/after PubAck and mark-delivered loses no obligation and eventually republishes the same fact |
| T5 | Concurrent workers, reclaimed leases, stale ACK, wrong stream, zero/nil ACK and DB mark failure enforce current-token acknowledgement |
| T6 | NATS outage leaves successful mutation pending; bounded retry/restart drains; poison row remains visible without starving other work |
| T7 | Duplicates beyond broker dedupe and permutations of join/leave, ban/unban, tree and properties converge through current reads |
| T8 | Search refreshes created/updated/members and current count, removes public-to-private, handles empty/unknown keys and distinguishes timeout/permission/NotFound; failed commit gets no ACK; two replicas cannot commit a stale held fetch |
| T9 | Concurrent FROZEN/PURGED fence and delayed events cannot resurrect or bypass P3 |
| T10 | Real broker/projection integration uses provisioned `chat_events` and `search-indexer-chat-v1`, actual PubAck and persisted Search state |
| T11 | Exactly one existing `space.tree_changed` subject; ownership bytes/visibility unchanged; reserved voice-room event count zero; dedicated invalidation tests unchanged |
| T12 | Invite/account moderation data absent from public WS/push/catalog/log payload; Realtime filter unchanged, no disconnected/unauthorized delivery claim |

Run affected Go unit/integration/race/lint checks and protobuf lint/format/breaking/
generation checks from TESTING. Realtime tests, only if touched, obey the Windows
restriction. Test source, payload and transaction boundaries plus Search; this is
not a whole-repository sign-off.

## Progress, decisions and follow-ups

Contract preparation is complete. Implementation and all runtime evidence remain
open. The accepted unordered invalidation model avoids inventing a shared
revision across independently owned streams; fenced current reads provide
convergence. P3 cleanup must include the new ordinary table before declaring
local purge complete. Deployment and final acceptance are separate gates.
