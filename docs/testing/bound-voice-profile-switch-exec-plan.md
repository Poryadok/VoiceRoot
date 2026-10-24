# ExecPlan: bound voice profile switching

## Purpose

Keep an A-owned voice membership through A → B → C. Explicit confirmed switch
must stop old local media and prove exact server cleanup before admitting the
frozen destination. Never reuse A's revoked Auth bearer or make B an ambient A.

## Context

The accepted BOUND-VOICE-v1 decision is BE-256 comments c2/c3 in project voice
TT. Canon is [multi-profile.md](../features/multi-profile.md#bound-control-после-переключения-accepted-target),
[Voice](../microservices/voice-service.md),
[Auth](../microservices/auth-service.md),
[architecture requirements](../ARCHITECTURE_REQUIREMENTS.md) and
[testing](../TESTING.md). Current `CallController` preserves passive voice but
its pending retry closure does not capture the dialog's auth generation;
`hangUp` calls EndCall and ignores the result. Neither proves bound cleanup.
BE-106 owns the ordinary journal/coordinator and BE-102 its committed facts;
BE255 owns Space media admission. These are shared consumers, not duplicate
authority to reimplement in the client.

## Scope

In: ordinary DM/group and Space self control, binding issuance/recovery,
allowlisted trusted delegation, frozen destination, reliable self leave and
real media proof. Out: MatchSquad, managed GAME_SESSION, federation, generic
Auth identity substitution and deployment/acceptance authority.

## Milestones

- [x] Canonicalize accepted binding and failure ordering.
- [x] Guard existing pending dialog destinations with the Auth request identity.
- [ ] Persist hash-only binding in the owning ordinary/Space lifecycle and expose
  committed-A issue/recovery, exact allowlisted controls and terminal readback.
- [ ] Wire serialized client cleanup and all ordinary/Space pending entry intents.
- [ ] Prove negative authority, races and actual media on an authorized immutable build.

## Detailed Steps

1. Capture `AuthController.gatewayRequestIdentity` when an existing DM/Space
   conflict intent is created. Before any destructive confirmation and after
   awaited cleanup compare that same generation/account/profile. Capture no bearer.
   Test B dialog → C and B → C → B, logout, cancel, and profile switch during cleanup.
2. Consume BE-106 room operation, member generation/fence operation, account epoch
   and SDK cleanup receipt. Voice generates >=256 random bits, stores digest only,
   and never mints A binding from B account equality. Space uses its existing
   admission operation/generation and retry request. No second journal/fence table.
3. Add additive proto/API inputs only to self-control/reissue/status/leave
   allowlist. Preserve current B/C principal in Gateway signed delegation and
   recheck fresh epoch, saved account and exact generation in Voice. Reissue
   checks A's current ownership/access/Role rights; self cleanup remains possible
   after access loss. End-all and new admission never accept a binding.
4. Preserve local stopped/LEAVING/UNKNOWN cleanup through failures and dismissals.
   Retry/read exact durable receipt after lost acknowledgement; only authoritative
   cleanup plus local teardown opens destination admission with a fresh bearer.
   Group self leave preserves other peers; last group member follows BE-106 close.
5. Obtain independent auth/concurrency review, exact-head required hosted CI,
   then normal merge. Retain actual runtime and tester-appointment gates.

## Validation

- Focused Flutter tests: passive A preservation; stale dialog before teardown;
  switch/logout during cleanup; same-identity refresh; newer intent and double confirm.
- Service negatives: binding alone/bearer alone/revoked A; foreign account;
  wrong room/action/generation; stale epoch/floor; forged legacy headers; unavailable
  ownership/DB/floor. No grants, facts or fence mutation on rejection.
- Deterministic barriers: leave/reissue, remote end/eject, late old receipt after
  rejoin, lost response, SDK applied then timeout, DB/Redis outage and restart.
  Terminal cleanup may replay; old binding cannot touch new incarnation.
- Real Gateway/Auth and pinned self-hosted LiveKit with two media clients for
  DM/group/Space: record immutable SHA/version, A identity and received audio/video
  counters across A → B → C; confirmed old disconnect; peer continuation on self
  leave; cleanup/fence receipts before new target. Record stale-SFU-token limits
  from BE-106 honestly. Recording adapters and opt-in skips are not live proof.

## Progress

- Accepted decision read; author-owned WT33 at base `698d4f0`.
- Canonical contract and this plan prepared before production edits.
- Eleven new negative cases reproduced the original extra EndCall/admission or
  local teardown before production edits. Initial missing dependency resolution
  and sqlite DLL failures were setup failures, not RED evidence; normal pub get
  and the repository verified-cache sqlite prefetch resolved them.
- Guard plus existing passive/call-controller tests passed 43 cases, zero skips;
  targeted Flutter analyze reported no issues. Same-identity refresh coverage is
  a separate positive regression, with final result recorded in TT.

## Decisions

- Existing Auth request generation is the frozen intent identity; refresh may
  rotate credentials without retargeting the intent. B → C → B is a new generation.
- A stale confirmation is discarded before calling hangUp; it must not stop A.
- Client guard is one safety prerequisite, not completion of bound controls.

## Risks And Follow-Ups

BE-106 journal implementation is concurrently owned by its author; final bound
adapter needs its committed API and actual PostgreSQL proof. Current EndCall is
not accepted self group cleanup; do not enable new group confirmation paths until
the exact binding cleanup adapter is wired. Docker/PostgreSQL and GitHub transport
have shared blocker tasks; reuse their evidence rather than repeat probes.
