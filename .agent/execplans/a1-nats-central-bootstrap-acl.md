# ExecPlan: A1 central NATS bootstrap and JWT ACL

## Purpose

Pre-provision every deployed JetStream durable centrally and issue service JWTs
whose Core, JetStream INFO, request/reply, delivery, pull, and ACK grants match
the reviewed topology. Hosted proof must show allowed traffic and denied
neighboring subjects under the TLS leaf/JWT hub.

## Context

- `docs/PLAN.md` A1, `docs/DEPLOYMENT.md` NATS JWT activation,
  `docs/CONTRACT_MATRIX.md` NATS matrix, `docs/TESTING.md`, and
  `docs/CONTRIBUTING.md` govern scope and verification.
- `deploy/templates/nats-{realtime,notification,search,analytics-chat}-bootstrap.yaml`
  currently provision 32 durables. The seven new runtime bind-only contracts
  are supplied by the Chat/Messaging, Bot/Matchmaking/Space/User owners.
- `src/backend/pkg/cmd/nats-jwt-fixture` accepts an ACL manifest but the repo
  has no reviewed production-intent manifest. The hosted proof uses a small
  synthetic ACL.
- The staging owner adds three stream subjects on overlapping lines; merge
  that PR before editing those lines.

## Scope

- In: central bootstrap templates, ACL intent manifest, fixture validation,
  contract tests, hosted positive/negative proof, production staging issuer,
  and NATS ACL documentation.
- Out: service runtime changes, Analytics Runner, local Windows Go/race/Compose.

## Milestones

- [ ] RED: tests require all 39 fixed consumers and exact JWT grant boundaries.
- [ ] GREEN: bootstrap supports push queue, multi-filter, pull, and policies.
- [ ] GREEN: reviewed per-service and bootstrap ACL passes fixture validation.
- [ ] GREEN: separate issuer writes no-overwrite 0600 seeds and exact four
  Kubernetes Secrets from validated external TLS inputs.
- [ ] Hosted TLS/JWT proof passes positive and neighboring denial cases.
- [ ] Branch CI passes at exact head; PR merged by merge commit; master verified.

## Detailed Steps

1. Inventory the existing 32 consumers and the seven supplied fixed shapes.
2. Add contract tests for missing/drifted durables, no app CREATE/UPDATE,
   scoped reply inboxes, exact ACK and pull grants, and bootstrap-only writes.
3. Run RED. Implement bootstrap script and manifest in small cycles, rerunning
   focused tests after each change.
4. Extend hosted proof to load the canonical ACL and exercise allowed/denied
   service requests, deliveries, and ACKs over JWT/TLS leaf.
5. Add issuer RED tests for exact Secret keys, TLS SAN/chain validation,
   no-overwrite, 0600 outputs, and a JSON List accepted by staging restore.
   Implement fresh-destination issuance with no key material in logs.
6. Merge-refresh `origin/master` after staging and runtime PRs, reconcile exact
   contracts, run focused checks, then exact-head CI and `ci-gate`.

## Validation

- [ ] `go test ./cmd/nats-jwt-fixture` from `src/backend/pkg`.
- [ ] `go test ./cmd/nats-jwt-issuer` from `src/backend/pkg`.
- [ ] NATS bootstrap/rollout contract scripts and `git diff --check`.
- [ ] Hosted `nats-jwt-leaf-hosted-proof` through CI at exact branch head.
- [ ] Final ACL and bootstrap inventory compare all deployed durables.

## Progress

- [x] Read project, TDD, worktree, git, PLAN A1, NATS docs; lease slot 7.
- [x] CodeGraph explored NATS bootstrap and fixtures; graphify queried topology.
- [x] Existing inventory: 7 Realtime, 7 Notification, 4 Search, 14 Analytics/Chat.
- [ ] Add RED contracts.
- [x] Added RED durable/ACL tests, then provisioned 40 consumers and a
  checked-in canonical ACL; fixture tests green.

## Decisions

- Service reply subscriptions use only `_INBOX.voice.<service>.>`; application
  users never receive `$JS.API.>` or consumer CREATE/UPDATE grants.
- ACK grants are bound to exact stream/durable prefixes with only a terminal
  token wildcard. Pull user uses exact MSG.NEXT API.

## Risks And Follow-Ups

- Chat/Messaging publisher connections currently need CustomInboxPrefix changes
  from their runtime owner; keep ACL scoped while those changes land.
- Hosted proof requires Linux CI; local Windows NATS/race/Compose is excluded.
- Staging restore accepts `secrets.json` as a Kubernetes List with exactly four
  Secrets in `voice-staging`, not a YAML bundle. The issuer also writes
  individual YAML manifests and a protected signing-seed backup.
