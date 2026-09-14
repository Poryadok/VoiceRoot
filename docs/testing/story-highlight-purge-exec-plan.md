# ExecPlan: permanent Highlight retention and archive media deletion outbox

## Purpose

An expired Story that belongs to any Highlight remains readable through that Highlight with its media intact. Archive cleanup deletes ordinary expired Stories after 30 days, but it must never delete media and then retain a Highlight link. Once the final Highlight link is removed, ordinary cleanup may delete the Story and schedule idempotent File deletion.

## Context

- Product lifecycle: `docs/features/stories.md` («Жизненный цикл стори», «Архив и Highlights»).
- Service ownership: `docs/microservices/story-service.md`, `docs/DATA_STORES.md`; Story Service owns `story_db` and calls File Service for R2 deletion.
- Current code: `src/backend/story/internal/store/store.go` and `src/backend/story/internal/jobs/jobs.go` perform `list -> FileDeleter -> delete`, which leaves a TOCTOU window between selecting a story and adding it to a Highlight.
- Architecture decision: `tmp/slave-driver/story-highlight-purge-sol-contract.md` (2026-09-14), which is authoritative for this change.
- Constraints: no local Testcontainers, Compose, Realtime, race, or localhost harnesses. Database integration coverage runs in hosted CI.

## Scope

- In: Story-owned migration, transactional logical purge, Highlight mutation locks, durable outbox leasing/dispatch, Story docs, and deterministic integration/fault coverage.
- Out: changes to Story audience rules, Highlight visibility, File Service API/proto changes, R2 policy changes, and cross-service database access.
- Documentation gap resolved by Sol: a Highlight permanently retains its member Story and media until all Highlight memberships are removed.

## Milestones

- [x] Record Sol’s DB-first/outbox contract and add RED documentation/test contracts.
- [x] Add `story_db/000004_archive_purge_outbox` and load it in all Story integration migration helpers.
- [x] Replace pre-delete File calls with one transactional, bounded `StageArchivePurgeBatch`.
- [x] Make Highlight add/remove/delete membership transactions lock affected rows in UUID order.
- [x] Add a leased outbox dispatcher with retry/backoff and idempotent File deletion.
- [ ] Pass hosted CI integration coverage and independent review.

## Detailed Steps

1. Keep the current branch intact while tests are reviewed. The RED phase adds only this plan, docs, and test contracts.
2. Add migration `000004_archive_purge_outbox`: `highlight_stories(story_id)` index; partial expired Story purge index; `story_media_deletion_outbox` without an FK to `stories`, immutable operation ID equal to the Story ID, File ID, availability, attempt/error fields, timestamps, lease token/until, and ready-claim index.
3. Implement `StageArchivePurgeBatch` in one `story_db` transaction using PostgreSQL time, a bounded ordered candidate query and `FOR UPDATE OF stories SKIP LOCKED`. It inserts media operations then rechecks eligibility while deleting; any mismatch rolls back. Text-only Stories have no outbox operation.
4. Make Highlight membership mutations lock the owning Highlight and affected Stories in UUID order. Add blocks behind a staged purge and returns NotFound after purge commits; purge skips an Add-held lock and observes its committed link. Remove and Highlight deletion serialize the final-link transition.
5. Replace `RunArchivePurgeOnce` pre-delete File calls with staging plus a startup/short-interval dispatcher. A dispatcher claims ready or expired rows using a token/fenced lease, calls File only after the Story is absent, deletes only its own completed lease, and backs off failures or ambiguous outcomes with capped exponential DB-time delays. Every File call has a bounded deadline, so an unresponsive client releases its lease for a later tick. File deletion is idempotent by immutable File ID.
6. Update `migrationSQL` helpers so hosted Story integration tests apply migration 000004. Keep every race test channel/barrier driven; do not use sleeps.

## Hosted integration coverage

The following tests are executable hosted integration coverage. They use explicit database/dispatcher barriers and direct DB-time transitions; no test sleeps or starts a local harness.

| Scenario | Fixture/barrier | Green assertion |
| --- | --- | --- |
| Post-commit crash | Stage a media Story, commit, then simulate process exit before dispatch; create a fresh dispatcher from the same `story_db`. | The Story remains absent and exactly one durable operation is claimed and delivered after restart. |
| File failure and ambiguous result | A File deleter returns a definite error, then an error after recording the immutable File ID as deleted; each attempt is released through a channel. | Both attempts persist retry/backoff state; a later idempotent call completes the operation without restoring or relinking the Story. |
| Two dispatchers and stale acknowledgement | Dispatcher A claims and pauses. Advance injected DB time past its lease; dispatcher B claims the same operation and completes. Then release A. | A’s stale token cannot delete or acknowledge B’s lease; only B’s completion is terminal. |
| Final unlink concurrent with add | An archived Story is in two Highlights. Remove one link, then gate the transaction which removes the final link while another Add is ready. | A committed Add retains the Story/media; a committed purge makes the Add `NotFound`; neither interleaving leaves a link to deleted media. |
| Add-first concurrent with purge | Commit Add through an `addCommitted` channel, then release the purge scanner through `purgeAttempted`; FileDeleter exposes `fileCallStarted`. | Purge skips the linked Story and `fileCallStarted` receives no event; Story/media/link remain intact. |
| Text-only and scanner rerun | Stage text-only and media Stories, then invoke the scanner twice with the same DB time. | Text-only deletes with no outbox row; media creates one immutable operation and rerun creates no duplicate. |
| Unresponsive File call | A non-cooperative deleter blocks behind a channel while the dispatcher deadline expires; a second tick through that same deleter runs while the call remains in flight. | The runner returns, persists `DeadlineExceeded` retry state, releases the lease, and the second tick persists a retry without spawning another blocked File call. An independent dispatcher can still reclaim an expired lease. |

## Validation

- [x] `go test -short ./...` from `src/backend/story` compiles and passes after implementation.
- [ ] Hosted `backend-go-integration-pr (story)` runs full Story integration tests, including deterministic add-first/purge-first, final-unlink/add, two-dispatcher, lease reclaim, failure/ambiguous/post-delete crash, text-only, and callback-after-logical-delete cases.
- [ ] Hosted `backend-go (story)`, `golangci`, and PR `ci-gate` are green for the exact PR SHA.
- [ ] Independent review confirms docs, migration, locks, outbox and tests agree with Sol’s contract.

## Progress

- [x] Earlier guard restricted adding a Story to Highlights until it entered the archive.
- [x] Independent review found the FileDeleter-before-delete TOCTOU; prior simple `NOT EXISTS` guard is insufficient.
- [x] Sol selected DB-first logical purge plus Story-owned outbox.
- [x] Added executable hosted integration coverage for migration, stage/restart, callback-after-logical-delete, retries, stale leases, final unlink/Add, text-only, scanner rerun, and a non-cooperative File deadline; local full integration execution is intentionally prohibited.
- [x] Independent review accepted the completed production/test cycle before hosted CI.

## Decisions

- DB-first logical purge: it makes purge and successful Highlight add mutually exclusive at transaction commit, so a Highlight never retains media deleted by the cleanup worker.
- Story-owned outbox: File/R2 side effects happen after the DB commit and survive process crashes; it is at-least-once and File deletion must be idempotent.
- PostgreSQL locks/DB time: provide the required linearization and avoid local-clock or timer-based race tests.

## Risks And Follow-Ups

- An existing migration helper currently reads only 000001–000003; implementation must update every Story integration package before hosted full tests can validate the new schema.
- The prior guard commits remain on the branch and must be replaced, not layered with another pre-delete File call.
- CI is the only permitted environment for Testcontainers coverage under the fleet restriction.
