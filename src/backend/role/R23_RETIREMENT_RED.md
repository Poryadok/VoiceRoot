# R23 Role permanent retirement — frozen RED manifest

This manifest freezes Cycle 5 before production GREEN. Its base is
`9757f7c25a8b6ed736006624a5392a5849f28614`, which contains merged R23 contract
PR #283 and Space lifecycle PR #287.

## Canon

- `docs/features/roles.md`, “Permanent Space retirement (P3 target)”
- `docs/microservices/role-service.md`, “P3 permanent Space retirement” and
  ownership-v2 ordinary transaction boundary
- `docs/features/spaces.md`, “P3 convergent lifecycle contract”
- `docs/microservices/space-service.md`, “P3 Space lifecycle coordinator”
- `protos/voice/role/v1/role.proto` and
  `protos/voice/common/v1/space_lifecycle.proto`
- `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`,
  `docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/TESTING.md`, and
  `docs/CONTRIBUTING.md`

The accepted behavior is permanent retirement after `PURGE_DECIDED`. Only the
authenticated Space workload may call protocol v1 `RetireSpace`. The request
binds canonical Space/deletion UUIDs, a positive generation, `purge_decided_at`,
and the immutable root manifest. Retirement refuses every `PREPARED`
ownership-v2 operation. Otherwise one transaction stores the permanent fence
and compact receipt, deletes ordinary Role data, and emits one final Space-wide
policy invalidation. Exact replay is byte-identical; changed binding and missing
receipt evidence fail closed. Retirement fences delayed v1/v2 ownership,
ordinary, bootstrap, and identifier-reuse work before replay or mutation.
Ownership calls against a retired Space return `FAILED_PRECONDITION`; ordinary
and bootstrap calls return `UNAVAILABLE`. The ordinary listener does not expose
`RetireSpace`; only the trusted TLS listener may route it.

## Frozen tests

| Test | Contract evidence |
| --- | --- |
| `TestSpaceRetirementMigration_UpPreservesLifecycleAndPersistsExactBytes` | 000012 extends the 000010 table, preserves rows, stores exact request/response bytes, and derives the 30-day boundary from database `retired_at` |
| `TestSpaceRetirementMigration_ExpiredFullBytesCompactButPermanentEvidenceCannotChange` | full bytes compact only after the DB boundary; compact receipt and fence remain immutable |
| `TestSpaceRetirementMigration_RetiredLifecycleIdentityCannotMovePastAuthorityFence` | retired lifecycle identity cannot move; the original Space remains denied before authority work |
| `TestSpaceRetirementMigration_DownPreservesPreexistingLifecycleWhenEmpty` | empty DOWN removes only 000012 objects and preserves the preexisting lifecycle table/rows/OID |
| `TestSpaceRetirementMigration_DownRefusesNamedPermanentEvidence` | fence/receipt evidence returns named PostgreSQL SQLSTATE `55000` and survives |
| `TestSpaceRetirementMigration_DownWaitsThenRefusesConcurrentRetirement` | DOWN locks before its post-wait evidence check |
| `TestSpaceRetirementMigration_DownWaitsForActualReceiptFirstStoreRetirementThenRefuses` | receipt-first runtime and DOWN use one lock order, avoid deadlock, and preserve committed evidence/schema |
| `TestNewRoleGRPCServers_OrdinaryListenerDeniesRetireSpaceBeforeHandler` | ordinary/legacy transport returns `UNAVAILABLE` without entering the handler |
| `TestRetireSpace_HandlerIsImplemented` | generated embedding cannot masquerade as implementation |
| `TestRetireSpace_ExactReceiptAtomicCleanupAndFinalPolicyInvalidation` | exact receipt fields/hash, atomic ordinary cleanup, one final invalidation |
| `TestRetireSpace_RefusesPreparedButAcceptsTerminalOwnershipLedgers` | prepared refusal; terminal v1/finalized/aborted v2 acceptance |
| `TestRetireSpace_ExactReplaySurvivesResponseLossAndRestart` | response-loss recovery, byte-identical restart replay, and unchanged fence/receipt/epoch/outbox snapshot |
| `TestRetireSpace_CompactReplayRequiresSemanticEvidenceBeyondHash` | post-30-day replay reconstructs its receipt and rejects changed purge/manifest semantics even under forged accepted hash/bytes |
| `TestRetireSpace_ChangedBindingAndReceiptLookupFailureFailClosed` | conflicts and lookup outage preserve the exact committed snapshot |
| `TestRetireSpace_FailpointRollsBackFenceReceiptCleanupAndInvalidation` | injected receipt failure rolls back cleanup, fence, receipt, epoch, and outbox together |
| `TestRetireSpace_RejectsUnknownAndInvalidRequestsBeforeMutation` | canonical non-nil UUIDs, manifest, valid microsecond timestamp, and unknown-field rejection |
| `TestRetireSpace_RequiresTrustedCanonicalSpacePrincipalBeforeMutation` | exact verified Space workload fields and request binding |
| `TestRetireSpace_DelayedOwnershipOrdinaryAndBootstrapCannotRecreateAuthority` | ownership is `FAILED_PRECONDITION`; ordinary/bootstrap is `UNAVAILABLE`; snapshot/events stay unchanged |
| `TestRetireSpace_OperationLockPrecedesSpaceAndWinsAllAuthorityRaces` | `pg_locks` proves operation-before-Space order; queued Prepare/Finalize/Abort/ordinary/bootstrap cannot overtake retirement |
| `TestRetireSpace_RequestHashRuleIsFrozen` | FQN-NUL-deterministic-protobuf SHA-256 vector |
| `TestRuntimeListener_AllowsOnlyBoundSpacePrincipalToRetireSpace` | protected TLS listener and exact Space workload binding |
| `TestRuntimeListener_RetireSpaceRejectsWrongBindingBeforeHandler` | wrong issuer/audience/RPC/request hash/tampered body rejected with exact `UNAUTHENTICATED` before handler |

Existing `TestOrdinaryHandlerFences`, ownership-v2 final/migration tests, Voice
policy epoch tests, and capability activation-hold tests remain required and
must not be weakened while making this suite green.

## Reproducible RED

From `src/backend/role`:

```powershell
go test . ./internal/store ./internal/grpcsvc ./internal/principalruntime -run 'SpaceRetirement|RetireSpace' -count=1
```

Expected pre-GREEN roots are all required:

1. `000012_space_retirement.up.sql` is absent.
2. `RoleGRPC` inherits generated `RetireSpace` and returns `UNIMPLEMENTED`.
3. the ordinary listener reaches the handler instead of denying retirement.
4. the protected listener rejects `RetireSpace` before the handler.

No production file, generated output, capability activation, Voice permission
path, `docs/PLAN.md`, `docs/TODO.md`, or `docs/todo/**` belongs to this RED.

## GREEN handoff

The next writer may add only `src/backend/migrations/role_db/000012_*retirement*`,
focused `internal/store/space_retirement*`, `internal/grpcsvc/space_retirement*`,
the minimum protected-listener/runtime wiring, and exact Role documentation.
It must preserve the production ownership capability activation hold. Run the
focused suite after each behavior, then the full Role suite and race checks from
`docs/TESTING.md`. Independent test review is required before that writer starts.
