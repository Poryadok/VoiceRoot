# ExecPlan: Preserve sessions during concurrent refresh

## Purpose

Keep a valid persisted staging session when an overlapping refresh using an older, one-time refresh token returns 401. After a hard reload or a delayed 401, the client should use the newer session already persisted instead of clearing it and sending the user back to sign-in.

## Context

- Docs: `docs/features/auth-and-contacts.md` (“Сессии и безопасность”: 15-minute JWT access token, 30-day opaque refresh token, logout revokes one session); `docs/microservices/auth-service.md` (refresh-token rotation is one-time); `docs/TESTING.md` (Flutter verification guidance).
- Code: `src/frontend/lib/state/auth_providers.dart` (`restore`, `refreshOn401`, proactive refresh and storage clearing); `src/frontend/lib/backend/auth_session_storage.dart` (shared persisted session); `src/frontend/test/auth_controller_test.dart`; `src/frontend/lib/bootstrap/voice_app_bootstrap.dart` (one restore call per bootstrap widget).
- Current state: staging user-service logs record successful `CompleteOnboardingStep` and `GetOnboardingState` calls. Auth access logs include a refresh 200 followed closely by a refresh 401. A hard reload returned the browser to sign-in; the authorized test account was signed in again. The log records lack account/request correlation, so the refresh race is a plausible mechanism, not proven as the sole cause.
- Constraints: staging only; no production access, account deletion, reset, or data wipe. Do not inspect `debug.md`. Preserve the user's current staging session. No Auth service or database contract changes are planned.

## Scope

- In: make refreshes initiated by restore, 401 recovery, and proactive refresh share one in-process operation where they use the same saved session; prevent a stale definitive failure from clearing a newer persisted session; add regression coverage for overlap and stale failures.
- Out: backend refresh-token policy, cross-tab coordination, account recovery flows, and unrelated realtime reconnect behavior.
- Documentation gaps: repository docs define one-time refresh rotation but do not explicitly define client recovery when an old token's late 401 races a successful rotation. This plan infers that the client must preserve a replacement session already persisted; it must still clear the same current session on a definitive rejection.

## Milestones

- [x] Add focused red tests for concurrent restore/refresh recovery and stale-token failure after a replacement session has been persisted.
- [x] Review test assertions and ensure they distinguish an old-token 401 from a current-token 401.
- [x] Implement shared refresh single-flight and compare-before-clear handling.
- [x] Run focused Flutter tests, analysis, and the repo-required Flutter CI checks.
- [ ] Review the final diff; update A1 staging evidence and deploy only after PR CI passes.
- [ ] Recheck staging reload behavior without changing or deleting account data.

## Detailed Steps

1. Reconfirm Auth/Flutter test conventions and current `AuthController` refresh paths.
2. Add tests using controlled asynchronous refresh results: allow token A to rotate and persist token B, then deliver a stale 401 for A; assert B remains persisted and authenticated. Add a same-controller overlap assertion so concurrent refresh paths share work where applicable. Keep the existing current-token 401 test to assert genuine rejection still clears storage.
3. Run the focused tests before implementation and confirm each new assertion fails for the intended storage-clear or duplicate-refresh behavior.
4. Implement the smallest controller-local coordination and compare the stored refresh token/session before clearing on definitive failure. Do not weaken genuine 401 handling.
5. Run focused tests after each behavior, then Flutter analyze and the documented `make flutter-ci` check.
6. Have a separate reviewer inspect race ordering, test strength, and error handling. Resolve critical findings before PR.
7. After merge and successful staging deployment, use the logged-in staging test account only to verify reload restores or preserves the current session; collect only sanitized Auth/User status evidence. Update the A1 ledger with exact SHA and result.

## Validation

- [ ] `cd src/frontend && flutter test test/auth_controller_test.dart` proves regression and genuine-rejection behavior.
- [ ] `cd src/frontend && flutter analyze` reports no issues in the changed Flutter module.
- [x] `make flutter-ci` passes for the exact change before merge (2026-10-01; exit code 0).
- [ ] Exact-SHA CI and a data-preserving staging deploy pass; manual reload leaves the authorized test account signed in and the onboarding state remains dismissed.

## Progress

- [x] Read project auth/session and Auth service behavior docs; reviewed CodeGraph paths.
- [x] Confirmed a hard-reload sign-in outcome and a 200/401 refresh-log pair; causation remains unproven.
- [x] Added controlled regressions for stale-token rejection, same-token single-flight, restore vs. profile switch during refresh, and profile switch during the initial storage read. The stale rejection, single-flight, and initial-read tests were confirmed red before their corresponding fixes.
- [x] Implemented token-keyed single-flight refreshes shared by restore, 401/proactive recovery, guest conversion, and email-verification promotion. Definitive failures adopt a newer persisted session before clearing; a still-current rejected session continues through the existing sign-out path.
- [x] Fenced restore with generation/session snapshots across the initial storage read and subsequent async work; it commits through the profile-switch path.
- [x] Focused `auth_controller_test.dart` passes after the change, including the existing current-token invalid-token clearing test.
- [x] Independent review confirmed the profile-switch and shared-refresh findings are addressed; the remaining cross-tab storage race is recorded below.
- [x] Final `make flutter-ci` passed against the completed diff (2026-10-01; exit code 0; analyzer emitted 9 info-level issues under the repo's `--no-fatal-infos` policy).
- [ ] After PR CI and data-preserving staging deployment, recheck reload on the already authorized test account and record the exact result.

## Decisions

- Preserve a replacement session if storage has advanced beyond the token that failed. This handles late results from one-time rotation without suppressing rejection of the current token.
- Share in-flight work by refresh token inside a controller. If storage has moved to another refresh token, adopt that session when handling the older token's rejection. No cross-tab mutex is introduced.
- Keep account deletion and staging resets out of scope.

## Risks And Follow-Ups

- Current staging logs are not correlated to the browser account or request initiator. The fix will harden a credible race, but manual reload after deployment is still required to validate the staging symptom.
- The storage API has no atomic compare-and-clear operation. The persisted-token check prevents clearing when the replacement session is already stored, but it cannot close every ordering across separate browser tabs/controllers. If traces implicate that cross-tab window, evaluate a browser-wide lock or backend rotation grace strategy separately with security review.
- Restore now commits through the profile-generation guard used by profile switching, so a late successful response cannot replace a completed profile switch. The A1 evidence still needs a staging reload after deployment.
