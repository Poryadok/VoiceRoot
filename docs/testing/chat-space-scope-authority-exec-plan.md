# ExecPlan: native Chat Space-scope authority

## Purpose

Role and Space obtain a narrow authenticated snapshot of Chat's own attachment
and lifecycle facts. A chat moved away and back cannot reactivate a decision
bound to its previous attachment. This is an input to the active Space chat
permission/invitation consumer, not a user permission or a P3 activation.

## Context

Sources: `docs/microservices/chat-service.md`, `docs/microservices/space-service.md`,
`docs/DATA_MODEL.md`, `docs/DATA_STORES.md`, `docs/ARCHITECTURE_REQUIREMENTS.md`,
`docs/TESTING.md`, and TT `BE-A2-SPACE-CHAT-PERMISSIONS` c2-c4 published as
`SPACE-CHAT-AUTHORITY-v1`. Producer task: `CHAT-SPACE-SCOPE-AUTHORITY`.
Base: develop `61e1477bec0fcc4cb59e70f9a26cb84fdc126683`.

Current Chat stores native `chats.space_id` and a separate durable
`chat_space_lifecycle_fences` state/generation. Production creates Space chats
under `spacemutationlock`; P3 snapshots, restore and purge use the same fence.
There is no native attachment incarnation/revision. Role target validation is
not a replacement for this snapshot. Federation authority remains restricted.

## Scope

Add a dedicated protobuf service, PostgreSQL persisted attachment heads and
transactional source/lifecycle maintenance, own-DB snapshot, dedicated signed
Role/Space mTLS transport, generated clients, negative and concurrent tests.
No membership, Role decision, content, title, external DB access, stand rollout,
acceptance appointment, ordinary permission changes or Federation expansion.
172 owns separate `chat_lifecycle.proto`/Realtime listener; preserve it.

## Milestones

- [x] Meaningful native PostgreSQL RED for missing persisted attachment identity.
- [x] Migration and snapshot prove stable cosmetic edits, move ABA, recreate,
      lifecycle ABA, source revision coverage and nondisclosure.
- [ ] Production signed Role and Space transports pass exact binding/replay and
      alternate-listener negatives; generate and share immutable DTOs with155.
- [ ] Affected checks and independent ACL/concurrency review; portable source
      and evidence accepted by integration121.

## Detailed Steps

1. Inventory all native consumed-field writers and lifecycle transitions.
2. Add real PostgreSQL assertions before production migration/read code.
3. Persist a permanent per-chat head with positive nonwrapping revision and
   randomly allocated attachment incarnation; maintenance occurs in the source
   transaction. Cosmetic changes preserve the incarnation. Deletes preserve
   the head so recreation cannot reset its revision. Lifecycle changes advance
   the revision without replacing the independent lifecycle generation.
4. Read all facts in one PostgreSQL statement snapshot, checking head/source
   consistency. Missing/cross-Space/unsupported scopes fail closed.
5. Add dedicated transport for exact service:role and service:space only,
   retaining shared signed principal/TLS/JWKS/replay semantics. Bind operation
   ID, complete request hash, audience and full method; ordinary listeners deny.
6. Generate clients, run affected Go/proto checks in owned Linux Docker, obtain
   independent review, fix findings with the same reviewer, freeze and hand off.

## Validation

- Native PG: initial backfill, title edit stability, move-away/back, deletion and
  same-ID recreation, managed/type reassignment, lifecycle freeze/restore,
  rollback, same-chat concurrent updates and overflow fail-closed.
- Handler: malformed IDs/version/unknown fields, missing/foreign/managed scope,
  unavailable source, no content or membership projection.
- Real TLS/JWKS/replay: Role+Space positive, unsigned/forged/wrong caller,
  audience/method/hash/request ID/reused JTI negatives, ordinary listener denial.
- Affected module short/full tests, vet/lint, proto lint/format/generation parity;
  exact assembled/hosted CI stays with integration121.

## Progress

- [x] Atomic TT claim; contract-only prerequisite across all gates.
- [x] Own WT20 lease and actual develop base; adjacent172 scope coordinated.
- [x] Test-first native persistence proof: Linux Go1.26.5/PG16 RED expected nil
  incarnation; native GREEN includes actual two-connection lock wait, source/head
  snapshot coherence, ABA and nonwrapping revision guards.
- [x] Generated contract `db45a41a57ae5b1872a8e06097b9ad123ddeabb5` delivered155.
- [x] Real TLS/JWKS/replay -> production handler -> actual PostgreSQL all25
  migrations passed for both Role and Space, including zero-source-call denials.
- [ ] Final frozen source parity, component checks and independent review.

## Decisions

- Read one own-DB statement snapshot rather than network calls or a lock held
  over I/O. Remote consumers retain their accepted double collection/CAS.
- A separate dedicated service avoids widening Federation or existing protected
  lifecycle mutation listeners. It grants no access by itself.
- Native source triggers maintain heads without adding advisory locks after row
  locks. Existing creation/P3 barriers remain, and a single statement snapshot
  sees complete committed source/head/fence versions. No Chat reparent endpoint
  exists today; future native writers receive trigger coverage automatically.
- Chat's existing common fence enum omits persisted PURGED. The additive DTO uses
  its own complete lifecycle enum, agreed with155, rather than mislabeling it.

## Risks And Follow-Ups

High risk: ACL boundary, durable migrations and ABA/concurrency require an
independent reviewer and actual Linux PostgreSQL execution. Source proof does
not establish downstream feature completion or deployment/acceptance.

One initial component run failed because its native export omitted unchanged
`deploy/` and `docker-compose.yml` fixtures; this is runner incompleteness,
not a product assertion failure. The complete immutable fixture export is used
for the corrected main-package run. One new configuration test originally
asserted that Space could never use its preexisting manifest RPC; corrected to
assert the actual dedicated scope-listener allowlist, preserving existing Space
lifecycle/manifest authority and the new narrow scope boundary.
