# ExecPlan: Realtime targeted `dm_peer_deleted` fan-out

## Purpose

Make Realtime translate the typed `chat.dm_peer_deleted` JetStream event into a
targeted WebSocket `dm_peer_deleted` frame for the surviving DM participant.
The frame carries the chat and recipient profile identifiers required by the
Flutter P4b terminal-DM-state contract, without a deleted account or profile
identity.

## Context

- Docs: `docs/features/auth-and-contacts.md` (live acceleration only for the
  surviving `recipient_profile_id`), `docs/microservices/chat-service.md`
  (event fields), `docs/microservices/messaging-service.md` and
  `docs/ARCHITECTURE_REQUIREMENTS.md` (durable recovery remains
  `GetMessages.dm_peer_state`), and `docs/microservices/realtime-service.md`
  (WebSocket operation and no replay).
- Contract: `protos/voice/events/v1/jetstream_events.proto` defines the typed
  `ChatStreamEvent.dm_peer_deleted` oneof at tag 19 and `DmPeerDeleted` with
  exactly `chat_id` and `recipient_profile_id`.
- Client compatibility: historical P4b test
  `src/frontend/test/t056_p4b_chat_room_controller_test.dart` accepts the
  live frame only when both fields bind it to the loaded current profile/chat.
- Code: `src/backend/realtime/chat_events_consumer.go` maps `chat.events` to
  `fanoutEnvelope`; `wsHub.broadcastToProfile` is the existing profile-target
  delivery path. IDs crossing service boundaries are UUIDs per
  `docs/DATA_MODEL.md`.

## Scope

- In: documentation correction; fail-closed mapping of the typed arm;
  UUID validation of both required IDs; targeted profile fan-out; unit and
  JetStream/hub regression coverage.
- Out: producer changes, proto changes/regeneration, Flutter changes, User
  lookups, chat membership lookup, REST recovery, durable event deduplication,
  session-epoch/T057 changes, and broadening the event to chat subscriptions.
- Documentation gap: the Realtime op table said `d.chat_id` only. This plan
  corrects it to the proto/client-compatible two-field payload.

## Milestones

- [x] Read the governing docs, ChatStreamEvent proto, historical Flutter P4b
  contract, and existing Realtime consumer/hub patterns.
- [x] Commit the doc correction and RED tests without production changes
  (`b65d730f`).
- [x] Obtain independent Luna and Terra reviews of the RED commit; no critical
  findings were reported.
- [x] Implement the smallest mapping and targeted fan-out change.
- [ ] Run focused tests, formatting and static checks; commit GREEN.
- [ ] Obtain a fresh independent implementation review and resolve blockers.

## Detailed Steps

1. Add tests for a valid tag-19 event: its returned envelope has operation
   `dm_peer_deleted`, JSON `d` has exactly `chat_id` and
   `recipient_profile_id`, and the target is the recipient profile.
2. Add table-driven negative mapping tests for missing, nil, and malformed
   chat/profile UUIDs; each must return `ok=false` so malformed input cannot
   fall through to a broadcast path.
3. Add a JetStream-to-hub test with the intended recipient and a different
   profile subscribed to the same chat. It proves only recipient connections
   receive the event, guarding against accidental `broadcastToChat` use.
4. Run the focused Realtime test command and record the expected RED failures
   before touching production code. Commit only plan/docs/tests.
5. After RED review, add the `ChatStreamEvent_DmPeerDeleted` case in
   `chatEventBytesToFanout`: validate both UUIDs, construct the two-field JSON
   payload, return the recipient profile target, and reuse the existing
   profile branch in `subscribeChatEvents`.
6. Run the same test command green; run `gofmt`, `go vet`, and diff checks.
   Do not add a dedup store: duplicate semantics stay with the existing
   JetStream/event handling policy.

## Validation

- [ ] `cd src/backend/realtime; CGO_ENABLED=0 go test -run 'TestChatEventBytesToFanout_DmPeerDeleted|TestSubscribeChatEvents_DmPeerDeleted' .` — attempted for RED; package setup is blocked before tests by the missing local module-cache source `github.com/prometheus/client_golang/prometheus` (declared in both `realtime/go.mod` and `pkg/go.mod`). Network use is out of scope.
- [x] `gofmt -w chat_events_consumer.go chat_events_consumer_test.go` completed.
- [ ] `cd src/backend/realtime; go vet ./...` — attempted after GREEN and
  blocked at the same missing local Prometheus module source before analysis.
- [ ] `git diff --check` and final scoped diff contain only plan, Realtime
  documentation, consumer, and consumer tests.

## Progress

- [x] Scope and source contracts established; implementation is not started.
- [x] RED tests written. The focused invocation reached package setup but could
  not execute them because the local Go module cache lacks
  `github.com/prometheus/client_golang/prometheus`; this is an environment
  blocker, not a passing result.
- [x] RED review accepted: fresh Luna and Terra reviews found no code blocker.
- [x] GREEN implementation complete: `DmPeerDeleted` validates both UUIDs,
  emits the exact two-field frame, and returns only the recipient profile ID
  for the existing profile fan-out branch.
- [ ] Focused test and `go vet` execution remain blocked at Go package setup
  by the missing local Prometheus module source; `gofmt` completed.
- [ ] GREEN review accepted.

## Decisions

- Validate only wire shape/UUID syntax in Realtime. The event already names
  the surviving profile; querying User or Chat would add forbidden coupling
  and is not required by the contract.
- Deliver through `broadcastToProfile`, not `broadcastToChat`, because P4b
  treats `recipient_profile_id` as a binding field and the event must not leak
  to another profile subscribed to the same DM.
- Keep `chat_id` in the frame even though it is not a routing key: the client
  uses it to bind the live acceleration to its selected known history.

## Risks And Follow-Ups

- This is an ephemeral acceleration only. Clients must still recover deletion
  after reconnect through Messaging REST; no replay or persistent tombstone is
  introduced here.
- Existing consumer delivery/dedup semantics are intentionally unchanged. No
  durable correctness claim is added for duplicate NATS deliveries.
- Host Go dependency resolution may be blocked by the known Windows TLS/proxy
  issue documented in `docs/TESTING.md`; if it occurs, record the exact error
  rather than treating it as a code failure.
