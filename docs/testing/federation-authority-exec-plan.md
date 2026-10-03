# ExecPlan: Federation authority foundation

## Purpose
Implement a running, fail-closed master Federation authority service for the game sprint. Nodes enroll, operators approve identity, and only an active node with its registered mTLS certificate plus expiring credential can obtain its placed Space snapshot and a signed short authority lease.

## Context
Canonical sources: game-integrations-exec-plan T70–T72; architecture/game-federation sections 3–7; DATA_MODEL; TESTING; CONTRIBUTING. The specification branch supersedes old deferred/fallback prose. Existing Federation is health-only. Parent approved dedicated mTLS HTTPS v1 API. No master merge or staging deployment.

## Scope
In: service-owned PostgreSQL migration; enrollment/approval/status/rotation; immutable placement; complete typed signed snapshot; applied revision ACK lease; TLS runtime and Compose/production wiring. Out: game-node bundle, live revision stream, owning-service publishers, user grants, media admission/enforcement. Named activation consumer: future Voice Node policy projection and media verifier T72–T74; capability remains unadvertised until those integrate.

## Milestones
- [x] Freeze executable wire/security contract and tests.
- [x] Implement authority store and handlers; prove negative cases.
- [x] Wire deployment/migrations and run scoped checks.
- [ ] Obtain exact-head independent security review and resolve findings.
- [ ] Commit/push and draft PR to codex/game-sdk-federation-docs.

## Detailed Steps
1. Record versioned HTTPS requests, Ed25519 envelope and denial semantics in federation-service.md.
2. Write signature, strict schema, certificate/credential and stale revision tests before production code; observe failure.
3. Implement PostgreSQL transactions locking node then placement for coherent revoke/publish/lease; append-only placement and revision CAS.
4. Run go test ./..., go vet ./..., golangci-lint, and YAML parsing for the changed deployment manifests. The PostgreSQL lifecycle test starts a local PostgreSQL testcontainer; hosted CI is separate evidence.
5. Run graphify update, inspect diff, fresh read-only review, fix findings; push and draft PR.

## Validation
Negative tests: unsigned/tampered/wrong audience/expired envelope, unverified certificate, operator/node confusion, stale revision, foreign Space, suspended/revoked credential, replayed lease request, conflicting same revision. PostgreSQL tests verify durable restart and transaction constraints.

## Progress
- [x] Sources read; no production behavior exists.
- [x] Parent accepted HTTPS technical choice.
- [x] Added regression coverage before fixes for expired intermediate chains and invalid rotation credential exposure; red phase observed.
- [x] Added database coverage for 23505 duplicate-pin conflict and credential preservation, concurrent same-nonce issue, and issue/suspension serialization.
- [x] Scoped verification passed locally: `go test ./...` (31 tests, including PostgreSQL testcontainer), `go vet ./...`, `golangci-lint run`, `git diff --check`, and parsing all six changed staging/production YAML files.
- [x] Federation remains dormant in Kubernetes (`replicas: 0`) and has no Gateway route until the consumer exists.
- [ ] Exact-head independent review, commit/push, and draft PR remain.

## Decisions
- TLS 1.3 with mandatory CA validation; operator certificate SHA-256 pins separate from registered node pins. No forwarded certificate headers.
- Ed25519 signs exact base64url-decoded JSON bytes; issuer, audience, environment, node, Space, generation, epoch, revision and expiry are mandatory.
- Lease max 2s, renewal 500ms target, zero expiry grace; source authority expires within 5s and must be refreshed by future owning-service aggregator. This is a proposed budget, not measured media acceptance.
- Credentials are 32 random bytes, hash-only at rest, expire after 24h; rotation revokes old immediately (no overlap in foundation). Enrollment is operator mediated with endpoint ownership attestation; service does not fetch arbitrary endpoints.
- One bounded complete snapshot (1 MiB), page_count=1, complete=true. Future paging/stream requires separate schema and consumers.

## Risks And Follow-Ups
Full Federation acceptance is not claimed. A live authority publisher and node/media consumers remain prerequisites; old LiveKit bearer reconnect is not solved by this PR. Local PostgreSQL testcontainer passed; hosted CI evidence is pending. Production manifests require a pre-provisioned restricted database login and explicit operator secrets/certificates before activation.
