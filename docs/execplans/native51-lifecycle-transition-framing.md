# ExecPlan: Native51 lifecycle transition framing

## Purpose

Allow the documented Space deletion lifecycle to advance on a genuinely
installed native51 database while preserving exact current-11 evidence, native
membership history and fail-closed source guards.

## Context

- `docs/microservices/space-service.md` requires a bound Auth receipt, the
  freeze barrier and eleven participant receipts before terminal deletion.
- `docs/testing/listthreads-versioned-readmodel-exec-plan.md` requires stable
  effective membership lineage and fails closed during lifecycle freeze.
- Native44's source trigger requires a same-transaction frame, exact catalog,
  READ COMMITTED and the Space advisory fence. Existing positive, scheduling
  and terminal frame scopes have distinct authorization and cannot be interchanged.
- A real native51 run of signed creation, reservation and proof fails at
  `BeginLifecycleFreeze` with SQLSTATE 55000 before eleven-party testing. The
  exact diagnostic and native installer evidence are retained at
  `D:/Git/Voice/.treehouse/Voice-451d38/49/native51-scoped-1df8129c221c4973b515fb4fc78f2b49`.
- `docs/features/spaces.md` uses older ten-participant wording; the Space
  service document and migration34 require eleven, as does the requested test.

## Scope

- In: native lifecycle writer framing, exact operation/phase authorization,
  native51 negative and positive regression tests, and terminal proof if reachable.
- Out: guard bypass, broad frozen-state authority, rewriting immutable migrations,
  changing source or five-second observation deadlines, and changing product policy.
- Documentation gap: the feature page's older participant count conflicts with
  the Space service and migration34. This task follows the latter current contract.

## Milestones

- [x] Reproduce the unframed FREEZE_PENDING write on a qualified native51 source.
- [x] Prove freeze authorization and rollback negatives with genuine native51 fixtures.
- [x] Implement and independently review the narrowly scoped freeze frame.
- [ ] Prove every required lifecycle phase and current-11 terminal behavior.

## Detailed Steps

1. Trace each writer's locked current phase, authenticated operation and receipt
   checks; distinguish read-only retry, ordinary receipt progress, terminal purge
   and restore. Establish a per-transition authorization matrix before code.
2. Add a native51 test with genuine signed Space creator and operation/proof
   evidence. Cover no proof, wrong operation, duplicate or altered receipt,
   transaction rollback and state/ledger/head preservation. Run the expected red.
3. Introduce only private, operation-bound transaction-local capabilities at
   validated owner entrypoints; reuse DB cohort, source/episode and final clocks.
   Do not expose a generic caller-selectable frozen writer.
4. Exercise the real lifecycle sequence on a qualified source, including the
   ten-vs-eleven predicate and terminal deletion, then obtain independent review.

## Validation

- Qualified native50 then native51 installer/commit, followed by the tagged Go
  regression in an owned PostgreSQL fixture with zero skips and recorded hashes.
- Scoped store tests and applicable lifecycle/native guard security regressions.
- Independent security review of authorizations, catalog and rollback paths.

## Progress

- The qualified native51, two-member baseline rejects no-proof, wrong-binding,
  altered-receipt and direct SQL attempts while preserving stored state, then
  fails at the actual freeze write with SQLSTATE 55000. Evidence is in
  `D:/Git/Voice/.treehouse/Voice-451d38/48/native51-scoped-673efe9094d54e9d954977282ea88976`.
- Test review covers exact snapshots, member fanout and a post-frame blocked
  write/cancellation. It remains to observe the rollback path after a fix.
- Freeze-only implementation must not be represented as complete lifecycle or
  network Auth acceptance; all subsequent transitions and purge remain open.
- The qualified native51, two-member freeze-only regression passed in 1.95s,
  including observed post-frame head-write blocking, cancellation and exact
  rollback, on PostgreSQL 16. Evidence is in
  `D:/Git/Voice/.treehouse/Voice-451d38/48/native51-scoped-93f4c7993bf043e99dc221baf7b5be4f`.
  The independent security reviewer accepted the bounded frame and test on the
  current source. The typed Auth and Role receipts remain controlled fixtures.

## Decisions

- Preserve native guard and same-TX source completeness; never fake a marker,
  grant maintenance authority, or relax freshness to make the fixture pass.
- A positive LIVE frame and the SCHEDULE_PENDING-only proof frame do not grant
  later lifecycle transitions. Terminal framing is not an ordinary freeze grant.
- Freeze uses its own private operation/phase/manifest-bound purpose if review
  accepts the design; generic `withLockedLifecycle` receives no new authority.

## Risks And Follow-Ups

- Several distinct lifecycle entrypoints write the guarded aggregate, including
  direct schedule, recovery and restore paths outside `withLockedLifecycle`.
  A single generic wrapper could silently broaden authorization.
- Full terminal proof remains blocked until all necessary transitions are
  framed; a freeze-only pass is not eleven-participant acceptance.
