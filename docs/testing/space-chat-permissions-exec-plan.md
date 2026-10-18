# ExecPlan: Space-chat grants and target-bound invitations

## Purpose

An authorized Space member can invite someone into the parent Space from a
specific chat. Successful entry opens that exact chat under current ACL. A
current system Owner/Admin can separately manage chat addition policy and an
ordinary member's explicit official-publication grant. Sending messages or
managing settings never supplies either capability.

## Context

- Task `BE-A2-SPACE-CHAT-PERMISSIONS`; activation consumer is the existing
  `BE-A2-CHAT-MANAGEMENT-PERMISSIONS` Chat/Messaging/Gateway/Flutter vertical.
- Accepted parent TT decisions c1/c3/c4/c5/c6/c7 govern behavior. c4 is the
  explicit user choice: invite into the parent Space, then open the target chat.
- Canon: `docs/microservices/role-service.md`, `space-service.md`,
  `docs/features/roles.md`, `spaces.md`, `deep-links.md`, `docs/DATA_MODEL.md`,
  `DATA_STORES.md`, `ARCHITECTURE_REQUIREMENTS.md`, `TESTING.md` and
  `CONTRIBUTING.md`. Current implementation base is
  `c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c`.
- Existing Role ordinary transactions fence ownership/deletion under a Space
  advisory lock. The registry occupies bits 0–42; approval is bit 42. Existing
  Space entry policy refuses interactive admission until its real pipeline is
  available. Current Space membership includes ordinary and leased community
  sources; neither `joined_at` nor an event-consumer projection is a durable
  membership incarnation.

## Scope

Implement the Role-owned grant/toggle producer, narrow signed authority reads,
and Space CHAT_TARGET invite/redemption/destination producer. Preserve the
standalone implementation and its official-thread guard. Chat/Messaging/Gateway
and Flutter production files plus `chat.proto` remain the parent consumer's
scope; coordinate exact interfaces before shared proto edits. No generic
permission mutation, local membership for a Space chat, provider bypass,
deployment, acceptance appointment or Federation activation is included.

## Milestones

- [x] Register distinct capability bits and default masks with meaningful RED.
- [x] Resolve membership-source/incarnation and authority/CAS interface against
  the accepted BE-010 episode contract; record exact consumer interfaces.
- [ ] Add schema and signed Role decision/manager mutation paths with current
  membership, scope, lifecycle, revision and idempotency checks.
- [ ] Add immutable targeted invites, actor-bound durable redemptions and exact
  destination resolution through the actual admission pipeline.
- [ ] Generate compatible contracts and run real PostgreSQL negative,
  lifecycle, migration and concurrency checks.
- [ ] Obtain independent ACL/concurrency review, repair findings, and deliver
  immutable source/evidence to the develop integrator without losing custody.

## Detailed Steps

1. Reserve registry bits 43 `TEXT_CHAT_POST_AS_CHAT` and 44
   `TEXT_CHAT_ADD_MEMBERS`, Role migration 20 and Space migration 32 after the
   actual registry/migration inventory and live-writer coordination.
2. Test registry names and seed defaults before production edits: ordinary
   Member/Moderator receive add capability, no implicit individual official
   grant; Owner/Admin have both ordinary bits. Guest policy stays conjunctive.
3. Bind a Role grant to exact current Space membership source/incarnation.
   Membership removal/rejoin, community lease expiry, scope move, deletion and
   lifecycle uncertainty cannot preserve or resurrect an old grant. Resolve
   BOTH-source semantics with BE-010 c12–c16 before implementing this part.
4. Keep external authority reads outside database locks. Collect immutable
   scope/incarnation and monotonic revisions; evaluate under Role's transaction,
   revalidate owner revisions, and commit only using the agreed policy-CAS
   protocol. Unknown or changed authority is typed non-success. Persist complete
   actor/operation/body binding; changed-key reuse conflicts.
5. New target invites are immutable `(Space, chat, scope incarnation)` bindings.
   Generic redemption rejects targeted kind. Journal original authenticated
   account/profile and operation; pending approval cannot lose or overwrite a
   target. Reuse real ban/cap/guest/phone/CAPTCHA/questions/approval admission,
   never a mocked or bypassed equivalent. Pin missing owner implementations as
   dependencies and keep activation closed.
6. Existing members do not consume another use or create membership. Completed
   replay is historical admission evidence, not current ACL authority.
   Destination returns typed IDs only after current membership incarnation,
   live target/tree, visibility and guest checks. Unavailable target after join
   retains legitimate membership and does not claim navigation success.

## Validation

- Focused Linux `go test ./permissions` drives registry/default RED–GREEN.
- Real PostgreSQL owner tests cover manager denial, fake system-role names,
  allow/deny, grant revoke/demotion/remove-rejoin, policy-CAS races, frozen and
  retired Spaces, exact replay and changed binding with no unauthorized writes.
- Target invite tests cover pending/restart/approval, two distinct pending
  targets, current-policy changes, one-use concurrent replay, existing member,
  target move/delete, inviter authority loss and generic-handler downgrade.
- Production signed principal/caller tests reject forged actor/scope, wrong
  caller/RPC/hash and unavailable dependencies; no permissive fallback.
- Changed contract generation, buf lint/format/breaking and affected Go
  module/vet/lint checks precede independent review and exact-head CI.
- Run compiled/native tests only inside owned Linux Docker runners. A recorded
  testhelper ContainerIP adapter may replace Docker Desktop's slow mapped-port
  path; it changes no product code or assertions. Never claim shared deployment
  or acceptance from local tests.

## Progress

- [x] Atomic task claim and own WT19 lease at the immutable base.
- [x] Read accepted parent contracts and actual registry/store/admission seams.
- [x] Confirm live scopes: 144 report-target RPC is disjoint; 145 reserves no
  Role/Space grants; 150 has no live edits. Historical 44/66/69 are not writers.
- [x] Registry/default test meaningfully RED: Linux test reached assertions and
  failed on unknown `TEXT_CHAT_POST_AS_CHAT`, rather than compilation/setup.
- [x] Focused registry/default GREEN; broad permissions package requires the
  Docker socket for its existing PostgreSQL migration test (initial missing
  socket is an environment failure, not a product result).
- [x] Bounded Astra decision `SPACE-CHAT-AUTHORITY-v1` in task c2–c4 resolves
  typed source selection and final Role CAS; no second membership model.
- [x] Focused registry/default GREEN after actual unknown-name RED.
- [x] Existing-role data backfill PostgreSQL RED–GREEN, preserving same-name
  custom roles and explicit allow/deny overrides.
- [x] Shared typed membership binding rejects ambiguous sources, invented
  legacy generations, missing publication/episode identities and oversized
  database counters. Ten invalid cases reached assertion RED, then GREEN in
  Linux; strict canonical round trips pass.
- [x] Shared binding codec frozen as `5bc6afda85333d3ddf27feb5b44de5fd5f9edfc0`;
  shared Role protobuf/converter imported from producer183 `fd322d7` as own
  `6f2502632`. Existing receipt fingerprints are unchanged.
- [x] Local PostgreSQL persistence: five tests pass without skips (124.639s).
  Stale revision and local denial have no effects; old allow replay cannot
  undo revoke; scope and source bindings remain exact; two independent pools
  commit the same operation once; outbox failure rolls back the grant, receipt,
  invalidation and revision together. These tests do not prove remote authority.
- [x] Full shared memberadmission and Role permissions packages plus affected
  permissions/store vet pass in Linux. Initial broad registry run found two
  stale count/order expectations, corrected from 43 to the accepted 45 rights.
- [x] Publish `SPACE-CHAT-AUTHORITY-v1` and ready coherent prerequisites:
  `BE010-ROLE-QUALIFIED-AUTHORITY` (actual183),
  `BE010-SPACE-ADMISSION-AUTHORITY` (actual186), and
  `CHAT-SPACE-SCOPE-AUTHORITY` (actual176). Producers consume the published
  contract, not unfinished downstream runtime. No historical custody transfer.
- [ ] Wire production signed handlers, bounded remote double collection and
  final Role CAS against immutable qualified Role/Space/Chat implementations.

## Decisions

- Ordinary TDD with one author; ACL/concurrency/schema risk requires an
  independent final reviewer. No automatic strict/delegated TDD was requested.
- Reuse separate signed owner reads; Federation-only AuthoritySource endpoints
  are not widened to supply this feature.
- Shared membership binding is ORDINARY generation/publication identity (with
  an explicit persisted legacy arm), or COMMUNITY owner generation/member
  generation/episode UUID. BOTH requires an explicit source selector; no source
  means denial, unknown authority means unavailable. Grants never rebind to a
  new episode. Existing unproven Role tuples remain unresolved, not implicitly
  migrated to either currently live source.
- The accepted mutation ordering is Space/Chat collection S1/C1, local fenced
  Role snapshot R, remote S2/C2 comparison, then local fenced revision-CAS and
  one commit of the business outcome. No active writes precede final CAS, and
  no post-commit authority check can relabel a committed grant as denied.
  Use at most two attempts within a conservative monotonic five-second/lease
  budget. Remote removal after final observation has the accepted overlapping
  outcome; every new decision must reject a retired source.
- Reserve new bits without renumbering current permissions. An official bit
  alone never replaces the explicit membership-scoped grant.
- Reuse the existing Role migration15 full authority revision clock after exact
  coverage verification; migration20 extends its triggers to local grants and
  policies. Producer183 extends the same clock for qualified provenance/floors.
  Immutable historical operation reads do not bump it. Dedicated durable local
  outcomes and outbox are not proof of delivered Space audit/client events.

## Risks And Follow-Ups

- Current interactive admission and membership-episode authority are incomplete;
  their accepted contracts must be integrated or explicitly pinned before the
  targeted flow is enabled. This is not a reason to bypass current restrictions.
- The bounded contract is fixed by task c2–c4, but actual shared BE010 writers,
  qualified Role readers and Chat authority provider must still be integrated.
- Hosted CI/GitHub delivery remains subject to the centralized transport check;
  deployment and final acceptance remain separately authorized functions.
