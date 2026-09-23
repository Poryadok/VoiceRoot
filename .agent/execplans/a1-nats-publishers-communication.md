# ExecPlan: A1 NATS publisher stream administration removal

## Purpose

Move JetStream stream ownership for Messaging, Chat, File, and Moderation out
of deployed publisher processes. Publishers must only validate their exact
centrally declared stream and fail closed when it is absent or does not contain
their required subjects.

## Context

- Docs: `docs/PLAN.md` A1, `docs/DEPLOYMENT.md`, `docs/TESTING.md`,
  `docs/CONTRIBUTING.md`, and the four service documents.
- Contract: `docs/CONTRACT_MATRIX.md` names domain streams and event subjects.
- Runtime inventory: exactly four production publisher files contain stream
  administration: `messaging/internal/messageevents/jetstream.go`,
  `chat/internal/chatevents/jetstream.go`, `file/internal/fileevents/jetstream.go`,
  and `moderation/internal/moderationevents/jetstream.go`. The preflight is
  below the 20-file/1500-line split threshold.
- There is no existing central JetStream bootstrap definition; compose and
  staging/prod only enable the broker. This slice owns a small central
  declarative bootstrap/validation definition and its static tests, but not
  shared Compose/staging/prod wiring or NATS JWT activation.

## Scope

- In: exact stream declarations for `message_events`, `chat_events`,
  `file_events`, and `moderation_events`; publisher validation; focused tests.
- Out: User/Social/Role/Voice/Matchmaking, Realtime #444 acknowledgement flow,
  broker credentials/JWT ACL activation, and Compose/staging/prod wiring.

## Milestones

- [x] Inventory and branch/worktree setup.
- [ ] RED: publisher tests reject absent or incomplete central streams without
  attempting stream creation or update.
- [ ] GREEN: replace runtime admin with read-only exact validation.
- [ ] Add central bootstrap/validation declarations and static validation.
- [ ] Run focused Go/static tests, review diff, commit and push.

## Detailed Steps

1. Add test doubles that record `StreamInfo`, `AddStream`, `UpdateStream`, and
   `PublishMsg`; prove absence/mismatch yields an error before publish and no
   administration calls occur.
2. Make the four publishers call read-only validation. Require the named stream
   and every declared subject; retain successful publish envelopes and PubAck
   behavior.
3. Add a central, deployment-owned declaration with exact stream names,
   subjects, limits retention, seven-day max age, and file storage; add static
   validation that rejects wildcard subjects and drift from service constants.
4. Keep deployment wiring deferred until Realtime #444 merges and a normal
   merge refresh is performed.

## Validation

- [ ] Focused Go tests for each publisher package.
- [ ] Static/bootstrap definition tests.
- [ ] `git diff --check` and final scoped diff inspection.

## Decisions

- Publisher mismatches fail closed: an operator/bootstrap process owns recovery;
  a workload principal must not have JetStream management permissions.
- Exact declared subjects are required; a broad subject such as `chat.>` does
  not satisfy validation because it obscures ACL and stream ownership drift.

## Risks And Follow-Ups

- Hosted CI is needed for broker proof; no Windows local broker/Compose runtime
  is authorized for this task.
- Bootstrap application and deployment wiring remain intentionally deferred.
