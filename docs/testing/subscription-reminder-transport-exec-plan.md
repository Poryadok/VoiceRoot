# Subscription reminder hint transport (BE-118-RT)

## Purpose and canon

Deliver the accepted `subscription_grace_reminder` subtype to the authorized
primary profile without representing broker acceptance as a durable in-app effect.
Sources: [notifications](../features/notifications.md),
[Notification service](../microservices/notification-service.md),
[screen controls](../design/screen-controls.md) §5.4 payment-failure banner,
[data retention](../DATA_STORES.md) A7, and
[WS boundary](../ARCHITECTURE_REQUIREMENTS.md) WS vs REST.

## Agreed wire

Anton-146 (Notification consumer owner) explicitly accepted this partition and
semantics in coordination with Anton-148. This agreement does not activate the
unfinished Notification worker, protected primary lookup or Subscription authority.

Exact NATS subject: `notification.subscription_grace_reminder`. JSON version 1:
`schema_version`, `notification_id`, `target_profile_id`, `aggregate_kind`
(`PERSONAL` or `SPACE`), `aggregate_id`, `grace_revision` (canonical positive
decimal string), `day` (1/3/7), `plan` (`premium` or `space_pro`), `window_end`
(canonical RFC3339Nano UTC ending Z), and `space_id` only for SPACE, equal to its
aggregate ID. UUIDs are canonical nonzero UUIDs. Notification uses its existing
deterministic logical ID for this aggregate/revision/day.

WebSocket op is `notification`; client `d` contains only `schema_version`,
`type=subscription_grace_reminder`, `notification_id`, `target_profile_id`,
`plan`, `day`, `window_end`, and optional SPACE `space_id`. PERSONAL aggregate ID
is a raw account ID and MUST NOT cross into client data. No payer, provider,
billing identifier or external payment URL is client hint data.

Notification atomically inserts the unique durable in-app row AND retriable hint
intent after current primary/policy/final Subscription authority checks. The
insert receipt completes insertion. PubAck completes only the separate hint
intent. Realtime ACK means a finite local fanout attempt, not client receipt.
Duplicates preserve the same ID; the client collapses them. Realtime and client
suppress at `now >= window_end`. Offline has no socket/receipt/replay guarantee;
there is no new history/read-sync product or global WS catch-up. Logical outcome
retention and P30D privacy purge follow DATA_STORES.

## Scope and ownership

Anton-148 owns shared hint codec, dedicated Realtime consumer/fanout, client
recognition/banner and focused tests. Anton-146 owns insertion/worker/primary
authority; Anton-147 owns Subscription boundary authority. PR929 remains frozen.
Owner67 permits the isolated additive startup/ACL/issuer/bootstrap union, while
PR929 remains untouched. Root confirmed the absent historical source actor is no
additional veto; live write ownership remains with its real custodian67.

## Milestones and validation

- [x] Agree exact internal/client field partition with consumer146.
- [x] Test strict schema, expiry, privacy partition and deterministic identity.
- [x] Test real NATS -> authorized WebSocket, wrong targets, revoked sessions,
  invalid data, retries and offline semantics in Linux.
- [x] Test client binding/dedup/expiry and canonical banner/action.
- [x] Register narrow startup/ACL/bootstrap after owner coordination.
- [ ] Independent source review, immutable push/PR and real Luna custody.

## Risks and remaining prerequisites

No provider activation, DEV/STAGING mutation, credentials or acceptance is in
scope. A transport test cannot qualify unavailable current Subscription authority
or an unimplemented durable Notification insertion/intent transaction. Existing
supported billing navigation must retain ordinary authorization; Space purchaser
targeting never grants Space ownership. Unsupported billing actions must remain
an explicit prerequisite, never a fabricated external URL or successful no-op.

Current Flutter has no purchaser-authorized Space Pro billing-management route.
Space hints render their Space context, but expose no substitute personal billing
action. Personal hints open the existing Subscription settings and its existing
management flow. Full Space reminder action remains open until that documented
route is implemented. No externally supplied payment URL is followed.

`REALTIME_SUBSCRIPTION_REMINDER_HINTS=true` explicitly opts in only after the
real Notification insertion/intent worker and protected authorities are ready.
All checked-in deployments retain the default disabled setting. Central bootstrap
owns `notification_reminder_hints` (exact subject, limits/file, MaxAge P7D) and
the per-instance explicit-ACK/deliver-new consumer; Realtime never provisions it.
The publisher returns the actual stream sequence to the Notification owner for
the canonical raw-identity purge ledger. No raw internal payload logging is added.

## Execution evidence

- Linux Go1.26.9: codec tests pass; real NATS/WS and canonical manifest ACL tests
  pass with race detection; scoped issuer validator tests and vet pass.
- Windows Flutter: 32 focused notification/client/widget tests pass, including
  actual personal navigation, distinct Space context, profile reset, expiry and
  deduplication. Scoped analysis: no issues.
- Linux bootstrap contract plus two voice subject-preservation checks pass.
- Initial codec test run failed at missing symbols (scaffolding only, not a
  behavioral RED claim). Integration fixture was corrected to provision the new
  stream's exact push target. No historical-source or deployed acceptance claim.
