# ExecPlan: User-authoritative Search profile projection

## Purpose

Make Search's profile projection converge from User's durable, revisioned authority after JetStream retention loss, without deriving normalized keys from raw profile values.

## Context

- Docs: `docs/microservices/user-service.md`, `docs/microservices/search-service.md`, `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`, `docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/TESTING.md`, and `docs/CONTRIBUTING.md`.
- Frozen delivery contract: the task brief is the contract supplement for this vertical. It fixes protocol v1, User-owned positive contiguous `source_revision` and gap-free `journal_offset`, transactional journal/outbox, Search-only TLS principal on `:9093`, and revisioned Search inbox/projection semantics.
- Current base: `b76599da` (`#427`), containing normalized profile keys/backfill. Search currently serves the legacy generation; no public Gateway route is in scope.
- Constraints: User remains historical authority; Search accepts only verified normalizer v1 keys; no raw-key fallback; older binaries must continue during expand-first migration; Windows does not supply local Compose/Realtime/race evidence.

## Scope

- In: User schema/proto/mutations/journal/outbox/baseline; protected snapshot/journal/checkpoint RPC; Search TLS client, inbox and conditional projection apply; migrations/config/deployment documentation; RED, unit, contract and hosted Compose coverage.
- Out: shadow generation bootstrap/activation/rollback, ranking changes, public Gateway surface, raw-key fallback, and A4 erasure policy.
- Documentation gap: service docs do not yet describe this frozen contract. This PR must document deployed configuration and service contract alongside code rather than infer alternatives.

## Milestones

- [ ] Define expand-first protobuf and PostgreSQL storage for User journal/outbox and Search inbox/projection.
- [ ] Establish RED tests for atomic mutation/outbox, PubAck recovery, protocol/key validation, and revision conflict/tombstone behavior.
- [ ] Implement User authoritative writer, leased publisher, baseline and protected TLS RPC.
- [ ] Implement Search protected client/replay and conditional projection apply.
- [ ] Add contract/config/deployment documentation and generated artifacts.
- [ ] Run focused verification, push PR, obtain exact-head hosted CI including `ci-gate`, merge-commit, verify `origin/master`, and return the Treehouse lease.

## Detailed Steps

1. Locate current User mutation/store and Search profile index seams; use existing protected-principal implementations as the TLS/replay template.
2. Add tests first, confirm each new test fails for a missing behavioral seam, then minimally implement and re-run each focused package.
3. Add versioned event/RPC messages and regenerate Go/Dart code; run `make buf-ci` and compatibility checks.
4. Make every profile create/update/delete transaction write one complete immutable journal event and an outbox record with stable event ID and protobuf bytes; publish leased records with `Nats-Msg-Id` and mark delivery only after PubAck.
5. Expose only Search's TLS-protected snapshot/journal/checkpoint RPC on `:9093`, enforcing the request-bound JWT/replay/TLS contract before User store access; implement signed opaque snapshot cursor pagination.
6. Implement Search bootstrap/replay client and durable inbox. Apply higher revision only; no-op stale or identical payloads; quarantine equal revision with different hash; atomically tombstone DELETE to prevent stale resurrection.
7. Add resumable baseline v1 journal/outbox generation and deployment/configuration notes; preserve legacy serving.
8. Update this plan with concrete files/tests/results, update Graphify after source edits, then complete PR/CI/merge lifecycle.

## Validation

- [ ] Focused User and Search Go tests: atomic write, crash-before-PubAck, TLS/principal/config rejection, cursor/replay, normalizer/key rejection, duplicate/out-of-order/conflict, and delete no-resurrection.
- [ ] `make buf-ci` and generated-code checks after proto changes.
- [ ] Hosted CI exact PR head: required jobs and `ci-gate`, including User→Search protected Compose evidence.
- [ ] Post-merge `git fetch origin` and verification that local/master equals `origin/master`.

## Progress

- [x] Treehouse lease acquired: `D:\Git\Voice\.treehouse\Voice-d5628e\3\Voice` (`6fc3acc25074d0cdb27b317085d71840`).
- [x] Branch created from `origin/master`: `feature/search-projection-authority` at `b76599da`.
- [x] Read project workflow and service documentation; CodeGraph located User/Search seams.
- [x] Completed RED→green validator cycle for missing, unsupported and mismatched v1 normalized keys; Search uses `pkg/searchnormalization.V1` and has no raw-key fallback.
- [x] Completed RED→green conditional-apply cycle for higher revision, exact duplicate, same-revision conflict, DELETE tombstone and stale resurrection prevention.
- [x] Ran `graphify update .` after source changes.
- [x] Added expand-first User journal/outbox and Search inbox/checkpoint migration artifacts.
- [x] Added generated v1 User protected snapshot/journal/checkpoint protobuf surface (ordinary listener remains unimplemented).
- [x] Wire User profile creates, updates and soft deletes to transactional journal/outbox writes. `profiles.search_projection_revision` is incremented in the same transaction; primary onboarding insertion is included before commit.
- [ ] Wire Search projection storage to the protected transport/replay consumer.

## Decisions

- The user-supplied frozen contract resolves the absent service-doc detail for this vertical; no alternate public or raw-normalization path will be introduced.
- This is a single atomic PR because User history, protected transport and Search conditional projection only form a safe consumer together.
- Profile mutations use a transaction-aware store API for every projected create/update/delete path; invoking the journal helper after an existing store call remains prohibited because it would create orphanable state.
- `ProfileStore` now owns the mutation transaction and appends a v1 upsert/delete before commit. `search_projection_transaction_integration_test.go` uses a forced outbox-trigger failure to prove an update rolls back with zero journal/outbox rows. The focused integration test requires hosted Linux CI because Windows rootless Docker panics before it can start PostgreSQL; local `go test ./... -short` passed (139 tests, 25 packages).
- PR A adds the expand-only User `ACCOUNT_INACTIVE` inbox/overlay and keeps its Auth deletion consumer default-off. PR B may enable it only after dedicated broker credentials/ACLs (Auth exact-subject publish; User subscribe only), a staging/prod migration-job proof for `000017_account_lifecycle_search_tombstone`, and a canary; no current manifest activates it.

## Risks And Follow-Ups

- The vertical is broad and spans User, Search, protobufs, deployment and CI. Hosted CI is required for protected Compose proof on Windows.
- Shadow-generation activation and rollback remain explicitly deferred, so legacy Search serving must stay intact.
