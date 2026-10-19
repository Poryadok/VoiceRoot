# Notification ordinary chat push quota (BE-195)

## Purpose and context

Bound ordinary chat push bursts without changing unread/in-app delivery or
critical notifications. Accepted delegated decision: BE-195 comments c4/c5.
Behavior canon: notifications.md, notification-service.md and architecture rate
limits. MessagePusher is the activation consumer; PushDispatcher remains common
to critical and other out-of-scope delivery and must not carry this quota.

## Scope and decisions

Only MessagePusher new_message/message_request/reply/mention: 60 distinct
logical messages per recipient in a rolling 60 seconds. Identity is canonical
recipient/chat/message UUIDs, independent of notification variant and device.
Redis atomically stores ADMITTED/SUPPRESSED for 192 hours from first decision
(message stream max age 168h + 24h); retries never refresh either decision or
window score. Quota denial settles ordinary push; Redis/identity errors retry
without an unmetered fallback. Redis data loss resets this soft QoS state.
Per-device policy/lifecycle checks remain authoritative. Reserve only at the
first eligible device, before grouping; no refunds after provider failure.
Calls, system, reminders, match/voice, Story/social/direct RPC/bulk paths are out
of scope. No generic dispatcher or runtime Redis readiness gate.

## Milestones and detailed steps

- [x] Accept bounded contract and acquire own worktree from develop.
- [x] Canonicalize policy and write regression before production (initial test-only
  compile failed on the missing API; no semantic RED is claimed).
- [x] Implement atomic Redis script/config, lazy MessagePusher wiring and metrics.
- [x] Linux Redis concurrency/replay/expiry/error and dispatch policy tests.
- [ ] Component short tests, vet/lint, immutable source proof and independent review.
- [ ] Transfer exact reviewed source to accepted real integrator121; hosted checks
  and rollout/appointed acceptance remain separate.

## Validation and risks

Run Redis tests against actual owned Redis plus fast miniredis edge controls;
dispatch regression must prove denied/error attempts have zero provider and
grouping effects and retries/multiple devices consume one logical decision.
Check configuration activation and critical-path independence. No host Windows
test executable/listener while user away. All containers private/no host ports.
The quota does not deduplicate provider delivery or promise billing/security
durability. Existing at-least-once duplicate delivery remains possible.

## Progress

Owned WT20/base7bd7dfba, BE-195 owner184. Native Linux Go1.26.5 focused66 PASS
0skip, including actual Redis7 script concurrency. Full short352 PASS/36 explicit
integration skips; actual JetStream quota Redis-error NAK/recovery/quota-denial
ACK test PASS. Vet PASS. Initial incomplete dependency cache and missing export
of existing manifest fixtures were setup failures, corrected before final run.
An initial new dispatcher fence test lacked its actual chat scope; completing
the fixture also prompted an explicit mismatched payload/input scope guard.
Lint, final source binding, independent review and integrator custody pending.
