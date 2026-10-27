# A4 account lifecycle producer contract

## Purpose and status

Define the Auth lifecycle authority and User complete-profile coverage that
Matchmaking needs to retire old terminal routing at its original account-deletion
deadline. This is an independently agreed target, not a shipped producer, an activated
capability, or a whole-account erasure acceptance claim.

The delivery consumer is the owner of
`A4-MM-TERMINAL-RELEASE-ERASURE`. Auth implementation remains in `BE-019` and User
implementation in `BE-020`; documenting this boundary does not complete either.

## Context

- [A4 PLAN](../PLAN.md#a4--репозиторий-готов-к-безопасному-внешнему-alpha).
- [Account recovery and erasure](../features/auth-and-contacts.md#удаление-аккаунта--gdpr).
- [Auth](../microservices/auth-service.md) and
  [User](../microservices/user-service.md) own their respective authority.
- [Retention](../DATA_STORES.md) and [data boundaries](../DATA_MODEL.md)
  require original deadlines, service-owned cleanup and protected fences before
  restored data is served.
- The existing `voice.identityerasure.v1` Role/User-purpose methods do not grant
  Matchmaking a new purpose or establish complete account-profile coverage.
- The accepted owning decisions in `BE-020` c4-c9 and the bounded Anton-195
  task-2 memo constrain this continuation. The memo selects terminal routing
  behavior while leaving exact restoration of other durable content separate.

## Scope and ownership

One author maintains the producer semantic contract, Auth/User service sections,
contract-matrix rows and a versioned synthetic producer fixture. Auth/security
review and User-coverage/MM review are independent of authorship.

Do not change Matchmaking source, its frozen PR954, `DATA_STORES.md`, the MM
service document, existing runtime listeners, migrations, credentials or provider
activation. An additive proto record is possible only after its semantic mapping
and purpose/currentness binding are established; do not invent an RPC, subject or
caller grant to make an unavailable producer appear connected.

## Semantic obligations to preserve

### Auth authority

Account deactivation atomically commits an opaque stable deletion fence, a new
never-reused deletion cycle, its original schedule and immutable `purge_at`, a
positive monotonic lifecycle revision, and the durable producer outbox. Transport
arrival, retries, event timestamps and Redis TTLs never restart the deadline.

Restore and expiry serialize on the same owning row using authoritative DB time.
Restore wins only strictly before the original deadline. Each cycle's decision,
decision time and revision are immutable; a new deletion has a new cycle and a
higher revision. A protected observation distinguishes the current account
revision/state from the immutable decision of the specifically queried cycle.

### User coverage

User owns enumeration and the joined coverage record. It imports the exact Auth
fence/cycle/revision/deadline under its account lifecycle/admission row, closes
creation/binding changes for that cycle, and seals complete retained ownership
coverage plus its outbox in the same local transaction. Every create, bootstrap,
import and binding path participates in that serialization. No lock or database
transaction crosses an Auth/User RPC.

Coverage includes retired or deleted profile identities still referenced
downstream. Caller-selected IDs and ProfileCreated replay do not establish
completeness. The sealed record binds its schema, Auth tuple, User coverage
revision, full set or sealed bounded pages, canonical hash/count and an explicit
completeness result. A stale cycle cannot change current eligibility or append
new profiles to an old sealed manifest.

### Consumer and recovery boundary

Matchmaking installs the original deadline from Auth even while coverage is
pending. Missing coverage blocks affected admission/completeness and never
extends retention. At the deadline, old terminal routing is irreversibly
`ROUTING_ERASED`; this does not assert Auth `ACCOUNT_PURGED`.

A later authenticated observation of a restore committed before the deadline
may permit fresh admission after current eligibility checks. It cannot recreate
old sessions, queues, effects or final-leave work. Restore committed at or after
the deadline is forbidden. This says nothing about resurrecting match history or
other recoverable content.

Protected recovery installs current permanent fences and passed local deadlines
before serving. Legacy events cannot invent a deadline, prove complete coverage
or clear versioned fences. Backfill must recover original scheduling evidence;
missing evidence remains an explicit migration gap. Compatible rollback must
preserve this enforcement floor. Consumers receive neither HMAC keys nor reverse
identity mappings.

## Semantic records v1

The [synthetic vector](a4-account-lifecycle-producer-v1.json) fixes a portable
semantic fixture. Its JSON container is not an RPC/event payload or an authority
signature. Actual producer records require authenticated, request-bound current
or terminal reconciliation and the declared capability before activation.

| Record | Required semantic fields |
|---|---|
| Auth schedule | Opaque lifetime deletion fence, never-reused cycle, positive `schedule_revision`, original `scheduled_at`, immutable `purge_at` |
| Queried cycle decision | Exact schedule identity, `RESTORED` or `ACCOUNT_PURGED`, positive decision revision greater than schedule revision, committed decision time |
| Current Auth fact | Positive current lifecycle revision, current state and current cycle where applicable; never inferred from an old queried-cycle decision |
| User complete coverage | Schema version 1, exact Auth schedule, positive coverage revision, explicit completeness, total count, canonical hash and complete sorted set or sealed bounded pages |

The schedule revision is immutable even after a higher restore decision/current
revision. User's old sealed manifest remains bound to its schedule, not silently
rebound to the restore revision. New profiles after restore belong only to the
next complete deletion manifest. Unknown/missing fields or exact tuple collisions
are not permissive defaults. Revisions/counts in the fixture use canonical
unsigned decimal strings so its values do not depend on JSON number precision.

### Manifest integrity v1

`sha256-framed-user-coverage-v1` hashes this byte sequence in order:

1. ASCII `voice.account-lifecycle.user-coverage.v1` followed by one NUL byte.
2. Unsigned 32-bit big-endian schema version 1.
3. Length-framed UTF-8 Auth fence, then cycle: unsigned 32-bit big-endian byte length
   followed by the exact bytes, with no normalization or delimiter.
4. Unsigned 64-bit big-endian Auth schedule revision.
5. Original schedule time, then original deadline, each signed 64-bit big-endian
   Unix seconds followed by unsigned 32-bit big-endian nanoseconds in `[0,999999999]`.
6. Unsigned 64-bit big-endian User coverage revision; one byte for completeness
   (`0`/`1`); unsigned 64-bit big-endian total profile count.
7. Every canonical lowercase profile UUID in increasing byte order, each using
   the same length-framed UTF-8 representation; duplicates are invalid.

SHA-256 is represented as 32 bytes in a binary binding and 64 lowercase hex digits
in the fixture. Its base64 preimage permits independent implementation checking.
Changing a bound field changes the digest. A matching digest on `complete=false`
or a selected subset is still not complete coverage. Full-roster digests and raw
manifests are not permanent replay selectors; erase them with their classified
payload and retain only the approved opaque outcome/fence.

Bounded delivery pages must bind the immutable descriptor, page order/count and
exact total manifest. A partial page set cannot become complete. This semantic
hash does not allocate pagination RPCs or claim existing selected-ID observations
can enumerate the complete set.

### Vector obligations

- The first closed set includes an active profile and a retired referenced
  profile. The post-restore profile is excluded from that immutable set and is
  included in the next cycle's new complete manifest.
- Auth commits restore at `D-1s`; MM has no restore observation at `D` and Redis
  is unavailable. Old routing becomes `ROUTING_ERASED` without inferring Auth
  purge. At `D+1h`, verified predeadline restore plus current eligibility permits
  fresh user-initiated work only. No old payload or full-roster digest is revived.
- Restore at equality is rejected; old cycle revision 42 cannot cancel new cycle
  revision 43. Missing coverage never changes either original deadline.
- Changed cycle, revision, deadline, count or digest conflicts. Legacy signals,
  unbounded raw Redis keys, unprotected backup serving and unsupported rollback
  remain activation denials.

## Milestones and detailed steps

1. Reconcile the existing owning decisions and current proto/service boundaries.
2. Specify the smallest versioned semantic records and synthetic hard-case
   fixture, including manifest integrity and current-versus-queried-cycle rules.
3. Add Auth/User owner sections and contract-matrix status/legacy rows. Add proto
   only if the mapping is determined without borrowing existing purpose grants.
4. Obtain independent Auth/security and User-coverage/MM review, record actual
   agreement, then publish the immutable source/fixture to the named consumer.

## Validation

- Parse and independently recompute the synthetic fixture's manifest integrity.
- Check local Markdown links and `git diff --check`.
- Run the committed current-diff contract against the actual base.
- If proto changes, perform canonical format/lint/breaking/generation checks and
  reconcile any generated consumer paths before claiming the proto slice ready.
- No fixture or documentation check substitutes for producer transaction,
  protected transport, MM deadline, Redis outage or recovery runtime proof.

## Progress

- [x] Supported custody resume preserves both task stages and historical assignees.
- [x] Fresh isolated lease and source branch verified at the current master.
- [x] Anton-195 decision, A4 PLAN and primary feature/service/data canon read.
- [x] Compare prior owning Auth/User decisions without extending the MM exception
  to User recoverable content or allocating a duplicate lifetime handle.
- [x] Draft semantic records, manifest fixture and owner/matrix sections.
- [x] Validate the frozen packet and obtain both independent reviews at
  `6bf8e22583976363aca135c4265887f222e43936`: Anton-140 Auth/security and
  Anton-172 User-coverage/MM. The latter independently rebuilt all three framed
  manifest preimages and digests (`BE-020` c14).
- [x] Publish only actually agreed contract evidence to the consumer.
- [x] First producer component: acquire the existing account row lock before
  sampling `clock_timestamp()` in the deletion transaction. Commit
  `56610364b422139a11b564927340854908e4d7e0` preserves the existing outbox,
  session-epoch floor and idempotent operation seams. A real held-row test first
  failed with a caller clock seven days ahead; Linux/JDK 25 GREEN ran five tests
  with no failures, errors or skips. Anton-140 independently cleared this
  four-path component; this does not qualify the full producer.
- [ ] Retain immutable queried-cycle schedule/decision facts and durable producer
  intent using the existing opaque lifetime handle, cycle and revision lineage.
- [ ] Implement protected request-bound current/terminal reconciliation and
  complete User coverage with creation-path closure, then connect the named MM
  consumer without borrowing a Role/User-purpose grant.

## Runtime implementation findings

The existing `role_identity_lifetimes` row already owns the stable opaque handle,
cycle and current revision. Do not allocate another identity lineage. Its journal
does not retain authoritative decision timestamps, and an ACTIVE current fact
alone cannot prove which prior cycle restored. Historical missing decision times
remain unavailable rather than receiving an invented timestamp.

The exact Auth identity catalog currently rejects additional lifetime triggers,
and its migration-history validator requires a complete declared loader profile.
Any additive cycle-history integration must update those compatibility proofs
explicitly, preserving fail-closed rollback and backup behavior. The linked-OAuth
producer owns Flyway V32/canonical 000033 and its history profile. The frozen
Subscription authority source `ee7f2b87e9c996d26e670bd7a0a70505a0d42633`
also contains Flyway V33/canonical 000034 subscription capture. The next A4
migration must reconcile both exact predecessors before choosing its own slot.
That Subscription-purpose capture uses raw account identifiers, operation IDs as
its cycle and a separate source revision. It is not the agreed opaque-lifetime
contract, complete User coverage or a generic MM grant. Preserve its existing
purpose and do not silently substitute those identifiers for the A4 tuple.

## Decisions and follow-ups

`ROUTING_ERASED` and `ACCOUNT_PURGED` are different authorities. The unresolved
exact-content restoration question does not block this terminal-routing slice;
it must not be silently resolved for other content owners here.
The [existing A4 backlog](../todo/backend.md) records that content-owner decision.

Producer runtime, complete historical ownership backfill, protected current and
terminal reads, capability activation and compatible restore tooling remain
implementation/acceptance work. This packet must label them explicitly rather
than infer them from existing account-only events or caller-selected snapshots.
