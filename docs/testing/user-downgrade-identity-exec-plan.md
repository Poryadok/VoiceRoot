# Atomic legacy downgrade selection

The configured ProfileStore downgrade writer must acquire its owning IdentityTx
before reading account profiles and apply its existing kept-set freeze/unfreeze
updates as one commit. Existing User22 triggers own captured LIFECYCLE revisions.
The source stack starts at 5904a90f0f8cc8fb9bd3a524b1659c5fc3de1e54.

Canon: [User](../microservices/user-service.md) and
[multi-profile](../features/multi-profile.md). The newer A7 target has a separate
cycle-bound subscription-freeze overlay and is not implemented by this legacy
FrozenAt method. This slice changes transaction ownership/atomicity only; it
does not claim that target or change the existing kept-set policy.

First causal RED injects failure on the second actual freeze and proves the
first profile/LIFECYCLE must not have committed. GREEN uses I before a fresh
account-profile selection and keeps all updates/trigger revisions in one
transaction. Also hold I and change state before release to prove fresh reads,
preserve existing legacy mode, and cover duplicate/no-op selection.

FreeTierFreeze is a separate writer. There is no search outbox in the current
downgrade method, and no search visibility policy is invented here. No Auth
call, schema/proto/MM change, startup activation, whole-origin/completeness or
all-writer continuity/acceptance is included. Configured selection rejects an uncaptured legacy row before any update, rather than misrepresenting it as captured lifecycle history.

## Progress

- [x] Fresh WT93 leaseb470f64b607573f3c9ae9645bfd4d239 verified, clean5904 source.
- [x] CodeGraph caller, SQL and existing lifecycle trigger path inspected.
- [x] Actual RED: forced second freeze error left one earlier frozen profile committed (expected0,actual1), raw downgrade-red.log retained.
- [x] Five downgrade controls plus seventeen unchanged birth/archive controls PASS (22 total,18.580s), native Go1.26.2/owned PG16; go vet and diff-check PASS. Rollback, held-I fresh exclusion of a concurrently archived profile, keep/unkeep/repeat policy, unchanged search facts, uncaptured failclosed and legacy mode verified.
- [ ] Freeze and obtain independent review.
