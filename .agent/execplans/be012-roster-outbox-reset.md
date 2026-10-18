# ExecPlan: BE-012 native roster events and whole-party reset

## Purpose

A true Voice party membership change must atomically advance its public room's
roster version and save an immutable event. Matchmaking consumes that event once
and cancels the complete affected search party, including pending proposals.

## Context

Sources of behavior are `docs/todo/backend.md` (A3 MM/Voice accepted contract),
`docs/features/matchmaking.md`, `docs/microservices/voice-service.md`, and accepted
TT BE-011 decision `BE011-SOURCE-SEAM-v1` (comments c2-c3). Implementation base is
local develop `d4abb829622e9bd91b26842b61522e6840840867`.

Ordinary calls already own native membership, distinct `Room.RosterVersion`, and
compatibility participant events in `internal/ordinarycall`. Space's active
source is `voice_space_media_room_heads` plus `voice_space_media_admissions`,
owned by `internal/spacemedia`; R22 `roomlifecycle` is not its authority.
Space eligibility requires OPEN exact head, COMMITTED admission, ACTIVE
participant and non-null `projection_applied_at`. The first JOINED commit is
`MarkProjectionApplied`, not `MarkCommitted`. Space generation remains opaque
text; public room incarnation uses creator operation plus native head witness.

## Scope

In: native source counters, transactional immutable roster-event outbox, exact
owner eligibility-loss normalization, protobuf event/consumer, MM durable inbox
and whole-party reset with queue/cancellation recovery, meaningful Linux tests.
Reader/RPC and atomic MM enqueue are owned by Рудеус-145 / BE-011; reserve his
new `ordinarycall/party_cut.go` and `internal/partysnapshot` files. Protected
snapshot revalidation consumers must be coordinated with that author.
Out: invented reconnect producers, operator switches claiming authority coverage,
public activation, unrelated P3 changes, accepting/releasing a stand without TT
claims. The protected provider coordinator's async abandonment integration is in
scope under the later accepted decision below.

## Milestones

- [x] Native eligibility changes and exact loss CAS have rollback/replay proof.
- [x] Ordinary and Space publish the accepted A3 protobuf from durable bytes,
      retaining compatibility delivery with distinct event UUIDs.
- [x] MM inbox detects duplicates and conflicting identities/room versions;
      atomic reset covers searching, pending_accept and SOLO JOINED.
- [ ] Recovery removes stale queue rows and reliably emits one cancellation per
      session; proposal/squad revalidation uses the BE-011 protected snapshot.
- [ ] Affected Linux checks, independent review and immutable handoff to 121.

## Detailed Steps

1. Inventory every native writer and recovery path. Space head lock precedes
   signed account/profile identity locks; preserve Space barrier outer order.
2. Add real PostgreSQL tests for tentative PREPARED/COMMITTED state, committed
   projection JOINED, replay/no-op, REVOKING LEFT, transaction rollback/outbox
   failure and concurrent room mutations. Require consecutive versions and one
   frozen event per actual delta, ordered LEFT then JOINED by UUID bytes.
3. Add a separate A3 durable outbox shared by native writers. Preserve scalar
   ordinary compatibility encoder and its existing `(room,roster_version)`
   uniqueness. A3 has distinct stable event IDs, immutable occurred_at/payload,
   lease-fenced acknowledgement and Nats-Msg-Id event ID.
4. Expose writer-native exact-incarnation/member loss methods, adapted by 145;
   source changes return a typed sentinel, never remove a replacement admission.
5. Extend VoiceStreamEvent additively and regenerate owning committed bindings.
   Validate stream subject/ACL/consumer wiring from the existing NATS canon.
6. Add MM inbox/reset transaction and real PG duplicate/out-of-order/conflict,
   party-wide pending rollback and concurrent search tests. Coordinate captured
   party source fields with 145 before changing common schema.
7. Use owned Linux Docker Go1.26.5/PG16, no new Windows test.exe/listeners. Record
   exact source and commands. Run graphify update once after code changes;
   retain any pending graph result honestly.

## Validation

- Native tests prove membership/counter/outbox all commit or all roll back;
  DM two-accept and multi-End emit per-member versions, no-op emits none.
- Space producer tests prove projection eligibility and exact-generation loss,
  including recovery paths and concurrent serialization.
- MM tests prove exact inbox replay, contract mismatch fail-closed, absent-member
  JOINED reset by room, SOLO reset by profile, unrelated pending parties recover.
- Affected package tests, service short suites, vet/lint, protobuf checks and
  independent source review are required before delivery. No local PASS denotes
  hosted CI, DEV/STAGING or product acceptance.

## Progress

- [x] Claimed BE-012 as Рудеус-147; consulted active BE-011 consumer/author 145.
- [x] Read actual native writers and accepted source contract.
- [x] Initial checkout scarcity recorded in TT c2; useful BE-097 privacy review
      completed CLEAR c6 without duplicate runtime while waiting.
- [x] Acquired WT38 lease `34556080af0760fd6cbd9239158de19c`, holder
      `rudeus-147-be012`, branch `codex/be012-roster-outbox`, fresh actual develop.
- [x] Native RED/GREEN: Space first eligible JOINED, ordinary dual-topic events,
      exact-loss CAS, injected-outbox rollback, caller-owned room close and
      creator departure with survivor. Native source review artifact: 9deee676f.
- [x] MM three initial PG RED tests became GREEN at 2a0dc4581 against explicitly
      pinned external019. Two additional rollback/concurrent-fork cases PASS
      at backend1aa963ed7; durable NATS consumer proves ACK-after-commit and NAK
      after database failure. Publisher keeps frozen bytes/event identity.
- [x] Normal-merged 145's immutable 5fc1a35b5e6aa0b8aa638cde23ee5aaed3c21e6c
      into 52f59be69ce5d35b64c4860df550c77fd1ed548c: actual019 and SourceParty
      are now present, rather than only a test dependency artifact.
- [x] Downgrade retention RED at ec49e91bd: both original DOWN scripts removed
      delivered evidence. a2e336585 adds transaction/table-locked refusal; GREEN
      results are recorded separately when complete.
- [ ] Implement the common protected candidate gate and async abandonment saga.
- [x] Normal-merged 145's immutable candidate/validator API1fc23b7a5f8765a620d91e04dada58d260cbd46e.
      New squad/party_source_gate.go consumes exact private ValidatedAttempt;
      every call captures fresh whole-party authority outside SQL locks and
      returns the matching candidate for short locked VerifyTx at each boundary.
      Unknown returns no candidate and never resets a partial changed set.
      Definitive changes invoke one ResetVoiceParties transaction. Implementation
      b512dba20 realPG GREEN1PASS0skip4.540s, stubRED e41096383 retained.
- [x] Batch reset7fbddc4ca realPG GREEN1PASS0skip23.226s after da0b8dcdb RED:
      two changed parties share one pending match; injected cancellation failure
      rolls back both, success cancels both/abandons match, replay adds none.
- [x] Native independent review205 CLEAR on exact9de/base d4 plus a2e DOWN,
      BE012-NATIVE-PRODUCER-REVIEW c1. Scope excludes coverage/provider/rollout.
- [x] MM short b512:203PASS,153 explicitPGskips; only Compose fixture test
      failed because backend-only archive omitted root docker-compose.yml.
      Added exact frozen root file and reran only failed case PASS; vet exit0.
- [x] Additional partial-error adapter regression ea3856901 PASS1/0skip9.233s
      from complete immutable e410 archive plus exact implementation/test deltas.
      It proves an explicit authority error dominates partial ChangedPartyIDs.
- [x] Static lint terminal1:2 ST1005 strings corrected in4d6980458;3 unused
      startup-helper diagnostics remain until145's reserved main wiring exists.
      No suppression or fake lint PASS. Rerun when actual wiring changes input.
- [x] Graphify AST update attempted without output/progress; only exact owned
      Python44752/parent52736 canceled, wrappers terminal-1 and full own chain
      absent afterward. Graph update is NOT PASS; foreign processes untouched.
- [ ] Freeze portable checkpoint, safely recycle WT38 rather than holding a
      second scarce worktree. If supporting settlement function remains eligible,
      atomically claim it and obtain a fresh actual-develop lease before writes.
- [ ] Final graph update, affected checks, independent review and 121 acceptance.

## Decisions

- Separate A3 outbox preserves the existing participant compatibility payload and
  avoids shared-stream NATS dedup suppressing one of two distinct topics.
- Native Space roster version is independent from journal/head generations and
  disclosure projection counters; a unique public call_room_id is the room key.
- Coverage/legacy cutover is being resolved in BE-011 c5 by Astra169; native
  writer tests alone do not prove whole-source authority or authorize activation.
- Astra169 BE012 c9 / attachment1535c157ae3296729f9b20686baeddde accepts
  BE012-ASYNC-PROVISION-RESET-v1. Request and background coordinator both need a
  fresh candidate-cut validation before intent, each dispatch, and activation.
  Unknown remains unknown. Commit MAY_HAVE_CREATED before any provider Create;
  reset atomically latches ABANDONED and a frozen issued-provider obligation set.
  Already-issued unknown operations replay exact Create only for receipt/cleanup;
  unissued providers never start after abandonment. Completion requires all
  required creation and teardown receipts; generic terminal errors prove no absence.
  145 owns candidate capture/hash/validator; 147 owns consuming coordinator/reset.
- Genuine historical provider custodian90 confirms narrow successor boundary in
  BE059c12 / BE012-ASYNC-PROVISION-SETTLEMENT c1. Actual old task is BE-059,
  not BE-005. No active90 writer/WIP/lease; full BE059 ownership/CI stays90.
  Supporting function is AVAILABLE, with no receiver or responsibility transfer
  inferred. Actual claimant must accept169c9 and use own fresh-develop lease.
  No provider file edits occurred in this checkpoint.
- Additional Astra BE011c16 MEMBER-ACCOUNT-AUTHORITY-v1 is accepted and read:
  145 owns private same-cut account/epoch tuples+digest, migration022 and local
  all-account ban guard. Settlement021 must acquire sorted account advisory
  transaction locks before match/party/session locks, then use a separate READ
  COMMITTED ban read. Current gate1fc API is an implementation checkpoint, not
  certification of that new extension; consume its exact successor before enablement.

## Risks And Follow-Ups

- BE-011/BE-012 rollout criteria form a semantic package cycle. Astra169 proposes
  jointly versioned implementation activation artifact and real staged evidence;
  do not fabricate PASS or silently weaken gates. Revise supported edges only
  with exact actual revisions/owning actors when artifacts exist.
- Source eligibility depends on revocation/expiry normalization, not silent
  reader filtering under an unchanged version. Every writer path must participate.
- Owned Linux runner voice-rudeus147-be012 uses Go1.26.5, PostgreSQL16 and real
  NATS2.11 testcontainers, Docker bridge only. Proofs are under
  D:/Git/Voice/tmp/swarm/rudeus147-be012. No shared DEV/STAGING claim or hosted
  CI acceptance has been acquired. Local tests do not imply deployment PASS.
- Native review packet is durable outside WT38: PROOF-NATIVE.md attachment
  4ba2036167d90d7b14a4e383a191ac20 and SHA manifest817e28bae302c7c89b5e6a98e269eaf7.
  Bundle verify PASS; binary/ZIP/one bounded base64 attachment attempts returned
  backend internal_error, so no binary TT attachment ID is claimed. Local ZIP
  and separately decoded ordered chunks reconstruct identical SHA256; c12
  documents the actual portable paths. Do not lose this checkpoint on recycling.
