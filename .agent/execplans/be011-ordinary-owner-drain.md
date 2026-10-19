# ExecPlan: ordinary Voice retirement owner

## Purpose

Allow the cold-cut executor to obtain a durable ordinary-call owner receipt only
after exhaustive retained native capture, trusted retired-realm qualification,
native terminal transitions and exact projection/account cleanup. A resource
receipt alone cannot mark ordinary calls drained.

## Context

TT `BE011-ORDINARY-OWNER-DRAIN`, developer Rudeus-160, consumers 191/145 and
integration owner 121. Base c72bf372; normal dependency merge e66bfe44 supplies
the retirementproof leaf. Sources: docs/PLAN.md A3; ordinary lifecycle in
docs/microservices/voice-service.md; docs/DATA_STORES.md voice_db;
docs/DATA_MODEL.md identity and retained evidence rules; accepted BE011 coverage
contract and 169 owner-retirement c4/c6. Maintenance authority remains mandatory,
distinct from serving authority and coverage ACTIVE.

## Scope

New ordinarycall/cutover_drain files, migration025, focused tests, and a private
caller-transaction extraction of PostgresStore.Mutate. Keep the existing public
Mutate transaction, complete identity locks, fences, membership index and both
outboxes unchanged. Do not edit 145 party_cut/main/snapshot, 191 issuer/leaf,
Space owner files or MM settlement. No flags, deployed capability or generic
retirement upgrade. Existing whole-realm/authority qualification remains open.

## Milestones

- [x] Capture and immutable journal prepare/replay, including historical members,
  admissions, effects, outbox and unknown issuance markers.
- [x] Trusted proof acceptance with retained resolution bytes and exact recheck.
- [ ] Native close/abort and qualified effect completion, exact projection cleanup,
  terminal receipt and crash/retry recovery.
- [ ] Actual Linux PostgreSQL/race negatives, source review, portable source/proof
  and explicit accepted integration handoff.

## Detailed steps

1. Add contract tests for atomic capture/close, immutable replay, changed incarnation,
   omitted historical evidence and absent authority. Observe a real failing oracle.
2. Use migration025 with immutable work/target/acceptance evidence and conservative
   DOWN refusal. Preserve all source rows and semantic facts.
3. Extract native mutation transaction mechanics without changing their order;
   prepare seals the pre-close native snapshot and elects closure in that transaction.
4. Resolve proof/work through TrustedVerifier; rederive every provider/unknown
   marker from retained sealed bytes on each read. Persist resolution bytes/hash.
5. Qualified completion may satisfy only old external media effects. Native
   operation/generation CAS, exact projection/fence cleanup, journal/roster/outbox
   and final receipt remain required. Never invoke a replacement realm by name.
   Consume BE011-REDIS-CAPTURE-PARTITION-v1 (268 artifact
   3236326bb96428a35bacf33d2a74fa1e56146c2fa47fa90023ac4748db245d3d):
   all raw capture and ownership partition seals precede any Redis mutation.
   Carry explicit owner work, legacy capture/work, partition and proof refs;
   execute serial predecessor/postimage steps with protected receipts. An ordinary
   Projection.Apply call without participation in that seam cannot qualify.
6. Verify exact current source in owned portless Linux Docker. Record pending
   cleanup as pending; no unsupported authority fallback or fixture activation.

## Validation

Real PG capture rollback/commit/retry, all retained evidence, changed expected
version/incarnation, writer interleave, canonical target mutation/omitted marker,
zero proof, copied proof/cut, cleanup failure, newer projection/fence preservation,
crashes around acceptance/cleanup/receipt and exact lost-ack identity. Check
ordinary native suites, race and vet; compare native event/outbox versions.

## Progress

- [x] Atomic task160e1 and migration025 reservation after actual TT scan.
- [x] Actual c72 base, own existing WT39 lease; e66 normal dependency merge.
- [x] Two direct CodeGraph requests bounded out; controlling Git source inspected.
- [x] First real PostgreSQL RED: caller transaction helper unavailable (42.873s
  including a test cleanup timeout; added deferred rollback before assertions).
- [x] Full ordinary race suite PASS 58.426s, 39 tests/0 skip/0 fail, including
  real PostgreSQL transaction and existing ownership/conversion/outbox cases.
  Subsequent vet PASS: original runner89045 terminal0, container7a52ce2213a3
  exited0; immutable c93eedc1 overlay over a clean full backend snapshot.
- [x] Capture/close and authoritative reader implemented with migration025;
  actual PG capture RED 4.965s -> GREEN 9.167s, followed by vet0.
- [x] Unsupported historical JSON discrimination RED 5.978s -> GREEN 5.357s;
  orphan/stale membership-index discrimination RED 28.349s -> GREEN 20.115s.
  Reader rederives all UNKNOWN markers; prepare refuses stale index evidence
  before native synchronization can overwrite it.
- [x] Final exact capture/migration PG-race PASS 9.325s (test8.27s), vet0;
  runner19464 terminal0. Overlay8878dbff260c858e875e11280cf8dc954b4ff75f435e83fe33fedaf9c26db847,
  all3928 backend file identities/SHA checked before execution, inventory
  2a6d8aa265c797b7c5b8fdda512a07363e9dfbc8f3519dacb1ae178f75a94183.
  Empty025 DOWN/UP succeeds; populated DOWN refuses and preserves exact work
  and target rows. Authority is explicitly a component fixture, not installed
  maintenance qualification. No Redis writes or final receipts implemented.
- [x] Resource-proof acceptance: actual stub RED7.061s -> PG-race GREEN9.582s
  (test8.53), vet0, original52952 terminal0. Exact3930-file preflight inventory
  f617e01c3e6581fda31704afee57b2185ac0cdc366e55cb9f5ae9a06c9c07a20,
  overlay65924450be0faf34f27f78e01e37c958ec7d18692cf656bfc0a56a34bb2051e3.
  Checks zero proof, wrong receipt, fresh observation denial before/after saved
  acceptance, retained exact resolution and lost-ACK replay, and final native
  incarnation change between external verification and the owner transaction.
  Two intermediate failing runs corrected test fixtures (SQL JSON/column CHECK,
  then invalid aggregate); neither is classified as a product defect or PASS.
  Work remains PREPARED; acceptance changes no native/Redis effects.

## Decisions

Retain all historical aggregate JSON and native outbox records, rather than only
the current membership index. Ordinary provider issuance has no complete grant
ledger; explicit unknown source markers must require protected historical
confinement. No source cleanup means an unavailable owner receipt, never DRAINED.

## Risks and follow-ups

Production maintenance authority and protected historical issuance/resource
composition belong to191 and remain unavailable. This task supplies the native
owner, not an operator permission, rollout, coverage issuer or acceptance.
