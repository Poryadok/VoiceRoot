# ExecPlan: account-scoped profile mutation limits

## Purpose

BE-220 bounds profile creation/deletion churn across User replicas and active
profile switches, before either mutation can reach PostgreSQL or publish events.

## Context

- [Multi-Profile](../features/multi-profile.md#защита-от-злоупотреблений)
  requires creation/deletion anti-spam but does not prescribe numeric limits.
- [User](../microservices/user-service.md), [data ownership](../DATA_STORES.md),
  [data model](../DATA_MODEL.md) and [testing](../TESTING.md) govern implementation.
- Current handlers resolve the account from existing authenticated incoming
  metadata; CreateProfile then counts profiles, DeleteProfile soft-archives.
- Existing `USER_REDIS_ADDR` is wired by Compose and both Kubernetes environments.
- Base: `c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c` (actual local develop).

## Scope

Implement a required User handler dependency and production Redis limiter for
CreateProfile/DeleteProfile only. Preserve subscription caps, profile ownership,
privacy initialization, archival and existing auth/transport boundaries. Account
erasure, switching, primary-profile bootstrap and protected-principal migration
remain separate operations. No rollout or acceptance authority is introduced.

## Decisions

Under the user's delegated ordinary product choices, choose **10 attempts per
rolling hour per account per operation**. Ten allows two full five-profile
cycles while bounding repeated churn; create/delete have independent budgets.
The existing account identity, not requested profile ID or active profile, owns
the key. Authenticated attempts consume budget even if later validation,
ownership, subscription or database checks reject them; no refund races.

Redis TIME supplies the shared clock. One Lua operation prunes expired entries,
checks the bound and adds a uniquely sequenced admission. Both sorted-set and
sequence keys expire after one hour without admission; no payload is stored.
Missing configuration/dependency or Redis errors yield generic UNAVAILABLE;
exhaustion yields RESOURCE_EXHAUSTED (Gateway 429). There is no memory fallback.
Actual caller cancellation remains typed, before any database access.

## Milestones

- [ ] Meaningful existing-handler RED for absent limiter before closed database.
- [ ] Atomic Redis admission plus fail-closed handler/production wiring.
- [ ] Multi-client concurrency, window, TTL, account/operation and error tests.
- [ ] Existing fixtures explicitly supply their private real miniredis limiter.
- [ ] Focused PostgreSQL cycle proves denied requests leave data unchanged.
- [ ] User short suite, race/vet/lint, independent review and portable delivery.

## Detailed Steps

1. Test both real RPC methods with authenticated context and a closed pgx pool;
   absent limiter must return UNAVAILABLE instead of reaching the database.
2. Add the Redis implementation in `internal/profilemutation`, then required
   `ProfileMutations` dependency and pre-database gates in both handlers.
3. Reuse the production presence Redis client in main, including a non-nil
   unavailable limiter when USER_REDIS_ADDR is absent; never silently disable it.
4. Exercise two limiter instances, a concurrent burst, exact sliding-window
   boundary, repeated denial, TTL, outage, corrupt key, cancellation and scopes.
5. Run one real PostgreSQL fixture through alternating create/delete cycles and
   shared limiter instances; verify the denied extra request changes no rows.
6. Update owning docs and stale TODO, then carry the exact source to the real
   independent reviewer and develop integrator121 with execution evidence.

## Validation

Execute compiled tests only inside owned Linux Docker runners. Reuse the known
local ZIP module cache recipe; no direct host module metadata scan, host test.exe
or host listeners. Focused tests and User short/race/vet/lint cover this change;
hosted exact-head CI, deployed behavior and final acceptance remain distinct.

## Progress

- [x] Canon, current handlers/constructors, Redis wiring and ownership inspected.
- [x] Ordinary quota choice and failure policy recorded in TT BE-220 c1.
- [ ] RED/GREEN and verification evidence recorded below after execution.

## Risks And Follow-Ups

The limiter is shared ephemeral anti-abuse state, not an ownership or retention
authority. Existing profile-count concurrency and account-erasure semantics are
not changed. Raw metadata trust must not be expanded; BE-181 owns protected S2S.
Real Redis/production verification and independent source review are required
before claiming their respective gates passed; miniredis alone is not deployment.
