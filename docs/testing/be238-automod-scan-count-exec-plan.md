# BE-238: durable AutoMod scan count

## Current correction: finite outcomes (Astra199 TTc17)

Original combined FAIL is retained in attachment738ebb533dad4be5dce9151de0ed394d,
source f453a544d4e0acd823bafbd48daf8f39d16d3d0e: one-CPU RPC batch observed11
instead of37 and held fixture observed1 after lock release. Full original stdout
and backend ordering were not retained. Neither permanent loss26 nor a specific
late-commit cause is established. Earlier focused PASS/source CLEAR do not
qualify this combined scenario.

Decision attachment418eebb278a8253c685a62eb64c52e70 retains bounded best-effort
persisted samples, clarifies canon and requires exactly one finite terminal
outcome per eligible evaluation. No retry, worker, queue, timeout increase or
moderation-policy change. Author142 owns fresh WT35/base8984552dd; task/CI121.

- [x] Linux RED: eligible evaluation exposes no terminal outcome (0.097s).
- [x] Private-registry collector, controlled recorder seam, real production store.
- [x] Exact37 direct acknowledged PG increments/reopen, exact37 handler eligibility.
- [x] Child cap/quarter/skips/no retry, original decisions/errors preserved.
- [x] RPC terminal reconciliation and server-quiescent durable bounds; healthy ack.
- [x] Row/table locks held through attributed backend termination, exact0,
  separately acknowledged recovery1; committed-but-error fault injection.
- [ ] One-CPU full grpcsvc with complete stdout, exact images/catalog/CPU/pool
  diagnostics. Merge foreign f453 only in separate proof branch; deliver cleanfix.
- [ ] Exact independent review, portable bundle/restore, explicit121 intake.

Client cancellation, RPC completion, pgx cleanup alone, fixed sleeps or equal
reads do not prove server quiescence. Stop submitters, keep the lock, close the
unique-application fixture pool and observe no corresponding server backends
before release. Finite observation failure fails the fixture. Process diagnostics
have crash/restart limitations and do not form a lossless evaluation ledger.

Owned one-CPU Linux Go1.26/PG16 focused successor PASS4.521s, zero skips:
direct acknowledged37/newpool37, handler eligibility37/ack37/bypass0, synthetic
commit-then-error delta1/unconfirmed1/no retry. RealRPC E37=A7+U30+S0; after
all unique-application server backends disappeared, D9 satisfies7<=9<=37.
This is observed uncertainty, not a claim that30 writes were lost. Row and table
each returned clean/spam decisions under500ms caller deadlines, observedU2/A0,
closed their unique pool and proved backend count0 while lock remained held,
then read durable0 inside the lock transaction. Recovery acknowledged1/durable1.
Original action history/baseline, missing table/singleton, migration down/up and
business validation remain checked. Default resolved GOMAXPROCS2/NumCPU16,
pool max16/min0; complete focused stdout is retained in the external delivery
artifact. A first diagnostic-only runner expected cgroupv2 paths on actualv1;
it ran no tests and is retained as infrastructure failure, not behavioral RED.

Component short tests and Linux go vet PASS; isolated host static lint reports
zero issues (no host tests/listeners). Diffcheck PASS. Graphify AST update was
attempted and canceled after bounded no-output execution; own process chain
settled, no graph-update PASS claimed. Combined full-service gate remains open.

## Purpose and context

`GetAutoModStats.messages_checked` counts persisted samples eligible from
configured `CheckMessage` evaluations, including allowed messages, instead of
action-log rows. Sources:
`docs/microservices/moderation-service.md`, `docs/features/reports.md`,
`docs/DATA_STORES.md`, `docs/TESTING.md`, PLAN A4 and TT BE-238. Base is actual
develop `45cf501e85a05666b8c99813dc94c281f11552e2`; developer142 owns a separate
Treehouse lease. No DEV/STAGING, host test listeners or acceptance authority.

## Scope and decisions

Under the user's autonomous routine product-decision delegation, start at zero
when the new aggregate is introduced. Historical clean scans cannot be recovered
from actions, so do not seed fabricated history. Persist only a count and start
time, no content/profile/message identifiers. A retried RPC is a new evaluation:
the current request has no message ID/idempotency key. Nil AutoMod bypasses do
not count. Counter failures log a generic warning and preserve existing scan
decisions/errors, so instrumentation can undercount during failures. Existing
`blocked` aggregation and spam/sanction policy remain outside this correction.

## Milestones and detailed steps

- [x] Confirm current source/canon and atomically claim task/own lease.
- [x] Add real-PostgreSQL regression: clean/repeated/blocked calls, action-log
  independence, concurrent exact increments, durable readback and failure
  preservation. Demonstrate meaningful RED before production changes.
- [x] Add narrow additive moderation migration, atomic aggregate store operation,
  stats read and configured-handler recording; update owning canon.
- [x] GREEN focused real Linux tests; affected short suite/vet/lint and migration
  rollback behavior.
- [ ] Independent migration/concurrency source review.
- [ ] Preserve immutable commit, portable bundle and exact results; accepted
  ant121 integration/CI handoff. Hosted CI/deployed acceptance remain separate.

## Validation

Run Go1.26 in an owned Linux Docker runner with real Docker socket for existing
testcontainers PostgreSQL16 harness; no generated Windows programs. Use cached
module data, own build cache/output/container names. Short mode is not DB proof.
Require named PostgreSQL cases PASS without skips; record infrastructure failures
separately. Check exact durable count after concurrent evaluations and reopen a
pool, no fabricated historical baseline, old allowed/blocked responses on metric
table failure and unchanged blocked calculation. Run component short suite and
vet/lint once final source is coherent.

## Risks and follow-ups

The singleton row serializes metric writes; do not change moderation policy or
add unbounded per-message storage to avoid it. Best-effort persistence can
undercount and is documented rather than hidden. Full service traffic/production
load and dashboard field presentation require separate exact-environment proof.
Current internal-moderator transport and spam-policy gaps remain separate tasks.

## Progress

Meaningful RED on real PostgreSQL16: with two historical action rows and zero new
evaluations, old code incorrectly returned `messages_checked=2` instead of zero.
Earlier cache/mount setup failures are not behavioral RED.

GREEN final `go test ./internal/grpcsvc -run
'^Test(AutoModScanStats_RealPostgres|ModerationPlatform_CheckMessage_spamMuteFirstSecondOffense)$'
-count=1 -v -timeout 5m`: both named tests PASS, no skips, 32.908s. The new case
checks 37 evaluations including 32 concurrent calls, separate pool readback,
counter-table failure preserving allowed/blocked/invalid responses, actual
migration down/up with new zero baseline and unchanged action history, and
missing-singleton read failure. Testcontainers containers terminated normally.

`go test -short ./... -count=1` PASS; `go vet ./...` PASS in the owned Linux
Go1.26 runner. Offline module proxy used read-only host download zips with own
Linux module/build volumes. Host static `golangci-lint run ./...` PASS, zero
issues, using own TEMP/TMP/cache after the shared lint lock was busy; no host
test executable or listener was run. `git diff --check` PASS.

Implementation uses a single atomic UPDATE rather than an upsert: a missing
singleton is a persistence error, not an implicit new baseline. No test inputs
or old moderation action/error policy were weakened. Review, exact-head hosted
CI/integration and deployment/acceptance remain separate unfinished gates.

## Reviewer147 counter-lock correction

Review147 TTc6 identified that synchronous metrics before business evaluation
can exhaust the caller deadline on a counter lock. Owner121 confirmed no other
writer and retained integration custody. Fresh lease20 is based on actual
develop8493fa547529d789a88f0a991810ddf87e8922c4, with a39e420 normally merged.

Correction plan: first add deterministic PostgreSQL row/table-lock regression
through the real gRPC client, with allow/block responses required before the
request deadline while the lock remains held. Demonstrate RED. Then evaluate
the business decision first; perform synchronous best-effort persistence only
afterward with a short child budget and reserve most remaining caller time.
No goroutine, queue or lifecycle machinery is needed. Keep zero baseline and
atomic UPDATE; skipped/expired metric writes can undercount. Re-run original
concurrency/durability cases without relaxing their expectations, component
checks and same147 exact-successor review. Record actual proof before121 intake.

Correction proof: actual PostgreSQL16 RED reproduced `DeadlineExceeded` on both
held-row and ACCESS EXCLUSIVE table-lock subcases (8.557s); the new test began
only after each lock was synchronously acquired. GREEN after moving persistence
to a deferred post-decision helper: both original and held-lock tests PASS,
zero skips, 8.399s. Each held-lock subcase returned clean allow and fresh spam
`spam_mute` while the transaction remained alive; locked attempts left zero
persisted count and a post-release evaluation restored count1. Original exact37
evaluations/32concurrent assertions remain unchanged and PASS. Child timeout is
min(50ms, remaining caller budget/4); canceled/sub-ms budgets skip, with no new
goroutine. Linux component-short suite and go vet PASS; isolated static lint
zero issues and diffcheck PASS. Source was checked over normal merge56ff1c90 of
current8493 and originala39. Same147 immutable successor review remains next.
