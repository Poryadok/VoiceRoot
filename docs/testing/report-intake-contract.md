# Opaque global report intake (BE-137)

Owner decision: anyone knowing the target ID may submit; acceptance must not
reveal existence. The requester remains authenticated and the existing own-active-
profile prohibition remains. This is global moderation, not Space-local reports.
The receipt/state/worker decisions below were selected autonomously under the
owner's delegated mandate (TT BE-137 c3–c5).

## Public receipt and trusted reporter

Gateway signs the exact normalized `CreateReportRequest` for
`/voice.moderation.v1.ModerationService/CreateReport`, audience `moderation`,
with verified account/profile/session epoch and current request ID. Moderation
verifies the delegated credential, current epoch and replay admission; raw
identity metadata and the ordinary listener cannot create an intake.

Every valid non-self request commits a real reports row then returns `202`/OK,
`CreateReportResponse.report.id` only. Other fields are unset. No target lookup,
viewer ACL, target account mapping, automod or publication occurs in that wait chain. Receipt
ID construction never rereads a worker-mutated row. Owner failure does not alter
otherwise healthy intake. Store failure returns generic unavailability, never
success without commit. Input JSON/description/session/self errors remain explicit.
There is no reporter status endpoint or WS/push outcome callback. A receipt is
not a capability to read evidence or decide a complaint.

TT BE-137 c11/c12 explicitly refine the requester authority under the delegated
autonomy decision, without changing target non-disclosure. Before intake SQL,
Moderation resolves the authenticated reporter's current opaque subject from
protected Auth. Its account, canonical subject, lifecycle revision and session
epoch must match the verified Gateway identity; malformed or unavailable Auth
authority gives uniform generic unavailability for every target type. A valid
current deleted requester or epoch mismatch cannot submit. No target-dependent
request runs synchronously. LIVE Auth restriction alone does not create a new
reporting prohibition.

Migration `000008_report_subject_provenance` adds nullable reporter and validated
target subject/revision pairs. Fresh intake records the reporter account and
opaque subject in the same account-then-subject serialized transaction as its
receipt, using BE-045's `BindSubjectTx` and `RequireLiveSubjectTx`. A permanent
opaque terminal fence or newer conflicting lifecycle state wins over stale LIVE.
No raw account tombstone or profile/account UUID inference is introduced.

Validation resolves the persisted reporter subject and any LIVE User target's
subject outside SQL. It locks all accounts in sorted distinct order, then all
subjects, then its still-current claimed report row, before activation and the
BE-046 hook. Reporter soft deletion after acceptance does not retrospectively
change report eligibility; its verified purge deadline still applies. A known
terminal reporter or target discovered while the target columns are still NULL
scrubs pending work instead of attaching erased identity. Auth NOT_FOUND or
FAILED_PRECONDITION text is never terminal evidence. Unknown authority retries.

The shared boot/restore owner injects `ReportPrivacyAcquire`, which returns a
generation-bound `ReportPrivacyCheck`. Nil closes intake, validation and dispatch.
The captured check runs before a transaction and before commit or dispatch, and
must stay invalid after closure/reopen. It supplements authoritative SQL terminal
and deadline checks. It does not promise atomicity across Auth, SQL and a broker.
Actual shared startup/import/recovery wiring remains a separate activation gate.
Legacy NULL subject provenance needs owner-authorized cleanup/backfill and a
current recovery cut before admission; existing account evidence is insufficient.

After authoritative replay and terminal participant cleanup, the shared
coordinator calls `ReportStore.RequirePrivacyRecovery(ctx)` before P5. This
read-only check rejects unresolved legacy NULL provenance, missing/remapped or
older local subject bindings, terminal residue and expired lifecycle deadlines.
Both reporter and VALID User-target bindings must be coherent; an unknown PENDING
target is allowed. Accepted reporter soft deletion before its purge deadline is
retained. The check neither repairs rows nor synthesizes terminal state, invokes
owner RPCs or opens a separate readiness flag. A failed check requires the
protected authoritative owner recovery path; it is not a permanent READY waiver.

## Durable state and effects

`reports.validation_state`: `PENDING`, `VALID`, `MISSING`, `DELETED`. Claims,
attempt count, next-attempt time and sanitized failure code are durable. A worker
claims bounded work without holding a transaction across network calls. A fresh
principal/request ID is used on each retry. Failure/configuration uncertainty
stays PENDING with bounded backoff; attempts never silently discard accepted work.

LIVE atomically changes a still-pending claimed row to VALID and records stable
domain/analytics outbox effects. Only VALID rows enter staff queues, report counts,
resolve/assignment and sanction-by-report paths. User-target threshold evaluation
uses the authoritative User account mapping; no profile-as-account fallback.
BE-046 owns audience/count/case policy. The activation transaction invokes one
`ReportValidated(context.Context, pgx.Tx, ValidationFact)` hook to enqueue that
independent work; it never runs a competing legacy threshold writer. The fact
contains server `created_at`, verified submission account when present, and the
User target account mapping. It does not invent profile/subject revisions:
BE-046 obtains those authorities before counting a vote. Legacy identities without
verified submission account remain manual-only. A hook failure rolls back
activation and its publication evidence. Account/target serialization and
transactional effect insertion prevent duplicate sanctions/actions.
Dispatch is at least once with stable IDs and consumer dedupe, not network exactly
once. P3 removal/scrubbing wins over a stale validator; completion never recreates
a removed reports row or revokes an existing sanction.

MISSING/DELETED terminally dismiss the internal intake with `TARGET_NOT_FOUND` or
`TARGET_DELETED`, scrub description/evidence and cause no queue/count/sanction or
report-created publication. Reporter response remains unchanged.

Unresolved legacy pending/reviewing rows re-enter PENDING validation, retaining
historical publication provenance so validation cannot replay old report-created
events. Historical closed rows/sanctions remain history, without reissue/revocation.
Migration must expand before new binaries; remove legacy writers before relying
on intake validation fences. Rollback must preserve admitted work/outbox evidence.

`report_intake_outbox` keeps unique `(report_id, kind)`, event UUID, fixed subject,
serialized bytes, next attempt, attempt count, safe failure and delivery time.
Domain protobuf and analytics protojson retain their existing wire formats. A
bounded publication attempt locks accounts, subjects, its report, then event; a committed purge wins
before a later publish. A lost ACK retries identical ID/bytes. Delivery checks
the deployment-owned JetStream contract and sets `Nats-Msg-Id`; it neither creates
production streams nor claims network exactly-once semantics.

At verified terminal account purge, BE-045 calls
`ReportStore.PurgeReportAccountLinksTx(ctx, tx, accountID)` under the common account
lock, before removing its authoritative raw-account/opaque-subject mapping.
The participant removes matching verified reporter/validated-target reports and
their outboxes, detaches linked sanctions and leaves a minimal `TARGET_DELETED`
reason. It never revokes a sanction or closes an appeal. Failure rolls back the
owner purge transaction. Unknown legacy profile/account mappings require the
separate User-owned cleanup authority; UUID equality is never a substitute.

The same terminal transaction invokes `PurgeReportSubjectsTx(ctx, tx, subjects)`
after the lifecycle owner inserts its opaque fence, including the no-account-
mapping restore branch. This removes reporter/validated-target subject matches
and cascading work without guessing the erased account. Effective sanction and
appeal snapshots remain; participant failure rolls back the entire purge.

## Reporter transport configuration

The separate mTLS listener has only CreateReport. `MODERATION_INTAKE_` settings
are `GRPC_LISTEN`, `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`,
`GATEWAY_JWKS_URL`, `GATEWAY_JWKS_CA_FILE`, `REPLAY_REDIS_ADDR`,
`REPLAY_REDIS_PASSWORD`, `EPOCH_REDIS_ADDR`, `EPOCH_REDIS_PASSWORD`. Any present
setting activates strict validation; partial configuration fails startup. The
Gateway JWKS endpoint is exact HTTPS, current and next distinct RSA keys only.
Replay and current Auth session floor use separately configured Redis clients;
missing/noncanonical/outage floor never becomes default epoch one.

Gateway's dedicated `GATEWAY_MODERATION_INTAKE_` settings are `GRPC_ADDR`,
`TLS_CA_FILE`, `TLS_SERVER_NAME`, `CLIENT_CERT_FILE`, `CLIENT_KEY_FILE`.
The client has no automatic gRPC retry or plain-client fallback. Activation
requires the canonical Gateway signer and strict session epochs. Category alias
normalization happens before deterministic request hashing. Only authorization
and fresh request ID metadata leave this route; raw HTTP identity is ignored.
Ordinary Moderation listeners deny CreateReport even when intake is unconfigured.
Runtime workers, owner transports and the positive Compose path must all be
integrated before claiming BE-137 complete; these settings alone are insufficient.

## Minimal protected owner decisions

Each method is an additive RPC on the owner's existing service, available only
on its dedicated protected listener. Caller is signed `service:moderation` with
issuer `moderation`; no reporter subject is forwarded. Exact audience, RPC,
deterministic protobuf hash, request ID, temporal/JWKS rotation and shared replay
checks precede a domain binding check and read-only owner query. Ordinary listeners
deny these methods. No public REST route or content-returning lookup is added.

| Owner / audience | Exact method | Request fields | Response |
|---|---|---|---|
| User / `user` | `/voice.user.v1.UserService/ResolveReportTarget` | `report_id`, `profile_id` | state, account ID only when LIVE |
| Messaging / `messaging` | `/voice.messaging.v1.MessagingService/ResolveReportTarget` | `report_id`, `message_id` | state only |
| Space / `space` | `/voice.space.v1.SpaceService/ResolveReportTarget` | `report_id`, `space_id` | state only |
| Story / `story` | `/voice.story.v1.StoryService/ResolveReportTarget` | `report_id`, `story_id` | state only |

LIVE means authoritative existence without logical deletion. Private, blocked,
nonmember, hidden, ghost/FOR_ME messages and retained expired/archive/Highlight
stories remain live; caller visibility is irrelevant. User honors both profile
deletion and its authoritative Auth account-deletion overlay, while freeze or
suspension alone is not deletion. DELETED requires a retained deletion/tombstone;
MISSING means authoritative absence. DB/dependency/config/verifier uncertainty,
including an uncertain Space freeze transition, is unavailable. Unknown/unspecified
or malformed success is a dependency failure, never LIVE. No content, viewer
counters, membership mutation or retention extension accompanies the decision.
A later restored live target may qualify on a new complaint; old terminal
receipts do not reopen. The snapshot is not authority for later content access or
privileged action: those retain their current owner checks.

## Verification boundary

Verify actual committed receipt-only responses across all target states, zero
owner calls before response, blocked-owner independence, worker restart/retry,
strict signed transport negatives, no pending queue/count/action bypass,
competing-worker and purge races, stable outbox/automod effects, invalid-target
scrubbing and legacy publication provenance. Source, isolated Linux runtime,
hosted CI, deployment and independent acceptance evidence remain distinct.


## Four-owner runtime configuration

Each owner uses `<OWNER>_REPORT_TARGETS_` (`USER`, `MESSAGING`, `SPACE`, `STORY`):
`GRPC_LISTEN`, `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`,
`MODERATION_JWKS_URL`, `JWKS_CA_FILE`, `JWKS_CLIENT_CERT_FILE`,
`JWKS_CLIENT_KEY_FILE`, `REPLAY_REDIS_ADDR`,
`REPLAY_REDIS_PASSWORD`. Any present setting enables strict complete startup
validation. Sole trusted issuer is moderation; adding other JWKS issuers fails.
The shared principal's existing S2S cache duration settings retain their defaults.
Replay keys include owner namespace and credential JTI, shared by receiver replicas.

Moderation uses `MODERATION_REPORT_TARGETS_` with `TLS_CA_FILE`, `CLIENT_CERT_FILE`,
`CLIENT_KEY_FILE`, and each owner's `<OWNER>_GRPC_ADDR`/`<OWNER>_TLS_SERVER_NAME`.
Any setting requires all four clients and the shared canonical Moderation issuer.
There is no report-specific signer or JWKS publisher. The shared key ring uses
`MODERATION_PRINCIPAL_SIGNING_KEYS_DIR` and `MODERATION_PRINCIPAL_ACTIVE_KID`,
with exactly two distinct RSA keys. Shared certificate-only mTLS JWKS uses
`MODERATION_PRINCIPAL_JWKS_LISTEN` (default `:8443`),
`MODERATION_PRINCIPAL_TLS_CERT_FILE`, `MODERATION_PRINCIPAL_TLS_KEY_FILE`,
`MODERATION_PRINCIPAL_CLIENT_CA_FILE`, and exact path `/internal/principals/jwks`.
Each owner's outbound JWKS client has its own mounted certificate/key and CA;
missing or partial identity fails startup. JWKS publication has no JWT or public
domain-readiness dependency, avoiding a reciprocal startup wait.
No settings disables the worker, preserving accepted PENDING work and ordinary
listener denial. There is no ordinary-client or automatic gRPC retry fallback.
`NATS_URL` is required for enabled worker startup; transient stream/publish errors
remain durable retries. Worker shutdown completes before its transports/pool close.
BE-046 owns the single VALID hook; without it validation remains manual, without
the legacy synchronous threshold engine. Numeric ownership is scan000005,
BE045000006, intake000007, report subjects000008, reserved shared boot000009,
BE046000010; duplicate numeric versions are invalid. The isolated report proof
applies the actual committed catalog; an unwritten reserved version is not a
tested migration or an implemented startup barrier.

The opt-in local/CI Compose overlay `docker-compose.report-targets.yml` extends
the existing Phase0 fixture without replacing Gateway signing identity. Generate
its separate report-target fixture first. This is a local compatibility template,
not authority to deploy or claim DEV/STAGING. Production mounts separate trust,
signing keys and replay state through its normal deployment secret mechanism.

The four-owner runtime test compiles each actual owner grpcsvc test binary inside
Linux, starts bounded subprocess fixtures with real owner handlers/stores and
mTLS/JWKS/replay, then drives signed Moderation client validation and real NATS.
The User subprocess's current Auth deletion checker is an explicit false fixture;
current Auth protocol and production deployment are separate evidence. Owner SQL
fixtures use isolated minimal schemas, not a claim of full owner migration proof.

Reproduce the connected proof inside the repository Linux Docker runner with
ash scripts/ci/report-target-intake-proof.sh; set disposable
REPORT_INTAKE_TEST_DSN and REPORT_INTAKE_TEST_NATS_URL. It resets the supplied
Moderation public schema and owns the two stream names; never point it at a stand.
It generates ephemeral PKI and Linux owner test binaries, then removes them.
The opt-in Compose template requires both PHASE0_FIXTURE_DIR from the existing
Phase0 generator and REPORT_TARGETS_FIXTURE_DIR from
scripts/dev/generate-report-target-fixture.py <new-directory>. Layer order:
base, phase0, report-targets. No host ports are added by this overlay.

Full production activation remains gated on BE046 sole-hook integration and the
verified terminal-account late-callback fence: an owner snapshot predating purge
must not attach a newly erased account afterward. The partial raw-account link
purge participant alone is not proof of that fence. No deployment/acceptance is
claimed by the local proof.

## Legacy privacy recovery and restore boundary

Accepted BE-137 c18/c19 and c23/c24 separate recovery provenance from submission
authority. A dedicated Moderation-only User recovery RPC may recover a retained
historical owner, including a soft-deleted profile. Its result is
`OWNER_RECOVERED_LEGACY`, never `VERIFIED_AT_SUBMISSION`: it does not activate an
old report, enqueue events or make a threshold vote eligible. Missing history is
`INCOMPLETE`; absence, an elapsed guessed deadline or an Auth lookup error is not
an irreversible erasure fact. The ordinary raw-header owner resolver is unchanged.

Report recovery mutations and their opaque journal entries share one SQL
transaction. Entries use an existing report UUID and slot, an opaque Auth subject
or terminal report-redaction disposition and opaque evidence references. They
contain no profile/account identifiers, identifier hashes or report contents.
Terminal dispositions dominate older bindings and survive raw report deletion.

Ordinary `LIVE_CURRENT` startup verifies the current local storage and a finite
local journal view; no external sink or export acknowledgment is required.
`DECLARED_RESTORE` requires a protected offline bundle with exact store lineage,
artifacts and independently preserved complete history through the selected final
cut. A final source capture follows writer fencing and drain; alternatively an
actually retained and verified native WAL/PITR history establishes that cut.
A periodic dump, matching old stores or an artifact seal alone cannot establish
a missing post-backup tail. Export and import produce recovery evidence, never
readiness; the shared coordinator verifies the full required cut vector before
opening its process-bound gate. Unresolved legacy rows keep the read-only privacy
checker closed until authoritative recovery or exact proven redaction completes.
