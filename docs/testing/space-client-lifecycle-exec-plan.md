# ExecPlan: Space client lifecycle (BE-133)

## Purpose

Expose the documented Space lifecycle through the authenticated Flutter shell.
The first consumed slice lets an ordinary member leave from the Space header,
with confirmation, explicit owner guidance, safe failure handling and local
media cleanup. BE-133 remains open for audit, transfer and recovery lifecycle.

## Context

- Behavior: [Spaces](../features/spaces.md), particularly ownership and audit.
- HTTP: [Gateway A2 contract](../microservices/api-gateway.md#a2-space-rest-contract-target).
- Runtime: [Space Service](../microservices/space-service.md).
- Verification: [TESTING](../TESTING.md), [CONTRIBUTING](../CONTRIBUTING.md).
- Sources: `VoiceSpacesClient`, `SpaceTreeColumn`, `SpaceLeaveActions`,
  `LeaveSpaceSheet`, `CallController` in `src/frontend/lib/`.
- Existing empty-body `POST /api/v1/spaces/{id}/leave` returns 204; Space owns
  membership removal and rejects an owner before removal. Target idempotency,
  audit filters and transfer/recovery coordination are separate backend seams.

## Scope

The first slice consumes the existing leave route without changing the backend
contract. UI permission hints do not replace backend authorization. Keep the
broader feature status unchanged until runtime and independent acceptance exist.
Do not expose unimplemented transfer/delete/restore controls as working actions.

## Milestones and progress

- [x] Characterize the existing route and owner denial.
- [x] Add confirmed leave and an accessible header entry at narrow width.
- [x] Preserve selection on denial; show an uncertain outcome without blind retry.
- [x] Fence duplicate submission, session generation and newer navigation.
- [x] Detach matching profile/Space media and fence an in-flight room join.
- [x] Run focused HTTP/state/widget/call regression tests (36 tests passed).
- [x] Complete static analysis (`flutter analyze --no-pub`: no issues).
- [ ] Obtain independent source review of the exact head.
- [ ] Publish PR, required CI and merge after GitHub transport recovery.
- [ ] Consume BE-008 audit route and BE-009 transfer coordinator when available.
- [ ] Implement proof-bound transfer and later delete/restore UI against their
  real activated contracts, preserving operation IDs and coarse failures.
- [ ] Prove the documented multi-client Space lifecycle in the live environment.

## Detailed steps and validation

From `src/frontend`, run `flutter test --no-pub test/space_leave_test.dart
test/leave_space_sheet_test.dart test/call_providers_test.dart
test/space_invites_permission_test.dart`, then `flutter analyze --no-pub`.
The new tests exercise the public client, a real narrow Space header, owner
guidance in English/Russian, rejected mutation diagnostics, profile generations,
newer selection, matching media and late join/disconnect completion. Existing
call and invite tests protect neighboring behavior. Required CI and compose
proof remain independent gates; local mock execution is not live acceptance.

## Decisions

- Keep the shipped empty-body leave contract. It has no durable operation ID;
  network/5xx responses stay uncertain and refresh membership rather than retry.
- Read Auth's real request generation synchronously. Lazy provider notifications
  can coalesce a switch-away/back and cannot establish response ownership.
- Local cleanup never invokes generic EndCall after membership removal. It
  detaches only the matching Space/profile and retires its pending join, so a
  late callback cannot clear another call or reconnect the departed room.
- Put the lifecycle menu beside the Space title; keep the existing toolbar in
  a horizontal scroller so the new entry remains reachable at narrow width.

## Risks and follow-ups

Local media cleanup does not prove cross-service server roster revocation or
participant delivery. Live proof must cover those paths, audit persistence and
transfer/owner leave. Full A2 acceptance, deployment and tester appointment are
not implied by this slice. Review and integration custody remains with a real
ant; task comments preserve the exact source and resource continuation.
