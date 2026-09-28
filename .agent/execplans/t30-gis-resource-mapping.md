# ExecPlan: T30 GIS resource mappings

## Purpose

Persist app/environment scoped game resource mappings and immutable GIS operation receipts. Exact retries must return the original result after restart; conflicting requests or mapping replacements must fail. GIS must authorize only the exact (application_id, environment_id, binding_id, chat_id) tuple from its own store.

## Context

- Frozen docs: docs/architecture/game-integration-api.md §1–2; docs/microservices/game-integration-service.md T32 ownership; docs/testing/game-integrations-exec-plan.md T30; docs/testing/game-integrations-acceptance.md T15 runtime prerequisite.
- Chat receipt source: protos/voice/chat/v1/chat.proto ProvisionManagedChatRequest/Response; src/backend/chat/internal/grpcsvc/gameintegration_chat.go; src/backend/chat/internal/store/managed_chats.go; Chat migration 000014_managed_chats.up.sql.
- GIS store: src/backend/gameintegration/internal/registry/Store; migrations in src/backend/migrations/game_integration_db.
- Current Messaging consumer seam: GameAppChatResourceAuthority in src/backend/messaging/internal/grpcsvc/game_message_processor.go.
- Constraints: no writes to Chat/Voice databases from GIS; no public route is specified for T30; frozen shared docs from PR #537 remain untouched.

## Scope

- In: additive T30 section in the GIS service owner doc, GIS-owned Postgres mapping/receipt tables, scoped operation/mapping persistence, exact tuple authorization lookup, private GIS-to-Messaging lookup route, Messaging service identity/configuration, and focused GIS tests.
- Out: Messaging client and ApplyGameMessage wiring (owned by active T16 processor/permit work), T31 party/match orchestration and Chat RPC producer, public game API, Chat/Voice writes, staging and real-provider tests.
- Namespaces are independent across app/environment; only a request whose verified app/environment differs from its declared or queried scope is denied. Same scoped external key cannot be remapped.

## Acceptance criteria

- Exact operation ID + request hash returns its persisted immutable receipt after the store is recreated.
- On uncertain Chat completion, the exact Chat operation ID and deterministic protobuf request hash are retried; Chat's returned chat ID is persisted as the creation receipt reference. No second Chat resource is created.
- A changed GIS request/hash, operation reuse with a changed request, or remapping a scoped external key to a different kind/ID/proof conflicts.
- The same external key may be independently mapped in another app/environment.
- Chat mappings reference the exact (application_id, environment_id, operation_id, request_hash, chat_id) Chat receipt. Voice mappings require a same-scope Chat receipt and chat ID.
- Authorization requires an active GIS-owned binding/chat relation and exact (application_id, environment_id, binding_id, chat_id); missing, retired, tombstoned, rejected, or mismatched scope fails closed.
- GIS never accesses Chat/Voice databases.

## Milestones

- [x] Record the T30 mapping, Chat receipt and private lookup contract in the GIS service owner doc.
- [x] Add failing tests for operation replay/conflict, scoped key isolation, Chat receipt checks, exact tuple lookup, proof rotation and mTLS identity.
- [x] Add GIS migration and store methods; make each focused test green.
- [x] Run focused GIS tests, migration/package tests and static checks; one full GIS module run passed before the final listener/migration lifecycle additions.
- [ ] Review final diff and complete normal branch/PR lifecycle on codex/game-sdk-federation-docs.

## Detailed steps

1. Confirm target branch is fetched and create a dedicated task branch from exact target.
2. Document mapping, Chat receipt, idempotency, namespace, tuple lookup, and fail-closed private service identity rules in the GIS owner doc; leave the other frozen #537 docs untouched.
3. Delegate test authoring in internal/registry/resource_mapping_test.go; review assertions before implementation.
4. Add 000011_t30_resource_mappings up/down migration and registry persistence/lookup implementation.
5. Run RED test first, then focused RED–GREEN–REFACTOR loops for retries, collisions, namespace separation, receipt proof and exact authorization tuple.
6. Implement the private mTLS listener and v1 Messaging WorkloadProof verifier with the bounded previous-key overlap; keep the public listener separate.
7. Inspect the final diff, run scoped checks, obtain one independent exact-head review, commit and push, open PR to codex/game-sdk-federation-docs, monitor hosted checks, repair, and merge with a merge commit. Never merge to master or deploy staging.

## Validation

- `cd src/backend/gameintegration && go test ./internal/registry -run 'Test(CreateResourceMapping|AuthorizeAppBindingChat)' -count=1`
- `cd src/backend/gameintegration && go test ./internal/httpapi -run 'Test(MessagingResourceMapping|MessagingWorkload|MessagingAuthorization|MessagingMTLS)' -count=1`
- `cd src/backend/gameintegration && go test . -run 'TestLoadConfigMessaging' -count=1`
- `cd src/backend/gameintegration && go test ./... -count=1`
- graphify update . after code changes.
- No real-provider or staging checks.

## Progress

- [x] Confirmed exact target and dedicated branch codex/t30-gis-resource-mapping.
- [x] Verified Chat returns a durable chat ID and persists operation ID, deterministic request hash, and receipt.
- [x] Add additive GIS service contract section.
- [x] Write/review RED tests and address the delegated test review findings.
- [x] Implement and verify GIS registry, route, rotation, configuration and migration.
- [ ] Obtain exact-head review, hosted checks, commit/push, PR, and feature-target merge.

Final local evidence on the current diff:

- Focused GIS registry tests, including migration down/up and replacement-pool receipt replay: pass.
- Focused Messaging lookup/rotation/mTLS tests and config/listener bind/shutdown tests: pass.
- `go vet ./...`, `golangci-lint run ./...` (0 issues), `go mod tidy -diff`, and `git diff --check`: pass.
- `graphify.exe update .`: exit 0; graph/report rebuilt. Existing graph HTML is over its configured visualization limit and `src/frontend/linux/runner/my_application.h` has a pre-existing parser warning.
- A full `go test ./... -count=1` run passed before the final lifecycle-only additions (registry 113.727s). The latest final-diff attempt is environment-limited: an existing Testcontainers bootstrap test panicked with `rootless Docker is not supported on Windows`; the registry package completed and passed (115.671s). Earlier full run also passed the GIS package set before lifecycle additions.

## Delegation / ownership state

- Test author: child agent `/root/t30_gis_resource_mapping_luna/t30_test_author`, model GPT-6 Luna, effort medium; exclusive file `src/backend/gameintegration/internal/registry/resource_mapping_test.go`; handed off before production edits.
- Independent reviewer: child agent `/root/t30_gis_resource_mapping_luna/t30_test_review`, model GPT-6 Luna; reviewed the corrected tests read-only and is reserved for one exact-head PR review.
- Both agents worked against the dedicated worktree `C:\Users\Sergey\.codex\worktrees\t30-gis-resource-mapping\Voice`, branch `codex/t30-gis-resource-mapping`.
- The feature target was refreshed normally; origin `codex/game-sdk-federation-docs` and branch base remain `a7b411a660600ff09603783d71a241356dc1d196`.

## Decisions

- GIS operation request hash covers the full canonical GIS mapping request, including resource kind, scoped key, target intent, and Chat receipt reference. Binding↔Chat tuples are separate T31 accepted-roster state and are excluded from the T30 mapping operation. The Chat creation request hash is stored separately and uses the exact deterministic protobuf hash from Chat.
- A Chat mapping points to its own Chat ID and immutable Chat operation receipt. A Voice mapping stores its associated Chat ID and the same-scope Chat receipt tuple.
- Cross-scope attempts are denied without revealing a foreign row; identical external key text in another app/environment is independent.
- T30 mapping writes have no runtime caller until T31 passes the actual Chat RPC response tuple into GIS. The registry unit fixture proves generated-proto request hashing and tuple validation, not an actual Chat RPC; T31 integrated acceptance owns that evidence.
- Messaging-to-GIS has no existing identity route. T30 implements a dedicated HTTPS+mTLS Messaging service identity plus body-bound WorkloadProof v1 and a minimal signed response. GIS accepts a current and one distinct previous 32-byte key for at most five minutes; no key ID is added, and the response uses the key that verified the request.
- T31's complete revisioned roster acceptance is the sole writer of `game_resource_binding_chats`; absent, expired, revoked, or incomplete roster state denies lookup.

## Risks and follow-ups

- T31 must provide a verified Chat provisioning caller and pass the actual Chat response receipt into GIS. T16 owns the Messaging mTLS client and ApplyGameMessage wiring against the published route.
- T51 must resolve recipients through GIS-owned mappings and fail closed on absent or ambiguous results.
