# ExecPlan: protected public Space creation in Compose

## Purpose

Make the ordinary verified regular user's `POST /api/v1/spaces` reach native
Space creation and a real Role Owner grant, allowing the A1 daily/channel
journey to proceed without legacy authority fallback.

## Context

- Canon: `docs/PLAN.md` A1/A2, `docs/features/spaces.md`, Gateway verified
  creator dispatch, Role caller matrix (`role-service.md`), Space native
  membership contract, `docs/TESTING.md` and `docs/CONTRIBUTING.md`.
- Reproduction: immutable `35fe4707859932ccc418f425f7f0a203ad7f243a`, public
  daily/channel tests return 503 before creator dispatch.
- Receiver: `5690d43c4f5b8f2c1e4bb4c8ebf8300090b09b07`, finite Creator source
  CLEAR in `REVIEW-A1-CREATOR-5690d43c4f5b`, assembled with the reproduction
  source by ordinary merge. Gateway224 and Auth225 remain exact producer inputs.
- Missing integration: Compose omits creator transports, and Role's trusted
  listener does not yet expose BootstrapSpaceRoles. Space's bootstrap helper
  still uses its ordinary Role client.

## Scope

- Add only exact signed Space-service BootstrapSpaceRoles on Role's existing
  mTLS listener, with complete request hash, replay and canonical UUID checks.
- Deny Bootstrap on Role's ordinary listener. Native Creator uses the signed
  protected client and fails closed without its dependencies.
- Reuse Role's existing transaction and same-owner idempotency; a different
  Owner cannot be installed by bootstrap. No generic role mutation authority.
- Wire the canonical media Compose overlay, preserving private TLS trust,
  signing-key isolation, native33 DDL and Auth Space-purpose session floor.
- Exclude interactive admission/full ban/Bot activation, shared DEV/STAGING,
  hosted CI and final acceptance. No nil-role or unsigned production fallback.

## Milestones

- [x] Reproduce and locate absent creator wiring and protected bootstrap.
- [x] Reserve task and acquire an isolated lease; revalidate exact source review.
- [x] RED protected listener and caller-binding tests in owned Linux runner.
- [x] Minimal Role receiver/Space caller and Compose configuration.
- [x] Linux tests, vet and actual native Auth/Role/Space/Gateway composition.
- [x] Public regular create, Owner ACL and negative transport/auth assertions.
- [ ] Independent exact-head review, actual A1 daily/channel consumer rerun.
- [ ] Explicit accepted delivery to the integration owner.

## Detailed Steps

1. Exercise the actual mTLS/JWKS/replay listener with valid Space credentials,
   replay, wrong issuer/RPC/hash and raw identity metadata; ordinary deny.
2. Add exact Bootstrap routing and fail-closed request validation.
3. Sign the native Creator's Role call using its configured protected client;
   component fixtures explicitly inject a signed controlled Role dependency.
4. Add Compose contract checks and configuration for Gateway/Space/Auth/Role.
5. Build immutable native Linux images/source in an owned no-host-port network.
   Use real public Auth registration/verification and real PostgreSQL/Redis.
6. Assert public create 200, real Owner grant/ACL, guest/forged/revoked denial,
   protected-only bootstrap and full body/replay binding. Record precise pins.
7. Have the A1 runtime owner rerun daily/channel on the reviewed delta.

## Validation

- Role principalruntime/principalgrpc tests including actual mTLS listener.
- Space signed Creator/caller tests and affected package/service checks.
- Real PostgreSQL bootstrap atomicity and same-owner/different-owner invariants.
- Compose fixture/credential mount contract checks, go vet and diff checks.
- Actual public Compose proof remains distinct from component fixture PASS.

## Progress

Task owner342, consumer302, native producer186, review352 and integration121.
New lease34 is isolated; no shared stand or hosted CI authority is held.

Role listener RED 0.350s became race GREEN for principalruntime/principalgrpc
(31.568s/1.034s), with focused canonical-field negatives 1.995s. Role short/vet
passed. Space caller race passed 1.438s, full root race passed 4.942s and vet
passed; short package checks passed after restoring omitted snapshot fixtures.
Compose protected-transport and credential-scope checks passed (2 tests,
23.456s). The final public proof passed 2.18s with five real deny cases:
anonymous, guest, revoked session, forged actor and duplicate JSON field.
Actual Auth floor and Role Bootstrap reported grpc OK; owner invite creation
succeeded while an outsider's attempt failed. Space image a9dc0b9 and Role
image d3959fa were built from this source; other images retain exact35fe inputs.

Original Daily and Channel now pass creation/owner invite but fail at invite
join with `403 protected lifecycle listener required` (5.89s/2.07s). This next
boundary is tracked as `A1-SPACE-JOIN-PROTECTED-TRANSPORT` and consumes the
existing69/186 admission work. Full A1 remains open. The protected Creator
regression is added to the isolated A1 runner and manifest without removing any
existing cases; hosted proof remains the integration owner's separate stage.

## Decisions

The Role caller matrix already authorizes exact Space-service bootstrap.
Implement that boundary instead of enabling the legacy ordinary transport.
The native Creator is independent of unrelated memberAuthority/Role9096 setup.

The first assembled public RED reached Auth but received method-not-found on
9091: Auth225 deliberately serves Space floor only on the dedicated 9092 mTLS
listener. Use a complete `SPACE_AUTH_SESSION_FLOOR_*` client tuple, separate
from the ownership proof transport. Never downgrade the floor or omit the
client identity to make composition pass.

## Risks And Follow-Ups

Role bootstrap follows native Space commit; ambiguous Role failure must not be
reported as a complete public journey. This slice preserves failure visibility
and does not invent a distributed publication/compensation protocol. Full native
admission and lifecycle qualification retain their separate gates.
