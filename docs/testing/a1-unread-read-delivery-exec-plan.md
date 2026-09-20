# ExecPlan: A1 unread, read, and delivery consistency

## Purpose

Prove the documented A1 per-member message state vertical: a DM recipient can
advance delivery/read state without affecting the sender's own unread state,
and each group/channel member keeps an independent unread cursor and count.
The durable chat-list state must remain the post-reconnect authority; Realtime
only transports the explicitly ephemeral acknowledgement events.

## Context

- Docs: `docs/PLAN.md` A1, `docs/features/text-chat.md` (chat-list metadata,
  MarkRead and delivery state), `docs/ARCHITECTURE_REQUIREMENTS.md` (dual
  paths), `docs/microservices/messaging-service.md` and
  `docs/microservices/realtime-service.md`.
- Code/tests: Messaging gRPC/store read-state and delivery acknowledgement
  consumer tests; Gateway compose contracts where they can prove the public
  integration.
- Constraints: do not run local Realtime Go/race/Compose/localhost tests.
  Realtime runtime evidence is hosted CI only. No global WebSocket catch-up or
  A2 Space behavior is in scope.

## Scope

- In: documented durable unread/read behavior for DM, group, and channel;
  durable DM delivery-state derivation; contract-level proof of the Realtime
  acknowledgement boundary.
- Out: read-receipt privacy policy beyond existing coverage, group/channel
  delivery ticks, view-count implementation, global WS history catch-up, and
  Space behavior.
- Documentation gap: the feature docs explicitly say group/channel list-badge
  parity is a partial shipment. The plan will not assign a stricter group or
  channel read transition unless the existing service contract already defines
  it; any missing normative transition is reported rather than invented.

## Milestones

- [x] Establish current Messaging/Realtime/Gateway contract evidence.
- [x] Add focused contract coverage only for documented, observable transitions.
- [ ] Make the smallest repair if a RED test exposes a defect.
- [ ] Run permitted scoped checks and hosted CI; merge only after required gates.

## Detailed Steps

1. Inspect current read-state, delivery-state, and public Gateway tests against
   the documented dual-path boundary.
2. Add focused Messaging regression/contract tests for per-member count and
   cursor isolation for every already-defined chat type, plus DM durable
   delivery/read derivation where that contract is defined.
3. Run the focused test target to record RED. If the test instead demonstrates
   existing behavior, retain it as GREEN evidence and do not change semantics.
4. Implement only the repair necessary to satisfy a documented failing test.
5. Run permitted Messaging/Gateway checks. Push a PR, observe exact hosted CI,
   repair/retry only where evidence identifies a cause, merge with a merge
   commit after all base/head gates are green.

## Validation

- [ ] Focused Messaging tests prove the documented state transitions.
- [ ] Permitted non-Realtime local static/unit checks pass.
- [ ] Hosted CI supplies the Realtime runtime evidence.

## Progress

- [x] Lease acquired: Treehouse slot 14.
- [x] Documentation-first plan written.
- [x] Contract inspection: existing A1 integration coverage proves DM/group/channel
  per-member read cursors/counts, DM delivery derivation, and group/channel
  tick suppression. Added negative delivery-ack decoding contract cases.
- [x] Focused GREEN: `go test . -run '^TestDeliveryAckFromEvent' -count=1`,
  `go test ./internal/store -run TestDeriveLastMessageDeliveryState -count=1`,
  and privacy-boundary unit tests passed. A true local RED/green integration
  cycle is infeasible because the PostgreSQL test harness panics on Windows
  rootless Docker; this is recorded for hosted CI rather than bypassed.
- [ ] Repair/green evidence.
- [ ] PR, hosted CI, and merge.

## Decisions

- Durable list state is tested in Messaging because the docs designate it as
  authoritative after reconnect; WS acknowledgement is not treated as durable
  list state.
- Group/channel unread tests may only assert behavior that their current
  contract and docs define. The explicit partial-shipment note blocks invented
  parity behavior.

## Risks And Follow-Ups

- Local Realtime execution is prohibited for this task, so hosted CI remains
  mandatory evidence for runtime delivery handling.
- If docs do not define a group/channel transition exposed by the existing API,
  record the precise gap for A1 ownership rather than silently extending scope.
