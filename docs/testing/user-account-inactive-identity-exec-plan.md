# Serialize the existing account-inactive projection

The existing User ApplyAccountDeleted consumer records a durable inbox event,
ACCOUNT_INACTIVE overlay and revisioned Search Delete for every retained
profile, including already-soft-deleted profiles. See
[User account deletion projection rollout](../microservices/user-service.md#account-deletion-projection-rollout).
Auth remains the sole account-state authority. This legacy overlay is not an
A4 terminal/current proof, purge decision, deadline or complete-history receipt.

Source stack begins at 25307fd7f758121a72bbc308752402bdb94d3383. Configured
composition must acquire I before the inbox/account/profile rows, then reuse
the unchanged transaction body and search journal/outbox. No new Auth call,
event shape, Ack behavior, activation flag or credential is introduced.
The consumer continues to Ack only after store commit. Ordinary legacy
composition and event duplicate/rollback semantics remain unchanged.

First causal RED holds I and invokes the actual configured method: its current
ordinary transaction fails prematurely with NOWAIT55P03. GREEN waits before
its first write and keeps inbox, overlay, profile revisions, existing captured
LIFECYCLE changes and all Search Delete records in one commit/rollback.
Controls cover duplicate events, outbox rollback, already-archived profiles,
and retained uncaptured legacy data without inventing coverage qualification.

Do not deny legacy account inactivation merely because a retained row lacks a
canonical association: missing historical evidence stays incomplete. No new
terminal facts, Auth lifecycle policy, restore semantics or erasure is inferred.
Hold PR publication/retarget while the current reviewed stack progresses.

## Progress

- [x] Fresh WT93 lease1f141f40754c1cae1587d70c6d40b297 verified; clean25307 base.
- [x] CodeGraph consumer/store/lock path and exact rollout contract inspected.
- [x] Actual RED: configured overlay failed prematurely with identity_meta NOWAIT55P03 while I held; raw account-inactive-red.log retained.
- [x] Six account-inactive controls plus prior28 PASS (34 top-level tests and2 subcases,34.904s), native Go1.26.2/owned PG16; go vet and diff-check PASS. Verified waiting, duplicate/concurrent duplicate, complete outbox rollback+same-event retry, already-archived inclusion, unchanged legacy and uncaptured-row handling without terminal facts.
- [ ] Freeze and independently review; PR publication held.
