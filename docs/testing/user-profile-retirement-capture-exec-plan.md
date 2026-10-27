# Serialized User profile soft archive

Extend the opt-in ProfileStore composition from PR959/PR960 to the existing
secondary-profile SoftDeleteProfile writer. Canonical multi-profile deletion is
soft archive; this slice does not perform raw deletion or assert Auth purge.
The source stack begins at 44e6544f9bd69d50be552d63ec9c3d0a8c90c14e.

The current ordinary search transaction reaches User22's statement gate with
NOWAIT and fails if another owner holds I. The configured writer must acquire
its owning IdentityTx first, then read/update the owned non-primary profile.
The existing identity lifecycle trigger and search journal/outbox must share
the same commit/rollback. Do not invent a second RETIRED or terminal event.

First causal RED: capture a real secondary birth, hold I, call configured
SoftDeleteProfile and observe its premature lock failure. GREEN must wait for
the owner, then update fresh profile state and atomically publish the existing
LIFECYCLE and search deletion facts. Preserve primary/foreign/already-archived
refusal and legacy behavior; prove outbox failure rolls all effects back.

No startup activation, historical completeness, account/source origin receipt,
all-writer closure, hard erasure, Auth/proto/schema/MM change or A4 acceptance
is included. Uncaptured legacy state remains insufficient for coverage.

## Progress

- [x] Fresh WT101 lease1018cf3cd2e1710b50b42cfe92a8b576 verified; clean44e base.
- [x] CodeGraph caller/guard/trigger path inspected.
- [x] Actual configured archive RED: premature identity_meta NOWAIT lock failure SQLSTATE55P03 while another owner held I; raw retirement-red.log retained.
- [x] Six retirement controls plus unchanged primary4/secondary7 PASS (17 total,14.548s) on native Go1.26.2 and owned PG16; go vet and diff-check PASS. Configured writer waits before fresh profile lock/update, refuses uncaptured legacy rows, preserves legacy mode, and rolls profile/lifecycle/search back together on outbox failure. Foreign/primary/repeated archives are refused and concurrent archives have one effect.
- [ ] Freeze and independently review.
