# ExecPlan: BE-077 and BE-078 scheduled lifecycle

## Purpose

Activate authenticated schedule creation and owner lifecycle operations with the same payload admission as immediate sends. Preserve exact retry identity across immediate and scheduled results.

## Context

Sources: `docs/microservices/messaging-service.md` sections Idempotency, Scheduled messages and Scheduled RPC shape; `docs/features/text-chat.md` Send options; `docs/DATA_MODEL.md`; `docs/DATA_STORES.md`; `docs/ARCHITECTURE_REQUIREMENTS.md`; `docs/PLAN.md` A6; `docs/TESTING.md`; `docs/CONTRIBUTING.md`.
The existing proto/Gateway/client contract and migration 000015 are shipped; Messaging currently rejects schedules. Immediate sends already have exact request receipts.

## Scope

In: UTC future <=365 days and DM-only true online schedules, shared retry namespace, owner pending list/update/cancel/send-now, atomic message/outbox and sent transition, race tests. Out: BE-079 due worker/presence polling, live DEV/STAGING and final acceptance. The frontend deployment gate remains disabled until the worker consumer exists.

## Milestones

- [x] RED validation and lifecycle tests (missing validation/store/event helpers reproduced before production implementation).
- [x] GREEN create/replay and owner pending lifecycle.
- [x] GREEN atomic send-now and concurrent repeat/cancel races; attachment cancellation/release check added.
- [ ] Focused checks, independent review, pushed PR and contract publication.

## Detailed Steps

1. Add table-driven boundary/negative validation tests before production changes.
2. Add durable schedule store with original immutable fingerprint, revision, preallocated UUIDv7 dispatch identity, owner-scoped cursor reads and conditional mutations.
3. Reuse normal send admission. Add the narrow shared SendMessage seam agreed with Anton-39; create schedules only after normal validation, before insert/outbox. Replay is serialized against immediate requests.
4. Send-now reuses full ordinary send validation and attachment acquisition. Context-bound completion hooks condition schedule transition on pending/version in both existing insert transactions; rollback removes message/outbox on race.
5. Add real PostgreSQL tests for replay conflicts, ownership, retained fields, pagination, idempotent cancellation, atomic rollback and concurrent transitions. Add API tests with existing authority fixtures.
6. Run focused tests then `go test -short ./...`, `go vet ./...`, lint and diff checks; update Graphify. Merge Anton-39's completed branch preserving ancestry before final checks.

## Validation

- `go test ./internal/grpcsvc -run Scheduled` proves input validation and RPC behavior.
- `go test ./internal/store -run Scheduled` proves real PostgreSQL persistence and transactions (never infer PASS from skips).
- `go test -short ./...`, `go vet ./...`, `git diff --check` qualify the affected module.

## Progress

- [x] TT criteria read; BE-077 and BE-078 claimed by Anton-60.
- [x] Anton-39 agreed narrow SendMessage and insert completion hook seams; migration 000038 reserved.
- [x] Validation boundaries, real PostgreSQL store lifecycle/races, bufconn owner RPC lifecycle, stable pagination and concurrent send-now checks pass.
- [x] Short module suite, `go vet`, lint and integrationtest helper checks pass.
- [ ] Independent exact-head review and hosted CI.

## Decisions

No proto changes are required. Anton-39 retains view/MarkRead/converter/hydration semantics. New lifecycle files and migration belong to Anton-60. UI owner Anton-58 consumes the existing normative contract. Automatic dispatch remains BE-079.

UUIDv7 is reserved durably at dispatch rather than schedule creation, preserving message chronology for far-future schedules; a payload/schedule update invalidates an unused reservation. Original create identity remains immutable after updates and terminal transitions. Namespace rows are removed with physical result purge so they do not retain raw sender/chat/client identifiers. The shared integration-test migration helper applies schema 000038 after 000015, ensuring old immediate-send fixtures exercise the common namespace. Schedule timestamps use PostgreSQL microsecond precision; retention tests supply exactly representable timestamps.

Independent review found two corrections: list-specific Chat authority errors must retain NotFound/PermissionDenied, and update must carry the original Space admission through durable commit. Both have discriminating RED-to-GREEN regressions. Update now uses a typed continuation inside the normal admitted send validation and the same chat/Space transaction lock; it never renews its two-second authority budget. A real PostgreSQL held-row test proves expiry leaves the original payload unchanged.

## Risks And Follow-Ups

Concurrent update/cancel/send-now must never commit a stale validated payload. Original create retry fingerprint stays immutable after update. Schedule creation must publish no event. Send-now must not reuse client_message_id. Shared locks must not starve the connection pool.
