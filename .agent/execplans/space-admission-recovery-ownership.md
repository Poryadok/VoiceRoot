# ExecPlan: Space admission live-writer and recovery ownership

## Purpose

An immediate or periodic Space media recovery pass must not steal an admission
from its current live writer. Crash recovery must retain exact operation, room,
account-fence, roster and event identities; a delayed Redis callback must not
resurrect an operation after a recovery takeover and cleanup.

## Context

TT `VOICE-SPACE-RECOVERY-LIVE-ADMISSION-RACE` c2-c3 is the accepted Astra design
and validation contract. Sources: `docs/microservices/voice-service.md` native
admission/P3 sections, `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`,
`docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/TESTING.md`, `docs/CONTRIBUTING.md`.
The journal currently enumerates every unfinished row without writer authority;
the recovery branch uses its stale enumeration snapshot. Existing per-Space
Redis gates fence lifecycle transitions, but not admission ownership takeover.

Base: dc0acdefec7e5c413988adf71f86d12bff4017bc plus the explicitly accepted
6780407299a3beb77e92b32c551ca7cfe3e63e94 factory/external-PG test dependency.
Author Рудеус-303 owns WT27, lease 92ced5d47768018d24d083e9c129ddbd.

## Scope

One durable owner token, monotonic owner epoch and DB-time lease covers the
head-before-Prepare window through projection confirmation or exact cleanup.
Forward/owner cleanup SQL verifies that authority under canonical lock order.
Recovery claims only expired/relinquished authority and branches on fresh state.
Per-admission Redis dispatch gates add nonce/epoch fencing at the actual sink.
P3 barriers, provider retirement, dormant roster relay and public API remain
under their existing contracts. No deployment or activation is part of this fix.

## Milestones

- [x] Deterministic real-PG regression shows immediate recovery stealing a live writer.
- [x] Durable owner acquisition/renewal/relinquish and recovery CAS fence SQL mutations.
- [x] Redis sink rejects suspended callbacks after takeover/deny and gate rebuild.
- [ ] Production factory and focused PG/Redis race suite pass with zero skips.
- [ ] Independent concurrency review of immutable source, accepted handoff to121.

## Detailed Steps

1. Exercise separate foreground/recovery stores at head, PREPARED, FENCED and
   COMMITTED-not-projected boundaries, without sleeps. Save actual RED output.
2. Add an additive owner schema and strict empty/fenced cutover validation.
   Generate tokens before the first head mutation; never alter operation IDs,
   media/room/P3 generation or immutable event bytes during takeover.
3. Check current token/epoch and unexpired DB lease in every owner mutation;
   renewal cannot revive expired/replaced ownership. Unknown acknowledgements
   remain recoverable and never infer rollback.
4. Claim recovery under Space barrier, room resource, then identity lock order;
   return locked current row. Filter unexpired owners before LIMIT so they cannot
   starve recoverable work. Hold ownership over the whole cleanup sequence.
5. Extend Redis dispatch tickets with admission nonce/epoch. A takeover fences
   old dispatch before exact drain and cleanup completion. Absent gates rebuild
   through RESTORING and fresh PG authority, and delayed callbacks retain their
   original ticket across retries.
6. Add deterministic expiry, two-claimant, stale enumeration, gate loss,
   suspended callback and crash-boundary coverage. Update owning service/storage
   docs and record migration/mixed-writer restrictions.

## Validation

Owned Linux Docker runner, internal network only, PostgreSQL16 and Redis7 with no
host ports. Reuse readonly cached Go toolchain dependencies. Actual source is
copied to a native Linux directory; no generated Windows executables/listeners.

- `go test -race -count=1 ./internal/spacemedia ./internal/store` with actual
  external-PG/Redis fixtures; distinguish legacy skipped tests from required proof.
- Focused production factory immediate Run/ReconcileAll reproduction.
- `go vet` affected packages, pinned lint and migration checks.
- `graphify update . --no-cluster` after source changes, account for live process.

## Progress

- [x] Atomic TT claim and actual own worktree lease recorded.
- [x] Accepted source dependency678 normally merged with integrator121 ACK.
- [x] Initial PREPARED/FENCED/COMMITTED writer RED, immutable baseline source-red.tar.
- [x] Expiry/two-claimant CAS, current-row recovery, head-before-Prepare,
  renewal-versus-orphan lock, pagination and crash-after-drain focused PG checks.
- [x] Real Redis forward and cleanup pipeline hooks prove competing admission
  gate takeover occurs after the gate read and before actual EXEC.
- [x] c5 stale shared BeginRoomLeave RED; accepted Astra c6/c7 decision replaces
  shared APIs with explicit owned arguments and separate restrictive targets.
- [x] c7 real PG/Redis matrix: takeover before DENY, after DENY ACK and after
  Redis drain; stale sink loaders leave gate bytes unchanged, stale epoch/nonce
  cannot complete, fresh exact restrictive target re-fences successfully.
- [x] Affected short suite and go vet pass on source-hash-bound native snapshot.
- [x] Production factory/P3, actual public leave and no-NATS captured-drain
  wiring pass under race detection; external Auth/provider remain fixtures.
- [x] Pinned full Voice golangci-lint 2.6.1 on Go1.26.5: zero issues.
- [x] Negative WATCH oracle fails on all actual before-EXEC races when only
  admission WATCH keys are removed; original Linux file restored with SHA check.
- [x] Lost response after successful actual Redis DENY EXEC does not produce a
  PG fence acknowledgement or release; explicit retry completes safely.
- [ ] Final split package race qualification, full module short suite, immutable
  source pin and independent review/handoff. Oversized combined PG runs timed
  out despite passing assertions; preserve these as incomplete, not PASS.

## Decisions

Use the accepted c2-c3 durable/sink protocol. A process mutex, age threshold,
slower coordinator or disabled recovery cannot satisfy this contract.

c6/c7 resolves the capability-stripping seam: owned Begin/Complete and owned
sink loaders take mandatory explicit ownership and reject before terminal
idempotence or Redis mutation. Trusted public leave/ACL and P3 captured drain
use separate restrictive targets sampled from persisted exact state. A takeover
invalidates their epoch and requires a fresh target and DENY, never a fallback
from failed owned authority. Actual Redis cleanup checks the target tuple and
current DENY nonce/epoch in WATCH/EXEC. P3 keeps its capture and provider proof
requirements; a new factory RED exposed its tuple-only cleanup adapter, now
upgraded and verified through the same explicit restrictive sink boundary.

Evidence resides in ignored tmp/swarm/rudeus303-admission. Early WIP logs do not
claim immutable-head proof. Later run JSON records native backend source hashes
before and after execution; final exact-head validation remains required.

## Risks And Follow-Ups

Independent review144 found two bounded P2 gaps in historical head2b89355:
missing-gate allocation preceded strict sink validation, and captured drain
validated its immutable tuple after beginning restrictive SQL effects. The
successor validates before allocation and compares the full expected tuple
inside the canonical transaction before writes. New real-PG/Redis race oracles
failed against the historical implementation and pass after these fixes.

Astra289 clarified the lease linearization contract in review task c2: acquire
the owner row lock first, then issue a separate fresh DB-time/token/epoch check.
Post-lock-valid transactions retain that lock to commit and may finish after
deadline while blocked downstream. A pre-acquisition wait cannot cache freshness;
renewal cannot resurrect expiry. Deterministic blocked-owner negatives (mutation
and renewal), downstream-expiry ordered positive, cancellation rollback and
next-transaction rejection cover this distinction. No final-clock scatter or
cross-transaction authority is implied. Successor proof/review remains pending.

Existing persisted rows require explicit fenced migration reconciliation; never
invent live-owner proof for historic rows or mix old unfenced writers. Redis
snapshot rollback/multi-primary fencing remains the existing infrastructure
boundary. Source CLEAR and component tests do not prove provider/DEV/STG acceptance.

## Integration follow-up: Voice15 rollback retention

The reviewed runtime was composed at `dd37c623`: full Realtime short/race,
Voice production-factory PG/Redis/NATS and Voice short/static/build checks pass.
A separate integration audit found that migration15 DOWN could probe an empty
ownership table while a first INSERT was uncommitted, then drop the table after
waiting for that writer to commit. The deterministic two-connection PostgreSQL
regression failed against the old SQL: DOWN returned success and removed the
committed ownership row. The empty DOWN/UP control passed.

DOWN now takes an ACCESS EXCLUSIVE table lock before its retention probe, with
the check and DROP in one explicit transaction. The regression observes the real
lock wait, commits the writer, then requires refusal and exact row preservation;
both it and empty DOWN/UP pass with race enabled. Affected vet and pinned lint
pass. The existing populated-refusal/startup fixture is checked separately for
connection rollback compatibility. UP and all runtime authority bytes remain
unchanged. Exact successor review is required before develop intake; no provider,
installed writer-fencing or acceptance qualification follows from this fix.
