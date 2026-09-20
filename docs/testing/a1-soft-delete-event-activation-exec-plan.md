# ExecPlan: A1 soft-delete durable event activation proof

## Purpose

Activate the already implemented Auth `user.account_deleted` → Chat
`chat.dm_peer_deleted` → Realtime `dm_peer_deleted` path in the hosted A1
profile-handoff proof. The proof must observe the live, recipient-targeted
event while retaining the existing REST authority checks.

## Sources and contract

- `docs/PLAN.md` A1: deletion revokes sessions, blocks both DM directions,
  hides fresh snapshots, and has one local non-persisted deleted-user marker.
- `docs/features/auth-and-contacts.md`: a selected known DM returns
  `dm_peer_state=DELETED`; the live event contains only `chat_id` and the
  surviving recipient profile, and cannot replace REST recovery.
- `docs/microservices/chat-service.md` and `realtime-service.md`: Chat emits
  `chat.dm_peer_deleted` only to the surviving profile; Realtime delivers it
  as non-replay live acceleration.
- `docs/TESTING.md` and `.github/workflows/ci.yml`: T106 runs in the hosted,
  isolated `compose-a1-flutter-profile-handoff` PR job.

## Scope

In: attach a recipient WebSocket to T106 before Auth deletion, assert exactly
one `dm_peer_deleted` frame for the known DM, assert its two-field privacy-safe
payload, then retain all existing session, snapshot, send-deny, and byte-stable
history/cursor assertions.

Out: a new deletion protocol, durable tombstones, restore/purge, attachment
GC, global WS replay, or any Windows-local Compose/Realtime run.

## RED → GREEN

1. RED: make the T106 test require the live event; before its event observer
   exists, the exact assertion times out in hosted CI.
2. GREEN: connect and subscribe the surviving profile before deletion; collect
   matching frames and assert one targeted frame with no deleted identity.
3. Duplicate source contract: retain and run the existing Chat durable
   `TestAccountDeletedConsumer_RedeliveryReusesStableTargetIdentity` proof. It
   drives source redelivery and verifies stable child IDs; JetStream message-id
   de-duplication prevents a second Chat fan-out.

## Verification

- Fast static/Flutter test compilation where feasible on the lease; no local
  Compose, Realtime runtime, race, or localhost execution.
- Hosted PR `compose-a1-flutter-profile-handoff` proves the end-to-end Auth →
  Chat → Realtime path on the exact head.
- Hosted Chat test proves source redelivery does not duplicate fan-out.
- Require `ci-gate` green on the same exact head before merge.

## Completion checklist

- [ ] T106 observes one targeted, privacy-safe live event.
- [ ] Existing REST authority and stable-history assertions remain unchanged.
- [ ] Hosted exact-head CI and `ci-gate` are green.
- [ ] Merge commit is verified on `master`; Treehouse lease is returned.
