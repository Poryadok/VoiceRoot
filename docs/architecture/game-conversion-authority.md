# SDK conversion authority contract (G01/Q03/Q07/Q08/Q10)

This contract fixes the cross-service seam for both SDK account conversions.
Auth owns the operation and account credential state. Game Integration owns
player bindings and app/env grants. User owns profiles and historical author
records. Voice owns active media admission. Each service commits only its own
database. A successful Auth HTTP response never substitutes for a missing
owner receipt.

## Identity and revisions

- `operation_id` is a server-generated UUID; an Auth operation is unique by
  source account, conversion mode and client idempotency key. Reusing the key
  with a different request hash is `409`.
- `binding_id` is stable; `binding_revision` starts at one and increases for
  every owner, profile, character, status or authority change. Every mutation
  carries its expected revision. The Game Integration database serializes
  these mutations with row locking and compare-and-swap.
- `authority_epoch` is monotonically increasing per binding. An already
  accepted token, device proof or node snapshot with an older epoch cannot
  admit a new read, write, subscription or media session.
- At most one account/profile pair is active for a binding. A binding in
  `frozen` or `transferring` grants no new access to either party. Source
  credentials are fenced *before* target credentials can be issued.
- Auth's source account type is `sdk-account`. The target is a newly created
  regular account or an independently proven existing regular account. A
  conversion never calls guest conversion and never merges on email/name.

## Auth state machine

`prepared → previewed → confirmed → frozen → owners_ready → activated → retired`.
Before `confirmed`, cancellation is allowed and has no binding effect. From
`confirmed`, crashes retry the same operation until `retired` or an explicit
operator recovery outcome. `frozen` is a safe serving state. An operation
cannot skip a state on retry; status returns its last durable state and owner
receipt versions. An unknown or delayed receipt leaves it in its current
state. Terminal failure does not silently reactivate source credentials.

Auth verifies a current source proof and a separate target proof. For
existing-account conversion, it also verifies exact selected target profile
ownership. Preview reports target binding occupancy, profile limit, sanctions,
permitted history/membership and voice-session conflict. An occupied binding
is `409` with no automatic replacement. Confirm uses an explicit preview
revision. A stale preview is regenerated before any freeze.

## Game Integration receipt API

The following are authenticated Auth→Game Integration workload calls. The
caller supplies `operation_id`, source/target account IDs, target profile ID,
binding ID, expected binding revision and a canonical request hash. The
service authenticates Auth's workload identity independently from player and
developer credentials. Each endpoint stores `(operation_id, stage, hash)` and
its receipt in one transaction; retries return the identical receipt; a
different hash returns `409`. No generic service token can invoke them.

Wire version is `v1`. Auth first reads
`GET /internal/v1/conversions/preview?binding_id={uuid}&target_account_id={uuid}&target_profile_id={uuid}`;
the response contains `binding_id`, `source_account_id`,
`binding_revision`, `authority_epoch`, `policy_revision`, `target_occupied`,
`active_grant_digest` and `preview_revision`. `preview_revision` is the
SHA-256 hex digest of canonical JSON containing those fields in that order.
It is computed by Game Integration from current committed state. If the
binding, roster policy, lifecycle or target occupancy changes, it changes.
Auth stores the exact preview response and requires the same revision at
confirm; a mismatch returns `409 PREVIEW_STALE` before freezing anything.

Mutation endpoints are `POST /internal/v1/conversions/{operation_id}/freeze`,
`/transfer` and `/activate`. Every request has a JSON `version: 1`,
`binding_id`, `source_account_id`, `target_account_id`,
`target_profile_id`, `expected_binding_revision`, `expected_authority_epoch`,
`preview_revision`, and `idempotency_key` equal to `operation_id:stage`.
`request_hash` is the lower-case SHA-256 hex of canonical JSON excluding
`request_hash`; both sides verify it. `transfer` also carries
`freeze_receipt_id`; `activate` carries `transfer_receipt_id`,
`user_receipt_id`, `user_profile_revision`, `voice_receipt_id` and
`voice_media_generation`. Fields not relevant to a stage are omitted, not
silently ignored; unknown fields and versions fail closed.

Each `200` receipt contains `version`, `receipt_id`, `operation_id`, `stage`,
`request_hash`, `binding_id`, `source_account_id`, `target_account_id`,
`target_profile_id`, `binding_revision`, `authority_epoch`,
`policy_revision`, `grant_digest`, `state`, and `committed_at`. Auth checks
all identity fields, stage, request hash, monotonic revisions/epoch, and
receipt linkage before advancing its operation. A `202` response has an
`operation_id`, `stage`, `state: pending`, `retry_after_ms` (100–1000) and no
receipt ID. A retry never extends authority or command execution leases.
`409` denotes stale revision, occupied binding or different idempotent body;
`503` means authority unavailable and is retryable with the same operation.

| Stage | Atomic Game Integration effect | Receipt |
|---|---|---|
| `freeze` | Lock binding, check source owner/revision, set `frozen`, increment revision and authority epoch, fence app grants and node authority | `binding_id`, new revision/epoch, source owner, `frozen_at`, receipt ID |
| `transfer` | Check frozen epoch/operation; set pending target owner/profile, recompute current game grants from signed roster without role/history union | new revision, target identity, policy revision, grant digest, receipt ID |
| `activate` | Check User/Voice receipts and target proof; switch active owner once, increment epoch/revision, leave source retired reference | new revision/epoch, target owner, `activated_at`, receipt ID |

The API may return `202` while a node has not acknowledged the required
authority epoch. It must not return an activation receipt until every
currently serving placement is fenced or its lease has expired. An offline
node remains pending and cannot serve with a stale snapshot. Game Integration
does not create an Auth token.

## User and Voice receipts

User checks target profile ownership and lifecycle/sanctions at the current
revision. New-account conversion creates one primary profile through User's
normal account/profile rules; existing-account conversion selects an existing
owned profile. User records an immutable source-author tombstone referencing
the retired account and operation. It never rewrites old message authors or
grants old history to the target by conversion alone. Its receipt contains
profile ID, owner account, profile revision and tombstone revision.

Auth approval uses a dedicated read-only User eligibility RPC with exact
`account_id` and `profile_id`; it returns ownership, deleted/frozen state and
profile revision. `SwitchProfile` is mutating and cannot be used for preview
or consent. Auth must recheck account type from its database and revalidate
User eligibility at code exchange; scoped token issuance preserves the
selected profile rather than substituting the primary profile.

Voice receives the frozen authority epoch, ejects any source active session,
and refuses reconnect under the old epoch. It checks the account-wide target
session conflict and returns either a handoff receipt with media generation
and observed ejection time or an explicit conflict. Activation requires this
receipt; a short JWT TTL alone is insufficient. A target session is created
only through normal Voice admission after activation.

## Recovery floor

Master keeps a non-restorable tombstone journal keyed by retired source
account/binding with highest operation ID, authority epoch and binding
revision. Every backup restore or node resnapshot takes the maximum of local
and master floors. A backup containing an earlier owner, grant, key or command
cannot lower that floor or recreate access. Restore remains fenced until
reconciliation receipts are complete. Raw provider subjects remain scoped to
their app/env and do not appear in User/Voice payloads.

## Acceptance assertions

For every boundary, crash immediately before and after the owner commit,
repeat the same request, then try a different payload with the same key.
Observe one active binding owner and stable receipts. Race source reconnect,
target login, unlink, Voice join and backup restore against freeze/activation.
Verify old tokens, device keys, node snapshots and media access fail after the
fence; historical author remains visible only under existing ACL. Required
integration tests use real Auth, Game Integration, User and Voice stores and
cannot be counted complete with no-op receipt providers.
