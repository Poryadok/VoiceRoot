# ExecPlan: session-bound email-verification recovery

## Purpose

Make email registration and guest conversion recoverable after reload without
exposing an email-address OTP surface.  A restricted session owns verification;
the client reads an authoritative state and can resume verification or bounded
promotion recovery before it enters the regular product.

## Context

- Docs: `docs/PLAN.md` A1; `docs/features/auth-and-contacts.md` (guest
  conversion and email verification state); `docs/microservices/auth-service.md`
  (OTP Redis fail-closed contract and ConvertGuest);
  `docs/ARCHITECTURE_REQUIREMENTS.md` (three attempts / ten minutes, one send /
  minute); `docs/TESTING.md`; `docs/CONTRIBUTING.md`; and
  `protos/voice/auth/v1/auth.proto`.
- Existing code has pending-email accounts, a six-digit ten-minute OTP and a
  durable guest-conversion worker, but REST OTP lookup still permits raw email,
  register/convert do not send under the issued restricted session, and no
  typed status recovery contract exists.
- User decision is authoritative where it adds detail: server states are
  `GUEST`, `EMAIL_PENDING` (`NONE`/`ACTIVE`), `PROMOTION_PENDING`, and
  `REGULAR`; send/verify/status are session-bound; public registration is
  anti-enumeration; resend invalidates prior code without resetting attempts;
  fourth attempt is 429 with `Retry-After`; and successful promotion returns a
  replacement regular session before client routing.

## Scope

- In: Auth schema/service/gRPC/REST, Gateway exposure, generated clients,
  Flutter persisted state/UI, tests and the listed canonical docs.
- Out: password-reset redesign, verified-email change, onboarding redesign and
  token-storage migration.  Pending-email replacement requires a restricted
  session and current password; it retains the account/profile/history.
- No conflict found: the decision narrows the feature docs' existing pending
  session and Redis OTP requirements.  It deliberately replaces the prior raw
  email OTP transport with a typed session-bound contract.

## Milestones

- [x] RED: Auth contract/integration tests describe states, authenticated send
  and verify, error uniformity, limits, and promotion recovery.
- [x] GREEN: minimal Auth schema/service/proto/REST implementation passes.
- [x] RED/GREEN: Gateway and Flutter status/resume UI persist and replace the
  session before routing.
- [ ] Docs, focused checks, compose recovery proof, review, PR/CI/merge.

## Detailed Steps

1. Add failing Auth tests for anonymous/register/convert persistence, bearer-only
   OTP APIs, stale-code invalidation, shared invalid/expired error, attempt
   budget and 429 `Retry-After`, promotion 202/status recovery, and email
   replacement password gate.
2. Add `VerificationStatus` / operation state to the protobuf and regenerate
   Java/Go/Dart artifacts.  Add durable status and pending-email fields/migration
   only where current persistence cannot represent the contract.
3. Make registration and guest conversion issue restricted sessions, then call
   the authenticated send path exactly once.  Resolve send/verify exclusively
   from validated session claims; retain a generic public register result.
4. Implement status and promotion worker recovery.  Verify consumes a code
   atomically, returns 202 while promotion is pending, and returns a fresh
   regular session only after the durable promotion finishes.
5. Add Gateway mappings and Flutter repository/controller/UI recovery.  On
   bootstrap read status, persist returned replacement sessions before routing,
   and show retry-only promotion pending state.
6. Update feature, Auth microservice, architecture OTP, client TODO, testing,
   and proto/REST contract docs; run `graphify update .` after code edits.

## Validation

- [ ] Auth focused Maven unit/integration tests (RED then GREEN).
- [ ] `make buf-ci` after generation.
- [ ] Gateway affected Go test with race where required.
- [ ] Flutter analyze and focused widget tests, then `make flutter-ci`.
- [ ] Compose Auth recovery proof and PR `ci-gate` on exact head/base.

## Progress

- [x] Read governing A1/Auth/architecture/testing/contributing/proto documents.
- [x] Mapped existing Auth OTP and conversion code.
- [x] Write and execute Auth RED tests.
- [x] Implement bearer-only email verification, initial send, typed status and
  `202` promotion acknowledgement; regenerate proto clients.
- [x] Update Flutter conversion/recovery client and controller status read.
- [ ] Complete broader Auth/Gateway/Flutter checks and PR lifecycle.

## Decisions

- Session binding is the authority boundary: no raw email is accepted by email
  verification send/verify/status endpoints.  Password reset remains unchanged.
- The existing guest account type represents a restricted pending identity; the
  new typed state distinguishes anonymous guest from pending-email recovery.
- Delegate review is normally required by strict TDD, but the task explicitly
  requires one writer and no nested agents; the author performs serialized test
  and implementation review checkpoints instead.
- `otp_rate_limited` returns a conservative `Retry-After: 600`; the existing
  throttles remain the authority and keep purpose-specific resend/attempt state.

## Risks And Follow-Ups

- This changes public protobuf/REST contracts and generated clients together;
  compatibility must be demonstrated by buf and Gateway/Flutter checks.
- Redis must remain fail-closed in JDBC deployments.  A local memory profile may
  use its existing explicit in-memory throttle only for tests.
- `make buf-ci` is currently blocked by an unavailable local Docker Desktop
  Linux engine; this is external infrastructure, not retried unchanged.
