# ExecPlan: Space participant moderator mute (BE-273)

## Purpose

An authorized Space moderator can restrict another current member's audio in
one logical voice room. The target cannot bypass it by rejoining, restarting or
using screen-share audio. Clearing permits a later explicit user unmute and
never starts a microphone. This is the accepted implementation target, not
evidence of runtime completion, rollout or acceptance.

## Context

- Accepted decision: TTenrec `voice` task BE-273, comments c2-c4,
  `BE273-MODERATOR-MUTE-v1`, specialist Рудеус-34. The owner published the
  structured contract with those references. c1's product question is resolved.
- Product: [A2](../PLAN.md), [voice-chat](../features/voice-chat.md),
  [roles](../features/roles.md), [Voice contract](../microservices/voice-service.md#moderator-mute-accepted-target-be-273),
  [Role](../microservices/role-service.md), [Gateway](../microservices/api-gateway.md),
  [data ownership](../DATA_MODEL.md#space-moderator-mute-ownership-be-273-accepted-target),
  [stores](../DATA_STORES.md), [architecture](../ARCHITECTURE_REQUIREMENTS.md).
- Implementation seam: enabled BE255 durable Space admission, room-head/account
  fences, signed delegated-user listener, protected Role room-grant resolver and
  exact-incarnation revoker/reconciler in `src/backend/voice/`. Do not use the
  source-disabled R22.3 lifecycle as a second coordinator.
- BE-044 owns timeout authority and protected Space resolver/grant additions;
  coordinate its exact branch/head before modifying the same interfaces.
  BE-102/106 ordinary committed-membership journal is not this Space operation.
- Developer base for this contract stage: `698d4f064f032ea74abf3ede2017100b26c8ee69`.
  The existing self-only voice state and all-track `CanPublishAudio` mapping do
  not implement durable target moderation or an audio-only source contract.

## Scope

In: protected target RPC/REST, durable per-room/profile restriction and CAS,
source-specific grants, exact current-incarnation cleanup, recovery/replay,
authorized additive state and focused checks. Reuse existing authority and
state-delivery paths. A changed restriction temporarily interrupts all target
media; subsequent fresh admission can listen and use independently allowed
video. No target self-clear or force-unmute.

Out: DM/group/MatchSquad/GameSession/federation policy, broad full-roster/watchers,
moves, new auto-rejoin, R22.3/D2/D3 activation, a second ordinary outbox or
lifecycle coordinator, deployment and acceptance appointment. Strict SFU
deny-before-connect verification is separate scope; do not claim cryptographic
revocation from RemoveParticipant or original JWT expiry.

## Milestones

- [x] Accepted c2-c4 reflected in feature/service/data canon and this plan.
- [ ] Coherent additive proto, protected route and presence-aware validation
  pass compatibility checks and deny-before-effect tests.
- [ ] Durable versioned state/operation/effect and admission serialization pass
  real PostgreSQL concurrency/crash-recovery tests.
- [ ] Role source projection, grants and all audio-enabling controls enforce the
  restriction without granting unproven video/share sources.
- [ ] Exact-incarnation ejection and retired-bearer reconciliation pass failure,
  restart and pinned self-hosted real-media proof within 2 s p95 / 5 s max.
- [ ] Independent security/concurrency review and exact-head CI pass; PR merges
  with required approval. Runtime/deployment/acceptance evidence remains separate.

## Detailed Steps

1. Read current BE-044 exact head/protocol before runtime writes. Keep timeout
   and moderator-mute distinct; timeout denies even muted-listener admission.
   Inspect existing BE255 admission/revoker and request ledger with CodeGraph
   before code changes, then verify current source because the index may lag.
2. Write risk-first RED tests at the existing Space persistence/service seams.
   A two-instance barrier must prove that a join/reissue started before mute
   cannot finalize an unrestricted later admission; a late SDK removal cannot
   eject its replacement. Establish the fake/real configuration early.
3. Extend existing Voice/Role protobufs additively and regenerate all consumed
   Go/Dart trees through repository scripts. The mute boolean must have
   presence. Add the protected user-listener RPC and Gateway route using the
   existing principal, request-hash, Auth-epoch and canonical-path binding.
4. Add a new Voice-owned additive migration without rewriting shipped
   migrations. Persist logical room/profile flag/version, actor/request receipt,
   pending effect and exact captured admission tuple in the same room-head
   transaction. False state keeps its version; zero means never-created. The
   existing 24h replay key and longer-lived CAS have different responsibilities.
   One pending transition per target blocks competitors. No network transaction.
5. Implement pre-decision permission/membership/discovery checks, active-target
   first mute, offline current-member clear, no-op and replay. Changed input
   conflicts; exact completed replay requires current actor/discovery and never
   reapplies a superseded decision. No deny may emit events or change admission.
6. Extend the same protected Role grant projection with explicit versioned
   camera/share capabilities. Persist issued source vector/moderation version.
   Mic requires speak and no restriction; share audio additionally requires
   share; camera/share video use their separate permissions. Empty allowed vector
   forces canPublish=false; legacy/unknown capability cannot allow a source.
   Guard self-unmute, broadcast and equivalent floor/audio enabling as well.
7. Changed flag sets/leaves self mute true and queues immediate durable cleanup
   via BeginRoomLeave/revoker/CompleteRoomLeave and exact fence release. Unknown
   SDK outcome blocks target admission; never roll back the committed flag or
   remove by profile alone. Retry/recover using bounded independent contexts.
   Keep exact retired identities in the existing Space reconciler, including
   after active-state replacement and original token expiry. Recheck permanent
   deletion/leave/move facts before effects and preserve other targets' progress.
8. Persist one stable authorized state-change intent per changed moderation
   version, with existing delivery semantics. Do not disclose target/state to
   occupancy-only audiences. Redis/NATS failure cannot erase the restriction or
   manufacture success; success needs confirmed cleanup/projection receipt.
9. Run the matrix below; record exact source, pinned LiveKit version/config,
   test counts/skips, timings and limitations. Add client identity/state mapping
   checks if its consumer changes. Seek an independent security/concurrency
   reviewer against the exact diff; publish PR and carry CI/merge as an ant.

## Validation

These checks remain required; they have not run in this docs-only stage.

| Risk | Required proof |
|---|---|
| Principal/disclosure | Spoofed actor/raw headers; wrong RPC/hash/audience/session epoch; absent JWKS/Role/Space; wrong canonical IDs; member-list deny; disconnected authorized actor; Role deny/Owner resolution; self-target. Every pre-decision deny has zero effect. |
| State/privacy | Mute/clear/no-op; no self-unmute while restricted; explicit unmute after clear; self mute/deafen/leave; moderator replacement; leave/rejoin/empty/reopen/restart/Redis loss; separate room/profile; fresh room UUID; offline current-member clear; no force-unmute/floor grant. |
| Source grants | Decode mic/camera/share-video/share-audio permutations; malformed/legacy capability fails closed; empty vector disables publish; old patches/audio enabling obey restriction; timeout and current Role/Auth deny still win after clear. |
| Concurrency | Real PostgreSQL and two service instances with deterministic barriers: join/reissue vs CAS, opposite decisions, stale version/replay, leave/move/rejoin, replacement media/account fence and late RemoveParticipant. No duplicate change, resurrection or removal of replacement. |
| Recovery | SDK executed then timeout/NotFound/outage, cancellation, ambiguous commit, crash at each boundary, Redis loss, NATS failure. Same operation converges; changed request conflicts; other target remains usable. |
| Actual media | Pinned self-hosted SFU and two clients send/receive audio. Measure ejection from commit at 2 s p95 / 5 s max, muted fresh admission listens and independently allowed video works; mic/share audio and inconsistent audio-kind/video-source claims fail. Clear never starts mic; explicit authorized unmute restores actual audio. |
| Old bearers | Replay original and SFU-refreshed bearer after original 60 s, observe exact retired identity and re-eject within bound. No original-expiry GC, permanent revocation or zero-transient-audio claim. |

Run `make buf-ci` and required breaking/generation checks for proto changes;
targeted full Go tests, vet/lint and PostgreSQL concurrency/recovery checks for
Voice/Role/Gateway per [TESTING](../TESTING.md). A Windows-restricted Realtime
consumer, if actually changed, needs Linux execution; do not run its restricted
suite locally. Testcontainers skips, successful handshake or SDK mocks alone do
not prove media delivery/enforcement. The final enabling change needs the
feature/compose E2E and measured pinned SFU behavior before claiming completion.

## Progress

- [x] Atomically claimed BE-273; accepted contract published with c2-c4 evidence.
- [x] Canon describes target semantics and corrects refreshed-bearer expiry
  limitation without changing runtime status.
- [x] BE-044 owner contacted; shared runtime seam remains under its active work.
- [ ] Independent contract review and documentation PR/CI integration.
- [ ] Runtime implementation and all required proof above.

## Decisions

- Accepted c2-c4 are the bounded product decision; do not invent replacement
  self-clear, expiry, hierarchy or permanent all-media denial.
- Existing BE255 Space admission owns serialization and exact cleanup; existing
  Role projection owns rights, BE044 owns timeout. No competing authorities.
- Canon first is necessary because the old inventory's JWT-expiry assertion and
  current all-track publish mapping are insufficient for this audio contract.
- The docs-only stage has no behavior requiring a new mirrored test. Validate
  diff/links/decision coverage, then independent review; runtime needs risk-first
  TDD and real media proof.

## Risks And Follow-Ups

- Shared BE044 resolver/grant edits must be reconciled at an exact safe head;
  separate worktrees do not make incompatible protocols safe to combine.
- Self-hosted refreshed bearers require retained-identity recovery. SLA failure
  is failed validation, not authorization to weaken the contract. Do not infer a
  safe tombstone GC window from original token TTL.
- Full roster/state reads are not activated by this task's documentation. Additive
  state delivery must preserve current explicit audience authority.
- Deployment, DEV/STG proof and final appointed acceptance remain unclaimed.
