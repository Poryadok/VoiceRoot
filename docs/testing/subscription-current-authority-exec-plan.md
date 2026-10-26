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

## Decisions

The privilege fence is a real database boundary: drain old DML under table locks,
remove effective direct/inherited write rights, and use separately limited new
runtime roles. An environment boolean, caller generation or owner/superuser
connection is not equivalent. READY does not certify an account: a truthful
current writer and account lifecycle provenance remain independently required.

## Risks And Follow-Ups

Auth paths require current owner coordination. Production role/process inventory,
provider ordering, privacy restore and real account ingress qualification remain
gates; tests cannot fabricate those receipts. BE-014 snapshot sessions/journal
and all downstream convergence, and full BE-028 privacy/provider behavior remain
open until actually implemented and qualified. No broad task completion is
claimed from a source substage.
