# Atomic User profile birth capture

## Purpose and boundary

Join actual secondary profile insertion, the existing canonical identity BIRTH
and the existing search outbox in one User IdentityTx. A configured protected
ParentVerifier obtains evidence before I; CreateBirthTx rechecks it after I.
This is future mutation capture, not whole-origin or complete-history evidence.
A profile BIRTH fact does not establish whole-account or source origin.

The governing behavior is User-owned profile creation and identity capture in
[User](../microservices/user-service.md), with the ownership boundaries in
[architecture](../ARCHITECTURE_REQUIREMENTS.md). The accepted A4 semantic target
is recorded in BE-020; whole-origin evidence remains a separate prerequisite.

## Implementation steps

1. Exercise actual CreateSecondaryProfile with identity guards active and the
   existing ParentVerifier authority seam. Require the exact profile ID, BIRTH,
   reservation and search event to commit together.
2. Compose existing IdentityTx.SQL(), CreateBirthTx and ordinary INSERT/outbox;
   fetch Auth evidence and prepare selectors before taking I. Preserve existing
   legacy deployment mode; no qualified source may admit an unqualified writer.
3. Force failure after identity preparation/outbox append and require all four
   effects to roll back. Retry conflicts through whole-transaction rollback.
4. Complete primary and retirement parity before activation; then implement
   explicit admission/closure/sealing with independently verified origin.

## Validation

Use an owned PostgreSQL 16 fixture and the existing User installer/keyset test
support. Preserve actual RED before production changes, then run scoped native
tests and diff checks. No new Auth/proto/migration/MM path is part of this slice.

## Progress and limits

- [x] Fresh guarded worktree verified at da2b007a.
- [x] Existing CreateBirthTx, VerifiedParentProof and identity lock order inspected.
- [x] Actual guarded secondary INSERT failed before composition with SQLSTATE 23514 (New profile requires canonical birth preparation); raw secondary-red.log retained.
- [x] Atomic secondary writer implemented; seven focused PostgreSQL tests pass, including outbox rollback, whole-transaction retry, authority failure, mixed-writer denial and legacy controls. Native Go 1.26.2; go vet passed.
- [ ] Independent exact-head source review and remaining primary/retirement/concurrency follow-ons.

Order is I, existing parent/account fence, profile rows, search journal/outbox.
No Auth call runs under I. This does not turn the raw ACCOUNT_INACTIVE search
overlay into a new admission authority. Existing accounts remain INCOMPLETE;
pins/empty scans are not origin proof. Primary/retirement/bypass writers,
whole-origin provenance, continuous capture, seals, protected transport and
runtime/A4 acceptance remain open. No deployment capability is activated here.
