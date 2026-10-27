# Atomic legacy free-tier freeze

The configured ApplyFreeTierFreeze writer must acquire the owning IdentityTx
before a fresh account profile selection, then commit all excess-profile
freezes and existing User22 LIFECYCLE revisions together. Source base is
d066dda2c1e957e8ce87544052e9157bab6487b1. Canonical current runtime use is
SwitchProfile in [User](../microservices/user-service.md).

Preserve the legacy policy: primary-first/earliest profile ordering, free cap2,
skip already-frozen excess profiles, and do not unfreeze retained slots. The
newer [A7 target](../features/multi-profile.md) uses cycle-bound subscription
overlay provenance; this transaction slice does not implement that target.

Causal RED injects failure on the second actual freeze and proves the earlier
profile must not remain committed. GREEN additionally waits before selecting:
an existing owner archives the earliest secondary before releasing I, so the
next earliest profile must remain in the retained slots. Cover repeat/no-op,
already-frozen state, captured-row qualification and unchanged legacy mode.

No downgrade-selection behavior change, new search visibility/outbox, Auth
call, schema/proto/MM change, startup activation, historical completeness or
all-writer continuity/acceptance. Configured selection rejects uncaptured legacy rows before updates instead of misrepresenting lifecycle coverage. Hold PR publication/retarget while the current
reviewed stack progresses; source freeze/review can proceed independently.

## Progress

- [x] Fresh WT93 lease2728d18b7aa23a02642880dbdbdcd8a5 verified, clean d066 base.
- [x] CodeGraph runtime caller and current SQL/trigger path read.
- [x] Actual RED: second excess freeze failure left one earlier freeze committed (expected0,actual1); raw free-tier-red.log retained.
- [x] Six free-tier controls plus twenty-two unchanged prior controls PASS (28 total,24.222s), native Go1.26.2/owned PG16; go vet and diff-check PASS. Verified full rollback, waiting before fresh retained-slot selection, no-op cap, unchanged existing frozen state/timestamp, repeat, uncaptured failclosed and legacy mode.
- [ ] Freeze and independently review; PR publication held.
