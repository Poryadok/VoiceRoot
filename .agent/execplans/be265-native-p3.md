# BE-265: current-native P3 barrier and honest completion

## Outcome and authority

Repair the configured protected Voice P3 participant and current Space-media
admission/recovery paths. PostgreSQL owns native membership and immutable cleanup
targets; Redis remains a projection. A successful ejection is not credential
retirement. Current self-hosted LiveKit is EJECT_ONLY, so possibly disclosed or
historically unknown credentials keep FROZEN/PURGED pending without a receipt.
Other LIVE Spaces continue their ordinary admission behavior.

Accepted decisions: TT BE-265 c3/c4 native barrier/capture/replay, superseded on
provider expiry by c8-c10 BE265-P3-PROVIDER-COMPLETION-v2. Owning canon:
docs/microservices/voice-service.md P3 and Space media, and space-service.md P3
activation. No provider verifier, Cloud migration, public deletion coordinator,
deployment or acceptance authority is included.

Actor Рудеус-160, developer, gpt-6.1-sol medium, focused current source and TT.
Base c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c; branch
codex/be265-native-p3-barrier; own WT39 lease4dbccbceaeb6be715b2ce6dd4a553112.
Resource protocol acknowledged: no DEV/STAGING claim or listener/firewall change.

## Milestones and validation

- [x] Atomically claim BE-265; reconcile accepted provider contradiction.
- [x] Add regression for successful eject + refreshed credential: no receipt,
  same captured work on retry, no LIVE after pending FROZEN.
- [ ] Install shared transaction-aware two-int Space advisory barrier, generation
  binding on all forward native writers and fixed durable grant authorization.
- [ ] Capture immutable native and legacy exact targets under restrictive fence;
  drain outside SQL locks, including absent Redis, retain unresolved work.
- [ ] Fence delayed Redis projections with monotonic lifecycle watermarks;
  missing watermark rejects projection until PostgreSQL-backed restoration.
- [ ] Replace effect-success completion with explicit operation-bound provider
  proof predicate; prevent stale/expired requests and old unproven receipts
  from authorizing LIVE. Expose honest capability to future coordinator.
- [ ] Real Linux PostgreSQL/Redis composition and barrier races; existing affected
  package tests/build. Independent concurrency review remains separate.
- [ ] Update owning canon, collect immutable bundle/proof, accepted handoff121.

## Invariants

Lock order Space(0x53504345,hashtext canonical UUID) -> room/head -> sorted
account/profile identity keys -> membership/fence/outbox. Fresh READ COMMITTED
fence query after acquisition. No remote IO while SQL locks are held. Initial
generation0 requires the migrated source schema; existing historical rows remain
unknown for credential coverage. Every forward CAS checks saved lifecycle
generation and LIVE. Cleanup is exact and permitted while restrictive.

Restrictive reservation and sealed exact targets commit together; retry uses
those targets, never today's arbitrary Space membership. No delayed freeze room
deletion with stable SFU names. EJECT_ONLY/UNKNOWN never produces full receipts;
no original60s/quiet period/empty roster substitutes for future rejection.
Provider proof must bind operation/generation/request/manifest/coverage and all
issuer/refresh/handshake paths. Current production has no such proof provider.

## Risks / continuation

Current developer proof: immutable c08+regression RED (old callback minted receipt)
then migrated draft GREEN real Linux PostgreSQL. Current-native/empty legacy/
absent Redis composition and fixed grant replay/freeze checks PASS. Six actual PG
wait-ordered forward-writer races PASS (claim/prepare/fence/commit/projection/repair).
Affected store/grpc/livekit short checks PASS. Full native PG run exposed two old
test wait markers expecting the inner room lock; the second writer now correctly
waits on the outer Space barrier. Updated markers plus cleanup rollback are under
targeted rerun; do not call the earlier full run PASS.

Named unresolved concurrency risk for Astra166: PG authorization followed by
SetSpaceLifecycle may bootstrap a missing LIVE0 Redis watermark from a paused
pre-freeze authorization after Redis loss. Current draft therefore is NOT ready
for integration. Proposed nonce-before-PG protocol is in TT continuation; no
strong completion is claimed. Continue independent BE012 dependency integration
and exact cleanup/tests while bounded review is scheduled by queen.

BE012 native dependency a80f649cb supplies migration013 and transactional
spacemedia.BeginPartyRoomClose. Replace direct P3 CLOSING/ACTIVE->REVOKING writes
with its room-first sorted LEFT/counter/outbox helper in the caller transaction.
147's exact helper test PASS9.75s; full combined source requires normal merge and
fresh affected proof, never cherry-pick only the helper. 145 withdrew proposed014
and uses015; BE265 actual014 is reserved. No deployment/activation follows.

Migration number and BE106 identity hunks require actual reservation reconciliation
with actor1 (message sent, no wait dependency). Historical receipts are unproven;
do not rewrite immutable bytes. Full provider qualification and actual deployment
remain OPEN even after internal checks. Native Windows tests prohibited; use
owned Linux Docker, no host listeners. No implementation or runtime PASS yet.
