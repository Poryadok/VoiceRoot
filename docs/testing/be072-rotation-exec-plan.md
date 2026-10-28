# ExecPlan: Bot session invalidation on credential rotation

## Purpose

After either Bot credential rotation commits, previously captured Bot authority
must not admit a new runtime effect. New sessions may still receive queued
ordinary message deliveries; old interaction capabilities cannot be reused.

## Context

Sources: [Bot tokens](../features/bots.md#токены-и-безопасность),
[Bot service](../microservices/bot-service.md), and TT BE-072 comments c3–c5
(BE072-ROTATION-v1, Рудеус-170; retained authorship). Base is
`f40a7ddf5ad1f9d22441635773f7abdf2e7baaf2`.
Current rotation only changes credential bytes; Hub and durable events have no
generation binding. The protected Space sender already holds ordered guards and
uses an independent pool for Messaging's authority callback.

## Scope

One coherent vertical: additive Bot/event epoch migration, atomic owner-checked
rotation, captured request authority, durable event and autocomplete binding,
polling/webhook/deferred/runtime sink guards, typed Hub cleanup and rehydration.
No deployment, live credential changes, public epoch API, provider activation,
acceptance appointment, CI policy changes or native Windows tests.

## Milestones

- [x] Reconcile prior decision and current protected send path; claim and lease.
- [x] Record canon and meaningful failing behavior tests.
- [x] Implement epoch binding and all production sink callers.
- [x] Execute Linux PostgreSQL/HTTP2/webhook/race regression proof.
- [ ] Obtain independent security review, then PR lifecycle handoff.

## Detailed Steps

1. Add positive `bots.session_epoch`, nullable event epoch; cancel/scrub legacy
   unbound non-message capabilities without discarding message intents.
2. Capture bot ID, epoch and token hash in the same authentication read. Guard
   final synchronous native sinks using sorted actual Space keys → bot SHARE →
   whitelist → delivery/event. Reuse the protected sender's ordered transaction;
   never add Space locks after acquiring bot/event locks.
3. Rotate under bot UPDATE lock after owner/live recheck; replace only named
   credential, increment epoch once, cancel/scrub old pending/deferred session
   events in UUID order, commit before returning secret. Do not retry ambiguous
   commit automatically.
4. Bind new accepted interactions/autocomplete requests to current epoch under
   the bot guard; preserve original epoch and stable IDs on retries. Use random
   autocomplete UUIDs. Guard completion and final waiting-response effects.
5. Qualify Hub pending/deferred/cache records by bot/epoch; monotonic purge floor,
   nonblocking completion, no DB/network under Hub mutex. Rehydrate only live,
   matching durable epochs. Hub is never the authority source.
6. Run the finite c5 matrix with explicit latches and two pools; review code and
   tests together. Keep the native 25-second PollEvents transport tap unchanged.

## Validation

Linux Go 1.26 container with Docker socket for existing Testcontainers fixtures;
no host ports. Focused rotation tests first, then affected store/grpcsvc/dispatch
tests, `go test -short ./...`, `go vet ./...`, and race checks. Actual commands,
executed counts, source identity and missing evidence must be recorded here.
Required cases include atomicity/rollback/owner checks, old/new sessions and
message preservation, actual native sink ordering/HTTP2 blocked send deadline,
webhook claim fencing, deferred concurrency/restart, autocomplete identity/cache,
Space/whitelist/deletion regressions, Hub races and migration cutover.

## Progress

- Read c1–c5, current canon and exact CodeGraph paths.
- Owned worktree `D:/Git/Voice/.treehouse/Voice-451d38/8/Voice`, lease
  `7167a79b5559b9485b7e1b8849893bc6`, branch
  `codex/be072-bot-session-rotation`; developer Dimas-40.
- Initial RED command: `docker run --rm --name dimas40-be072-red` with checkout
  `/work`, Docker socket, `TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal`,
  `golang:1.26`, then `go test ./internal/store -run
  '^TestCredentialRotationRevokesSessionCapabilities$' -count=1 -timeout=3m -v`.
  Owned session 23545. Initial run omitted explicit container CPU/memory limits;
  observed actual HostConfig memory/NanoCpus/quota/period all zero. Engine
  inventory: 16 CPU, 33561067520 bytes RAM, 105 running containers. No other
  containers are being cleaned up. Subsequent runs require an explicit bounded
  resource plan (approximately 2 CPU/3 GiB for owned Go+PostgreSQL case).
- RED terminal: both `/token` and `/webhook_secret` cases failed expected
  `canceled`, actual `pending`; 2 executed cases, 16.33 seconds, exit 1.
  Fixture terminated owned PG containers `1dd81e027f21` and `b30b66047a99`.
  Runner was removed; no PASS. Durable TT evidence: BE-072 c7.
- Subsequent bounded runner plan: Go `--cpus=1.5 --memory=2560m`,
  `GOMAXPROCS=2`, `go test -p 1 -parallel 1`; Bot-owned testdb fixtures limit
  each PG container to 0.5 CPU/512 MiB, one test at a time. Latest preflight:
  engine 16 CPU/33561067520 bytes, 102 running containers. Compile-only
  short run selects no test cases; it is not PG/race/security proof.
- TT c10–c17 record compile-only, PG rotation/serialization, Hub RED→GREEN,
  full Bot short (108 top-level, 216 pass events, 193 explicit integration skip
  events), and three real HTTP2/deferred/webhook runtime regressions. These are
  earlier source checkpoints; exact source digests remain in TT.
- TT c19–c20: actual schema9 native membership source initially failed the
  historical exact-v8 runtime fence. Both native consumers now require exact
  `store.BotSchemaVersion` (9), preserving source UUID and generation/counters.
  Focused store run: 8 top-level / 11 case bodies, zero skips, 69.952s GREEN.
  Includes atomic rollback/owner checks, both rotations, captured listed polling
  authority, new-session message recovery, eight concurrent rotations, delete,
  owned backend termination and legacy migration scrub.
- TT c21: real unread HTTP2 with production 25-second tap, token and secret
  rotation: 2 bodies GREEN, 70.981s. Native second Send held the bot guard;
  rotation was observed waiting in PostgreSQL, native handler terminated first,
  and a fresh streaming connection received three preserved stable-ID messages.
- TT c23–c24: two pools/Hubs protected deferred completion, both rotations,
  produces one final Messaging effect; remote stale Hub cannot repeat it.
  Durable autocomplete completion on another replica, foreign Bot/invoker
  isolation, random IDs, duplicate denial and epoch-isolated cache pass for
  both rotations. Actual HTTP retry after killing only the owned guard backend
  is denied before a second native POST. The first remote observation remains
  an explicitly ambiguous external effect, not a distributed zero-byte fence.
- New local-Hub timeout test produced meaningful RED after remote rotation:
  returned accepted-command timeout rather than NotFound. Queued-result final
  Messaging denial already passed. Timeout now revalidates durable capability;
  TT c27 records GREEN regression proof.
- TT c27: source `5c4969d4`, 56 top-level tests / 82 pass events, zero skips or
  failures across store (89.301s), grpcsvc (289.381s), consumer (54.439s).
  Includes protected membership source/callback independence, stale claims,
  deferred/DM/chat-type recovery, autocomplete, current webhook destination and
  secret, and scope/whitelist/Space revocation.
- TT c29: source `2249aef1`, Linux `-race` on dispatch/store/grpcsvc, 12 top-level
  / 14 leaf bodies, zero skips/race reports. Captured slash claims cannot POST or
  reopen after either rotation; claim expiry is reread after an actual event
  row-lock wait. Hub floor/full channel/late rehydrate and cross-replica deferred
  completion pass. Test runner 1.5 CPU/2560 MiB plus sequential PG16 0.5 CPU/
  512 MiB, asserted from actual container HostConfig.
- TT c30–c31: six mechanical pinned-lint findings fixed without changing the
  authority algorithm or policy. Final source `e6d7aad9`: full Bot short across
  14 packages (108 executed top-level, 216 pass events, 205 explicit integration
  skips), `go vet ./...`, and repository-pinned golangci-lint v2.6.1 all GREEN,
  zero lint issues. Short skips are separate from real runtime evidence above.

## Review And Delivery Checkpoint

Independent security/concurrency review is required before integration. Review
the atomic owner/epoch cutover, same-read captured hash/epoch, sorted upstream
guards, original transaction on every HTTP retry, durable completion and cache
binding, and schema9 native source compatibility. The source includes the
original BE072-ROTATION-v1 capability limit: already observed remote effects may
remain ambiguous after database connection loss; no distributed zero-byte fence
is claimed. No live credentials, provider activation, deployment or acceptance
were performed. Developer checks do not appoint an acceptance tester.

## Decisions

- Both rotations advance one internal epoch; each changes only its named
  credential. Unchanged token after secret rotation can establish a new session.
- Bot commit is durable authority cutover. Healthy held database guards serialize
  actual bounded synchronous sink invocation. Already submitted remote bytes or
  side effects may finish after rotation, especially after transaction loss;
  current sinks provide no distributed epoch fencing.
- Ordinary messages survive rotation. Old interaction capabilities do not.
- Extend current `WithMessageSend`; no generic outer bot-first lock wrapper.
- Durable `interaction_completed_at` separates transport delivery from consumed
  final capability, preventing a second restored Hub from repeating completion.
- Each actual HTTP attempt/redirect checks the original held transaction and
  claim. Connection loss cannot rehydrate its authority from a healthy pool.

## Risks And Follow-Ups

Security/concurrency and migration require independent expert review. Stop/drain
old Bot replicas/streams before schema cutover and activation; old binaries do
not enforce epochs and rollback to them is not security preserving. Local tests
and review are not DEV/STG rollout or final acceptance.
