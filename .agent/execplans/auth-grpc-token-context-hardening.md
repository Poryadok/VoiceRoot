# Auth gRPC token-context hardening

## Sources

- `docs/microservices/auth-service.md`: Auth owns 2FA and protected gRPC operations.
- `docs/todo/backend.md`: `lastAccessToken` makes direct concurrent gRPC unsafe;
  disable-2FA and the rest of the Auth event matrix are separately open.
- `docs/features/auth-and-contacts.md`: 2FA lists only TOTP and backup codes;
  it does not specify a disable flow or its confirmation/step-up semantics.
- `docs/CONTRACT_MATRIX.md`: names `user.events` but does not specify the missing
  Auth event payload/reliability contracts.

## Scope and acceptance criteria

1. Authenticated gRPC methods that currently fall back to `lastAccessToken` must
   derive the bearer only from the current RPC metadata and fail as `invalid_token`
   when it is absent. A preceding or concurrent call must not grant authority.
2. The behavior is verified through an in-process server with the production
   `AuthorizationServerInterceptor`: no-header `Enable2FA` and E2E backup calls
   fail even immediately after `Register`; calls with that account's bearer work.
3. Existing direct gRPC tests attach per-call headers rather than depending on
   ambient service state.
4. Do not add disable-2FA or event publication behavior: docs leave their
   required confirmation/step-up, payload and durable delivery semantics open.

## TDD sequence

1. Add focused metadata-isolation tests and run them RED against the current
   fallback implementation.
2. Remove process-global token memory; use the request Context resolver for each
   covered protected operation. Run the focused tests green.
3. Update existing gRPC 2FA fixtures to attach authorization headers, then run
   the focused Auth gRPC and Maven suites.
4. Inspect the diff and run Maven verification before commit/PR.

## Files and verification

- Production: `AuthGrpcService.java`.
- Tests: `AuthGrpc2FATest.java` and a focused token-context regression test.
- Commands: `mvn -B -Dtest=AuthGrpc2FATest,AuthGrpcTokenContextTest test`, then
  `mvn -B test` from `src/backend/auth`.

## Risk / containment

This intentionally breaks unauthenticated direct protected gRPC calls, which is
the security fix. Public session-issuing calls remain unauthenticated. The change
does not alter account-erasure confirmation semantics.
