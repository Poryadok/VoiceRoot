# ExecPlan: Space audit REST read

## Purpose

Expose the documented authenticated Space audit query over REST without allowing
raw identity headers or the ordinary Space listener to bypass delegated identity.

## Context

The API and disclosure source is [Gateway A2](../microservices/api-gateway.md#a2-space-rest-contract-target).
The ledger/cursor source is [Space](../microservices/space-service.md).
BE-132 commit `f80640a6479639cac91a622942b3e8a7aa01faf8` supplies reviewed
gRPC filter mapping and filter/viewer/key cursor binding. This branch consumes
that immutable dependency; BE-132 remains responsible for its own delivery.
Gateway currently exposes no audit route. Space's existing delegated listener
verifies the exact request, Gateway issuer, audience, session epoch and replay.

## Scope

GET `/api/v1/spaces/{space_id}/audit-log`, strict query validation, exact signed
request, protected Space listener cutover and existing current membership/Role
checks. No mutation, ledger writer, composite Chat creation, lifecycle activation,
deployment or final acceptance is included. Remaining BE-008 scope stays open.

## Milestones

- [x] Reuse reviewed BE-132 dependency and isolate WT40.
- [x] Demonstrate absent REST route and ordinary-listener bypass regressions.
- [x] Implement strict REST mapping and protected read cutover.
- [ ] Verify negative transport and affected modules; obtain independent review.
- [ ] Required exact-head CI and merge after GitHub transport recovery.

## Detailed Steps

1. Add REST regressions for optional presence, UUID, timestamp, page range,
   duplicate/unknown queries, no-store response and no ordinary fallback.
2. Reuse Gateway lifecycle signing and error redaction; map fields before hashing.
3. Allow GetAuditLog on the delegated listener, reject it on the ordinary listener;
   retain current Space membership and exact SPACE_VIEW_AUDIT_LOG checks.
4. Run focused RED/GREEN, Gateway/Space short tests and vet. Reuse BE-132's
   PostgreSQL proof recipe and extend protected transport evidence where needed.
5. Review source and tests together for ACL/privacy risk. Preserve exact commits,
   CI/review references and resumable delivery custody in TT.

## Validation

- `go test -short -run '^TestSpaceAuditRead' .` in Gateway proves REST transport.
- `go test -short ./internal/lifecycleprincipal ./internal/grpcsvc` in Space
  proves protected read authorization and affected contracts.
- Affected module short tests, vet and existing hosted PostgreSQL audit proof.

## Progress

Meaningful RED observed REST 404 instead of 200/400/503, ordinary audit reaching
the handler, and signed listener rejecting valid audit calls. GREEN: Gateway
750 short tests and Space 390 short tests; both module vet checks passed.
REST empty serialization and no-store behavior were corrected against the
contract. Docker daemon is unavailable, so explicit hosted PostgreSQL proof
remains required. The 27 focused Gateway cases also passed `-race`. Workflow YAML
parsed and `git diff --check` passed. No runtime proof claimed.

## Decisions

Use the existing delegated listener and issuer instead of adding a rival signer.
The REST route fails unavailable when that transport is not configured, never
falls back to raw metadata. Unknown audit actions remain valid query values.

## Risks And Follow-Ups

GitHub SSH/API outage blocks remote head readback, publication and hosted checks.
Local PostgreSQL proof availability must be checked before claiming integration
execution. Ordinary-listener audit consumers must migrate to the signed surface;
the Flutter audit vertical is a separate BE-133 consumer, still open.
