# ExecPlan: separate Space-local reports

## Purpose

A member can explicitly complain about a readable Space message to that Space's
moderators. A currently authorized local moderator can inspect only permitted
local complaints, assign a reviewer and record a decision. Platform complaints,
reporter identity and platform sanctions remain outside this local surface.

## Context

- Product: [reports.md](../features/reports.md#локальные-жалобы-space).
- Owning API/data/authority: [Moderation](../microservices/moderation-service.md#space-local-report-jurisdiction-accepted-target),
  [Role](../microservices/role-service.md#moderation-local-report-decision-contract-accepted-target),
  [Gateway](../microservices/api-gateway.md), [data stores](../DATA_STORES.md).
- TT `voice/BE-066` c3 records the explicit user choice for isolated local
  reports. c4/c5 contain the earlier bounded security analysis and recommended
  product package. Later user delegation directs autonomous ordinary choices;
  this plan selects that narrow A-E package, not fictitious separate human
  approval of its details.
- Acquired source: develop `ba14ae774b2ea7858d7165e5aade9816aec34e90`.
  Existing Moderation RPCs use global `Report` and raw legacy metadata. They are
  not suitable local authority. New API/table/handlers must be isolated.
- Chat owns membership/history intervals; Messaging owns exact message
  visibility, deletion, timestamp and canonical Chat binding. Space owns live
  Space/node ownership. Role owns `MODERATION_MANAGE_REPORTS`. No service may
  read another service's database.

## Scope

Local message reports in main feed/threads of group/channel Space chats;
explicit local-vs-platform submission; private reporter; current content ACL;
typed same-target references; versioned assignment/terminal review; positive
delete/move minimization and lifecycle cleanup. No local profile/voice/Story/DM
targets, copied snapshots, global staff bypass, dual submission, integrated
sanction or local appeal system. Existing global intake semantics are unchanged.

## Milestones

- [x] Record the bounded product choice in owning feature/service/data docs.
- [ ] Resolve exact protected target-owner protocol from current implementations.
- [ ] Add local DTO/RPC/generated contracts, independent persistence and guarded
  rollback. One owner reserves the next actual Moderation migration number.
- [ ] Execute meaningful negative RED, then implement protected ingress,
  authoritative target/Role adapters, per-row current ACL and workflow CAS.
- [ ] Wire Gateway and client local-vs-platform path and cleanup workers.
- [ ] Prove isolated Linux/PG/TLS negative matrix, independent exact-head review,
  accepted publication transfer to121 and assembled hosted CI.
- [ ] Authorized exact-version environment checks and appointed acceptance.

## Detailed Steps

1. Inspect `Chat.CheckMessageReadEntitlement`, viewer-scoped
   `Messaging.GetMessage`, Chat/Space protected ownership surfaces and their
   actual per-caller routing. Never adapt the no-viewer GetMessage as a viewer
   read or forward raw identity metadata. Determine whether one new protected
   owner RPC is required and bind all viewer/Space/Chat/message fields.
2. Make owner result semantics explicit: current allow, known forbidden/missing,
   positive deletion/move and dependency uncertainty are distinct. A current
   reader denial hides data but does not prove deletion. Specify finite list
   authorization before pagination without a caller-controlled tenant filter.
3. Add new local API and table independently of global `reports`. Preserve
   reporter provenance privately and define account/Space cleanup before
   activation. Prove forward/guarded DOWN with real retained local rows.
4. Author negative tests first for spoofed caller, foreign jurisdiction, revoked
   membership, unreadable message and deny/unavailable owner/Role. Implement
   narrow signed ingress and Role caller without relaxing other listeners.
5. Implement atomic workflow transitions and exact terminal replay. Competing
   versions cannot overwrite a decision. Prove authorization failures have zero
   local effects and never call sanction/threshold/event hooks.
6. Add client choice, submission receipt, local queue/detail/assign/resolve UX
   with loading/denied/unavailable/conflict states. Never render private reporter
   or claim an enforcement action occurred solely from Resolve.
7. Add positive delete/move and P3/account cleanup before enabling the route.
   Unknown owner responses preserve evidence and hide content. Rollback of
   cleanup must leave all row/work state unchanged.
8. Freeze immutable source/test artifacts for independent review. Sole publisher
   Рудеус-121 must explicitly ACCEPT delivery/assembled CI custody. Local proof
   is not deployment, hosted CI, privacy deadline proof or acceptance.

## Validation

- Jurisdiction matrix: local Owner/moderator A cannot list/get/assign/resolve
  platform reports or local B, even when IDs/targets are known. A platform Space
  complaint is not exposed to that Space's owner. Global staff has no implicit
  local bypass. UUID version is not authority.
- Identity matrix: raw profile/internal metadata, bad issuer/audience/RPC/body,
  replay, expired token and unavailable JWKS/epoch/Role fail before data/effects.
  Another valid service cannot use the new Moderation Role capability.
- Content matrix: message main-feed/thread visibility, group/channel Space
  ownership, membership history, ghost/FOR_ME/block restrictions, sender change,
  leave/ban/revoke, unknown/deleted/moved target. Reader denial differs from
  positive deletion. Lists never leak unauthorized summaries/cursors/counts.
- Workflow matrix: authorized create/assign/reassign/clear/resolve; unauthorized
  assignee; current revoke before each new request; competing expected versions;
  exact terminal retry and divergent terminal conflict; no-op/rollback has no
  version/event/sanction change. Assignee is not an exclusive decision lock.
- Privacy/lifecycle: no reporter fields in serialized local outputs/events,
  typed same-target evidence only, no copied message body, bound description/note,
  positive delete/move scrub both open and closed content, no cleanup on timeout,
  freeze deny, account/Space cleanup and restore cannot expose old content.
- Real Linux PostgreSQL migration, concurrency and rollback cases; affected Go
  package race/vet/lint; proto format/lint/codegen/contract compatibility;
  Gateway API and Flutter critical-path tests; required exact-head hosted CI.
  Shared DEV/STG checks require actual TT resource reconciliation and claim;
  appointed independent acceptance remains separate.

## Progress

- [x] Read parent decisions, current queue and prior exact-head review custody;
  atomically claim BE-066 as Рудеус-320 before writes.
- [x] Acquire own Treehouse WT44, lease `6d5887b144f73da883ce032170ba953c`,
  branch `codex/be066-space-local-reports` at actual develop base above.
- [x] Direct CodeGraph first; both bounded calls stalled and were terminated.
  No graph freshness proof; immutable/current source reads govern.
- [x] Record bounded protected owner-chain question in TT BE-066 c6; queen
  acknowledged its specialist queue. Consumer remains the real implementation
  owner320; no owner API or runtime capability is invented while unresolved.
- [ ] Resolve owner protocol and continue implementation; no runtime PASS yet.

## Decisions

Select earlier c5 A-E defaults under explicit autonomous delegation. This keeps
v1 within the requested separate local jurisdiction without adding privileged
content disclosure, reporter identity or cross-platform enforcement. Sources and
choices are canonicalized above before behavior code.

Request-time owner/Role authorization is explicit. Each request reads current
owners and performs a local versioned decision; no distributed immediate-revoke
linearizability is claimed. A later in-flight revocation is not solved by JWT
TTL. Stronger cross-service permits would need their own justified contract.

## Risks And Follow-Ups

The current source has no complete protected local-target resolver chain.
Existing viewer reads and Chat entitlement methods must not be broadened by
unsigned metadata or synthetic Space IDs. Exact owner protocol is a bounded
cross-service/security question before code. Reporter/account lifecycle and
list visibility require implementation as part of the same enabled vertical;
an isolated permission check or disabled foundation does not complete BE-066.
No source runtime, migration execution, deployment or acceptance is claimed by
this contract stage.
