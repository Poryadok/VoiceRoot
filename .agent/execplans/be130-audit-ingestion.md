# ExecPlan: protected audit ingestion

## Purpose

Space accepts authenticated Role/Chat administrative facts once. Exact retries
acknowledge the original effect; changed facts cannot replace it or resurrect a
retained audit entry. Producers and ingress share one frozen details codec.

## Context

- Behavior: `docs/microservices/space-service.md`, audit registry/details/retention;
  `docs/ARCHITECTURE_REQUIREMENTS.md`, signed principal transport.
- Decisions: TT voice BE-130 c3 BE130-REPLAY-v1 and c5 BE130-DETAILS-JSON-v1.
- Data ownership: `docs/DATA_STORES.md`, `docs/DATA_MODEL.md`.
- Existing ledger/outbox migration 000015; public AppendAuditEvent is unimplemented.
- Author13 owns WT31. BE131 author44 consumes shared `pkg/spaceaudit` and does not
  edit it. BE132 owns audit cursor/filter files; ingestion uses additive files.

## Scope

Shared Role/Chat codec/registry/replay hash, immutable ID receipt migration,
authenticated ingestion and actual negative/concurrency/integration evidence.
No producer domain mutations (BE131), public audit UI, P3 activation or deployment.

## Milestones

- [x] Freeze accepted c3/c5 behavior in owning canon and shared codec fixtures.
- [x] Shared codec/validation/replay hash passes meaningful acceptance cases.
- [x] Same-transaction global ID reservation and audit/outbox implementation;
  actual PostgreSQL replay/concurrency proof remains pending.
- [x] Signed Role/Chat-only listener, body validation and no legacy fallback.
- [ ] Independent security/concurrency review and exact-head hosted checks/PR.

## Detailed Steps

1. Add `src/backend/pkg/spaceaudit` registry, typed details builders, strict
   validation and versioned replay frame. Test first; give author44 the local
   immutable commit/API so producer work needs no duplicated codec.
2. Add global ID receipt backfill/trigger under migration lock. Preserve existing
   outbox atomicity, compensation and hard-delete guards. No FK cascade or receipt
   TTL; external receipt contains trusted origin/version/digest, not copied payload.
3. Add store ingestion in its own file. Authenticate/validate before lookup;
   receipt collision rolls back and uses fresh committed read to choose exact
   replay versus conflict. Never unconditional success after DO NOTHING.
4. Wire exact signed service transport using existing principal/JWKS/replay
   infrastructure. Reject unknown request fields, raw/duplicate metadata, foreign
   callers, request binding failures and action ownership mismatch before writes.
5. Run affected shared/Space tests and lint; real PostgreSQL tests prove concurrent
   duplicates/conflicts, rollback, retained receipt replay, local/external collision,
   backfill and compensation. Hosted evidence replaces unavailable local Docker.

## Validation

- Focused `go test ./spaceaudit -count=1` and `go vet ./spaceaudit` in pkg.
- Space affected short tests/vet; real PostgreSQL ingestion/migration tests without
  skips; signed gRPC positive/negative tests; race checks for concurrent receipt use.
- Frozen fingerprint vectors include negative/year9999 seconds and nanos differing
  within one database microsecond. Every body field and origin has conflict coverage.
- Required exact-head CI and independent review before normal merge.

## Progress

- [x] Required instructions, behavior docs and c3/c5 decisions read.
- [x] Reconciled lease31 and old fetch: CLI reports matching lease and no processes;
  old handle unavailable, tracked tree clean; no replacement network probe.
- [x] Shared details/replay tests: meaningful RED on stub behavior, GREEN focused
  package, scoped vet and golangci-lint (0 issues). Frozen digest vectors were
  generated independently with Python struct framing, not implementation output.
- [x] Receipt migration/store and protected listener implemented. All 31 transport
  tests passed, including real local mTLS/JWKS, JWT replay and Redis fail-closed;
  facade meaningful RED then GREEN. Space short suite 402 tests and vet passed
  before the final additional PostgreSQL fixtures.
- [ ] PostgreSQL fixtures compile under short mode but remain unexecuted: local
  testcontainers failed before domain execution (rootless Docker unsupported on
  Windows). Dedicated exact-head hosted workflow requires named PASS/no SKIP for
  wire-to-store, concurrency, retention, backfill, uint64 and migration roundtrip.

## Decisions

- Deterministic protobuf remains the credential hash, not the durable replay codec.
- Frozen replay frame hashes original validated strings and Timestamp seconds/nanos
  before SQL/JSONB normalization; trusted producer is compared separately.
- Receipts outlive ordinary 365-day payload retention until authorized terminal
  purge. Pre-migration already-deleted IDs cannot be reconstructed.
- Canonical details are a restricted ASCII codec for external Role/Chat facts;
  they do not reinterpret existing Space-local details or impose new mask quotas.

## Risks And Follow-Ups

Author18 assigned receipt-only bounded terminal minimization to BE130 after
accepted composite c4/c5: migration29 + cleanupAuditIngressEvidence(ctx,tx) erases
external full fingerprints only at permanent Space PURGED+30d, leaving nonpayload
global ID/Space/origin markers. Guard denies early/arbitrary mutation and rollback
of minimized payloads. Helper uses bounded1000-row SKIP LOCKED batches; author18
owns its call in common CleanupLifecycleEvidence. Actual PG guard/rollback and
race cases remain required. JWKS compatibility fixture now requires client mTLS
as the real Role/Chat endpoints do; all31 transport tests PASS, including failure
of a CA-only JWKS client. Graphify15902 deliberately interrupted after AST183/183
because this accepted follow-up changes its input; no final success is claimed
for that obsolete update, and final graph refresh follows the complete source.

Accepted composite decision BE-A2-COMPOSITE-CHAT-NODE c4/c5 is integrated from
author18's coherent immutable6678fee dependency (protocol86f48e6 and sharedcodec
ancestor). Migration000028 and checkAuditIngressLifecycle now validate the exact
persisted COMMITTED binding during freeze/purge and permanent terminal Space
fence before both receipt lookups, including fresh collision resolution. A new
PostgreSQL test covers changed binding, frozen original drain and a concurrent
terminal fence blocking even receipted retries. Full Space short424 and vet pass;
real PostgreSQL proof and independent review remain pending. Ordinary replay
c3/c5 is unchanged. This is source integration, not P3 activation or deployment.

Security/concurrency/migration change requires independent review. Current GitHub
outage blocks remote delivery/checks but not local development; do not duplicate
uncertain pushes/probes. P3 must later include compact receipts in its explicit
authorized minimization/fence protocol. No live environment or acceptance authority.
