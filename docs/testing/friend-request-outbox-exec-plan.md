# ExecPlan: Durable friend-request notifications

## Purpose

Make `SendFriendInvitation` return after its Social database transaction commits, while ensuring the matching `social.friend_request` event remains durably retryable until JetStream accepts it. A recipient with an open realtime connection must still receive the event without relying on the HTTP/gRPC request staying alive.

## Context

- Docs: `docs/features/friends.md` (a sent request is visible to its recipient); `docs/microservices/social-service.md` (Social owns friendship state and publishes `social.friend_request`); `docs/DATA_STORES.md` (Social owns `social_db`); `docs/ARCHITECTURE_REQUIREMENTS.md` (transactional outbox patterns).
- Code: `src/backend/social/internal/grpcsvc/social_friends.go`, `src/backend/social/internal/store/friendships.go`, `src/backend/social/internal/socialevents/jetstream.go`, and the existing `friend_accept_outbox` worker/store implementation.
- Current state: invitation state commits in `SendInvitationChecked`; the gRPC handler then synchronously publishes to JetStream with an empty request ID and discards the publish error. The two live staging timeout attempts leave commit status ambiguous.
- Constraints: keep Auth in Java and Social events in the Social service; do not publish before commit; preserve idempotent retries and accepted/declined friendship semantics; do not deploy to production or reset staging data.

## Scope

- In: atomic persistence of pending invitation plus durable request event; retry worker; stable request ID and duplicate-safe publication; tests for new, repeated-pending, and reopened-declined invitations; docs and staging evidence.
- Out: changing friendship policy, notification UX, unrelated Friends → Favorites behavior, and global WebSocket history/catch-up.
- Documentation gaps: the Social service docs name the event but do not specify the retry/outbox semantics or event/request identifier relationship; document those semantics without changing the public contract.

## Milestones

- [ ] Add failing store and worker tests for durable invitation publication, retry, and stable request identity.
- [ ] Implement the invitation outbox transaction and retry worker using the existing Social outbox pattern.
- [ ] Run focused Social tests and required service checks; update Social docs and the A1 acceptance ledger.
- [ ] Merge through the repository workflow, deploy only through an allowed data-preserving staging workflow, and verify live no-refresh delivery on a fresh eligible pair.

## Detailed Steps

1. Confirm the exact migration directory and existing Social test database helpers.
2. Add tests first: every successful invitation mutation must leave a pending outbox row in the same transaction; dispatch must retain rows on publisher failure and mark delivery after success; retries must preserve the request ID.
3. Add a Social-owned request outbox migration and store methods. Ensure re-sending an already-pending or reopened declined invitation refreshes one durable notification row rather than creating unbounded duplicates.
4. Add the request-outbox worker and stable JetStream message identity; remove synchronous publish from the gRPC handler.
5. Document behavior and rollout/rollback requirements, then run focused Social tests, service CI checks, docs checks, and `git diff --check`.
6. Deploy only after exact-SHA checks pass. Verify with one fresh staging pair and confirm both REST state and live recipient UI; do not reuse pairs whose result is ambiguous.

## Validation

- [ ] Focused Social store integration tests prove mutation/outbox atomicity, re-open handling, publisher retry, and stable event ID.
- [ ] Social service unit/integration tests pass.
- [ ] Repository-required checks for the changed Go service and docs pass.
- [ ] A fresh staging request appears in the recipient UI without reload, while the request REST snapshot reports the same request ID.

## Progress

- [x] Audited the existing `friend_accept_outbox` pattern and confirmed the invitation path currently publishes synchronously after commit.
- [x] Added failing store retry/identity tests and a publisher stable-message-ID contract test.
- [x] Implemented durable dispatch, atomic enqueue, migration 000006, stable JetStream message identity, and transactional suppression after accept/decline.
- [ ] Run focused and required verification.
- [ ] Verify and stage.

## Decisions

- Use a durable Social-owned outbox rather than fire-and-forget or merely extending the client timeout: a committed request must remain deliverable across process restarts and NATS outages.
- Use the persisted friendship row ID as the request ID so REST state and realtime notification refer to the same request, and retain a stable event ID across retries for JetStream deduplication.
- Treat delivery as at-least-once beyond the configured 24-hour JetStream duplicate window; cancel queued request events atomically when the friendship request is accepted or declined.

## Risks And Follow-Ups

- Migration rollout must precede the updated Social service; check the current staging workflow’s migration behavior and the NATS state-preserving deployment gate before choosing a deployment path.
- An outbox publish may succeed while marking delivery fails, so consumers may observe a duplicate after the stream deduplication window expires.
- Live request attempts already made with ambiguous timeouts remain consumed and must not be retried.
