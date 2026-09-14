# Story File protected media validation

## Purpose

Complete PR #368: Story publishes photo/video only after File validates the
author-owned unscoped upload through a request-bound protected RPC. Rejected
requests create no Story row or event.

## Context

Sources: docs/features/stories.md admission contract,
docs/microservices/story-service.md, docs/microservices/file-service.md,
docs/ARCHITECTURE_REQUIREMENTS.md Phase-0, docs/DATA_MODEL.md,
docs/DATA_STORES.md, docs/TESTING.md and docs/CONTRIBUTING.md.
The authoritative prior security decision is preserved in the captain's
tmp/slave-driver/handoff/pr340-story-file-sol-contract.md. PR #368 contains
additive protobuf/generated contracts and intentionally failing handler tests.

## Scope

File TLS listener, strict service verification/JWKS/replay, atomic locked File
predicate; Story closed input validation, signed client/JWKS, failure mapping,
deployment configuration and regression coverage. Exact Story reference claim
and LFP media-kind policy remain the explicitly documented followups; nonempty
LFP media returns FailedPrecondition. No foreign DB reads or metadata fallback.

## Milestones

- [x] Refresh existing draft by merge from master, retain history.
- [x] Correct and complete RED evidence and independent review.
- [x] File predicate and protected transport GREEN.
- [x] Story signed admission, configuration and no-write matrix GREEN.
- [x] Deployment contracts and independent security review.
- [ ] Hosted CI and merge: tracked by the live PR #368 head/checks and merge state.

## Detailed steps

1. Review existing RED tests with independent read-only author/reviewer.
2. Implement File validation in one transaction selecting all predicates with
   FOR SHARE; real PostgreSQL lock barriers prove concurrent mutation ordering.
3. Reuse existing principal primitives and service runtime conventions for TLS,
   HTTPS two-key JWKS and Redis replay. Ordinary listener denies this RPC;
   protected listener denies all other RPCs. Reject raw identity and invalid
   request binding before store access.
4. Replace Story duration metadata admission with explicit ValidateStoryMedia
   adapter. Sign fresh service tokens, overwrite outgoing metadata, publish
   sorted public rotation keys. Partial config fails startup.
5. Update canonical docs, config/deployment examples, existing test fixtures
   that relied on undocumented fail-open photo/video paths.

## Validation

Run focused RED before production changes, then go test ./... in File and Story
(Docker when host Go dependencies cannot download), golangci per module;
make buf-ci, make buf-go-pb-check, make buf-dart-check and breaking check.
Hosted required CI must pass on final PR head before gh pr merge --merge.
Transport tests cover strict metadata, wrong binding/caller, replay/JWKS outage,
unknown/expired keys, wrong CA and routing. No credentials in logs/artifacts.

## Progress

Draft refreshed to bb512d07; test-only commits a8b18a1b and dd6c468d preserve
independent RED work. Original Story RED proved persisted rows without File
attestation, File RED returned Unimplemented; expanded File RED was executed in
Docker, including both missing-lock failures. Tests were independently reviewed.
File predicate full tests passed (21.750s), TLS/JWKS/replay/config tests passed
(6.729s), both service short suites passed. Story full Docker run exposed two
legacy positive fixtures without File attestation; they now inject an explicit
validator while preserving assertions. That run also exceeded its four-minute
budget during existing integration setup; full run now has 15-minute budget.
Independent review found and fixed raw whitespace LFP media and missing explicit
CA/server-name checks. Added public-only sorted two-key loader tests, deployment
patch/network templates and rollout/rotation runbook. Full File module Docker
suite passed (grpcsvc 256.266s); Story grpcsvc passed (407.326s), all other
Story packages passed except an existing startup-test scheduling race. The test
had observed the DB commit before asynchronous File dispatch and asserted both
simultaneously. e4c367cf awaits both effects within the unchanged five-second
deadline, preserving exact-once/idempotency assertions; independent reviewer
approved and the full jobs package passed (49.026s). Both module linters and
short suites pass. Buf lint, host breaking, Go/Dart generation guards and
deployment YAML structural validation pass. Security review approved 325b4e64;
master refresh contains no changes to this scope. Hosted required checks remain
the final gate before merge; no production deployment has been performed.

## Decisions

Use the existing Sol handoff's safe validation scope; LFP format and exact claim
are not silently inferred. File verifier failures return Unauthenticated,
while Story maps internal credential/dependency failures to Unavailable.

## Risks and followups

Validation is not a distributed transaction; later File lifecycle state remains
authoritative. Current unsigned Story ingress is a separate migration. A
green handler-only suite does not establish protected listener readiness.
