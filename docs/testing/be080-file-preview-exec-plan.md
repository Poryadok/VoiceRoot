# BE-080 File preview projection

This living ExecPlan follows `.agent/PLANS.md`. Owner: Рудеус-259 (Sol 6.1,
medium); integration consumer: Рудеус-121. Original base:
ceb671962b3251985b203d20e3136a11c73e37f9. The continuation normally merged
develop 3071ddad390443de4119c796cdad9899ff08f7fd before further work.
The accepted authority decision is BE-080 c2, immutable attachment
f5a11478f6f3184228672491878866a7 (Astra264). It specifies the negative/race
matrix and deployment prerequisites; this plan does not grant activation.

## Purpose and contract

Existing plaintext non-Music file attachments must converge after File processing,
failure, deletion and expiry, including forwards and repeated attachment slots.
File owns complete redacted revisioned snapshots. Messaging owns exact private
MESSAGE reference bindings, persisted projections and ordinary message.edited
outbox effects. Real viewer admission and URL authorization remain authoritative.
No snapshot contains URLs, storage keys, viewers or chat routing claims.

Add FileProcessed.preview field 6 and a protected Messaging-only exact-reference
ResolveMessageAttachmentPreview RPC. Retain existing Music wire and revision
namespace. File state/revision/event bytes commit together; retries publish the
same event ID. Messaging persists snapshots before destination discovery, rejects
same-revision conflicts and terminal resurrection, and reconciles inserts under
ordered snapshot locks. Frozen destinations retain durable work for later retry;
purge never recreates messages. A bounded durable legacy reconciliation pass
closes preexisting history and lost broker wakeups.

## Progress

- [x] Source assessment and exact authority decision accepted in TT c3/c4.
- [x] Fresh develop worktree and nonoverlapping migration numbers reserved:
  File 000007, Messaging 000034.
- [x] Canonical wire, migrations and generated Go/Dart mirrors; buf lint passes.
- [x] Transactional File producer, publisher and protected resolver written.
- [x] Private admission bindings, monotonic snapshots and refresh dispatcher written.
- [x] Bounded legacy/pending recovery and source-correct forward selectors written.
- [x] Bounded Linux PostgreSQL/NATS negative/race/replay checks and scoped static checks.
- [ ] Immutable packet, independent review and accepted integration handoff.
- [ ] Complete release qualification, public composer/history/list journey and
  all-consumer rollout/credential/preservation gates; no activation claim.

## Validation and rollout

Use owned Linux Docker runners without host ports. Cover real PostgreSQL rollback,
revision 3/1/2 replay, conflicting equal revisions, terminal floor, before-insert
events, multiple slots and concurrent mutation; protected principal/reference
denials; frozen/restore/purge and restartable legacy work. Preserve Music and E2E
behavior. Verify additive proto compatibility and generated mirrors, affected Go
build/tests/lint, and unchanged four-consumer topology with isolated NATS.

Deploy schema/resolver first, then every Messaging consumer, then enable generic
File publication. Publication defaults off while outbox accumulates. An old
consumer ACKs generic events, so rollback requires pausing publication and retaining
outbox/reconciliation work. Shared stand checks and activation require their own
resource claim and version evidence; source checks cannot claim those outcomes.

## Discoveries and decisions

CodeGraph exact extension query timed out after 300 seconds; current source reads
confirm the accepted c72 contract is unchanged at this base. Existing File legacy
deletion uses NO KEY UPDATE to remain compatible with reference FK KEY SHARE.
Preserve this order and test concurrent reference acquisition rather than adding
an incompatible File FOR UPDATE lock. The new resolver must never auto-create a
missing Space fence via lockLiveFence.

Resolver lock order is existing Space SHARE -> File SHARE -> blob KEY SHARE ->
exact reference SHARE. Release takes blob before reference, so taking reference
first would create a concrete unscoped release deadlock. File SHARE is compatible
with Acquire's FK KEY SHARE and conflicts with physical File-state mutation.

Legacy repair uses a retained deterministic attachment intent and the same session
lock as abandoned-intent cleanup; both local transactions reuse that connection
and hold no row locks over RPC. A missing reference behind an unrelated existing
send intent is recorded as `existing_owner_intent_requires_reference_repair`, not
silently reacquired behind a keyed caller. It retries after authoritative owner
repair. This is a concrete tracked migration gap, not completed old-history coverage.

## Partial validation checkpoint (2026-10-19)

Owned Linux Go 1.26.5 runner e61ee76f6289 (no host listener) was stopped and
removed after all commands terminated. Real PG tests passed for File atomic
producer/retry/exact-ref/tombstone and protected listener denials. Messaging
race tests passed for generic PG+isolated NATS rollback/restart/duplicate delivery,
private multi-slot/forward refresh, retained terminal before first insert,
client fingerprint stability, legacy repair/freeze/restore and ghost suppression.
The multi-file lock-order test initially waited for the wrong SQL query text;
correcting the observation to include the actual SELECT FOR UPDATE produced a
passing scoped race rerun. Shared contract race tests passed 12 cases. Scoped
Messaging store/grpcsvc golangci-lint 2.6.1 (Go 1.26.5) reported 0 issues.

The first combined runtime fixture attempted every historical migration and
failed on the already-folded pins table; it now uses the established current-schema
component fixture and its real PG/NATS test passed. Neither fixture setup failure
is represented as a product PASS. Initial File race and listener runs also passed,
but their stdout-only result is supporting evidence, not the portable final log.

This is an immutable WIP checkpoint so the author can prioritize the accepted
BE-081 Phase-0 correction. Remaining: complete latest-source Go/static checks,
finite 12-case negative/race matrix (including real release/resolver concurrency
and external replay/JWKS/Redis denials), proto compatibility/mirror qualification,
four-consumer credential/preservation qualification, independent review and
accepted integration handoff. No publication/stand/acceptance authority is implied.

## Qualified continuation (2026-10-19)

Native Linux Go 1.26.5 ran in an owned 2-CPU/3-GiB runner, with an owned private
DinD fixture and no host ports. Source archives are immutable Git snapshots:
dd9877cb6 for the first component run, 84973e2c0 for the extended matrix and
890122b36 for the final one-line test cleanup correction. Production source is
identical across these three heads. Final source manifests and archive digests
are retained with the raw logs; the final documentation delta does not change
executed code.

| Contract area | Actual bounded evidence | Remaining boundary |
| --- | --- | --- |
| Producer durability and terminal reference reads | Two real PG tests pass under race, including failed PubAck byte replay, redacted tombstone, foreign owner, missing/frozen/purged fence, clock-expired denial and 24 actual resolver/Release/Acquire races | Not every process-kill window is separately injected |
| Consumer rollback and restart | PG plus populated isolated NATS, local outbox failure, restart, duplicate delivery and one message.edited effect pass | Public REST/history/list/composer journey is not executed here |
| Monotonicity and insertion | Five store tests pass: multi-file lock order, legacy recovery/freeze/restore/ghost suppression, repeated slots/forwarded copies and terminal-before-first-insert; shared closed snapshot contract passes 12 test events | Existing unrelated owner-intent migration gap remains explicit above |
| Protected authority | File principal suites pass 19 top-level plus 100 subtests under race, including the new exact preview RPC through real TLS/JWKS/replay transport, caller/binding/unknown/raw/duplicate denials and authority outage | Shared transport fixture uses actual TCP miniredis, not production Redis enrollment |
| Existing attachment pipeline | Eleven actual PG/race tests plus four subtests pass for E2E gates, processing/failure and viewer-authorized default/thumbnail/download behavior | No device/provider or Flutter E2E claim |
| Broker compatibility | Exact INFO/ACK/inbox-only Messaging actor processes preview replay/restart; actual CREATE and arbitrary-publication denials are observed; legacy neither-branch, Music and dual-branch rejection pass | This synthetic actor proof does not replace signed four-consumer enrollment/native preservation or all-new-replica rollout proof |
| Static and wire | File/Messaging builds pass; buf lint and breaking against 3071ddad pass; each changed Go wire has three identical mirrors | Dart generation is retained; no new Dart runtime qualification |

File short tests: 87 top-level and 144 subtest passes, 92 explicit test skips.
Messaging corrected short run: 258 top-level and 274 subtest passes, 249 explicit
test skips. File lint reports zero issues. Messaging lint on the exact diff
against 3071ddad reports zero issues; the broad run also records two unchanged
errcheck findings in game_message_cross_service_acceptance_test.go. They are not
silently disabled or claimed fixed. The initial broad run found a cancellation
warning in the new restart fixture; 890122b36 fixes it and both the scoped lint
and actual PG/NATS fixture rerun pass.

Preserve the failed setup evidence: the initial login shell reset PATH before
Go executed, and the first Messaging short archive omitted docker-compose.yml.
The corrected archive includes its exact Git bytes and the entire short suite
passes. These are not product failures or retrospective PASS claims. Graphify
update was bounded and cancelled (exit 1), so graph freshness is not certified.
Both concrete CodeGraph queries timed out and were accounted for.

The four canonical declarations, bootstrap ACL sources and preservation helpers
are byte-identical to the integrated base. Before activation, the existing
signed-credential/native preservation and all-replica upgrade obligations still
apply. Keep publication disabled until those actual rollout gates pass. This
packet is a component-review offer, not completion of all twelve release cases.

## Outcome

Implementation and component validation are ready for independent review.
No deployment, activation, whole-repository qualification or full public journey
claim is made. The task remains unfinished through its remaining verification
and delivery stages.

## P1 rollback correction plan (review378)

Review `REVIEW-BE-080-97d7b60ddf74` c1 / parent c11 correctly rejects the two
unconditional DOWN scripts. Pending publisher bytes, revision-only File state,
terminal-only snapshots, private bindings and durable reconciliation work must
not disappear during rollback/re-UP. No shared migration execution is authorized.

The correction is limited to File007/Messaging034 DOWN, focused real-PG tests
and this owning plan. First write/run the regression against the unchanged DOWN
scripts, then guard the actual DDL and rerun the same tests. Each DOWN uses one
DO statement with ACCESS EXCLUSIVE NOWAIT locks before any check or destructive
DDL. Busy writers cause a prompt rollback refusal; all acquired locks and DDL
share that statement's transaction. This avoids a check-empty-then-DROP gap and
does not wait in conflicting existing application table-lock orders.

File rollback is allowed only with no files and no preview outbox rows. Even a
default revision1 may already have been returned by the read-only resolver;
there is no durable issuance marker proving otherwise. An existing file thus
requires retaining the column, including when its outbox has been drained.
Messaging permits only empty snapshot/pending/error state, empty private
bindings and its single untouched zero-cursor/pass reconciliation seed. A busy
or non-pristine schema requires a compatible forward correction, not erasure of
authority to make rollback succeed.

Use an owned Linux/private PostgreSQL fixture with the real UP/DOWN SQL and
minimal prerequisite tables. Cover pristine DOWN/re-UP, each retained evidence
class, default/revision-only File rows and a two-connection first-writer oracle.
The oracle positively commits the writer and requires DOWN to refuse while
preserving its data/schema; it observes a real PostgreSQL lock wait or an early
NOWAIT refusal and discriminates an empty-check/late-DROP implementation. Preserve
RED and GREEN raw output bound to immutable source, then request genuine378
delta recheck through queen. Existing publication/stand/release limits remain.

Executed correction: immutable test-first14976543f retained both original DOWN
scripts and produced the expected real-PG RED: two pristine cases passed, all
17 retained-state/first-writer cases failed because destructive DOWN succeeded.
The six first-writer transactions reached PostgreSQL and committed positively;
no sleep-only or setup-error result is used as a race oracle. Correctioneed578485
changes only the two DOWN scripts. The identical two top-level tests/all19 cases
then pass under Linux Go1.26.5 race (26.620s, zero failures/skips). Package vet
and golangci-lint2.6.1 built with Go1.26.5 also pass, zero issues. Both executed
archive trees match native before/after manifests. Final plan-only updates do
not change tested SQL/tests. Graphify was attempted and bounded/cancelled with
exit1, so no graph-freshness claim is made; the direct CodeGraph query timed out.
The correction requires genuine378 delta recheck; prior HOLD is not silently
promoted to CLEAR, and broader component/release obligations remain open.

Final-check refinement: a caller may already hold an empty REPEATABLE READ
snapshot before a different connection commits the first record. Early table
locks alone cannot refresh that snapshot. Add a real stale-RR regression before
implementation, then explicitly refuse any isolation except READ COMMITTED in
both DOWN statements. This is the same retained-authority correction, not a new
lifecycle feature. Intermediate280d96f remains HOLD until the refined proof and
genuine378 recheck are complete.

The refinement is now executed: immutable6d76fe945 adds only the stale-RR
regression/plan while retaining intermediate280's SQL. Both RR cases fail for
the intended reason: DOWN accepts the caller's old empty view after the other
connection has positively committed a row. SQL correction784467881 refuses
non-READ-COMMITTED isolation. The complete three top-level/21-case suite passes
under native Linux PG16/race in73.397s with zero failures/skips. Final vet and
lint with the repository's actual root .golangci.yml pass, zero issues. Earlier
default-profile lint records remain historical rather than canonical-profile
proof. Each executed source archive matches all766 native files before/after.
The superseding final packet retains both meaningful RED cycles and GREEN runs;
intermediate c12 attachment manifests are explicitly incomplete, not final proof.

## Current MASTER composition continuation

Genuine author259 continues on confirmed MASTER candidate
78bc80b975bc7df00c8e8902ce58186b0a4e29f7. The exact65-path delta
from87483 to reviewedfd4de1 is applied with three-way context, preserving
current security/runtime fixes and tests. The three conflicting events Go
mirrors are regenerated from the unified RoleMutation plus FilePreview proto;
no descriptor is selected wholesale from the older branch. Existing SOURCE
CLEAR378 c3 resolves both rollback P1 findings; old component results remain
pinned historical evidence, not proof over this new composition.

The immediate qualification is current File/Messaging builds, focused real
PostgreSQL/NATS positive and negative preview paths, shared snapshot/Role wire
contracts and static/generated checks. This composition does not enable
publication or claim shared-stand, all-replica rollout or public Flutter
acceptance. The separate existing-owner intent with a vanished exact File
reference remains denied and durable, awaiting the canonical owner repair;
recreating another keyed operation is outside preview authority.

Current composition qualified code c29b873584a01c143500de8e3a6d457c63da66fa
is on the newer confirmed MASTER5113a511c5a13f1abe3a760a614641809ee7a76d.
Both File/Messaging builds pass. Real Linux PG16/NATS race tests pass: File
2 top/2sub, protected principal19top/100sub, Messaging restart1/store5,
shared snapshot/Role4top/35sub and rollback3top/21sub; zero failures/skips.
Both complete short suites and vet pass; their explicit integration skips
remain short-mode limits. Canonical root lint passes Messaging/shared. File
lint found only ST1020 on the exported worker comment; corrected the comment
without runtime changes and reran that lint. Local Dart generation has no drift.
Buf lint and breaking against preserved3071 pass. The current helper already
applies File004/005: two old preview tests duplicated that setup. Actual native
RED reproduced the duplicate table, then only redundant setup/import was removed,
keeping all assertions and current MASTER fixture ownership. Earlier dependency
and image-readiness setup failures are retained separately. Runtime source
archives/manifests and comment-only final delta remain explicitly distinguished.
Genuine378 original rollback SOURCE CLEARc3 is preserved; current composition
review and actual121 delivery/MASTER CI are separate subsequent events.

## K5 actual CI correction plan

Immutable0128f325 CI triage8ca2e8c4 identifies three BE080 causes: new
File-to-Events transitive module closure, File import formatting and strict
legacy lifecycle fixture missing migration007 outbox. Census all backend
go.mod files and every service Dockerfile before fixing only missing closure.
GameIntegration root and standalone Calls/Space/Subscription proto modules
need canonical local Events resolution; GameIntegration Docker must copy its
go.mod before download and generated source before build. Keep dependency
versions and security/runtime logic. Canonical format may reorder descriptor
imports; regenerate exact Go/Dart mirrors and explicitly rebind future native
inputs rather than reusing H5 authority.

Preserve exact table equality in the lifecycle fingerprint, adding only the
known preview outbox to the canonical schema and asserting it survives the
unrelated older lifecycle DOWN. Never change production DDL/retention guards.
OriginalK5 readonly Game build, buf format100 and actual PG fingerprintRED
precede minimal correction. Then all affected standalone/root readonly builds
and tests, actual GameIntegration Dockerfile build, buf format/lint/breaking
and generated parity, and realPG lifecycle/preview rollback cases must pass.
OwnWT31/privateLinux fixtures only; no shared stands/host tests/GH/F13 edits.
Genuine378 reviews frozen delta before actual121accepted delivery.

Census result:45 tracked backend go.mod files represent44 module identities;
19 reach File and15 are root Docker consumers. The two GameIntegration proto
module copies share one identity; neither reaches File. No module file is excluded.
The four missing local Events replacements are GameIntegration and standalone
Calls/Space/Subscription. Other Docker contexts copy Events directly or via
the Messaging pb ancestor. Standalone Calls/Space had no go.sum; the new
readonly checks exposed this baseline checksum gap, so exact existing-version
lockfiles are added without dependency upgrades. GameIntegration go.sum stays
unchanged because the new Events module is local. Original K5 readonly build
RED is the missing Events checksum/resolution; original PG exact fingerprint
RED is the expected11/actual12 equality, and canonical buf format RED is100.
The first corrected Game build/short/vet and actual Dockerfile build pass;
strict lifecycle and preview rollback PG suites pass. Final immutable archive
checks include the standalone lockfiles and retain all initial results.

Final qualified code984ad52ee passes readonly GameIntegration build/short/vet,
all three standalone proto compilations,19 local Events lookups and the actual
canonical Dockerfile build. The short suite has85top/106sub passes and93 explicit
integration skips; generated modules have no tests and are compile-only checks.
Actual PG lifecycle2top and preview rollback3top/21sub pass with zero skips.
Canonical root golangci-lint2.6.1 built with Go1.26.5 reports zero issues for
GameIntegration and File grpcsvc. All4202 final native input files match Git
before/after checks and after lint. All4161 Docker-context backend files also
match the immutable archive. Canonical buf format/lint/breaking pass; normalized
FileDescriptorSets are equal after import ordering/source-location normalization.
The three File Go mirrors match and Dart output has no drift. None of these
component results claims wholeCI/T16/T31E2E, native H5 successor authority,
rollout or acceptance. Current publisher121 must bind the precise new inputs.
