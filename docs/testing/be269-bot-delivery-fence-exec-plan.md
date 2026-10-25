# ExecPlan: Bot message delivery respects Space lifecycle receipts

## Purpose

After Bot acknowledges a non-LIVE Space fence, no new message recipient intent,
webhook attempt or polling message send may be admitted for that Space. An
already admitted bounded send may finish before the fence receipt commits.

## Context

Canon: `docs/microservices/bot-service.md` P3, the participant contract in
`docs/microservices/space-service.md`, and accepted R23 in `docs/PLAN.md`.
TT BE-269 c3/c4 records the accepted Astra lock protocol and runtime matrix.
Existing producer, delivery worker and polling handler are the consumers.
Existing lifecycle receipt storage and protected RPCs remain authoritative.

## Scope

Guard the existing message producer/worker/polling path and neighboring
installation/whitelist mutations with Space-first advisory locking. Preserve
recipient-set transactionality, deduplication, payload scrubbing and terminal
states. Imported Chat pages, command/deferred producers, retention and whole-P3
convergence remain open. No deployment or acceptance appointment is authorized.

## Milestones

- [x] Six PostgreSQL regression sources exercise new admission, stale work,
  polling/legacy cancellation, webhook/stream receipt barriers, installation
  lock order and restored fresh delivery.
- [x] Existing consumers use Space-first locks and fresh lifecycle/authority reads.
- [x] Bot short suite 84 PASS, `go vet ./...` PASS, `golangci-lint run ./...`
  zero issues; `git diff --check` PASS.
- [ ] Independent concurrency review.
- [ ] Exact-head PostgreSQL/runtime and required hosted CI before ordinary merge.

## Detailed Steps

1. Add real PostgreSQL regressions using existing lifecycle store fixture.
2. Attempt focused runtime tests; distinguish setup failure from observed RED.
3. Acquire complete distinct actual advisory keys in ascending order before row
   locks. Re-read lifecycle after waiting, then bot, whitelist and delivery/event.
4. Move installation/uninstallation ahead of bot locks; retain fence exclusive
   locking through receipt commit. Never upgrade a shared advisory lock.
5. Wire guarded polling sends; cached event payloads are only discovery hints.
6. Preserve exact source, checks, remaining matrix, restoration and custody in TT.

## Validation

`go test ./internal/store -run TestSpaceMessage -count=1` must execute without
skips on PostgreSQL-capable Linux. Full c4 matrix also requires barrier-controlled
concurrency, producer/worker and actual PollEvents integration. Local Windows
Docker is known unavailable; a setup failure is not a behavior RED/PASS.
`go test -short ./...`, `go vet ./...`, `golangci-lint run ./...` and diff checks
cover compilation and local component checks only.

## Progress

- Claimed by Рудеус-103; own WT38/base698d4f0, branch
  `codex/be269-space-delivery-fence`, lease7dbca0752df261bc6aa3376f0b37f5d8.
- Accepted protocol consumed from BE-269 c3/c4; no new product policy inferred.
- Initial focused PostgreSQL attempt failed during testcontainers setup:
  `rootless Docker is not supported on Windows`, before assertions. This is
  neither a meaningful behavioral RED nor database PASS. No setup retry made.

## Decisions

Missing local lifecycle head preserves existing admission semantics. Any stored
state other than LIVE denies. Binding changes roll back and retry discovery;
database failure never authorizes a send. HTTP remains at-least-once on commit
ambiguity. Terminal canceled records cannot be rearmed by retry bookkeeping.

## Risks And Follow-Ups

Concurrency/security change requires independent review and real non-skipped
PostgreSQL evidence. No local short-suite result proves the receipt barrier.
GitHub host transport outage is recorded separately; do not repeat probes.
The broader c4 matrix remains open: actual MessageHandler/PollEvents receiver
proof, freeze-wins fresh-snapshot barrier, captured rebind rollback, atomic
multi-recipient insertion failure, multi-Space overlap, uninstall/disable/purge
cycles, timeout/commit-loss and non-LIVE permanent-state variants. Do not equate
the six added source regressions with completed full participant verification.

## Review P2: bound native polling flow control

Independent reviewer115 found that the original synchronous polling callback
had no server-owned native stream limit, so a stalled polling client could hold
the shared guard indefinitely. Accepted Astra94 decision BE-269 c13/c14 fixes
the exact seam: derive a method-specific deadline in `InTapHandle`, before pinned
grpc-go 1.70 captures transport `ctxDone`/writeQuota. A child context in a stream
interceptor would not suffice. Keep Send synchronous and cancel the stored tap
CancelFunc on handler exit. All three listeners register the tap because each
registers the full BotService. The whole-RPC limit is 25 seconds, matching the
webhook attempt budget, and never extends an earlier client deadline.

- Meaningful RED: real HTTP/2 transport, no client deadline/Recv, two 1 MiB
  messages exhausted stream credit/write quota; unchanged no-op tap failed
  `server-owned cap did not terminate native flow-control blocked Send`.
- GREEN: native stalled Send plus modeled callback guard and deadline/scope/
  cleanup tests, ten repetitions each (20 PASS). Bot short suite 86 PASS.
- Focused `go test -race -short ./internal/grpcsvc -run TestPollingTransport`
  2 PASS; `go vet ./...` PASS, lint zero issues, diff check PASS.
- Added real PostgreSQL/actual `BotGRPC.PollEvents` composition source test with
  native Send observation, advisory-wait proof and eventual actual fence receipt.
  This test compiles but has not run here; existing Docker limitation is retained.
- No detached Send, forced DB-only release or client-imposed deadline workaround.
- Send nil is enqueue, not client ACK. Previously admitted queued bytes may reach
  the receiver after receipt; no active/new application Send survives its guard.
- Same reviewer115 must recheck the fix; full c4 runtime and exact-head CI remain
  open. Original source/bundle5f025d7 remains preserved for provenance.
