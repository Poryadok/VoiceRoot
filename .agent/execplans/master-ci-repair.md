# ExecPlan: repair master CI after PR #435 integration

## Purpose

Restore a green receiving CI and full `ci.yml` run on `master` after integrating
the account-deletion work. The repair must preserve documented product contracts
and make delivery/startup failures observable through regression tests.

## Context

- Docs: `docs/features/auth-and-contacts.md`, `docs/todo/client.md`,
  `docs/TESTING.md`, `docs/microservices/social-service.md`.
- Failing full run `35749452666`: Compose billing, phone-sync, Search, Space Pro,
  Stories, and A1 account deletion delivery.
- Phone contact sync is explicitly post-alpha; its documented response is
  `phone_contact_sync_unavailable` / HTTP 409.
- Architecture boundaries remain unchanged: Realtime owns WebSocket delivery,
  Search owns its JetStream consumer, and Subscription owns provider webhook
  verification.

## Scope

- In: correct stale Compose assertions; fix implementation/configuration faults
  proven by the failed run; add focused regression coverage; run receiving CI,
  merge, then run the full workflow on master.
- Out: enabling post-alpha phone-book sync or weakening documented delivery and
  webhook authentication behavior.
- Documentation gaps: production provider-event key rotation configuration must
  be derived from existing repository conventions before implementation.

## Milestones

- [x] Confirm the Phone Sync Compose failure against its documented contract.
- [x] Add red regression tests for File entitlement wiring, Search durable startup,
  Subscription event keys, and Realtime overflow; make their focused checks green.
- [ ] Obtain green receiving CI for the repair PR and merge it.
- [ ] Obtain a green full `ci.yml` run on master.

## Detailed Steps

1. Inspect the failing tests and their owning service contracts.
2. Change only the Compose phone test to assert its documented 409 result.
3. Add focused tests before correcting Realtime delivery, Search startup, and
   Subscription webhook-key wiring.
4. Diagnose billing and stories responses from their handlers before changing
   either expectation or implementation.
5. Commit and push the minimal repair; wait for receiving CI, merge, and run the
   full profile. Cancel a failing full run immediately and repeat from its first
   failed job.

## Validation

- [ ] Focused Go and Flutter tests for changed behavior are green.
- [ ] Receiving CI for the repair PR is green.
- [ ] Full `ci.yml` profile on master is green.

## Progress

- [x] Integrated #431 and #435 into #435; merged it after its receiving CI.
- [x] Fixed bounded Realtime JetStream startup retry in #436 and merged it.
- [x] Started full master run; cancelled it after early failures.
- [x] Correct Phone Sync contract, File entitlement wiring, Subscription event-key
  wiring, Search replay startup, and Realtime overflow behavior.
- [ ] Submit the repair and use CI to resolve any remaining Story-specific failure.

## Decisions

- Preserve the 409 phone-sync contract because `docs/features/auth-and-contacts.md`
  and `docs/todo/client.md` explicitly define it.
- Treat `dm_peer_deleted` as a delivery reliability issue rather than deleting
  the raw WebSocket assertion, because the product flow requires the event.

## Risks And Follow-Ups

- Compose failures may expose separate independently-owned defects; do not merge
  speculative behavior changes merely to make a smoke test pass.
- The full CI remains the release gate after focused tests and receiving CI.
