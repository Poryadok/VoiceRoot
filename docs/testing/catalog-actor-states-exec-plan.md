# ExecPlan: protected catalog actor snapshot

## Purpose

Chat and File need an exact ownership/deletion snapshot before their own catalog
actor generation gates admit work. This is a snapshot, never an atomic permit.

## Context

Sources: DATA_MODEL.md account/profile ownership; DATA_STORES.md user_db;
ARCHITECTURE_REQUIREMENTS.md service authentication; user-service.md protected
listeners and deleted_at; TESTING.md targeted TDD; CONTRIBUTING.md Git rules.
Root authorized the separate contract; consumers Anton-93/95 and integrator67
acknowledged isolated custody. Base: reviewed PR907 a479a15d. TT child:
BE085-USER-CATALOG-ACTOR-STATES, related to both catalog consumers.

## Scope

New CatalogActorAuthority service/proto/package, exact-pair read-only SQL,
catalog-only principal support, dedicated User listener, tests and docs.
Existing Messaging snapshot API/generated files/tests remain unchanged.
No events, account policy, deployments or acceptance.

## Milestones

- [x] Agree version1, 1..100 canonical exact pairs, unique profile IDs.
- [x] Negative principal/batch tests RED before production implementation.
- [x] Implement snapshot, dedicated registration and fail-closed config.
- [ ] Run focused tests, affected short suites, proto checks and review.

## Detailed Steps

1. Add a new internal catalogactor proto with no public annotations/routes.
2. Test exact Chat/File principals, binding, forged/raw identities, replay and
   unavailable verification; reject malformed, duplicate, oversized and unknown
   request fields before storage.
3. Read id/account_id/deleted_at in one SQL snapshot; wrong owner/missing becomes
   UNKNOWN without disclosing actual owner. Errors fail the entire request.
4. Register only the new service on USER_CATALOG_PRINCIPAL_ mTLS listener.
5. Verify all states and unchanged rows with PostgreSQL tests, actual signed
   transport, no public registration, and preserve Messaging short regressions.

## Validation

User: go test -short ./...; pkg: go test -short ./socialprincipal ./principal
./pb/voice/catalogactor/v1; focused catalog transport/PG tests, go vet;
buf lint, generated parity, current diff and independent review. PostgreSQL
skips or unavailable infrastructure are reported and never called proof.

## Progress

RED: capability test failed because catalog was denied; batch tests failed to
compile before the new handler. GREEN: 15 focused User tests including actual
PostgreSQL ownership/deleted_at/nonmutation and real mTLS/JWKS/shared miniredis
transport passed. User short suite: 311 passed. Principal/catalog package short
suite: 219 passed. Messaging s2s regression: 90 passed. Affected go vet passed.
Buf lint passed. Exact final source review remains pending. Miniredis transport
tests are not a live Redis or deployed service acceptance claim.

## Decisions

Use a new exact-pair service because Chat's legacy raw-header ownership resolver
cannot establish protected gate authority. Both consumers accepted the shape.
Return ordered exact tuple echoes; UNSPECIFIED=0, UNKNOWN=1, NOT_DELETED=2,
SOFT_DELETED=3 per Buf STANDARD and closed enum. No revisions,
leases, timestamps or implied Auth account eligibility.

## Risks And Follow-Ups

Consumers must reject incomplete/extra/duplicate responses and use their own
generation barriers. UNKNOWN denies admission, never proves deletion or restore.
Cross-service zero-lag deletion and atomic permits are outside this contract.
