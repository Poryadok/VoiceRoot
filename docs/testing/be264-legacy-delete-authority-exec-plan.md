# ExecPlan: legacy File deletion authority

## Purpose

Prevent uploader deletion and expiry from removing bytes retained by any live
reference or usable alias. Preserve singleton legacy deletion and durable retry.

## Context

Sources: `../microservices/file-service.md` P3 reference/GC contract,
`../DATA_MODEL.md`, `../DATA_STORES.md`, `../TESTING.md`, and TT BE-264 c2/c5
(accepted Astra decision). Base is 698d4f064f032ea74abf3ede2017100b26c8ee69.
DeleteFile and jobs expiry currently call storage before the reference ledger.

## Scope

In: always-on legacy deletion guard and durable blob intent, reuse GC, expiry
candidate starvation protection, music transition atomicity, regression cases.
Out: P3 activation, owner backfills, shared/dedup writers, retention policy.

## Milestones

- [x] Regression fixtures for references/aliases, singleton and crash recovery.
- [x] Shared authority wired into DeleteFile and expiry.
- [ ] Focused/module checks and independent concurrency review.
- [ ] Exact-head CI, real PostgreSQL proof, PR and gated merge.

## Detailed Steps

1. Lock file NO KEY UPDATE, then blob UPDATE; query reference/alias protection
   in a fresh statement after acquiring the blob. Never acquire fences afterward.
2. Commit terminal file/music event plus GC_PENDING, stable operation and
   irreversible deleted_at before touching storage. Replays retain first status.
3. Reuse existing GC claim/attempt execution; no nil-deleter success. Leave
   pending intent for background retry after ambiguous external effects.
4. Filter protected expiry hints and recheck eligibility transactionally.

## Validation

Run File `go test -short ./...`, `go vet ./...`, pinned golangci-lint;
run named BE264 real PostgreSQL tests without short on a valid Linux runner.
Concurrency/deletion risks require independent review. Shared local Docker
engine outage means no local PostgreSQL PASS or meaningful runtime RED may be
claimed; fixtures are test-first but compile/short skips are not runtime proof.

## Progress

- [x] Atomic owner134 claim; own WT39 lease9a6fd5af6610156d2f5e2dde0299f6b8.
- [x] Read accepted algorithm, File store/GC/music/expiry/RPC and existing tests.
- [x] Implement shared guard, transactional music transition, targeted GC and
  expiry candidate filter. Added five PostgreSQL test functions (including
  six protection subcases), FK/lock barrier, nil-deleter restart and variants.
- [x] File short suite: 216 PASS across 13 packages; go vet PASS;
  golangci-lint 0 issues; git diff --check PASS.
- [ ] PostgreSQL fixtures have not run. Initial focused command overlapped
  compilation with file arrival and failed build; it is not meaningful RED.
  Current short compilation passes, all DB tests skip; meaningful runtime
  RED/GREEN remains an explicit gap under the known engine outage.

## Decisions

Use existing schema and GC operation; protection is independent of listener and
authority flags. A usable empty-ledger alias vetoes deletion conservatively.
Commit failure never permits an external deletion. Existing GC_PENDING with no
deleted_at remains revivable; legacy prepare sets deleted_at durably.

## Risks And Follow-Ups

Full Astra c5 real-PG barriers, ambiguous commits, all-key retry and concurrent
expiry/GC must be demonstrated before merge. Shared GitHub transport outage
blocks publication; preserve portable bundle and exact next action if unchanged.
Existing file.expired publication is still best effort after synchronous GC
completion; a crash/pending external deletion can omit that event, as there is
no durable generic file.expired outbox. Music tombstones remain transactional.
Full c5 matrix (ambiguous prepare commit/completion commit, concurrent expiry
and GC, real Acquire at both winner barriers) remains open beyond these fixtures.
