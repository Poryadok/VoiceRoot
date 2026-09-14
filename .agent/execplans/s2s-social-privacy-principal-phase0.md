# ExecPlan: Social privacy Phase-0 verified S2S principal

## Purpose

Replace the unauthenticated `x-voice-internal-caller` path used by Social privacy
decisions with a deployable Phase-0 service-principal boundary.  Social will call
only User `GetPrivacySettings` and Space `AreCoMembers` through separate TLS
listeners using a fresh, request-bound RS256 service JWT.  The result is a
fail-closed migration that lets User and Space recognize only verified
`service:social`, never a caller-selected metadata value.

## Context

- `tmp/slave-driver/s2s-principal-sol-contract.md` is the authoritative Sol
  decision for this successor and supersedes PR #328's raw-header adapter.
- `docs/ARCHITECTURE_REQUIREMENTS.md` Phase-0 S2S rules require RS256/JWKS,
  request binding, replay protection, TLS, explicit allow-lists, and fail-closed
  errors.
- `docs/OPERATIONS.md` and `docs/DEPLOYMENT.md` define key rotation, TLS and
  deployment expectations. `docs/microservices/{social,user,space}-service.md`
  define the two privacy decision RPCs.
- Reusable primitives are in `src/backend/pkg/principal`; the Role protected
  listener is a reference seam, not a shared endpoint to extend.
- Current `src/backend/social/internal/s2s/privacy.go` emits raw caller
  metadata, and User's ordinary `GetPrivacySettings` trusts it. This is the
  defect under remediation.

## Scope

In scope:

- Social signing/key loading and per-call request-bound service principals for
  User privacy and Space co-membership.
- Dedicated User and Space TLS-only listeners, JWKS verification, atomic Redis
  replay records, allow-list enforcement, and handler defense in depth.
- The narrow migration: only Social uses the new listeners; ordinary listeners
  reject raw Social markers after cutover.
- Staging/prod manifests, secret mounts, services, NetworkPolicies, JWKS
  publication and a runbook with the two-key rotation sequence.

Out of scope:

- A generic migration of every legacy internal caller, any raw-header fallback,
  client-facing RPC change, federation, and local Compose integration execution.

## Acceptance criteria

1. Each Social call contains exactly one Bearer service credential and one
   server-owned request ID, with no forwarded or raw identity metadata.
2. The credential is RS256, has `iss=social`, `sub=service:social`, the exact
   audience/RPC/request hash/request ID, unique `jti`, a published `kid`, and a
   lifetime no greater than 30 seconds.
3. User and Space protected listeners reject absent, duplicate, malformed,
   forged, replayed, wrong-bound, and raw-identity requests as
   `Unauthenticated` before handler/store access; verified non-Social principals
   receive `PermissionDenied`.
4. The protected listeners are TLS-only and expose only their respective
   privacy decision RPC; a wrong CA fails before the interceptor.  JWKS cache
   obeys 30-second refresh, 2-minute hard expiry and 5-second unknown-kid
   cooldown.
5. Partial configuration fails startup. Manifests mount two distinct Social
   PKCS#8 keys read-only, configure consumer JWKS/TLS/replay settings, expose
   protected port 9091. Ordinary privacy RPCs reject raw Social authority;
   unrelated Social User profile/account lookups retain ordinary 9090 access.
6. Rotation and cutover are documented: publish both keys, validate consumers,
   switch active kid, retain the old key for at least 35 seconds, then replace;
   no dual-auth acceptance period exists.

## Milestones

- [x] Record the Sol contract and a self-contained plan.
- [x] Add executable RED contracts for outbound metadata, raw-header denial and
  deployment/listener rollout gaps.
- [x] Fresh independent review of RED tests.
- [x] Implement Social signer/config and outbound TLS clients, driving each RED
  assertion green.
- [x] Implement User and Space protected runtimes and ordinary-listener denial.
- [x] Implement manifests, NetworkPolicies, JWKS exposure, rotation runbook and
  hosted Compose coverage.
- [ ] Independent implementation review and exact-head hosted CI.

## Progress

- Original RED contracts reproduced by receiver/signer owners; integrated
  Social/User/Space short suites and principal/socialprincipal packages pass.
  Service vet/build/lint checks pass. No local Compose, race or Testcontainers
  execution was used.
- Independent read-only security review found no critical/high blockers;
  final test follow-up `4d78c5c8` was explicitly rechecked. Hosted evidence is
  still required before merge.
- `TestRuntimeTLSJWKSRedisIntegration` exercises shared replay across two
  runtimes, HTTPS JWKS, TLS/wrong CA, next-key use and Redis TTL on hosted CI.
  `social-principal-bootstrap-check.sh` runs the isolated Compose initializer
  twice and validates certificate names, key separation and private-file modes.
- Staging/prod secret preflight runs before any deployment mutation. Missing
  target secret material is an activation dependency; repository credentials
  are never reused to satisfy it.

## Detailed steps

1. Keep the RED tests in the Social, User, Space and principal packages. Run
   their package tests from `src/backend`; record the expected failures before
   any production edit.
2. Add a Social principal configuration loader that requires exactly two
   unencrypted PKCS#8 keys and an active kid whenever either protected target is
   configured. Build a per-RPC issuer from the existing principal package and
   a TLS client factory pinned to the configured CA/server name.
3. Add isolated User/Space listener runtimes patterned after Role only where
   the concrete method matrix matches. Verify bearer metadata before creating a
   principal context, use per-service Redis namespaces, and let the domain
   handler require the verified Social principal.
4. Route only the two Social adapters to 9091. Remove their raw caller header;
   reject a raw Social marker on the ordinary 9090 listeners. Do not change
   other callers in this slice.
5. Add deployment resources and tests for separate key mounts, TLS/JWKS/replay
   config, protected services, and least-privilege policies. Add the exact
   rotation/cutover commands to the deployment runbook.
6. Run focused Go checks. Do not run Realtime, race, Compose, localhost or
   Testcontainers work on this host; use hosted CI for Compose/TLS/Redis
   integration evidence.

## Validation

- RED evidence now: `go test ./social/internal/s2s ./user/internal/grpcsvc
  ./space/internal/grpcsvc ./pkg/principal` from `src/backend` fails on the
  missing verified-principal behavior and rollout resources.
- GREEN target: the same focused packages pass after implementation, followed
  by the repository S2S/deployment checks and hosted CI's exact commit gate.
- Hosted-only: the TLS/JWKS/Redis Compose boundary matrix and any Realtime
  verification. The local prohibition does not apply to the in-memory/bufconn
  unit tests above, but this RED branch intentionally does not start a server
  implementation.

## Decisions

- Security review refinement: a blanket Social-to-User 9090 NetworkPolicy deny
  would break existing `GetProfile` and `ListProfileIDsForAccount` adapters used by
  friend/contact/account flows. The narrow cutover protects only the two privacy
  methods and rejects raw Social markers there. Protected 9091 ingress is Social
  only; ordinary User 9090 remains reachable for the separately migrating RPCs.
  This does not expand the protected allow-list. The owner accepted this concrete
  refinement after comparing the Sol draft with current callers.
- Parallel ownership: Social signer/client implementation and User/Space
  receiver implementation use separate helper worktrees; PR owner owns deploy,
  contract documentation, integration and final review.
- No raw-header compatibility path: Sol explicitly selected a direct cutover.
- `Unauthenticated` represents absent/invalid proof, replay, TLS/JWKS failure
  and raw identity; `PermissionDenied` represents a valid non-Social principal.
- The protected endpoints are dedicated rather than adding another trust mode
  to ordinary public/internal listeners. This contains the migration and makes
  its NetworkPolicy reviewable.

## Risks and follow-ups

- Manifests and read-only mounts are implemented; actual target provisioning and
  synthetic cutover remain deployment duties described in the runbook.
- Existing internal callers still use legacy metadata for unrelated RPCs. They
  must not be silently migrated or granted Social's allow-list in this change.
- PR #328 remains historical until this successor exists and has linked RED
  evidence; do not merge it.
