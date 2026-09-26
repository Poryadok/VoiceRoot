# GAME-AUTH-01 implementation plan and acceptance

Owner: Java Auth. Product authority: docs/features/game-integrations.md and
docs/architecture/game-integration-api.md, frozen GAME-AUTH-01 section.
This is the first bounded implementation in T13/T15, not full P1 completion.

Plan written before production implementation:

1. Freeze provider/device/app isolation and lifecycle decisions in owning docs.
2. Independent tester authors real RS256 provider fixtures and negative tests;
   run red before implementing strict Google ID-token verifier.
3. Independent tester authors PostgreSQL challenge/exchange/session/revoke tests;
   run red before schema and transactional service implementation.
4. Implement separate sdk principal tables, atomic exchange/replay rejection,
   per-device ES256 possession, hashed opaque bootstrap sessions and revocation.
5. Wire opt-in JDBC Spring REST API and operator admission configuration. There
   is no enabled app by default. Google keys come only from a fixed HTTPS JWKS
   endpoint, with bounded fetch/cache and no request-controlled URLs.
6. Independent review of tests and implementation; fix critical findings.
7. Run selected tests and full Auth Maven suite; inspect required container skips.
   Update graph and commit/push bounded branch; no master merge or staging deploy.

Verification commands (from src/backend/auth):

    rtk mvn -B -Dtest=GoogleOidcProofVerifierTest test
    rtk mvn -B -Dtest=SdkIdentityJdbcIntegrationTest test
    rtk mvn -B test

Tables are Auth-only and mirrored in Flyway and golang-migrate. Public Voice
account/profile/session flows do not accept this distinct opaque token. Tests
must prove actual durable state, replay races, app/env binding, developer token
rejection, provider issuer/audience/signature/nonce/freshness, device key
possession/revocation, lifecycle denial and no guest/regular account creation.

Follow-on conversion slices and exact external Google acceptance prerequisites
are in the owning API document. SDK browser UI, permanent conversion, full
deletion/restore journal and downstream chat/voice grants are not implemented
by this first slice. Deployment stays gated until those consumers and public
rate limits exist.

## Evidence

- Provider tests authored independently: compile RED before types existed,
  then 33 tests green. JWKS inert stub: 12 tests / 11 expected failures,
  then 13 tests green including full-body timeout/cancellation.
- REST binding/validation: compile RED before controller/service; 4 tests green.
- Operator configuration: missing-class RED then 3 tests green.
- JDBC tests are real PostgreSQL/Testcontainers and include concurrent replay,
  admission caps, app/env isolation and permanent device revocation. Initial run
  skipped because Docker Desktop failed to start; skips are not acceptance.
- Independent review found lock-wait expiry and validation secret logging; both
  receive regression tests before corrections. Final results appended below.

Final local run on 2026-09-26: `rtk mvn -B test` completed with 600 reported tests,
zero failures/errors, **125 skipped** because Docker was unavailable. This is
475 executed passing tests, not a passing container acceptance gate. New SDK
non-container suites: Google verifier 34, JWKS 13, REST 5, configuration 3,
all passing. SDK JDBC reports 16 skipped test methods (parameterized cases
expand when Docker runs); real persistence/concurrency/Flyway acceptance is
still required. Docker Desktop failed startup on a stale analytics socket;
fleet coordinator owns daemon recovery. No provider live-login acceptance ran.

The secret-log regression showed the expected failure before its fix; the
provider subject 255/256 boundary similarly went red then green. Lock-wait
regressions cover challenge/provider/game expiry separately; runtime red/green
for those could not execute without PostgreSQL. Independent re-review accepted
both corrections, with no remaining critical findings. No tests were weakened.
The subsequent unknown-authority-field regression failed with HTTP 200 before
strict SDK request deserialization; the final targeted REST suite has 6 tests.

## Operator configuration and external acceptance

Activation is opt-in, JDBC only: `auth.sdk-identity.enabled=true` with
`auth.persistence=jdbc`. Default has no admitted applications. Configure an
indexed `auth.sdk-identity.applications` list with `application-id`,
`environment-id`, `client-id` (distinct Voice-owned Google OAuth client ID for
each environment), and `game-public-jwk` (public RSA signing JWK, >=2048 bits).
Duplicate app/env or Google audiences and private/malformed keys fail startup.
There is no client-supplied issuer/JWKS URL or game signing key. Changes require
an Auth restart; online registry revocation is a required next consumer.

The internal protocol acceptance client calls challenge, obtains a Google ID
token using that exact nonce and configured audience, obtains a game ticket
bound to the same nonce and independent subject digest, signs the enrollment
payload using its own P-256 key, then exchanges both proofs. All exact fields
and proof bytes are in the owning API document. The response can only be used
on SDK bootstrap session/revoke routes with device possession. It grants no
message, media, profile, guest conversion or regular account privileges.

Live Google acceptance needs an operator-owned Google Cloud OAuth application,
registered origin/redirect and two real consented test logins in each admitted
environment. Exercise wrong audience/nonce, lost-response fresh login, two
independent devices and revoke/re-enrollment with a replacement key. Fixture
signatures prove local validation, not the provider's browser consent or live
key rotation. No provider credentials or live-user token are committed.

Do not enable on public Gateway until app registry/admission, HTTP rate limits,
browser code+PKCE and downstream delegation gates are complete. Deployment and
restore remain held; conversion and tombstone reconciliation are next slices.

## GAME-AUTH-02 execution plan

Canonical next-seam sources: game-integration-api GAME-AUTH-02 and
docs/architecture/game-conversion-authority.md (integration commit 287d766b).
Captain owns registry policy/workload verification, User read-only eligibility,
Game Integration freeze/transfer/activate and Voice receipts. Auth owns browser
authorization, consent revisions, code exchange and durable conversion state.

Next red/green order: independently authored authorization policy/PKCE/device
tests, reviewed JDBC replay/lifecycle tests, minimal Auth implementation and
controller wiring; then conversion operation/status and strict owner receipt
consumption after the captain supplies concrete receipt contracts. No no-op
receipt provider and no permissive policy/profile fallback. Reuse ordinary Voice
registration/email verification for a new permanent target and current regular
proof for an existing target; never mutate sdk-account into legacy guest.

Files: sdkidentity package/tests; next Auth Flyway and mirrored auth_db migration;
owning identity API docs. Verification: focused Maven tests then scoped full Auth
suite; required PostgreSQL suites remain a gate. Keep PR490 draft, push focused
commits promptly, and update evidence without claiming skipped tests passed.

Implemented: public-client S256 authorization from an independently proven SDK
identity; current Voice-authenticated consent view and explicit selected-profile
approval; one-use hashed code and scoped linked bootstrap credential. Every
exchange rechecks source/device generation, policy revision, target regular
state/email verification/session epoch/logout, and User profile eligibility.
The linked credential preserves the selected profile and has no regular JWT,
refresh, active game binding or media authority.

Authorization is separately opt-in: `auth.sdk-authorization.enabled=true`
requires SDK identity and JDBC. Without the verified registry and read-only User
adapters, default beans deny every lookup. The interfaces are tested with explicit
fixtures only; no production workload adapter has been substituted with a fake.
The existing Voice browser/Flutter UI consumes the REST consent view and approval
routes documented in the owning API. UI integration, direct Voice-only first
login, binding activation, public Gateway limits and device-code consumer remain
subsequent work.

Independent tests preceded implementation; Maven captured missing protocol/service,
consent view/conflict, controller and configuration RED phases. Protocol31,
REST16 and configuration5 tests pass: 52 new non-container cases. All nine SDK
suites report 142 tests: 108 executed/pass, 34 PostgreSQL method entries skipped
without Docker. Parameterized database cases expand only during an actual run.
SQL replay, lifecycle and lock-wait acceptance remains open; skips are not green.
Independent test/implementation review corrected scope and clock fixtures,
pending-email authority rejection and the idempotency conflict status.

Full Auth verification on 2026-09-26: `rtk mvn -B test`, 125 suites / 671
reported tests, 528 executed/pass, zero failures/errors, 143 skipped due to
unavailable Docker. This remains a partial local verification result. No live
Google acceptance, cross-service receipt acceptance or staging deployment ran.
