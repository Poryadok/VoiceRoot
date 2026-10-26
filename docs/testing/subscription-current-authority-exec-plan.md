# ExecPlan: Subscription current authority

## Purpose

BE-014 and BE-028 establish the current Subscription authority consumed by
BE118-GRACE-BOUNDARY-AUTHORITY and Notification grace reminders. A signature over
a legacy or shadow snapshot must never become current authority. This plan does
not broaden BE-181's separate Messaging/PERSONAL-only decision.

## Context

The contracts are PLAN A7, features/subscription.md,
microservices/subscription-service.md (boundary read, account lifecycle and
downstream effects), microservices/notification-service.md, DATA_MODEL.md,
DATA_STORES.md, ARCHITECTURE_REQUIREMENTS.md Phase-0, TESTING.md and
CONTRIBUTING.md. The existing PERSONAL provenance decision is
DECISION-BE181-PERSONAL-PROVENANCE-v1, comments c3-c6 in project voice.

Implementation base is PR935 `ab167cf0e554e2d1268dd6c0905b0d4e2b232dab`.
Anton-144 explicitly released successor providerlifecycle, read/API/main and new
migration paths to author Anton-147 while retaining frozen PR935 delivery
custody. Migration 000006 remains immutable. One author owns tests and code.

Current source has normalized source-disabled provider transactions and atomic
snapshot/outbox/reminder writes. Main still uses legacy billing writers. It has
no qualified writer generation or account lifecycle ingress. Auth has durable
PENDING_FLOOR/PENDING_EVENT deletion work, but restore still publishes an
unversioned event. A new account command must use the actual durable Auth
operation and ordering; an unversioned event or test marker is insufficient.

## Scope

Canonical writer coverage, durable database-enforced old-writer cutover, current
snapshot provenance, local deadline transitions, account deletion/restore/purge
ingress, and the exact protected Notification read form one coherent source
slice. No production credentials, provider activation, stand rollout or final
acceptance are included. Existing source-disabled code remains so until every
required writer/account input is covered. Legacy rows begin UNKNOWN.

## Milestones

- [x] Re-read and atomically claim existing BE-014 and BE-028 separately; relate
  the actual reminder consumer prerequisite to both.
- [x] Obtain fresh own lease, exact frozen base and producer path acknowledgement.
- [x] Real database-role cutover rejects unsafe roles and drains/excludes old
  transactions; stale connections and restarted legacy writers cannot mutate.
- [ ] One strict normalized provider and local-boundary kernel owns canonical
  bytes, revision, provider order, journal, outbox and reminder changes atomically.
- [ ] Actual Auth durable schedule/restore/purge ordering feeds protected
  Subscription ingress; a fresh owner lifecycle check closes unknown admission.
- [ ] Qualified current read returns a complete snapshot, with exact minimum
  revision/deadline semantics and protected caller/request binding.
- [ ] Consumer agreement, negative transport proof, independent review, focused
  checks, serialized Graphify and real Luna source delivery custody.

## Detailed Steps

1. Test the real database privilege boundary first. Add new migration 000007;
   never edit an applied migration. Test inherited rights, unsafe superuser or
   owner roles, pre-cutover blocked transaction, live stale connection and new
   connection after cutover. PREPARING is default; no migration certifies rows.
2. Implement one shared generation-before-aggregate locking rule. CURRENT
   writers are the strict normalized provider reducer, exact local expiry and
   authenticated cancellation commands, Auth lifecycle commands, and integrated
   Space freeze/purge. Reads never take provider locks after aggregate locks.
3. Retain immutable provider replay outcomes but separately load latest current
   canonical bytes/hash/revision. Reject owner/superuser serving credentials,
   unqualified generation/account evidence, corrupt rows and lower revisions.
4. Coordinate Auth source paths before edits. Bind deletion to durable operation,
   account, epoch/cycle, DB effective time and P30D deadline. Restore requires
   actual same-cycle provider reconciliation; it never resumes recurring billing.
   Purge preserves only purpose HMAC fences and cancellation-only escrow, removes
   raw payloads and retained JetStream sequences, and emits truthful receipts.
5. Add dedicated protected request-bound Notification RPC with complete current
   snapshot response. Preserve BE-181 restrictions and existing Space matrix.
   Ordinary listener denies it. Client has min(500ms, remaining request/window)
   budget with no retry beyond deadline. Exact grace revision, state, deadline
   and payer are compared immediately before dispatch; mismatch suppresses.
6. Test each behavior before implementation, then scoped Go/race/vet/lint and
   protobuf generated parity. Test actual TLS/JWKS/Redis/DB transports rather than
   stubs represented as live authority. Obtain independent source review.

## Validation

The first cutover source stage has real PostgreSQL tests for a held old
transaction (actual `pg_blocking_pids`), existing/restarted old logins, inherited
and SET-only capabilities, owner/schema-owner denial, immutable replay versus
privilege drift, metadata controls, and omitted writable login detection. These
tests do not qualify current account authority. Required remaining evidence includes PostgreSQL
two-connection cutover and renewal/expiry/delete races, crash rollback/restart,
historical replay versus current read, shadow/legacy UNKNOWN, ambiguous verified
provider input UNKNOWN, exact deadline equality, account deletion/purge and payer
suppression, and request-bound TLS/JWKS/JTI replay/ordinary-listener negatives.
Private fake proof is distinct from real provider/target activation evidence.

## Progress

Source ownership and contracts were investigated before code. Actual claims and
portable initial source inventory are in the three task records; this plan holds
the implementation sequence rather than a competing tracker backlog.

Independent preliminary security review found SET-only role reachability, legacy
control-table rights and schema-owner bypasses in the initial cutover diff. Real
role tests reproduce the SET-only bypass before its correction. Membership
ancestor checks deliberately reject even a capability behind non-inherited
memberships. The installer also enumerates other login roles rather than trusting
an incomplete legacy-role manifest, and same-request replay rechecks current
privileges. The sole administrative exception is the actual installing session.
The fence state is `FENCED`, never `READY` or a per-account CURRENT marker.

Exact-commit review of `d5153d552bb6075846348739df583d0025ee0e14`
confirmed those corrections and found predefined server-file/program roles could
still bypass ordinary database ACLs. The role predicate now also rejects
`pg_read_server_files`, `pg_write_server_files` and `pg_execute_server_program`
through the complete membership closure. Grant-only negative tests cover old
runtime, writer and reader; they never execute programs or access server files.
The old-runtime program-role case causally produced a persisted fence before
the correction, so its rejection is tested independently of a guessed flag.

### Next fact: ordered Auth delete/restore capture

The successor normally merges OAuth predecessor
`5c0a104631e6b4cea8f5ad74c3b58e7cb3989b07` as copied source, without claiming
its independent CI or master integration. Existing V32/canonical 000033 and
their exact history profile remain immutable. The reserved next pair is
V33/canonical 000034; add a new exact history profile instead of rewriting an
accepted profile.

Capture actual durable `account_deletion_operations` insertion in its Auth
transaction, using that operation UUID as the deletion cycle/event identity and
the account's actual `deleted_at` as scheduled time. Capture the actual conditional
`JdbcAccountRepository.restoreDeleted` write in its statement transaction. The
restore fact is `RESTORE_REQUESTED`, never a paid grant: Subscription must still
reconcile the provider and enforce the original cycle/P30D boundary. Both facts
use a per-account source revision across delete/restore cycles, stable stored event UUID and complete
fact columns in an Auth-owned outbox. Replaying an operation, advancing its
delivery markers or retrying an already successful restore creates no new fact.

Tests drive the real Java transactional deletion starter, real JDBC repository,
actual Flyway catalog and PostgreSQL triggers; they do not manually insert a
CURRENT marker or invoke a pretend transport. A floor-store double isolates the
unrelated Redis dependency and is never claimed as live Redis proof. Required
RED/GREEN cases are deletion+fact atomicity, external-floor rollback, lost
response/replay identity, restore+fact rollback and a two-connection concurrent
restore. Legacy restore with no proven deletion operation remains a functioning
free path and creates no invented paid-authority fact. No new RPC, dispatcher,
purge completion, account CURRENT certification or runtime activation is claimed
by this bounded producer fact; those remain required later stages.

Receipt cleanup must not delete pending facts or reset the head: the deletion
cycle is an immutable logical reference, independent of coordinator-row retention.
Raw capture rows are Auth-owned and cascade only with the owning account; terminal
HMAC retirement and P30D dispatch/purge handling remain required before CURRENT
qualification. Fixed P30D is 720 hours, independent of a session's DST change.
Real regressions first erased both pending facts during receipt cleanup and
extended a Europe/Berlin deadline by one hour before these corrections.

The ordinary Maven lifecycle selected 22 cases with zero failures/errors/skips:
9 actual capture/rollback/replay/retention/DST/history cases; 2 existing deletion
repository cases; 6 source-catalog contracts; 4 finite-history tests; and the
actual fresh Flyway plus pinned canonical-loader parity case. This includes the
real owning Auth reader accepting the exact appended profile and rejecting a
changed database checksum. V32/000033 and their accepted profile are unchanged.
The configured Spring deletion/floor rollback regression also passes separately.
No full Auth suite, live Redis/provider, protected Subscription RPC or CURRENT
qualification is inferred from this producer package.

## Decisions

### Dedicated Auth owner observation implementation

Implemented source uses the separate canonical/Java-mirrored
`voice.auth.v1.AuthSubscriptionLifecycleService/ObserveSubscriptionAccount` RPC.
Go and Dart outputs are generated; the Auth mirror guard now covers both files.
The finite principal is issuer `subscription`, subject `service:subscription`,
audience `auth`, exact RPC/hash/request UUID and replay JTI, plus a pinned client
URI SAN on mandatory mTLS. Complete HTTPS JWKS and Redis startup preflight is
outside the 500ms request budget; authentication and SQL remain inside that cap.
The common Go signer actually emits a 20-byte base64url JTI. A genuine Go-to-Java
request first failed UNAUTHENTICATED against the draft UUID-only check; the
closed verifier now accepts both canonical nonce forms and the real chain passes.

The native account security-revision function/trigger is pinned too: V12 covers
every observed eligibility/status/session field, including email-pending and
deleted_at. No historical migration is edited. The fresh source denies unsafe
runtime roles, capture/guard drift, future facts, missing/lower/conflicting rows;
legacy/headless accounts explicitly remain UNRECORDED. A late RESTORE_REQUESTED
is retained as truthful input, never converted into a paid grant.

New 000009 records only protected inputs in subscription_db. The private Go client
observation binds exact request/hash, full canonical response/hash, installed
generation and the remaining request deadline. Ingest performs no network wait
under SQL locks, uses generation 17006 before account 17010, preserves ordered
fact identity/bytes and commits the OBSERVED head, facts and receipt atomically.
It rejects revision/security/session rollback, same-version drift, expired input
and generation changes. Existing 000006/000007/000008 are unchanged. The new
tables join the complete fence inventory; no old manifest replay auto-grants them.
There is deliberately no CURRENT promotion/read in this source stage.

Real fixtures exercise Auth's production server factory with PostgreSQL16,
Redis7, HTTPS JWKS, exact client certificate, ordinary-listener/peer/replay/hash
negatives and SQL/authentication deadline waits. A standalone actual Java receiver
then accepts the common Go issuer and feeds the real restricted-role Subscription
ingest transaction, including delete then restore and replay. Private wire fixtures
are separately labelled as local transaction/decoder tests, not live Auth proof.
The native Go fixture requires a compiled Auth test classpath; a configured
zero-skip run is required evidence, not its default short/unconfigured skip.

The owner read acquires the native Auth DDL barrier and validates the exact
owned capture functions/triggers/history, then lock the actual account before
its head and ordered facts under READ COMMITTED. It returns full account state,
source security/session revisions, observation DB time and retained lifecycle
facts. An untouched account explicitly has UNRECORDED capture coverage; missing
accounts, gaps, head/fact conflicts, lower revisions or changed capture metadata
never become a positive clean-account result. It does not expose eligibility or
borrow role-purpose selectors. Concurrent restore/delete tests must prove the
actual account-row linearization point and coherent post-wait facts. The next
protected transport uses a separate Subscription-only principal/mTLS capability,
request binding, replay protection and bounded deadlines; source observation is
not a paid grant or CURRENT certification.

### Generation-bound normalized provider writer

The next bounded kernel stage adds `ApplyFenced`, using a real installed limited
writer login and the exact generation under a transaction-scoped shared 17006
barrier before provider/event/aggregate locks. The installer holds that barrier
exclusively. The compatibility `Apply` entry refuses an installed fence, including
when accidentally called using the new writer credential. A source UUID alone,
installer session, changed generation or fence-control privilege fails closed.

New migration 000008 appends `subscription_kernel_writes` to the complete fence
inventory. Only a newly APPLIED canonical revision records `WRITTEN`, atomically
with its matching provider binding/version/outcome, canonical bytes/hash and
outbox. Historical replay, STALE and UNCHANGED do not certify old rows. This is
write provenance, not account CURRENT evidence; no public/protected read may
admit from it alone. No migration, runtime registration or provider is activated.
Existing 000006/000007 remain immutable. An already installed older fence does
not automatically receive new table grants through request replay; installation
and its immutable source/role manifest must be qualified before serving this entry.

Causal tests cover legacy-entry bypass (RED before correction), actual restricted
login, shadow replay, generation mismatch, evidence-trigger rollback, a real
two-connection blocking-pid generation drain, fence-control privilege drift and
deadline expiry during the generation wait. Auth's role-erasure selector/core is
not reused as Subscription lifecycle authority. Dedicated protected owner
snapshot/ingress, local expiry/account/Space writers and CURRENT qualification
remain the next coupled stages; the existing milestone boxes stay open.

The privilege fence is a real database boundary: drain old DML under table locks,
remove effective direct/inherited write rights, and use separately limited new
runtime roles. An environment boolean, caller generation or owner/superuser
connection is not equivalent. READY does not certify an account: a truthful
current writer and account lifecycle provenance remain independently required.

## Risks And Follow-Ups

### Auth source and recovery disposition

Astra-178's bounded architecture disposition is recorded in BE-028 comment c8,
with pointers in BE-014 c10 and BE118-GRACE-BOUNDARY-AUTHORITY c2. A dedicated
fresh Java Auth account/lifecycle snapshot and protected ordered ingress are
executable next. A7 does not require an external identity-erasure CURRENT adapter;
BE-181's separate Messaging/PERSONAL decision is not broadened. Do not reuse
role-purpose selectors, or treat an active/missing native row as complete paid
eligibility. Read the actual account, lifecycle head and matching facts under one
coherent owned cut and demonstrate concurrent deletion/restore interleavings.
This owner observation is an input to Subscription admission, never its result.

Recovery remains fail closed. Before Auth or Subscription database/PITR restore,
stop paid/source admission; preserve available lifecycle/provider journals and
permanent purge evidence outside the restored domain. After restore, reconcile
that actual retained evidence and admit only the proven cohort under a reviewed
generation. Missing or ambiguous evidence leaves that cohort UNKNOWN. Fresh UUIDs,
TLS, provider paid state, or a marker inside the same restored database cannot
prove a lost deletion/purge never happened. There is no automatic READY/CURRENT
on restart or generation rotation. No CURRENT promotion/read exists in this source
stage, and WRITTEN-only rows remain insufficient for one.

Before an admission/read implementation can be registered, its executable gate
must bind the declared restore/admission operation, surviving evidence supplier,
exact source/cut and completeness rule, and reject stale generations/in-flight
responses. Actual recovery qualification must exercise older Auth, Subscription
and simultaneous backups, missing evidence and generation rotation. This records
required future checks, not completed recovery proof or an invented external
service. The immediately ready source stage remains the dedicated Auth snapshot
and ordered ingress; the absence of an identity-erasure adapter does not block it.

Auth paths require current owner coordination. Production role/process inventory,
provider ordering, privacy restore and real account ingress qualification remain
gates; tests cannot fabricate those receipts. BE-014 snapshot sessions/journal
and all downstream convergence, and full BE-028 privacy/provider behavior remain
open until actually implemented and qualified. No broad task completion is
claimed from a source substage.

## Auth owner observation and ordered ingress source checkpoint

The dedicated Java owner snapshot and Subscription ingress are implemented in this source stage. The closed mTLS RPC is voice.auth.v1.AuthSubscriptionLifecycleService/ObserveSubscriptionAccount; its result is a complete coherent Auth input observation. Subscription stores only OBSERVED head/facts/receipts under the installed writer generation. No CURRENT promotion, paid decision or runtime activation is implemented.

The production listener factory is exercised with actual PostgreSQL, Redis, HTTPS JWKS and mTLS fixtures. The real Go Subscription signer/client calls that Java listener and ingests the returned ordered lifecycle chain into a separate actual Subscription PostgreSQL database. The configured race run passed without skips; private decoder/transaction fixtures are separate unit evidence. The final targeted Auth run covers 14 tests without failures or skips after remaining JDBC budget propagation; the preceding selected regression run covers 65 tests. Native lock and slow-JWKS cases retain the 500 ms total serving deadline.

Remaining admission gates include provider/account/local-expiry/Space reconciliation, fixed-cycle restore and purge fences, retention and controlled recovery qualification, then the complete protected Subscription boundary snapshot. WRITTEN and OBSERVED are insufficient admission evidence. Existing BE-014 and BE-028 criteria remain open.
