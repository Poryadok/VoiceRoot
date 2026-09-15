# Auth internal caller authorization and audit

## Purpose

Close unrestricted phone-hash lookup and account-status mutation while preserving
Social discovery and Moderation sanctions through authenticated service calls.
HTTP audit records the connected peer IP without trusting forwarded headers.

## Context and scope

Sources: PLAN A1, TODO backend Auth internal gRPC/IP audit entries,
auth-service.md security, auth-and-contacts.md, reports.md, and the Phase-0
principal contract in ARCHITECTURE_REQUIREMENTS.md. Auth remains Java. Existing
Auth proof TLS listener, verifier/JWKS cache and Redis replay guard are reused.
Existing Social privacy principal bootstrap supplies the deployment pattern.

The caller matrix is exact: `service:social` may invoke
`/voice.auth.v1.AuthService/ResolvePhoneHashes`; `service:moderation` may invoke
`/voice.auth.v1.AuthService/SetAccountStatus`. Both use audience `auth` with exact
RPC/request ID/deterministic protobuf hash binding and fresh 30-second credentials.
Gateway users and other service principals have no authority on these methods.
Legacy listener denies both methods even for correctly signed requests. No raw
metadata fallback, mTLS invention, session JWT substitute or public RPC route.

HTTP `peer_ip` is the servlet transport remote address captured before downstream
filters. Only valid IP literals are recorded; absent/invalid input is `unknown`.
Forwarded and X-Forwarded-For are not sources. This identifies the connected
proxy when proxied, not an authenticated end-user IP. MDC is cleared on success
and failure; no new retention or PII store is introduced.

## Milestones and ownership

- [x] RED Auth caller matrix, legacy denial, request binding and peer-IP tests.
- [x] GREEN Auth private listener/verifier, service adapter guards, private CA.
- [ ] GREEN Social/Moderation signed caller transports with negative coverage.
- [ ] Compose bootstrap, namespace Secret references and preflight cutover.
- [ ] Independent security review and hosted Auth/Go/deployment CI; merge commit.

Auth lifecycle owner writes Java/docs and integrates pushed delegate branches.
Independent Go caller and deployment writers use separate Treehouse leases.
IP tests are authored separately; reviewers only read the resulting integration.

## Detailed steps and validation

1. Freeze the caller/deployment matrix above; write tests before production edits.
2. Run focused Maven principal and HTTP tests to demonstrate behavioral RED.
3. Extend existing allow-list/listener and require verified context in adapters.
   Preserve ordinary Gateway Auth methods on the legacy listener.
4. Wire caller-owned signing keys, trusted Auth TLS endpoint, HTTPS JWKS and
   shared Redis replay. Explicitly reject partial configuration; no fallback.
5. Update old service integration tests to authenticate real requests, keeping
   phone-match and durable suspension/login-denial assertions intact.
6. Run `mvn -B test` (hosted Docker-backed integration is decisive), scoped Go
   short tests/lint and deployment script tests. Do not run local Realtime.
7. Review request binding, key separation, wrong callers, replay, TLS identity,
   cutover availability and log spoofing. Fix findings; rerun affected checks.
8. Refresh master with merge, push, verify hosted CI on that head and `ci-gate`,
   then merge using a merge commit. Never rewrite history or bypass hooks.

## Decisions and risks

Reviewer found that standard deployment has no Gateway/Space HTTPS principal
issuer runtime. Auth issuer configuration therefore selects complete groups:
Social+Moderation, Gateway+Space, or both. Empty, partial or unknown groups fail
startup. Standard deployment configures only real Social/Moderation endpoints;
the optional Phase-0 overlay supplies actual ownership issuer endpoints.
This corrects the initial four-required-issuer assumption, which would require
invented inactive endpoints or unrelated ownership activation. Disabled-group
credentials remain unauthenticated; they cannot acquire another issuer's keys.

Full caller/deployment wire is part of this PR: disabling old handlers alone
would interrupt known traffic. Application implementation and disposable Compose
evidence do not prove staging activation. Actual namespace Secrets and live
rollout evidence must be recorded separately if external access is unavailable.
Only one rerun is allowed for a proven external network failure; repeated same
failure is retained as evidence and is not retried again.

## Progress

- [x] Inspected current raw-marker callers, Auth proof listener and Phase-0 canon.
- [x] Assigned isolated Go caller and deployment work, plus IP RED tests.
- [x] Initial Auth RED: 5 boundary tests, 2 failures and 2 errors; IP RED: 13 tests,
  12 failures. Follow-up adapter and real HTTPS CA tests also failed before GREEN.
- [x] Focused Auth GREEN: 77 tests without skips. Console-encoder regression then
  demonstrated missing `peer_ip` in actual JSON (14 tests, one failure), fixed in
  the configured logback pattern without weakening the MDC checks.
- [x] Full host `mvn -B test`: 517 tests, zero failures/errors, 109 skipped because
  Docker is unavailable. Hosted `backend-auth` must execute all Docker suites.
- [ ] Go caller integration, final security review and hosted verification pending.
