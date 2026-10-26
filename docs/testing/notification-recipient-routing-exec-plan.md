# ExecPlan: Notification-owned message notification routing (BE-111)

## Purpose

Deliver live in-app message notifications to connected authorized members even
when they have not opened/subscribed to the chat. Notification owns policy;
Realtime owns WebSocket delivery.

## Context

The contracts are `docs/microservices/realtime-service.md` (In-app notification
fan-out), `docs/microservices/notification-service.md`,
`docs/features/notifications.md`, and `docs/todo/backend.md` BE-111.
Existing Realtime fan-out derives personal targets from chat subscriptions and
does not obtain Notification settings. Notification already resolves effective
policy and Chat's complete member pages, but needs current recipient Chat/Social
authorization for this new boundary.

## Scope

Message sent, mentions and reactions use a synchronous internal NATS routing
request containing the exact source event bytes. Notification returns only
allowed recipient/type pairs bound to the SHA-256 of those bytes. No public RPC,
new durable notification stream, inbox, message catch-up, or push provider change.
Realtime rechecks current delivery authorization and keeps lifecycle admission.

## Milestones

- [x] Add request/response binding and negative contract tests.
- [x] Implement Notification membership, Chat/Social authorization and policy.
- [x] Wire the real NATS request/reply boundary and Realtime consumer.
- [x] Prove unsubscribed delivery and mute/type/auth suppression on Linux.
- [x] Update contracts and run affected checks.
- [ ] Obtain independent review and deliver the pushed change through a PR.

## Detailed Steps

1. Add shared bounded, versioned live-routing transport in backend/pkg.
2. Write tests before implementation for policy and source binding.
3. Implement recipient auth using ordinary Chat GetChat metadata (no internal
   bypass), User-owned profile/account lookup and Social-directed blocks.
4. Bind Notification's queue responder on its existing NATS connection. Realtime
   requests the decision while processing its existing message stream event.
5. Apply the decision only to personal notification fan-out, preserving existing
   message subscriptions, archive activity and lifecycle/session gates.

## Validation

Use Docker Linux Go 1.26 with the owned worktree mounted at /workspace; never
run Realtime Go tests on native Windows. Run focused tests first, then affected
short suites, vet and meaningful race/real NATS boundary tests. Record exact
source SHA and dirty diff identity. Hosted CI and runtime acceptance are separate.

## Progress

- [x] TT BE-111 claimed by Anton-121; Bot120 has no overlapping files.
- [x] Worktree 81 leased exclusively; base de623fb325938de68252910e8e1b84f195e013a6.
- [x] Transport RED: the initial unavailable decision rejected valid allowed
  and empty decisions. Shared contract tests then passed with binding validation.
- [x] Linux `go test -short ./...` passed in Notification and Realtime.
- [x] Linux scoped `-race` passed for routing/authorization and the three actual
  Realtime message consumer scenarios. Both service `go vet ./...` passed.
- [x] Canonical-ACL embedded NATS tests passed for allowed, mute, suppressed type,
  authorization denial, forbidden forged response and missing responder grant.
- [x] Issuer and disposable-fixture suites passed. Issuer issuance tests require
  an actual named non-root Linux user; root and unnamed UID runs were rejected
  by the unchanged identity guard, then the `nobody` fixture run passed.
- [x] Existing Realtime consumer tests retain their assertions and explicitly
  provision the new Notification decision responder.
- [x] Notification and Realtime static lint reported zero issues.
- [ ] Final shared-package lint, Graphify and independent review/delivery.

## Decisions

- Use live internal NATS request/reply, without a new domain stream. A source
  hash binds the response to the exact event; timeout/errors yield no personal
  notification and never fallback to membership/subscription authority.
- Shared group/channel blocks are recipient→sender; DM checks both directions,
  following the existing Realtime privacy policy.
- Notification may publish only the exact dedicated routing reply wildcard in
  the reviewed ACL. Issuer and disposable-fixture validators have an exact
  owner-scoped exception; dynamic response permission remains disabled.

## Risks And Follow-Ups

This is an ACL/privacy cross-service change requiring independent review.
Large membership/policy reads share one bounded deadline and return no partial
allow set on uncertainty. Deployment and final product acceptance are excluded.
Old NATS credentials do not acquire the new subject/reply grants automatically;
their source-bound rollout remains an operator/integration action. No issued
credential, live provider, shared DEV/STAGING state or deployment is changed here.
