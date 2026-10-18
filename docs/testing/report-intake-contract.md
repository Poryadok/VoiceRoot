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
`CreateReportResponse.report.id` only. Other fields are unset. No lookup, viewer
ACL, account mapping, automod or publication occurs in that wait chain. Receipt
ID construction never rereads a worker-mutated row. Owner failure does not alter
otherwise healthy intake. Store failure returns generic unavailability, never
success without commit. Input JSON/description/session/self errors remain explicit.
There is no reporter status endpoint or WS/push outcome callback. A receipt is
not a capability to read evidence or decide a complaint.

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
bounded publication attempt locks its report then event; a committed purge wins
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
