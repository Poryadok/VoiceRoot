# ExecPlan: T07a Controlled Game Receiver

## Purpose

Implement the test-only `src/backend/controlledgame` Go module so a signed game
command callback is authenticated and accepted exactly once across duplicate,
conflicting, concurrent, restarted, and lost-ack deliveries. A committed
acceptance atomically persists the receiver inbox, one-shot effect ledger,
terminal result, and result outbox in the isolated game database.

## Context

- Contract: `docs/architecture/game-integration-api.md`, T03/T06 command
  envelope, canonical JSON/HMAC vector, receiver transaction and retry semantics.
- Module boundary: `docs/microservices/game-integration-service.md`; T07a is a
  separate test-only backend and database, never GIS production registry or
  `game_integration_db`.
- Milestone: `docs/testing/game-integrations-exec-plan.md`, T07a.
- Verification conventions: `docs/TESTING.md`; the module has its own `go.mod`
  and PostgreSQL Testcontainers contract suite.
- Current state: branch `codex/game-controlled-backend` at test-only commit
  `50f7cc5b5dee40aafe56c8cbdf2a0a46550c7ed8`, whose parent is the specified
  feature base `859f07ff3a62c41e298847ab000945a9976e6fa7`. Accepted tests are
  independent-review CLEAR and expect RED because implementation seams are
  absent.
- Constraints: do not implement GIS permit issue/revoke, Voice Q05 challenge,
  sender retry/DLQ, production registration, Docker/Firewall runtime tests on
  this Windows host, or master-target PR/merge.

## Scope

- In: exact callback method/path/header/body/HMAC validation; PostgreSQL inbox,
  command/body idempotency and one-command-per-operation collision handling;
  transaction-scoped effect callback; immutable result and result outbox;
  rollback, lost acknowledgement, restart, and concurrent redelivery behavior.
- Out: permit authority or expiry semantics, GIS/Voice service integration,
  outbound result delivery/retries, production configuration, and unrelated
  game behavior.
- Receiver callback bodies are bounded at 64 KiB. The receiver checks the cap
  before authentication and returns 413 when the body exceeds it; a body of
  exactly 65,536 bytes proceeds through normal authenticated handling.
- Signing credentials are provisioned per exact app, environment, and
  installation. Current credentials are accepted; overlap credentials are
  accepted only before their configured not-after timestamp; revoked and
  unknown credentials fail closed. GIS owns provisioning and its ten-minute
  overlap limit.
- A consumed one-shot action key is SHA-256 of UTF-8
  `voice-controlled-game-effect-v1\n{installation_id}\n{message_id}\n{action_id}\n{actor_proof.profile_id}`
  from the signed envelope. A distinct command or operation colliding on this
  key returns 409 without returning another command's receipt.
- Documentation gap: none for this receiver slice; the wire and durable
  receiver semantics are frozen in the API document.

## Milestones

- [x] Reconfirm accepted tests fail at missing implementation seams.
- [x] Implement canonical request authentication and focused non-container
  tests pass.
- [x] Align result fixtures to the complete frozen T03/T06 result envelope and
  receive fresh test review for that fixture delta.
- [x] Review raw-route/method and duplicate header tests before implementing
  those handler behaviors.
- [x] Implement PostgreSQL schema/store and transactional replay/conflict
  semantics.
- [x] Implement callback handler and pass short/handler tests.
- [x] Add `controlledgame` and the adjacent `gameintegration` selector to Go
  path filters, resolver, Makefile targets and CI tidy/integration lists.
- [x] Add a test-only, non-deployable controlledgame Dockerfile and exclude
  test-only changes from staging build, image promotion and rollout matrices.
- [x] Verify final focused non-container checks locally; use hosted Linux CI for the
  Docker-backed integration suite.
- [ ] Update T07a owning docs with implementation and exact verification
  evidence; independent implementation review clear.

## Detailed Steps

1. Preserve the accepted test-only commit and run
   `go test -short ./...` from `src/backend/controlledgame` to record the
   expected compile RED. Do not start local Docker-backed tests.
2. Implement `authenticateRequest` to match the frozen method/path/header,
   canonical timestamp/key UUID, +/-300 second skew, exact JCS body bytes and
   constant-time HMAC vector. Reject duplicate auth/content-type headers and
   any noncanonical origin-form path before routing.
3. Add a receiver-owned PostgreSQL migration/bootstrap and `OpenPostgresStore`.
   Store unique command ID/body hash, operation ID uniqueness, effect/one-shot
   key, canonical result bytes and result outbox. Keep schema in the separate
   module's dedicated test database.
4. Add `NewHandler(HandlerConfig)` and an explicit transaction-aware `Apply`
   callback. Authentication/validation precede transaction work. Same
   command+body returns the saved 202 receipt; same ID/different body and a
   different command for the same operation return 409. Any effect failure
   rolls back inbox/ledger/result/outbox together.
5. Run non-container tests and `go test -short ./...`; record that the
   PostgreSQL contract cases are skipped in short mode. Do not run forbidden
   Windows Docker/Firewall tests; hosted Linux CI supplies the full proof.
6. Update the T07a status/evidence in the owning execution plan and service
   docs, run formatting/diff checks, then obtain an independent code review.

## Validation

- [x] `cd src/backend/controlledgame && go test -short ./...` — local unit and
  HTTP contract checks; no Docker-backed execution.
- [x] `cd src/backend/controlledgame && go vet ./...`.
- [ ] Hosted Linux CI full `go test ./...` — PostgreSQL Testcontainers must run
  with zero skips and cover replay, operation collision, rollback, lost ACK,
  restart, and concurrent duplicate delivery.
- [x] `git diff --check`.
- [x] `golangci-lint run ./...`.
- [x] `go mod tidy -diff`, Git Bash `-n` for resolver/test scripts, YAML parse,
  and `git diff --check`.

## Progress

- [x] Read contract, service boundary, milestone, testing and TDD instructions.
- [x] Acquire fresh Treehouse slot 13 and verify exact branch/head/base ancestry.
- [x] Record expected RED; implement request authenticator and pass its focused
  direct-file suite.
- [x] Update test result fixture to the full documented result envelope and
  obtain the requested fresh test review before further production changes.
- [x] Add handler boundary tests and obtain fresh test review before handler.
- [x] Implement store and handler in small cycles.
- [x] Run local short tests, vet and diff checks; hosted PostgreSQL proof and
  independent implementation review remain.
- [x] Wire path-filtered CI selectors, Makefile Go module lists, CI tidy and
  integration matrices, and generic test-runner image build support.
- [x] Verify `controlledgame` is excluded from the staging image catalog and
  test-only changes produce empty build/promotion matrices by code inspection.
- [x] Address review findings with expiry-at-boundary, scoped credentials,
  one-shot cross-operation effect key, 64 KiB request bound, and `ci_global`
  selector separation from deployment-global inputs.
- [ ] Run selector shell tests; local execution is blocked because WSL has no
  `/bin/bash` and Git Bash lacks `jq`. CI runner has both dependencies.

## Decisions

- Use the exact accepted API and test vector; do not canonicalize/normalize
  request bytes before checking that they equal the frozen JCS representation.
- One command per operation is enforced by a unique operation key, returning
  conflict for a second command ID even when its body is otherwise valid.
- One-shot action consumption also has a distinct durable effect key derived
  from installation, message, action, and actor profile. This prevents a
  different operation and command from consuming the same action twice.
- `.github/ci/**`, `.github/workflows/**`, `scripts/ci/**`, Makefile, and
  golangci configuration use the `ci_global` selector. It widens CI without
  marking changes as deployment-global. CI-only and controlledgame-only
  changes yield empty staging build/promotion matrices and no rollout.
- The game effect is supplied through a transaction-aware callback. It returns
  the complete canonical result envelope (not opaque status bytes); the receiver
  validates its IDs/shape and persists that exact body with the inbox and
  result outbox. This lets the harness prove transaction sharing without adding
  actual game or GIS authority behavior.
- The frozen request contract rejects non-integer numeric values. The test-only
  canonicalizer therefore accepts JSON numbers only when they are exact safe
  integers within the IEEE-754 binary64 integer range (±9007199254740991),
  matching RFC 8785's binary64 data model without rounding signed request data.
  `TestAuthenticateRequestRejectsNonIntegerNumbersInArguments` covers the
  fractional-number rejection explicitly.
- Local full integration execution is withheld because the dispatcher forbids
  Windows Docker/Firewall tests; hosted Linux is the acceptance environment.
- Focused non-container tests cover `now == expires_at` => 410, exactly 65,536
  bytes reaching a fake acceptor, and 65,537 bytes => 413 before signature
  verification. Credential tests cover current, active/expired overlap,
  revoked, unknown key ID, and app/environment/installation mismatch.
- Current local checks: `go test -short ./...` (34 passed), `go vet ./...`,
  `golangci-lint run ./...`, `go mod tidy -diff`, script syntax checks, YAML
  parse and `git diff --check` pass. The Testcontainers cross-operation
  collision test remains hosted-Linux evidence.

## Risks And Follow-Ups

- The full integration contract cannot be claimed green until hosted Linux CI
  runs the Testcontainers cases without skips.
- GIS permit mint/revoke ordering, Voice challenge, and sender retry/DLQ remain
  explicit later slices.
