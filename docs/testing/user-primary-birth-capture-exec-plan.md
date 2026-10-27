# Atomic primary-profile birth capture

Extend the reviewed secondary composition in PR959 to the existing User-owned
EnsurePrimaryProfile path. Canon: [primary bootstrap](../microservices/primary-profile-bootstrap.md)
and [User](../microservices/user-service.md). The primary ID remains stable across
retries; explicit Auth-provided IDs, guest marker, onboarding and search effects
retain their existing meanings.

The scoped stack starts at cde7f30e900ee03e5ea8cf6da2ea487654276638.
Use the configured ParentVerifier and selector preparation before I; read the
primary under the owning IdentityTx and, only when absent, commit BIRTH, exact
primary INSERT, onboarding and search journal/outbox together. Repeat calls must
return the existing primary without another BIRTH or event. Failures roll back
all provisional identity/profile/onboarding/search effects.

First causal RED: real configured EnsurePrimaryProfile with guards active must
currently fail canonical birth preparation. Then implement the owning transaction
and prove atomic success, idempotent repeat, rollback and serialized competing
ensures using an isolated PG16 fixture. Preserve existing unconfigured legacy
behavior and all seven secondary capture controls.

No startup activation, retirement parity, source-origin receipt, complete-history
coverage, Auth/proto/schema/MM change or full A4 acceptance is part of this slice.
Historical accounts remain INCOMPLETE. Primary BIRTH is not account-origin proof.

## Progress

- [x] Fresh WT93 lease18ce24db4424f237db8a0f8ff39c9a01 verified, clean cde7 base.
- [x] CodeGraph call path and primary bootstrap contract read.
- [x] Actual configured primary INSERT RED: SQLSTATE23514 canonical birth preparation; raw primary-red.log retained.
- [x] Four primary tests plus seven unchanged secondary controls PASS (11 total, 10.370s), native Go1.26.2 and isolated PG16; go vet and diff-check PASS. Primary controls cover exact ID/onboarding, repeat preserving original ID/name/guest marker without new facts, complete outbox-failure rollback and deterministic concurrent ensures yielding one birth/profile/event.
- [ ] Freeze exact source and obtain independent review.