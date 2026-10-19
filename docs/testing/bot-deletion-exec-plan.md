# ExecPlan: BE-170 durable Bot deletion

## Purpose

Finish the existing DeleteBot task: revoke live access immediately, retain every
unfinished cleanup input durably, and return success only when all required
installation effects have acknowledged completion. Preserve sent messages.

## Context

The original docs/todo/backend.md item identifies DeleteBot's disabled-only
implementation retaining installations/whitelist. Product behavior comes from
docs/features/bots.md removal and docs/microservices/bot-service.md install /
uninstall; docs/microservices/role-service.md defines protected Bot authority.
Accepted BE-170 comments c3/c4 by251 specify global all-installation cleanup,
durable owner-authorized operation and target-owned fencing. Parent clarified
that full existing criteria must be implemented before merge. Native install
producer is in bot-native-install-intent-exec-plan.md;186 owns its Space receiver.

## Scope

Bot owns atomic owner verification, snapshot/revoke, sorted Space guard before
Bot row ordering, operation/effect persistence, retries and startup recovery.
Target services own their protected cleanup authorization, idempotent receipts
and rejection of late effects. No human default Role, arbitrary actor privilege,
new public cross-generation precondition, deployment or acceptance is implied.
251's BE-170 c8-c13 freeze the global-delete participant assertions and exact
receipt contract.44 owns Role storage/handler migration25;183 owns its actual
runtime/registration.186 owns Space native and qualified legacy receivers.
Bot/Gateway/Messaging integration remains under247's original task custody.

## Milestones

- [x] Real PostgreSQL RED: DeleteBot must not report completion while required
      remote cleanup is unavailable, including an installation still PREPARING.
- [x] Persist owner-bound global operation and all installation/whitelist
      snapshots in the same transaction as terminal revoke and mutable cleanup.
- [x] Serialize against install/whitelist enable through sorted complete Space
      set discovery, Space guards, Bot row lock and set revalidation/restart.
- [ ] Persist per-target exact request/key and verified immutable completion
      receipts. Missing authority/configuration retains nonterminal work.
- [ ] Consume real protected Space/Role/Messaging adapters and monotonic fences.
- [ ] Recover pending work on startup/periodically, without initiating token;
      stale worker claims cannot overwrite newer progress.
- [ ] Real lost response/restart/failure/concurrent deletion and late installation
      positive/negative proof, independent review and accepted121 integration.

## Detailed steps and validation

1. Test public DeleteBot against real PostgreSQL before changes. Assert a missing
   participant acknowledgement cannot return empty success; inspect committed
   operation, all snapshots, revoked token/whitelist and unchanged unrelated Bot.
2. Migration009 adds operation and effect authority with guarded DOWN. Every
   target snapshots Space, actor, actual installation ID, add/cancel operation
   and Bot generation plus disabled/current whitelist chats. Include whitelist
   without an installation, and admitted PREPARING/CANCELING operations.
3. Discovery reads all involved Space IDs; acquire their existing71431 guards
   sorted, then lock Bot and re-read. A changed set rolls back and restarts. Never
   acquire another Space guard after Bot. All writers reject terminal Bot state.
4. Preserve successful receipts across later failures; retries use stable saved
   requests, fresh delivery JWTs and bounded deadlines. Finish only after every
   target has completed the accepted effects. No nil adapter is skipped success.
5. Linux PostgreSQL interleavings cover both delete/install orders, duplicate
   deletion, whitelist enable, cancel/reinstall, freeze/purge and exact replay.
   Verify two Bots/Spaces remain isolated and messages are preserved. Guard
   migration DOWN with an observed two-connection first-writer test.
6. Add source-backed adapter tests once251/current owners freeze the actual API.
   Controlled fakes only prove Bot persistence; actual target receivers must
   prove late-effect rejection. Required affected tests/race/vet and wire gates
   precede review/merge.121 owns hosted CI and publishing.

## Progress and decisions

BE170c7 was an intermediate bounded producer disposition, superseded by explicit
parent clarification to finish full original criteria. Retained recovery rows
are not live installation authority: terminal Bot state and local whitelist
revocation deny use immediately, while immutable cleanup inputs survive failure.
Public DeleteBotRequest remains bot_id and response remains empty. Before full
completion return retryable Unavailable; authorized retry joins the same operation.

Atomic snapshot/revoke, sorted complete Space-set restart, stable root replay,
worker claim generation, immutable request sealing and receipt CAS have real
PostgreSQL/race component proof. This does not prove remote completion: only
the future typed participant adapter may validate a receipt before persistence.
Both DOWN8 and DOWN9 reject old Repeatable Read snapshots before locking; the
actual old implementation erased retained rows in the discriminating RED.
Default READ COMMITTED empty rollback and first-writer preservation are separate
controls. No retained journal is compacted or removed by successful cleanup.

The new dedicated Gateway mTLS listener verifies the exact delegated user,
request hash/JTI and a fresh protected Auth floor read before the Bot handler.
Streams are denied. Raw and mixed identity headers, stale epochs, replay and
foreign owners are negative cases. Actual Go-to-Java mTLS reads cover current9,
raised20 and unavailable0 test sources. Actual Auth outage before Bot admission
leaves the Bot live and creates no decision. Gateway's three producer routes
now use the existing protected audit transport. The strict HTTP selector rejects
static development credentials for all three routes; real JWT/JWKS and Redis
floor component tests also cover protected-only dispatch. Verified DeleteBot
admission is now included in the immutable decision: account/profile, epoch,
account type, exact method/request ID/hash and deterministic request bytes, with
no bearer token. Expiry is checked again after acquiring mutation locks. A real
mTLS-to-PostgreSQL test asserts the saved binding; its old-handler control fails
because the admission is absent. The explicit cross-process fixture now proves
real HTTP Gateway to mTLS Bot to real Java Auth to PostgreSQL for the empty
installation-set DeleteBot path. Current9 completes; raised20 and unavailable0
leave the Bot live with no root decision. Replay/floor Redis is explicitly
miniredis in this fixture. See `bot-gateway-lifecycle-interop.md` for reproducible
commands and boundaries. DeleteBot now requires the exact verified delegated
principal at its handler boundary, before owner reads or mutation; an ordinary
raw-identity call cannot create a decision. This cut follows the actual working
HTTP positive path and outage/stale controls. The separate install/uninstall
cut and full remote cleanup remain OPEN. These component results do not qualify
BE-170.

The Role deletion adapter validates the frozen root admission and snapshot before
building the private typed request, preserves native/legacy/orphan evidence as
separate arms, seals exact request bytes under the worker claim, and validates
every receipt binding before its durable CAS. Unknown fields, wrong actor or
request/evidence hashes, incomplete outcomes and invalid counts fail closed.
Its focused tests cover these codec boundaries only. The periodic worker and
actual Role/Messaging/Space participant composition remain unfinished; this
adapter alone cannot complete a deletion operation.

Messaging35 now has real PostgreSQL component evidence for exact sender/chat
pin removal, atomic decision/revision/receipt rollback, concurrent exact replay,
changed authority rejection, stale pre-cleanup Pin fencing without an outbox
event, and fresh authorized human repin preservation. The causal old-writer RED
actually committed pin=1/outbox=1 after cleanup. A digest-consistent corrupted
cached receipt with PENDING outcome also produced a real RED; replay now checks
all semantic receipt fields, not only its digest. Official full migrations UP35,
retained DOWN rejection with an actual first cleanup writer, old RR rejection
and empty default-RC DOWN are separate controls. This store component is not the
protected receiver or authoritative Chat-scope check; both remain required.

BE170c29/c30 adds a bounded Bot-only Chat scope observation using the existing
Chat-owned permanent head. Exact file custody is recorded in c31/c32 before
writes. No Chat DDL or stopped migration operation is involved. Bot whitelist
admission and Messaging cleanup must consume the actual current/retired source
with saved original evidence; missing Chat is not successful cleanup. This
observation is not a cross-service lease and introduces no public reparent API.

## Risks and follow-ups

Unknown legacy unqualified in-flight adds require explicit release reconciliation,
not a fabricated fence receipt. Role cleanup must preserve Owner/system roles.
Current service contract scopes unpin to saved current whitelist chats; historical
all-pins coverage cannot be invented from absent data. Do not compact unfinished
payloads or invent retention expiry. Terminal Space purge must not silently erase
global deletion inputs or substitute for downstream completion receipts.
