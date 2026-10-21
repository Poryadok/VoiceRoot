# ExecPlan: composite Space Chat node and Gateway result

## Purpose

Make a newly created Space Chat return success only after its durable Chat
operation and corresponding Space tree node have completed. Lost responses and
worker restart reuse the original identifiers and immutable result rather than
creating another Chat/node or duplicate audit event.

## Context

- TT project voice, task BE-A2-COMPOSITE-CHAT-NODE, actor Рудеус-18.
- Accepted decision: BE-131 comments c3/c4, BE131-COMPOSITE-v1. This plan does not
  reopen those semantics. BE-131 c8/c9 resolve initiating trust and BE-A2 c4/c5 resolve purge minimization; use verified original decision data, never raw headers.
- Canon: docs/microservices/api-gateway.md A2 Space REST target;
  docs/microservices/space-service.md audit/lifecycle; chat-service.md producer
  contract; DATA_MODEL.md and DATA_STORES.md forbid cross-service SQL/FKs.
- Current Gateway transcode_spaces_tree.go makes an unjournaled CreateChat then
  ordinary UpsertTreeNode. Current Space store/tree.go ordinarily records its own
  tree audit. Chat's existing keyed create snapshot predates the new composite.
- Current local base 698d4f064f032ea74abf3ede2017100b26c8ee69; GitHub transport is
  unavailable. Read remote state before any publish retry when it recovers.

## Scope

- Ant18: additive Space CommitSpaceChatCreation/GetSpaceChatCreationOutcome
  protocol/fixtures, Space atomic node/outcome store and protected handler,
  Gateway transcode_spaces_tree.go original-result consumer and negative tests.
- Ant44/BE-131: Role outboxes and Chat prepared operation, HELD audit outbox,
  autonomous commit/outcome recovery, activation/finalization and compensation.
- Ant53/BE-008: Gateway audit-read vertical, dedicated audit files and serveSpaces
  audit hook. No Space proto additions; do not concurrently edit his hook.
- This is one consumed deliverable with the Chat worker; do not activate an
  incomplete facade or claim a proto foundation as completion. No deployment,
  acceptance appointment, cross-service DB write or ordinary audit suppression.

## Agreed wire core

Consumer44 confirmed: protocol_version=1; operation_id, creator_profile_id,
space_id, chat_id, node_id, audit_event_id are canonical nonzero UUIDs;
chat_type is group/channel; create_request_sha256 is exactly 32 bytes and binds
the versioned normalized Chat create fields. Original occurred_at and details
remain immutable Chat outbox fields and need not be duplicated here.

Original trusted account/bot context is additive under the BE-131 c5 security
decision. It is persisted decision data from Chat's verified initiating context,
never a stored user bearer or raw header. Do not finalize that schema before the
specific decision, or claim the core-only proposal as a complete security API.

Space outcome is immutable COMMITTED plus its original typed SpaceTreeNode
snapshot, or ABORTED plus a fixed safe status. It repeats the exact binding.
Missing/outage is UNKNOWN and never authorizes compensation. Authenticate fresh
service:chat commits; bind authenticated outcome reads to the operation and caller
role. Gateway disclosure retains its verified caller/ACL requirements. Avoid a
Chat-to-Space proto import cycle.

## Milestones

- [x] Reconcile task ownership; split from BE-008 with actual owners53/44.
- [x] Confirm core binding and named consumer44.
- [x] Acquire own WT42 lease0d43c7beaac55dd090d9a1022f054669 branch codex/be-a2-composite-chat-node base698d4f0.
- [ ] Adopt complete accepted protocol/security decision in owning canon.
- [ ] Add tests demonstrating tuple conflicts, atomic outcome and retry behavior.
- [ ] Implement Space node/outcome and consumed Chat/Gateway wiring.
- [ ] Obtain independent security/concurrency review, required CI, normal merge.

## Detailed steps

1. Acquire treehouse lease only after a real slot is available. Read local AGENTS,
   status, branch/base; create codex/be-a2-composite-chat-node. Preserve other leases.
2. Read existing Space transaction/lifecycle/Role authorization and protected
   principal seams. Read full BE-131 c3/c4/c5 disposition and BE-130 replay/JSON
   contracts. Pin any context fields from the accepted caller decision.
3. Add additive proto definitions without a Chat/Space import cycle. Allocate
   unused field tags; generate canonical Go/Dart stubs by repository tooling.
   Share exact schema/fixtures with44 before his consumer implementation.
4. Test first: changed actor/Space/chat/type/node/audit/digest conflicts; wrong
   issuer/audience/RPC/body/request ID and unknown fields denied; unavailable
   authority has no terminal outcome; ordinary Upsert cannot reference HELD Chat.
5. Implement service-owned outcome storage under existing Space lifecycle lock
   and uniqueness constraints. Commit node plus COMMITTED snapshot atomically;
   deterministic domain refusal plus ABORTED atomically; neither can overwrite
   the other. Dedicated internal operation has no duplicate tree audit. Ordinary
   Upsert retains its existing audit. No lock spans network I/O.
6. Exercise concurrent pools, rollback/lost-ACK injection, exact replay after
   node move/delete, and commit-versus-abort. Return original result; never
   reinsert a subsequently modified/deleted node. Durable result fences have no
   arbitrary TTL. Integrate terminal Space purge/minimization deliberately.
7. With44, test crash after Chat prepare, after Space commit before ACK and before
   Chat finalize: worker without Gateway reaches COMPLETE or authenticated abort
   compensation, never fresh IDs/early event/early audit/partial usable Chat.
8. Gateway waits for Chat COMPLETE, reads authenticated original Space outcome
   and removes its ordinary second Upsert. Preserve client request_id and existing
   response wrapper. Test generic/direct/Space routes cannot bypass composite.
9. Run scoped checks and independent review; record actual executed counts and
   source identity. Network recovery requires remote ref readback before push.
   Carry PR through exact-head required CI and merge commit or explicit ant handoff.

## Validation

- Focused pure protocol/store-boundary tests first; record meaningful red/green.
- Space PostgreSQL migration/rollback/receipt/concurrency tests using real server.
- Gateway route/negative transport tests plus race when supported.
- Chat consumer crash/replay/HELD visibility/outbox fixtures with author44.
- buf lint/format/breaking plus generated Go/Dart drift checks for affected proto.
- Independent security/concurrency review of implementation and tests together.
- Required exact-head CI before merge; merge is not runtime/final acceptance.

## Progress

No source changes, branch, lease or tests yet. Actual treehouse get with --lease
--no-fetch failed because all40 trees are busy/dirty. No foreign lease touched.
Core schema confirmation is task evidence, not implementation or test proof.

## Decisions

- A distinct TT slice avoids concurrent ownership of BE-008's audit work while
  keeping the accepted composite tied to its real Chat consumer.
- Reuse the accepted composite/JSON/replay decisions; request no second product
  permission. Only the separately named initiating-actor cutover remains unresolved.
- Source/CI/merge custody remains ant18 or an explicit real-ant successor, never
  the queen. Pending network/resource work is dormant custody, not idle polling.

## Risks and follow-ups

- No reachable local Docker daemon was observed by other workers; required real
  PostgreSQL proof must run in an authorized supported environment, not be skipped
  and reported as PASS.
- Initiating actor cutover and terminal Space purge coordination are correctness
  prerequisites for the completed corresponding vertical, not disabled foundation
  exemptions. Preserve explicit gaps until implementation and evidence exist.
- Wake: released actual worktree lease; accepted BE-131 c5 security disposition;
  GitHub recovery for later delivery. No automatic wake mechanism is configured.

## Current checkpoint (2026-10-18)

Earlier resource/c5 blockers above are historical and resolved. BE131 c8/c9 initiating authority accepted. BE-A2 c4/c5 BE131-COMPOSITE-PURGE-v1 accepted: separate permanent nonpersonal operation/resource and Space purge fences from full evidence. Space deadline aggregate PURGED+30d; Chat deadline own successful purge+30d. Expired private reads fail COMPOSITE_RESULT_EXPIRED with no body; no fabricated abort/success. Chat freezes and drains prepared/HELD before manifest seal; actual audit/chat.created delivery resolves before purge. Frozen Space ingress accepts only original committed composite fact under shared Space tx lock; terminal purge closes it permanently.

Own WT42 proto/generated Go+Dart and binding/service receiver implemented. Meaningful RED11/GREEN11 binding; RED1/GREEN12 credential tests; actual mTLS/HTTPS-JWKS/Redis receiver tests GREEN13. Receiver runtime not production wired yet. Store/migration000028 in progress;4 real PG tests added but run fails before assertions because rootless Docker unsupported on Windows. No DB PASS, completion, deployment or acceptance claim. Consumer44 owns Chat and Bot authority/outbound signer;13 owns ingress, consumes our exact persisted-binding helper. CI/merge custody18.

### Consumed vertical checkpoint

Local protocol86f48e6, sharedcodecdb239b016, atomic store/purge6678fee; principal87fef6621, Gateway3207f158c and Bot/Chatprotoe867285fa consumed from44. Receiver, public outcome scope, Bot currentinstallation and fresh protected Role decision client now wired. Gateway SpacePOST uses protected Chat completion and original Space outcome, never second Upsert. Real Chat JWKS mTLS incompatibility gave meaningfulRED1 thenGREEN22; legacyGateway RED200vs401 thenGREEN8 focused. Broader Space394short and Gateway740short PASS; Space/Gatewayvet PASS. Later targeted76handler/grpc and6accounttype tests PASS;9Botdecision tests PASS. Actual PG tests remain environmentblocked and are not proven by short checks. Independent review and assembled Chat producer/recovery integration are still open; source is not release ready.

### R1 correction and develop handoff (2026-10-18)

Independent reviewer136 found that the composite audit-ID fence and global
receipt registry could both reserve the same UUID. Real PostgreSQL16-bookworm
Go/Testcontainers reproduced all three failures before the correction: existing
local receipt, existing Role receipt, and local takeover after composite commit.
Migration000031 now supplies the common audit-ID lock and trigger fence check;
Commit and both ingestion paths acquire it after Space and before reservation
rows. Original COMMITTED Chat delivery remains accepted and exact-replayable.
Receipt minimization moves to the end of the cleanup transaction and its UPDATE
guard acquires no new advisory lock. Space31 released by69 is used;32 is unused.

Scoped endpoint env is DOCKER_HOST=npipe:////./pipe/dockerDesktopLinuxEngine,
TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1, TESTCONTAINERS_RYUK_DISABLED=true.
First real original composite test PASS; correction run8PASS; expanded real
composite/cross-Space collision/uncommitted-local/P3 run12PASS including3subtests.
Space447short/17packages and vet PASS. These supersede the historical local
Docker setup blocker above, without replacing hosted CI or consumer proof.

User directive: finish current work and integrate into develop. Ant121 is the
sole develop writer. Ant18 retains task/check custody and sends immutable final
head/dependency order; no develop checkout, deployment or acceptance appointment.
Chat prepared/HELD/recovery, generic/direct/bot initiating cutover and participant
drain proof remain author44's explicit integration gap. Source review R1 delta,
hosted exact-head CI and assembled producer evidence remain required gates.

Independent121 migration follow-up found that origin=chat, equal Space and
COMMITTED do not prove the original fact. Actual PostgreSQL RED reproduced
wrong Chat, wrong creator and retained receipt without audit payload: all three
incorrectly passed the old preflight. The successor refuses every pre-existing
receipt/composite overlap as ambiguous, preserves both records and exposes no
bypass. This conservatively also blocks apparently matching historical overlaps;
they require explicit verified reconciliation, not implicit approval from hashes.
Current source has not been activated, so no historical overlap is assumed absent
without inspection and no production repair or data deletion is authorized here.

### PR875 explicit actor correction (2026-10-21)

Existing author18 accepted the named correction in TTc10/checkpoint before own
WT38 leasec59dea450237e8d0888d01527b4defbd. Base is exact
4ea5dbc551aa0abf890d0c745bb8d341e8622808; source review is
PR875-INDEPENDENT-CODE-REVIEW-ASTRA-20261021 c4/c6 by480. Both initiating
signers used compatibility AccountType, which defaults absent signed claims to
regular; the actual Space Chat POST also missed the strict delegation boundary.

High-risk JWT boundary, ordinary test-first workflow: real signed JWT/JWKS parser
must exercise missing/empty/trimmed raw type, explicit regular/guest and existing
sdk-account audit-only allowance. HTTP dispatch negatives cover missing JTI,
wrong/unconfigured issuer/audience, query/duplicate bearer, static credentials and
compatibility epoch mode. Reuse existing strict delegation and preserve ordinary
JWT compatibility. Only current production caller POST spaces/{id}/chats joins
the strict boundary; no producer/caller cutover is invented for the separate c3.

Scope: three Gateway production files, parser-backed regression tests and this
checkpoint. Checks: focused meaningful RED/GREEN, affected Gateway suite/vet/lint
and independent480 correction review through publisher121. User excludes CodeGraph
and Graphify until restored. No Chat producer/config/authority expansion, runtime,
CI, GitHub or deployment actions by18;121 is sole publisher.

Parser-backed test-first RED observed8PASS/15FAIL before the three production
edits. Initial host focusedGREEN34PASS is historical only: upon receipt of the
Linux/Docker-only restriction the pending host command had already completed;
no matching Go/test process remained and no further host tests were started.
Authorized final proof used golang1.26.2-bookworm Linux containers, readonly WT,
own Go caches, no published ports: full Gateway short suitePASS, focused34PASS
(5top-level plus29subtests,0skip/0fail), vetPASS. Linux CI-pinned lintv2.6.1 and
independent security correction review remain pending at source freeze. Actual
JWT/JWKS and Redis fixtures are local to the containers, not Windows listeners.
