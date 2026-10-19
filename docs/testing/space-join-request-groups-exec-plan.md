# ExecPlan: exact join-request group selection

## Purpose

An authorized approval request identifies its real parent case Q and one exact
invite-bound attempt A. Selecting an attempt is durable before remote owner
calls and never changes on retry when another child arrives. Q is grouping
metadata, not membership, proof, invitation or approval authority.

## Context

- Parent BE-A2-SPACE-CHAT-PERMISSIONS c22-c24, contract
  CHAT-TARGET-ADMISSION-IDENTITY-v1 within SPACE-CHAT-AUTHORITY-v4.
- Space service: versioned entry policy, immutable attempts/submissions,
  freshness/final decision/recovery; spaces feature entry verification.
- DATA_MODEL identifiers and service ownership, DATA_STORES Space inventory,
  ARCHITECTURE_REQUIREMENTS exact verified request binding, TESTING and
  CONTRIBUTING.
- Consumer69 owns Space35/36, private submission and approval writers, public
  admission RPCs/Gateway. Reviewed35 b841b63de plus6392d5250 and reviewed36
  eff5c5629 are imported as unchanged dependencies. Native33 remains186;
  Realtime34 remains its accepted integration source.
- New own WT41 lease fbaa5c6cd650b15fbe3b8502225247a0, branch
  codex/space-request-groups-155, actual develop base e77469b8. Historical WT19
  and its Graphify UNVERIFIED result are not authority for these writes.

## Scope

Add reserved migration32, dedicated group/store/tests and narrowly scoped docs.
Q groups only the same account/profile/Space/policy revision and digest. A child
belongs to one Q; different invites remain separate A. Existing35/36 structs,
locks, operation namespace and immutable request hashes are reused.

The producer exposes scoped AttachJoinRequestAttempt, read-only
LookupJoinRequestGroupForAttempt(ctx, spaceID, attemptID), root
ResolveJoinRequestApproval(ctx, JoinOperationInput, Q, optionalA), and scoped
VerifyJoinRequestApprovalSelection(ctx, op, Q, A). No public handler, approval
authority, R target completion, native publication or provider integration is
introduced. Actual consumer69 invokes attachment in its successful pending
submission transaction and final verification in its approval decision hook.

## Milestones

- [x] Reconcile actual155 ownership, live resources, reservations and new lease.
- [x] Read canonical identity contract and actual immutable35/36 interfaces.
- [x] Real PostgreSQL RED for missing Q grouping/selection behavior.
- [x] Implement atomic attachment and immutable exact public-operation selection.
- [x] Prove ambiguity, explicit selection, retry, foreign tuple rejection,
  rollback, concurrency/restart and local freeze behavior without remote calls.
- [ ] Freeze contract/source, independent review and accepted121 delivery.

## Detailed Steps

1. Add tests before the implementation. Use actual35 attempts and submissions,
   actual36 operations and real PostgreSQL. Run only owned Linux Docker with
   private PostgreSQL/network and no host ports.
2. Add minimal API stubs/schema to make the behavioral test executable; record
   actual missing-behavior RED separately from setup/compile failures.
3. Attach under the existing Space fence after successful manual submission.
   Cohort uniqueness and attempt uniqueness preserve one immutable association.
4. Resolver locks the existing public actor/operation namespace before Space.
   Existing exact Q selection returns its saved A, with no new candidate scan.
   New selection considers stored pending/ready children only; it never filters
   by ambient invite validity or picks latest/first. Zero children denies;
   multiple children without explicitA requires a selector.
5. Commit selection journal and existing36 selectedA together. A preexisting36
   operation with no matching Q journal is unavailable, not guessed Q=A.
6. Final scoped verifier reloads both journals and exact Q/A cohort under the
   Space fence. It performs no remote call and grants no approval. Consumer69
   remains responsible for current state, historical submit basis, owner guards
   and final fresh native publication.

## Validation

Use focused real-PG tests, race, vet, Space build and repository-pinned lint for
the changed packages. Compare actual recorded source to the frozen commit. Reuse
unchanged35/36 producer proof rather than rerunning unrelated infrastructure.
Migration32 must apply before35/36: no forward foreign keys. Runtime store
queries fail closed until those real ledgers exist. Down refuses admitted group
evidence; normal erasure follows the current canonical terminal Space P3 guard.
DOWN requires READ COMMITTED: locking alone cannot refresh an inherited
Repeatable Read/Serializable snapshot. A current-authority refusal precedes the
locks; after lock acquisition, fresh RC statements check admitted rows.

## Progress

Actual first behavioral RED (10.279s) reached a valid pending attempt and failed
at the missing group attachment. First two PostgreSQL tests then passed (5.840s,
zero skips), including restart and rollback. A separate SQL RED (7.263s)
demonstrated a live Q cohort rewrite; immutable mapping guards were added.
Final Linux PostgreSQL race proof: eight top-level tests passed in83.152s and
three lifecycle/schema tests with two subtests passed in52.833s, zero skips.
The latter exercises actual migration22 terminal authority: a forged purge
setting still produces55000 for all three live tables. Store vet and complete
Space build exited0. An intermediate lookup argument-order compile failure is
retained separately and is not runtime evidence. Repository-pinned Linux
golangci-lint2.6.1 (Go1.26.5, verified artifact SHA256
bf44279ebeb853f479227ff1374d94645d6c9f369325c799244e6508b057a2a2)
reported0issues/exit0 for the store package. Graphify accounting is a separate
check; review/delivery are not inferred from these tests.
Accounting role developer, Sol6.1 medium, existing
focused genuine continuation. Only queen delegates; independent review will be
made discoverable as a supported TT function after immutable source/proof.

## Decisions

- Q never equals A by inference; callers receive the actual server-owned Q.
- A raw group association or operation row is technical journal data, never an
  authenticated subject, current reviewer allow or transferable proof.
- Schema32 can precede35/36. References to future ledgers are validated at runtime
  inside the same fenced transaction, not by creating duplicate attempt tables.
- Exact public request hash binds Q and optionalA; the selection journal also
  stores those explicit values to detect internal adapter mismatches.
- Reviewer144 found a concrete downgrade snapshot gap in eaff19a. A real old-RR
  empty snapshot followed by concurrent first Q/A commit and DOWN reproduced
  the unsafe success (RED13.601s). The successor explicitly requires RC; bounded
  correction tests passed with race (two top-level/two subtests,13.845s, zero
  skips), including empty/populated RC controls; store vet exited0. Recheck
  remains in the same independent review function, not a new review.

## Risks And Follow-Ups

Native33/catalog qualification and all lifecycle/identity coverage remain their
actual producer dependencies. Existing EXACT25 source guard is not relaxed.
First-publication R completion and existing-member redemption remain distinct
parent feature stages. Successful review selection is not public admission
success. Full startup, real owner/provider assembly, CI and acceptance remain
open until their actual evidence exists.
