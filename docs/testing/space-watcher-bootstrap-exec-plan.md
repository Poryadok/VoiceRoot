# ExecPlan: authoritative Space watcher bootstrap

## Purpose

After a new authenticated WebSocket hello, a viewer receives a complete current
Space tree and the permitted full or occupancy room projection. Revoked or
changed disclosure cannot be resurrected by a delayed bootstrap or old queued
event. BE-110 owns bootstrap/disclosure; BE-103 consumes the roster wire/state
contract and BE-167 consumes scope invalidation.

## Context

- Canon: [Realtime watcher contract](../microservices/realtime-service.md#space-watcher-bootstrap-and-delivery-authorization-accepted-target),
  [Voice](../microservices/voice-service.md), [Space](../microservices/space-service.md),
  [Role](../microservices/role-service.md), [Spaces](../features/spaces.md).
- Technical decision: TT project `voice`, BE-110 comments c2/c3 by the independent
  Astra specialist; owner64 adopts these requirements without expanding audience.
- Source checkpoint: `c850c2e4d56b2130cf739dbcddde8faf6d72a663`.
  `pkg/authoritysource` and Space/Role `internal/store/authority_source.go` now
  provide complete owner facts with migration/catalog and durable revision
  checks. Their protected RPC/server requires `service:federation`; it is not a
  Voice or Realtime watcher endpoint.
- `space.proto` still has live `ListMySpaces` and `ListSpaceTree`; neither is a
  complete viewer discovery snapshot. Role media grants have no member-list
  disclosure decision. Voice/Realtime have no roster epoch/projection bootstrap
  or exact-frame fresh disclosure authorization API at this checkpoint.
- No Windows generated test executables or host listeners. Use owned isolated
  Linux containers without host ports; shared DEV/STG requires its own TT claim.

## Scope

Implement protected sources, complete viewer discovery, Voice projection and
Realtime bootstrap/dequeue together as a consumed vertical. Reuse proven owner
store transaction/catalog helpers without widening Federation transport rights.
No raw Role policy-detail broadcast, inferred recipient audience, global message
replay, deployment, acceptance or stronger post-revoke drain guarantee.

## Milestones

- [x] Adopt the finite authority/revocation decision in owning service canon.
- [x] Reconcile current source APIs and named consumers against develop.
- [ ] Purpose-bound source API and complete viewer discovery with executed
      negative/mutation coverage.
- [ ] Voice projection/source-vector snapshot and exact-frame authorization,
      consumed by Realtime hello/dequeue and the BE-103 producer.
- [ ] BE-167 typed whole-scope invalidation including pending/idle watchers.
- [ ] Independent security/concurrency review, exact-source Linux proof and
      accepted publisher handoff with immutable artifact.

## Detailed Steps

1. Add narrowly protected watcher-purpose source RPCs, with viewer/session and
   exact canonical scope in deterministic request bindings. Space owns complete
   materialized viewer discovery plus tree/room facts; Role supplies canonical
   member-list disclosure facts and source revision. Keep current Federation
   service caller checks unchanged. Decide generated messages and registration
   alongside actual clients, not as an unconsumed wire-only contribution.
2. Prove complete durable revision coverage before using an existing epoch:
   ordinary and alternate membership mutations, remove/rejoin ABA, ownership,
   deletion freeze, roles/overrides, tree/category/room edits and recreation.
   Source facts and revision share one transaction. Viewer discovery must detect
   an added/removed eligible Space as well as edits inside a known Space; a
   vector over only previously discovered Spaces is insufficient.
3. Implement the Voice common-cut reader: collect all complete owner facts,
   then reread every relevant revision. Require an unchanged vector within the
   shared deadline; include Voice snapshot/version/incarnation. Verify current
   delegated session/account/profile without ambient metadata fallback.
4. Persist Voice roster state, independent full/occupancy counters and immutable
   event identity atomically with its outbox. Build complete producer-owned
   explicit audiences from the cut. Bind delayed delivery authorization to the
   exact original event/hash/profile/projection/vector; never refresh recipients
   or mutate event bytes when source authority changes.
5. In Realtime install pending connection generation before buffering; consume
   complete signed owner snapshot pages with current disclosure on every page.
   Explicit completion plus current-authority revalidation must precede atomic
   generation compare-and-install. Cancel all pending work on profile/session
   change, disconnect or scope invalidation, including before first room arrives.
6. At the actual serialized socket dequeue call Voice fresh authorization;
   afterward recheck generation and session-epoch guard. Failure drops the frame
   and exposes snapshot recovery. Apply projection versions contiguously from
   the snapshot; do not require a client subscribe for explicit recipients.
7. Wire typed Space/Role invalidations to whole-scope reconciliation. Coordinate
   BE-103 wire/producer and BE-167 consumer symbols via actual immutable heads;
   optional hints and the active media index cannot narrow completeness.

## Validation

Use test-first Go integration cases and in-process protected transport fixtures
inside owned Linux runners. Tests must actually execute without skips; failures
from unavailable infrastructure are not behavior RED. New proto contracts need
buf lint/format/breaking and Go/Dart regeneration consistency. Run affected
Space/Role/Voice/Realtime packages, race checks for pending/dequeue state, and
real PostgreSQL migration/revision tests, then independent security review.

Required acceptance cases:

- Forged/duplicate metadata, wrong workload/RPC/audience/hash, replay, stale
  session, foreign profile, ordinary listener and mismatched canonical room deny.
- Missing membership, frozen/retired Space, unavailable Role or absent grant
  yields denial or exact occupancy, never participant identity/state.
- Mutation between collections, ABA, alternate writes, room recreation,
  nonpositive revision and restore cannot produce an accepted mixed cut.
- Page failure, cycle, duplicate, wrong viewer/filter/session, expiry,
  truncation, overflow and cancellation never partially activate; empty complete
  results work. Revoke between pages denies the next disclosed page.
- Pause before install, then revoke/disconnect/profile switch; late success
  cannot recreate the generation, including an initially empty pending map.
- Snapshot V, buffered V+1/V+2 reordering, UUID duplicates and full-only mute
  verify contiguous recovery without advancing occupancy or leaking fields.
- Queue old full data; revoke or change to occupancy with notification withheld;
  dequeue rejects. Repeat with reconnect/new tab and immutable outbox retry.
- A locally observed revoke after authorization still cancels write. Tests claim
  overlapping-read semantics, not an undocumented post-return drain guarantee.
- Idle room watchers invalidate without active media; hints cannot omit peers.
  Dependency/partial audience failure publishes no partial audience. Retries
  retain exact event identity/bytes; occupancy/control payloads expose no raw
  role/participant or internal recipient/source-proof data.

## Progress

Owner decision adoption and current-source readiness are complete. No source
API, runtime activation, RED/GREEN, review, deployment or acceptance is claimed
by this documentation checkpoint. Current task facts/resources stay in TT.

## Decisions

- Reuse owner transaction/catalog reading patterns, not Federation caller rights.
- Local maps are routing accelerators. Fresh Voice-owned dequeue authorization
  is the correctness path even if invalidation delivery is lost.
- Implement a coherent caller/source/consumer slice; an isolated index or
  disabled RPC foundation does not satisfy bootstrap.

## Risks And Follow-Ups

The materialized complete viewer discovery and purpose-specific protected
disclosure APIs are actual implementation prerequisites, not missing human
permission. A per-known-Space revision vector cannot detect a newly eligible
Space. Source restore/revision integrity and deadline-bound ownership checks
need real integration evidence before activation. BE-103/BE-167 may proceed only
against concrete interfaces; their preparation comments are not runtime proof.
