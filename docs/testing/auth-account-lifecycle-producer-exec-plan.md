# Auth exact-cycle lifecycle source

Implement the accepted A4 source component: preserve an opaque lifetime's exact
deletion cycle, original schedule/deadline and committed decision independently
of its current state, with immutable versioned delivery intent in the same
owning transaction. Auth remains Java/Spring. No public RPC, User-purpose
transport, NATS subject/grant, provider activation or full A4 acceptance is part
of this source packet. Legacy account_deleted delivery remains distinct.

The governing deletion/recovery behavior is
[Auth](../microservices/auth-service.md) and
[account recovery](../features/auth-and-contacts.md#удаление-аккаунта--gdpr).
The agreed semantic source contract is BE-019/BE-020 A4-ACCOUNT-LIFECYCLE-PRODUCER-v1
(reviewed docs commit6bf8e225); advisor195's exact-source milestone specifies the
source/transport boundary. A terminal restore fence is not physical erasure.

## Verified prerequisites and lineage

Start from master3835aa9b717d448f47bc549e842de5b263b86d06. Normally merge the
reviewed PR958 clock package037bb858f3160f23dcba31f7261d1b9a462fee8b into this
separate branch; PR958 and its lifecycle ownership remain with142.

Reconstruct only ten exact prerequisite artifacts: V32/canonical33 up+down and
OAuth history profile from live PR916050450f6; V33/canonical34 up+down and
Subscription history profile plus the two finite history readers from the
introducing11f60c0e392 commit. Do not merge the broad ee7 branch or import its
Subscription principal/RPC/runtime. Exact source/blob manifest is retained in
BE-019 evidence. V28 SQL7441885d, catalog-v1 JSON615e7e41 and validator e52e6e27
are unchanged across master, OAuth and Subscription predecessors.

Freeze and validate this coherent predecessor closure before allocating the
next A4 migration. Never number an A4 migration on bare master32 or overwrite
V32/V33. Both Flyway and canonical finite profiles must remain exact; function
changes need explicit versioned catalog/body-hash compatibility, not a relaxed
catalog-v1 validator or mixed history.

## Production test and implementation sequence

1. Use real PostgreSQL/JDK25 Linux owning repositories and actual role identity
   guards. Delete through the transactional starter, restore strictly before
   the original deadline, then query the old opaque cycle after current ACTIVE
   has cleared its cycle. Preserve a causal missing-history RED; a real local
   SQL query seam and positive stored-record control must avoid an always-deny
   fixture stub or setup-only failure.
2. Capture schedule/decision and stable versioned intent at the existing owning
   account/lifetime transitions. Retain account, lifetime, journal lock order;
   sample authoritative decision time after locks and use it for predicate and
   recorded outcome. No remote call occurs under those locks.
3. Preserve immutable old schedule/decision while newer current state advances.
   Keep opaque records independent of raw-account cascading deletion. Unknown
   pre-capture history stays explicitly unavailable, never guessed/backdated.
4. Prove transaction/outbox rollback, held-lock expiry/equality rejection,
   duplicate deletion/intent, next-cycle dominance and old-restore replay. Keep
   the existing legacy outbox identity/PubAck semantics and predecessor catalog
   controls. Source success is not transport/currentness/physical-erasure proof.

## Progress

- [x] Fresh WT93 lease9fcfab70acba808a997c4b0d930dc402 and clean3835 verified.
- [x] Exact live PR958 head/base/review/ref read; local normal merge only.
- [x] Ten predecessor blobs materialized and byte-verified against exact sources.
- [x] Static lineage validator PASS: all30 unique SQL SHA256/Flyway checksums, exact canonical32/33/34 profile prefixes, ten byte-identical source blobs; owned V28 function/catalog bytes unchanged. Linux compilation/compatibility tests remain separate checks.
- [ ] Allocate eligible successor schema/catalog and preserve actual source RED.
- [ ] Implement source capture and focused GREEN, then independent review.

User origin/complete coverage, protected dedicated observation/hint transport,
terminal physical cleanup, backup/rollback activation and restore-aware Search
discovery remain separate prerequisites. No consumer ACK, PubAck, current ACTIVE
or local User writer proof is relabelled as those authorities.
