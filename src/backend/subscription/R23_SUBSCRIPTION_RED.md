# R23 Subscription participant and provider dedup — frozen RED manifest

This manifest freezes the U4-B Subscription contract before GREEN. Its base is
`9757f7c25a8b6ed736006624a5392a5849f28614`, which contains the accepted R23
canon and generated Subscription lifecycle RPCs.

## Canon

- `docs/PLAN.md`, `T-RETIREMENT` and accepted decision `U4-B`
- `docs/microservices/subscription-service.md`, “P3 Space deletion and provider dedup”
- `docs/microservices/space-service.md`, “P3 Space lifecycle coordinator”
- `docs/DATA_MODEL.md`, “P3 Space deletion ownership and File references”
- `docs/DATA_STORES.md`, P3 additive durable data and retention
- `docs/ARCHITECTURE_REQUIREMENTS.md`, trusted lifecycle fences and HMAC key isolation
- `protos/voice/common/v1/space_lifecycle.proto`
- `protos/voice/subscription/v1/subscription.proto`

The accepted behavior is a required Subscription participant with ID 8. Only a
verified `service:space` principal bound to audience `subscription`, the exact
RPC and deterministic request hash may apply a fence or purge. `FROZEN` blocks
Space entitlement reads and mutations in the same transaction. Restore accepts
only the next `LIVE` generation. Exact replay returns the saved bytes; changed
bytes for the same operation and generation conflict; generation gaps reconcile
through Space. `PURGE_DECIDED` and `PURGED` cannot return to `LIVE`.

Purge atomically cancels renewal, removes Space entitlement and billing detail,
and records an immutable completion receipt bound to the root manifest. A
transient provider cancellation is recorded as a durable retry with a stable
idempotency key; billing detail remains until that retry succeeds. A failed
cleanup writes no completion receipt. Exact concurrent retry and restart replay
converge on one receipt. Full lifecycle request/receipt bytes retain for 30 days
from participant completion, then compact without removing the permanent
`PURGED` identity-reuse fence. Entitlement reads and mutations serialize behind
the same lifecycle row as `FROZEN`, so no stale read or mutation can overtake it.
The deleted billing detail includes raw `provider_event_id` and provider payload;
only the purpose-specific provider HMAC fence remains.

Provider-event idempotency survives billing-detail erasure. The permanent key is
`(canonical lowercase provider, HMAC-SHA-256(event ID))`, with input bytes
`voice-subscription-provider-dedup-v1`, NUL, provider, NUL, exact event ID.
Only provider, digest, first-seen time, stable terminal outcome class, key
version and unbounded retention remain. Key absence fails closed. A rolled-back
or transient attempt writes no fence; exact replay preserves first-seen time;
changed terminal binding conflicts. Paddle and CloudPayments use the same
permanent rule, and no public/admin/staff/break-glass read RPC exists.

## Frozen tests

| Test | Contract evidence |
| --- | --- |
| `TestR23SubscriptionLifecycleHandlersAreImplemented` | generated fallback cannot masquerade as either handler |
| `TestR23SubscriptionLifecycleRequiresCanonicalSpacePrincipalBeforeStore` | missing/raw/wrong kind, issuer, subject, audience, RPC and hash fail before storage |
| `TestR23SubscriptionPurgeRequiresCanonicalSpacePrincipalBeforeStore` | the same complete trusted-principal negative matrix protects Purge independently |
| `TestR23SubscriptionFreezeRestoreAndChangedBinding` | exact FROZEN replay, entitlement gate, mutation gate, changed binding, next LIVE and gap refusal |
| `TestR23SubscriptionFrozenReadAndMutationLinearizeBehindFenceTransaction` | queued FROZEN, read and mutation use one database linearization point; stale access cannot overtake the fence |
| `TestR23SubscriptionPurgeAtomicCleanupPermanentDedupAndRestartReplay` | actual participant-8 purge erases raw provider event ID/payload, commits only HMAC dedup plus immutable compact PURGED authority, enforces exact 30-day evidence retention, restart replay and identifier-reuse denial |
| `TestR23PaddleAndCloudPaymentsWebhookPathsFenceBeforeSideEffectsAndDenyPostPurgeReplay` | both real handlers fetch the canonical domain-HMAC key and persist its fence before business writes; a fresh service/store recomputes the database lookup and accepts post-purge replay without recreating entitlement |
| `TestR23PaddleAndCloudPaymentsWebhookFenceAndBusinessEffectsRollbackTogether` | forced entitlement failure rolls back HMAC fence, billing event and entitlement together for both real handlers; retry through a fresh service is a first delivery |
| `TestR23SubscriptionConcurrentPurgeAndCleanupFailureConverge` | rollback on cleanup failure and one byte-identical receipt across concurrent retry |
| `TestR23SubscriptionPurgePersistsProviderRenewalCancellationForDurableRetry` | for Paddle and CloudPayments, captures the billing row PK, exact provider event ID and stored details bytes; cancellation failure preserves the same row byte-for-byte, successful retry deletes that PK, and fresh-service/store post-PURGED replay keeps it absent |
| `TestR23SubscriptionServiceHasNoProviderFenceReadSurface` | provider fence has no RPC read surface |
| `TestR23ProviderEventHMACDomainVector` | exact domain-separated provider-event HMAC vector and byte sensitivity |
| `TestR23SubscriptionMigrationSchemaAndPermanentFences` | exact names, PostgreSQL types, nullability, primary/unique keys, no erasure-sensitive FKs, value/hash/positive-generation/terminal-retry coherence, and per-column immutable evidence in all three tables |
| `TestR23ProviderEventRecorderExactReplayConflictAndFailClosedKey` | same-transaction fence, rollback absence, canonical replay, first-seen stability, conflict and key fail-closed |
| `TestR23ProviderEventRecorderConcurrentOneDimensionMatrix` | concurrent identical first delivery, outcome-only conflict and key-presence-only failure each vary one binding dimension and leave one fence |
| `TestR23LifecycleEvidenceCleanupRemovesOnlyExpiredFullBytes` | 30-day full-byte compaction preserves permanent lifecycle/provider fences and old key versions |
| `TestR23SubscriptionMigrationGuardedDownRefusesEveryEvidenceClass` | named SQLSTATE `55000` refusal preserves each durable evidence class |
| `TestR23SubscriptionMigrationDownRechecksConcurrentCommitAfterLocks` | for each of the three tables, DOWN blocks on the writer, locks before its evidence check, observes the committed row and preserves schema/data |
| `TestR23SubscriptionMigrationEmptyDownRoundTripsWithoutCollateralDamage` | empty guarded DOWN drops only 000003 objects and preserves base Subscription data |

## Reproducible RED

From `src/backend/subscription`:

```powershell
go test -short ./...
go test ./internal/store -run 'TestR23ProviderEventHMACDomainVector|TestR23SubscriptionMigrationSchemaAndPermanentFences' -count=1 -v
go test ./internal/grpcsvc ./internal/store -run R23 -count=1
```

Expected pre-GREEN production roots are all absent:

1. `000003_space_lifecycle_provider_dedup.up.sql` and `.down.sql`.
2. concrete `SubscriptionGRPC.ApplySpaceLifecycleFence` and `PurgeSpace` methods.
3. transactional `SubscriptionStore.RecordProviderEventTx`.
4. `SubscriptionStore.CleanupSpaceLifecycleEvidence`.
5. injectable provider-event HMAC key, CloudPayments verifier/decoder and
   renewal-cancellation dependencies used by the real handlers without provider
   credentials or live billing calls; tests bind structurally and do not dictate
   production field names.

The first command compiles every package and fails only because generated
`UNIMPLEMENTED` still owns the two lifecycle RPCs and therefore cannot enforce
either trusted-caller matrix. The second proves the independent HMAC fixture is
green, then fails at the exact absent migration path. All database-backed tests
are executable behind that same migration root; webhook/provider tests use only
injected deterministic fakes. No production file, generated output, provider
credential, live billing call, `docs/PLAN.md`, `docs/TODO.md`, or other service
belongs to this RED.

## GREEN handoff

The next writer may add the exact 000003 migrations, focused store/provider
fence and lifecycle roots, concrete gRPC handlers, minimum trusted-principal
wiring, and precise Subscription documentation. Review these tests first. Keep
provider keys behind the Subscription workload and a distinct KMS/HSM family;
tests inject only non-secret fixture bytes at the transaction seam. Run the
focused R23 suite after each behavior, then the complete Subscription checks
from `docs/TESTING.md`.

`R23_SUBSCRIPTION_RED.sha256` is the machine-readable freeze for this manifest
and both executable test files.
