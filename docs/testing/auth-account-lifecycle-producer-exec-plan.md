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
- [x] Static lineage validator PASS: all30 unique SQL SHA256/Flyway checksums, exact canonical32/33/34 profile prefixes, ten byte-identical source blobs; owned V28 function/catalog bytes unchanged. Linux JDK25 compatibility checks then passed10/0failure/error/skip; the initial six-test run exposed three stale baseline matrix assertions. Both test matrices now byte-match the same verified predecessor source, preserving all unknown/mixed/dirty negatives.
- [x] Eligible successor V34/canonical35 allocated only after exact predecessor closure. Native producer RED (session71172): three tests, one intended failure, zero errors/skips. Actual delete then predeadline restore leaves current ACTIVE but old-cycle SQL query empty; stored-record/owning-transaction controls pass. Initial compile-only overload error is separately retained and is not causal RED.
- [x] Implement source capture and focused GREEN: Linux JDK25.0.4/PostgreSQL16.15, session36829 terminal0 BUILD SUCCESS,21 tests/0failures/0errors/0skips (native producer9, migration source7, gate/history5). Native suite38.53s. Report-gate harness also passes all positive/negative inventory/wiring cases.
- [ ] Independent immutable-source review; publication remains held.

The capture change is confined to V34/canonical35, its finite history profile,
and explicit catalog compatibility. Replace the existing account guard and
deadline materializer from their pinned V28 definitions, retaining account →
lifetime → journal ordering. On a new deletion record the existing lifetime's
cycle/revision and original schedule/deadline; before a restore clears current
cycle, record RESTORED with the same post-lock time used by its deadline check.
Deadline materialization records LIFETIME_TERMINAL, never physical ACCOUNT_PURGED.
Each new fact and immutable versioned intent commits or rolls back with the
account mutation. Pre-capture cycles remain unrecorded. The local query is a
persistence seam, not a protected observation or wire contract. Existing legacy
outbox and PR958 source remain unchanged. Relevant helper bodies and metadata
must be covered by an explicit successor catalog; catalog-v1 is not relaxed.

The private source intent has protocol_version=1 and a stable event_id, kind,
owner_handle, deletion_cycle_id, decimal-string lifecycle_revision, original
scheduled_at/purge_at and optional decision_at, encoded as UTC microsecond JSON
bytes with a retained SHA256. It contains no raw account/profile/provider IDs.
Facts and intent identity/bytes reject update/delete/truncate; only bounded
delivery bookkeeping may change. There is no publisher, worker receipt, RPC,
subject or consumer credential in this packet; pubacked_at is not acceptance.

Catalog-v2 is a complete explicit successor (9 tables,11 functions,9 triggers),
selected only by the exact locked A4 Flyway profile or canonical35 marker.
Catalog-v1 and both predecessor SQL versions remain byte-identical. A separate
nonowner JDBC fixture qualifies v2, rejects a replaced capture-helper body and
qualifies again after restoration. The first qualifier failure was the fixture's
extra direct barrier SELECT grant; correcting the fixture preserves the exact
PUBLIC SELECT ACL and does not relax production validation.

Native controls cover actual intent-failure rollback of delete and restore,
stable retry identity/bytes, next-cycle dominance without old-decision overwrite,
expired restore and opaque-history survival after raw account deletion, equality
rejection, immutable fact/intent denial, and an actual restore waiting across D
on the owning account lock. Compressed-time fixtures are boundary controls, not
production backfill or evidence of physical deletion exactly at D. The recorded
decision time is the fresh database cut after account/lifetime serialization;
it is not a claim about transaction flush completion time.

Portable raw evidence is retained under tmp/swarm/anton148-auth-cycle-producer:
producer-red.xml/log (causal missing history), producer-first-green.xml/log,
producer-controls-final.log and three final Surefire XML reports, plus report-
gate.log. Compile/command/setup failures remain separately retained, never
reported as behavioral RED. Final focused runs reuse unchanged generated proto
outputs via the plugin's documented local protoc.skip property; compilation and
all selected tests still execute. No coverage percentage is claimed.

User origin/complete coverage, protected dedicated observation/hint transport,
terminal physical cleanup, backup/rollback activation and restore-aware Search
discovery remain separate prerequisites. No consumer ACK, PubAck, current ACTIVE
or local User writer proof is relabelled as those authorities.
