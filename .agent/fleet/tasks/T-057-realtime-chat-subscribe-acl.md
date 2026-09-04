# ExecPlan: T-057 — Realtime chat subscription ACL

## Purpose

Make each lazy WebSocket `subscribe` prove that the active user/profile may
currently view the requested chat before it becomes a local subscription. A
denied or unavailable Chat check must leave the connection usable, emit the
generic `permission_denied` wire error, and grant none of the follow-on
subscription-gated actions.

## Context

- `docs/PLAN.md` keeps this work inside active milestone A1.
- `docs/microservices/realtime-service.md` defines the unchanged subscribe
  request/ack and sequence-bearing WebSocket protocol.
- `docs/microservices/chat-service.md`, `docs/DATA_MODEL.md`, and
  `docs/ARCHITECTURE_REQUIREMENTS.md` make Chat the owner of chat membership
  and profile-scoped access.
- `src/backend/realtime/ws.go` currently accepts every syntactically valid
  UUID and records it in both a connection-local map and `wsHub`.
- `src/backend/chat/internal/grpcsvc/members.go:GetChat` already authenticates
  caller profile metadata and denies non-members. Realtime must use that RPC
  with `x-voice-user-id` and `x-voice-profile-id`, never
  `x-voice-internal-caller`.

## Scope

- In: a fail-closed Chat `GetChat` subscription checker; generic denial wire
  mapping; `connReg.chats` as the sole subscription authority; removal/leave
  revocation for every local tab; tests and fakes.
- Out: a positive cache, changed subscribe/ack shape, message history catch-up,
  and a strict linearizable ordering between a membership event and an
  in-flight `GetChat` request.
- Documentation gap: none. The generic denial mapping and no-internal-caller
  constraint are explicit task requirements.

## Milestones

- [x] RED: compile and run the public WS handler against fail-closed
  valid-UUID denial, malformed-input compatibility, profile isolation, and
  local membership-event revocation expectations.
- [x] GREEN: add a Chat `GetChat` checker and pass it through the Realtime
  handler/bootstrap wiring.
- [x] GREEN: authorize subscribe before `wsHub.addChat`; use `connReg.chats`
  for every subscription-gated action; revoke matching local registrations on
  member removed/left events.
- [ ] Verify focused tests, `go test ./...`, lint, and review the wire
  contract.

## Detailed Steps

1. In the GREEN change, add a gRPC checker test server which records incoming
   metadata and returns each relevant status (`OK`, `NOT_FOUND`,
   `PERMISSION_DENIED`, `INTERNAL`, `UNAVAILABLE`, timeout, and a
   nil/misconfigured client). Assert user/profile metadata, the absence of
   `x-voice-internal-caller`, and a second `GetChat` call for duplicate lazy
   subscribe (no positive cache).
2. Add WebSocket tests for allowed/duplicate/unsubscribe and every generic
   denial. Assert a malformed UUID makes no Chat call; a denial emits one
   sequence-bearing `error`, does not emit an ack, does not enter hub state,
   and the socket remains usable.
3. Add tests that member `removed` and `left` events clear all local
   registrations for the profile/chat. Verify then `typing`, `mark_read`, and
   `delivery_ack` fail as unsubscribed, including multiple local tabs. Add the
   real `presence_update` WebSocket-op fan-out check after the checker seam is
   available; RED currently covers only the hub-level chat-scoped presence path.
4. Implement only after RED review: invoke `GetChat` with caller metadata and
   a bounded context; collapse all non-success results to the generic error;
   do not cache a successful result.
5. Replace the connection-local `chatSubs` authority with `connReg.chats` and
   centralize local revocation in the hub event path.

## Validation

- [x] Baseline attempt: containerised `go test ./...` began dependency download;
  host Go module cache is malformed (the Prometheus module is nested one level
  too deep), so host compilation cannot start.
- [x] `gofmt` and `git diff --check` pass for the GREEN change.
- [ ] Focused GREEN test execution is externally blocked: a disposable module
  cache reaches `proxy.golang.org` only to time out DNS, before compiling the
  package. No source test failure is being represented as a passing result.
- [ ] RED: focused `go test` must fail only on the missing ACL implementation
  / changed wire behavior, never because a test accepts an unauthorized chat.
- [ ] GREEN: `cd src/backend/realtime && CGO_ENABLED=0 go test ./...` and
  `golangci-lint run ./...`.

## Progress

- [x] Read the required project, A1, Realtime, Chat, architecture, data-model,
  testing, and contribution sources.
- [x] Created `fix/realtime-chat-subscribe-acl` from `origin/master`.
- [x] Established the current unsafe subscribe path and Chat `GetChat` caller
  metadata behavior.
- [x] Add compileable behavioral RED tests and stop for independent review
  before production code.
- [x] Add checker fake/status/metadata/timeout/INTERNAL/no-positive-cache and
  real `presence_update` fan-out tests with the GREEN dependency seam.
- [x] Document the lazy-subscribe fail-closed behavior and membership-event
  revocation in `docs/microservices/realtime-service.md`.
- [x] Implement the minimal Realtime-only checker, `connReg.chats` authority,
  and removed/left local-tab revocation.

## Decisions

- Fail closed on every checker error, including absence of configuration: an
  unverified subscription is not an authorized subscription.
- The current RED API has no checker injection seam. Its executable coverage is
  deliberately limited to public-handler fail-closed behavior; checker-specific
  metadata/status/timeout/INTERNAL/no-positive-cache assertions are mandatory
  GREEN tests, not claimed as RED evidence.
- `Chat.GetChat` is the minimal verifier because it already applies the caller
  profile's effective membership/deleted-for-self ACL. Realtime forwards only
  normal user/profile metadata and collapses every error to the generic WS
  denial; it does not add a Role or Chat schema dependency.
- Preserve `invalid_subscribe` only for malformed UUIDs before any checker
  call. Every well-formed denied/unavailable result is intentionally
  indistinguishable on the wire.
- Do not test a total order between revocation events and already-running
  authorization checks. Such an epoch/linearizability guarantee would need a
  separate design and is recorded as a follow-up rather than invented here.

## Risks And Follow-Ups

- A `GetChat` result can race a subsequent membership removal. The event path
  revokes extant local subscriptions, but strict event-vs-in-flight-check
  linearizability remains a deliberate follow-up.
- Existing bootstrap subscriptions are derived from Chat `ListChats`; GREEN
  work must preserve their documented degraded behavior while ensuring they do
  not become an alternate authority for post-revocation actions.
- Hub-level presence fan-out is RED-covered after local revocation. The actual
  client `presence_update` op needs a separate GREEN regression once the
  checker fixture can establish an allowed lazy subscription.
