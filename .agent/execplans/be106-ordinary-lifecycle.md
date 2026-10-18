# ExecPlan: durable ordinary Voice SDK lifecycle

## Purpose

Ordinary DM/group calls explicitly create their LiveKit room before becoming
visible. Ending a call or leaving a group removes current media before reporting
cleanup success. A canceled request, ambiguous SDK result or process restart
cannot resurrect a terminated call, release another admission's account fence,
or discard cleanup work. This is one complete ordinary-call vertical, not an
activation of the generic Space lifecycle framework.

## Context

Accepted decisions are BE106-ORDINARY-SDK-v1 in TT BE-106 c3/c4 and the shared
BE102-COMMITTED-ADMISSION-v1 in BE-102 c5. Canonical sources are
`docs/features/voice-chat.md`, `docs/microservices/voice-service.md`,
`docs/DATA_STORES.md`, `docs/DATA_MODEL.md`, and
`docs/ARCHITECTURE_REQUIREMENTS.md`. The ordinary contract is reproduced in the
Voice service document; the tracker links preserve decision provenance.

At baseline `698d4f064f032ea74abf3ede2017100b26c8ee69`, `VoiceGRPC` uses separate
Redis call writes and tuple-only account reservations; it does not call the
existing `livekit.RoomLifecycle` adapter. `roomlifecycle` is the separate,
source-disabled R22 stack. `main.go` already opens a PostgreSQL lifecycle pool.
`gameprovision.AccountVoiceFenceStore` is the shared account exclusivity table,
and Space-owned rows already use `admission_operation_id`.

Author: Рудеус-1, exclusive treehouse20, branch
`codex/be106-ordinary-lifecycle`. BE-250 stays frozen on its separate branch;
its existing widget/analysis/review evidence does not verify this change.

## Scope

In scope: ordinary DM/group Start/Accept/Join/Leave/End/Decline/timeout, room and
membership journal, exact-generation shared fences, SDK effects, Redis projection,
durable compatibility event intents, recovery worker, authoritative token/read
gates, and client mapping of opaque ordinary membership media identities.

Out of scope: Space, GAME_SESSION, MatchSquad, federation, generic R22 coordinator,
Space watcher/roster activation, a new public idempotency field, strict SFU
deny-before-connect, deployment or acceptance appointment. Existing dispatch guards
for these resource owners must remain before ordinary handling.

BE-102 owns additive participant protobuf/publisher/consumer/NATS scope. It consumes
the ordinary committed transition/outbox boundary; it must not add a second journal
or publish from tentative Redis writes. Both owners coordinate exact API/file scope.

## Milestones

- [x] Canon and this plan distinguish implemented opt-in runtime from unverified
  rollout/legacy drain/actual media acceptance.
- [x] Risk-first deterministic model tests expose admission/terminal/recovery races.
- [x] PostgreSQL journal and generation-bound shared fences implement the same
  transition model; real database tests prove two-instance serialization.
- [x] Ordinary handlers use the journal and SDK recovery/effect path end to end;
  every ordinary read/token path fails closed on unavailable/pending authority.
- [ ] Opaque ordinary media identities map back to profiles in affected client UI.
- [x] Compatibility event outbox preserves stable IDs/recipients and integrates
  BE-102 committed membership facts without parallel authority.
- [ ] Scoped service/client checks, graph update, independent concurrency review,
  pushed PR and exact-head required CI establish a reviewable result.

## Detailed Steps

1. Recheck migration allocation (baseline max `000011`). Add ordinary-specific
   tables for immutable room operations, membership generations, effects/outbox
   and compact terminal records. Reuse the shared account-fence table with explicit
   ordinary operation/generation ownership; preserve Space and legacy semantics.
2. Write negative tests before implementation: pending Start cannot issue tokens;
   terminal wins over pending Join/Accept; failed account/SDK check leaves no member
   fact; stale compensation cannot release another generation; equality expires
   ringing; late SDK create stays compensated. Use fake clock/barriers, not sleeps.
3. Define one storage transaction boundary. BeginAdmission durably captures exact
   trusted tuples and owns pending capacity/fences before external calls. Pending
   admission remains visible to SDK conversion quiescence but is not logical
   membership. No DB lock spans network work. FinalizeAdmission CAS verifies room
   status/version, operation/generation and fences and commits members plus intents.
4. Terminate/Leave transaction elects a single winner, invalidates pending admission,
   captures committed members only, and records exact cleanup/fence ownership.
   Non-last Leave removes one media identity; last Leave closes the room. SDK
   uncertainty keeps cleanup pending. Only confirmed absence permits exact release.
5. Add fresh-context bounded retries and persisted scheduling/worker CAS. Restart
   aborts unfinished preparation, repairs READY projection/outbox, resumes cleanup
   and expired ringing, and preserves valid ACTIVE calls. Unknown DB/effect results
   resolve through stored operation readback, not guessed rollback or TTL takeover.
6. Integrate all ordinary handler/read/token paths through this authority. Preserve
   initial and post-reservation SDK-conversion checks, Chat/privacy/identity gates,
   current group End authority and foreign resource guards. Unjournalled Redis
   records require an explicit validated adoption or controlled drain policy;
   neither synthetic account identity nor automatic unsafe adoption is acceptable.
7. Issue media identity `ov:<profile UUID>:<membership UUID>` per real admission;
   refresh retains it, rejoin changes it. Keep public profile IDs unchanged and
   update frontend resolution so old delayed removal cannot target a rejoin.
8. Store original compatibility signaling payload/recipients with stable UUIDs and
   order. Publish through the ordinary outbox; transport failure after commit never
   rolls back accepted state. BE-102 adds participant facts at this same boundary.
9. Run the validation below, update graph, obtain independent security/concurrency
   review, commit/push promptly, open PR and own CI/merge continuation. Keep actual
   rollout and two-client media acceptance explicitly separate.

## Validation

Risk tests must cover SDK create/delete/remove executes then times out, cancellation,
lost DB commit response, second-fence failure, Redis projection failure and restart
at each durable/external boundary. Concurrency cases include duplicate Join/Accept,
Accept versus exact deadline/Decline/End, pending Join versus last Leave, two Leaves,
leave/rejoin with late old removal, and stale worker completion. Existing Space,
GAME_SESSION and MatchSquad negative regressions must stay green.

Run focused `go test` first, then affected Voice packages and `go vet`/pinned lint.
Real PostgreSQL and two service instances with Redis prove store/CAS behavior;
`-short` or skipped Testcontainers are not this proof. Race checks need a supported
CGO host. Run affected Flutter tests/analysis for identity mapping. Exact-head CI
and independent review are required before merge. Reuse current adapter proof
(BE-106 c2: four RoomLifecycle unit tests), expanding only for uncovered behavior.

Pinned self-hosted LiveKit proof eventually requires two clients exchanging actual
tracks, non-final leave preserving the other participant, final close disconnecting
both, and stale reconnect detection/ejection. Unit/signaling tests do not prove it.
Local Docker was unavailable during earlier Auth checks; recheck before declaring
new integration execution. Current GitHub SSH/HTTPS/API transport is unavailable;
record failed publication/check attempts without fabricating PR or CI evidence.

## Progress

- [x] Real TT handoff e4 made Рудеус-1 owner; isolated branch created at baseline.
- [x] Accepted c3/c4 and BE-102 c5 read fully; source preflight confirmed missing seam.
- [x] Owning canon and resumable plan written before production edits.
- [x] Model negative tests preceded implementation; latest prior run passed 15
  model cases. Three PostgreSQL cases were skipped with `-short`, not verified.
- [x] Late SDK create requeues close with a revision; an older delete receipt
  cannot clear that newer compensation. Stable terminal event audience is frozen.
- [x] Shared identity lock helper passed two unit cases (signed hash ordering,
  collision deduplication, invalid identity). Store takes room then the complete
  identity union under explicit READ COMMITTED and checks conversion after locks.
- [x] Docker recovery verified through actual Go/Testcontainers endpoint
  `npipe:////./pipe/dockerDesktopLinuxEngine`. Fresh scoped PostgreSQL run passed
  seven tests, zero skips/failures; JSONL `tmp/swarm/be106-postgres-conversion.jsonl`.
  First real execution caught nil/empty immutable-event prefix comparison; fixed
  by comparing each preexisting event. Subsequent full scoped run passed.
- [x] c10 conversion identity barrier, native pending source/target-account
  snapshot and immutable cleanup binding implemented. Missing historical pending
  provenance fails closed. Coordinator cleanup preserves other group members.
  Remaining deterministic interleavings and startup wiring are still required.
- [x] Coordinator orchestration risk tests pass: unknown Ensure compensates the
  exact name, unknown Close retains CLOSING, canceled requests use a fresh bounded
  compensation context, projection failure cannot undo READY, restart aborts
  preparation while preserving committed ACTIVE. A transactional fake is used;
  these results do not prove PostgreSQL or actual media disconnection.
- [x] Structured short run reports 23 passed / 5 PostgreSQL skipped before the
  next reconciliation changes; artifact `tmp/swarm/be106-short.jsonl`.
- [x] Five focused projection tests pass against miniredis/Memory: old cleanup
  preserves new indexes, same-generation flags survive, rejoin resets flags,
  foreign ownership requires native proof, group chat survives initiator leave.
- [x] Own verified caller/callee epoch model tests pass. Caller Start binds its
  epoch; invited callee has none until its own Accept. Replay and no-op Join from
  another device never upgrade the existing admitted media epoch.
- [x] Ordinary Start/Accept/Join/Leave/End/Decline and token routing, native
  projection adapter and authoritative active lookup implemented. Handler test
  proves Start -> Accept -> opaque token -> End uses one journal. Latest short
  ordinary/grpc run: 229 passed. This is not runtime/actual-media evidence.
- [x] Opt-in runtime registers native coordinator, recovery/outbox workers and
  schema readiness. Legacy drain attestation and startup inventory are required;
  they do not constitute executed drain/deployment evidence. A nonempty native
  journal refuses legacy fallback after disabling configuration.
- [x] Coherent BE-102 dependency commits `37187d083` and
  `0361d8597f776fd02dd312cc6498d26d00bcd890` merged without rewriting history.
  Adapter freezes existing call envelopes and delegates new participant facts;
  delivery preserves saved payload, event ID, time, audience and PubAck semantics.
- [x] Fresh scoped PostgreSQL ordinary + existing SDK conversion suite: 12 PASS,
  zero fail/skip. New checks include committed native cleanup/projection, reversed
  identity acquisition and mismatched trusted account/profile with missing Redis.
- [x] Actual Redis 7 two-client WATCH/EXEC interleaving: one PASS, zero skip;
  invalidated first attempt retries and delayed old cleanup preserves new indexes.
- [x] Whole Voice short suite: 626 PASS / 166 SKIP, exit0. These skips are not
  database proof; scoped real database evidence above is separate.
- [x] Windows/amd64 CGO=1 race check over ordinarycall/admissionlock/store/grpcsvc:
  277 PASS, exit0. Pinned golangci-lint2.6.1: zero issues; scoped go vet: exit0.
- [ ] Graph update is still running at source freeze (owned session35893), AST
  extraction224/224 completed; unrelated Linux header parse warning recorded.
- [ ] Independent source review and develop integration belong to existing
  integrator Рудеус-121. No source review/CI/merge/deployment/acceptance is claimed.
  Finish-only user directive: no new work; preserve exact head/base/evidence for121.

## Decisions

- BE106-SDK-CONVERSION-SERIALIZATION-v1 (BE-106 c10) is accepted: native room
  lock precedes all sorted actual signed int32 hash keys for trusted account and
  profile UUIDs, using two-int namespace `0x564F4943`. Conversion takes its
  operation lock then that same identity union, never a native room/fence row
  lock. Fresh nonlocking snapshot statements follow the barrier under READ
  COMMITTED. Pending/fenced conversion denies source profile and the entire
  target account; activated conversion retains only retired source-profile denial.
  Cleanup stays permitted. Source provenance and operation/generation bindings
  must be immutable receipt data; unknown ownership is never absence. Existing
  Space/Game writers require their own coverage before global safety is claimed.
- BE011-SOURCE-SEAM-v1 (BE-011 c2/c3) names the A3 reader consumer: keep private
  JournalVersion and a separate RosterVersion, one consecutive positive version
  per true profile join/leave, UUID-byte ordered within a batch. DM Accept advances
  twice atomically. No-op/ringing/cleanup/retries do not advance it. Persist each
  actor's verified admission epoch, never an Auth floor-derived substitute.
  Reader accepts a common read-only REPEATABLE READ pgx transaction and takes no
  locks. Ordinary reconnect producer and Space writer coverage remain separate
  gaps; this implementation does not claim A3 activation readiness.

- Use one ordinary journal, not a Redis mutex/TTL journal or source-disabled R22
  activation: explicit accepted BE-106/BE-102 contract.
- SDK uncertainty means pending cleanup, never proof of absence. Durable state is
  authoritative; Redis remains rebuildable.
- Application denial plus confirmed current disconnect is the accepted guarantee.
  Reusable LiveKit bearer tokens can briefly reconnect until reconciliation ejects
  them; tombstones/retired identities are not collected solely at JWT expiry.
- Start has no new public retry guarantee. Internal retries reuse exact intent/name;
  pending/live repeat conflicts and lost successful response recovers via active call.

## Risks And Follow-Ups

The largest risk is a stale worker or ambiguous external result affecting a later
admission. Generation-bound shared fences and opaque media identity are mandatory,
not optional hardening. No foundation-only result counts as the completed task.
Legacy cutover needs explicit validated authority/drain evidence before enabling
the new path. An integration environment and actual rollout authority remain
separate from source delivery. BE-102 must not race writes to the ordinary journal,
handler transaction code, main wiring or account-fence implementation.

At source freeze, frontend opaque-identity presentation is the independently
owned `FE-BE106-ORDINARY-MEDIA-IDENTITY` consumer (Рудеус-113); its head and checks
must accompany develop integration before enabling. This author did not edit
frontend files or claim its validation. Ordinary A3 reader/reconnect production,
Space/Game participation in conversion's identity barrier and actual media
acceptance remain separate explicit gaps. No additional milestone is activated.
