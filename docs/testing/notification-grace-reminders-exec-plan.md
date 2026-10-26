# ExecPlan: durable Notification grace reminders (BE-118)

## Purpose

Persist revision-bound D1/D3/D7 reminder work so broker acknowledgement means a
committed local effect, never merely recognizing a payload. Deliver to the
existing payer primary profile through the ordinary Notification policy and
provider dispatch boundaries. In-app insertion is unique; push is at-least-once.

## Context

The accepted behavior is in `subscription-lifecycle-convergence-exec-plan.md`
(consumer inbox and Space Pro/reminder sections), `../features/subscription.md`,
`../features/notifications.md`, `../microservices/notification-service.md`,
`../microservices/user-service.md`, `../DATA_MODEL.md`, `../DATA_STORES.md`, and
`../ARCHITECTURE_REQUIREMENTS.md`. Verification follows `../TESTING.md` and
`../CONTRIBUTING.md`.

The current subscription consumer validates a legacy payload, logs and ACKs.
Notification has genuine token, policy, presence and PushDispatcher paths.
Realtime currently constructs in-app notifications in domain-specific event
consumers. User's read-only `ResolvePrimaryProfileIDs` filters `is_primary` and
`deleted_at`; its ordinary listener only has the legacy internal caller gate.
`EnsurePrimaryProfile` is a mutator and cannot serve as a recipient lookup.

## Scope

BE-118 owns Notification inbox/projection, logical uniqueness and leased channel
work. BE-117 owns Subscription producer/schedule/proto writes. The agreed wire
uses protocol 1, PERSONAL/SPACE aggregate key and origin grace revision; fixed
event ID per aggregate/revision/day; payer account ID, premium/space_pro and exact
optional Space ID; windows derive from the same-revision grace snapshot. D1 is
[start,start+P2D), D3 [+P2D,+P6D), D7 [+P6D,start+P7D).

## Milestones

- [x] Confirm the producer wire directly with BE-117 owner.
- [x] RED/GREEN durable inbox, immutable identity, projection ordering and windows.
- [x] Unique pending channel work and fenced retry with real PostgreSQL restart proof.
- [x] Real NATS regression: missing delivery runtime retains reminders for retry.
- [ ] Unique in-app insertion and actual push dispatch effects.
- [ ] Connect closed authoritative primary and final revision comparison.
- [ ] Connect genuine Realtime delivery and prove push/in-app through local fixtures.
- [ ] Independent review and delivery custody at exact pushed head.

## Detailed steps

1. Add focused real PostgreSQL tests under Notification internal code before
   production changes. Assert ahead retries, stale no-op, conflicting event bytes,
   logical uniqueness and expired-lease fencing.
2. Add Notification-owned migration and transactional processing. Exact replay
   must not repeat effects; a different body under one event ID is a mismatch.
3. Use DB time for windows and leases. A successor snapshot suppresses pending
   work; unavailable authorities retry only inside the open window.
4. At each dispatch use Subscription's protected current snapshot with minimum
   revision and authoritative User primary read. Apply ordinary settings,
   presence, quiet hours, release and Space lifecycle gates.
5. Prove real local PG/NATS/provider retry and crash cases, then run affected
   Go tests, vet and pinned lint. Serialize Graphify update with other owners.

## Validation

Run `go test ./internal/subscriptionreminders -count=1` (real PostgreSQL), focused
consumer/dispatch tests, `go test -short ./...`, `go vet ./...`, and pinned lint.
Tests cannot label stored pending work as provider or WebSocket delivery.

## Progress

- [x] Claim BE-118 atomically and acquire own Treehouse lease.
- [x] Inspect actual authority/store/transport paths with CodeGraph.
- [x] Implement and verify the durable slice: focused real PostgreSQL and race
  tests pass, including concurrent replay and changed identity/revision conflicts.
- [x] Prevent recognized grace reminders (including legacy unversioned payloads)
  and canonical lifecycle events from being ACKed by the logging-only consumer.
  Real NATS regression failed on the former ACK path, then passed with delayed
  NAK/redelivery; the original durable survives restart.
- [x] Notification `go vet ./...` and `golangci-lint run ./...`: pass, zero issues.
- [ ] Whole Notification short suite: all package checks pass except the existing
  Windows Unix file-mode assertion in `TestReleaseSnapshotPrivateBoundAndExclusive`
  (expected 0600, actual 0666). This change does not touch that path.

## Decisions

The producer wire agreement does not establish runtime activation. Do not select
the first profile, provision a profile, impersonate a payer, send email, or assume
an unavailable authority means grace/offline. Do not change frozen routing PR929.

## Risks and follow-ups

The protected `ResolveEntitlementAtBoundary` is documented but absent from the
current proto/runtime. A closed Notification caller path for User primary lookup
and a generic Notification-to-Realtime in-app transport also need implementation.
These are explicit activation prerequisites; until genuine paths exist, no live
consumer or provider is enabled and BE-118 remains incomplete. Restore/purge
bootstrap must also precede authoritative cutover. No real provider acceptance,
DEV/STAGING deployment or final acceptance is claimed by local tests.

The durable channel table contains **pending work**, not an in-app insertion or
provider receipt. No worker is registered and no unavailable transport is
substituted by a fake success. The live compatibility consumer now retains grace
reminders with broker-managed delayed retry while the genuine runtime is absent.
Durable poison quarantine/alerts, privacy purge/restore and P400D cleanup remain
cutover work; current rows are retained indefinitely and this package is not
activated. Existing related owner work is BE-181 (Messaging-only protected
boundary read), BE-047 (sanction in-app transport), and BE-111/PR929 (message
routing); none is silently declared to implement the reminder-specific boundary.
