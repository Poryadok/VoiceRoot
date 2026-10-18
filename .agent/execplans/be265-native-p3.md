# BE-265: current-native P3 barrier and honest completion

## Outcome and authority

Repair the configured protected Voice P3 participant and current Space-media
admission/recovery paths. PostgreSQL owns native membership and immutable cleanup
targets; Redis remains a projection. A successful ejection is not credential
retirement. Current self-hosted LiveKit is EJECT_ONLY, so possibly disclosed or
historically unknown credentials keep FROZEN/PURGED pending without a receipt.
Other LIVE Spaces continue their ordinary admission behavior.

Accepted decisions: TT BE-265 c3/c4 native barrier/capture/replay, superseded on
provider expiry by c8-c10 BE265-P3-PROVIDER-COMPLETION-v2, and c13/c14
BE265-REDIS-GATE-INCARNATION-v1. Owning canon:
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
- [x] Install shared transaction-aware two-int Space advisory barrier, generation
  binding on all forward native writers and fixed durable grant authorization.
- [x] Capture immutable native and legacy exact targets under restrictive fence;
  drain outside SQL locks, including absent Redis, retain unresolved work.
- [x] Fence delayed Redis projections with a per-Space incarnation gate;
  missing gate installs RESTORING before fresh PostgreSQL-backed restoration.
- [x] Replace effect-success completion with explicit operation-bound provider
  proof predicate; prevent stale/expired requests and old unproven receipts
  from authorizing LIVE. Expose honest capability to future coordinator.
- [x] Real Linux PostgreSQL/Redis composition and barrier races; existing affected
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
then migrated draft GREEN real Linux PostgreSQL. Final3 immutable overlay35616f6a
full PG P3 PASS40.919s and native PASS144.335s: JSON73 test/subtest PASS, zero SKIP
or FAIL. This includes current-native/empty legacy/absent Redis composition,
transactional ordered LEFTs, fixed grant replay/freeze and six wait-ordered native
writer races. Voice build and affected vet PASS. Final4 overlayf4ebcc14 adds exact
generic Add target/owned-room checks; store realRedis short PASS0.309s, grpcsvc
short PASS, Voice build and store/grpcsvc vet PASS. Earlier full short5packages
PASS; P3/native source unchanged by the final store-only guard. Historical failed
marker/domain-error runs and invalid tar are excluded from success evidence.

The c11 Redis-loss concurrency request is resolved by Astra166 c13/c14. One
per-Space gate holds random E, RESTORING/READY, generation and LIVE/DENY. Install
RESTORING(E) before fresh PG read and CAS exactly E to READY. Dispatch reads E
before fresh native authorization, then checks E/generation/native tuple atomically.
Generic state writers are metadata-only; cleanup removes only exact owned indexes.
Real Redis gate race/negative rollback fixtures passed on the earlier overlay.
Same-E stale snapshot/failover is explicitly outside nonce detection: actual
recovery must fence writes/old endpoints and rotate gates before exposure. There
is no infrastructure qualification claim. Independent exact-head review remains.

BE012 native dependency a80f649cb supplies migration013 and transactional
spacemedia.BeginPartyRoomClose. Replace direct P3 CLOSING/ACTIVE->REVOKING writes
with its room-first sorted LEFT/counter/outbox helper in the caller transaction
(wired in current source; normal merge resolved and awaiting final commit).
147's exact helper test PASS9.75s; full combined source requires normal merge and
fresh affected proof, never cherry-pick only the helper. 145 withdrew proposed014
and uses015; BE265 actual014 is reserved. No deployment/activation follows.

Migration number and BE106 identity hunks require actual reservation reconciliation
with actor1 (message sent, no wait dependency). Historical receipts are unproven;
do not rewrite immutable bytes. Full provider qualification and actual deployment
remain OPEN even after internal checks. Native Windows tests prohibited; use
owned Linux Docker, no host listeners. No full-task/stand/provider acceptance PASS.

Graphify AST update was attempted in WT39. Its own Python45116 (verified command
and parent21484) ran approximately10minutes with no output; bounded work was
stopped only for this owned process, wrapper/session terminal1. No graph update
PASS is claimed and no other Graphify process was stopped. CodeGraph frozen index
was qualified by actual source; two bounded read-only calls were terminated after
no response. Final source hashes and runtime tests are the evidence authority.

## Independent review correction

208 found a concrete startup publication race on frozen fa1528a7: the P3 loop
started before optional Space composition replaced Effects interfaces. Recovery
publication now occurs after final wiring and uses a distinct Service/Effects
snapshot. Meaningful Linux RED on old shared publication called the replacement
drain instead of the published final drain. Corrected immutable overlay88169345
passes the concurrent mutation regression with `go test -race` (1.061s), P3 short
(0.021s), Voice build and P3 vet. No P3 request serialization was changed:208
withdrew that finding after confirming deterministic wrapper encoding.

This is a bounded successor to fa1528a7; prior73 actual PG tests and realRedis
writer checks remain applicable to unchanged admission/capture/drain code. Same208
must recheck the immutable successor before121 integrates it. Graphify was tried
after the correction, own Python50500/parent44348 verified via RTK27404 and start
time; bounded no-output attempt stopped, session terminal1. No AST PASS claimed.
