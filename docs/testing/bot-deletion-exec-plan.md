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

Uninstall continuation: its existing required Role/Messaging cleanup uses a
separate per-installation `space_uninstall` decision and frozen target journal.
Migration012 is reserved by247 after all-ref and TT rev7873 collision checks.
Reuse the typed codecs and bounded claim recovery without issuing a global Bot
terminal fence. Native Empty cancellation is a local transport observation only.
Keep unfinished targets durable until their own receipts exist; no ordinary Role
or Messaging helper fallback. The initial4.689s response-code oracle is invalid:
the explicit all-participant Empty rule belongs to DeleteBot. The corrected
4.892s PG oracle instead fails on two forbidden ordinary Role calls. Public
Uninstall response semantics are not strengthened by that test.

Implementation steps: atomically bind verified initiating request, current
manage-bots decision and frozen source to cancellation; independently seal and
validate participant receipts; reclaim pending work after service recreation;
atomically commit the native ACK with local CANCELED retirement, preserving
accepted Empty semantics and the independently pending typed obligations.
Run actual PG failure/retry/duplicate/isolation checks with explicitly controlled
Role/Space fixtures before real protected participant composition. Retained
uninstall inputs remain discoverable to global deletion. No compactor or late-P3
proof provider is introduced.

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

Independent review82 found the first participant journal constrained only the
actor-to-Bot direction. A known Bot could change actor through a new decision.
Sequential real-PG RED and a separate concurrent first-bind RED through different
CHAT guards exposed this. The corrected immutable mapping is unique in both
directions; the losing transaction conflicts and rolls back decisions, cleanup
revisions, pins and receipts. Distinct legitimate Bot/actor pairs remain valid.

BE170c29/c30 adds a bounded Bot-only Chat scope observation using the existing
Chat-owned permanent head. Exact file custody is recorded in c31/c32 before
writes. No Chat DDL or stopped migration operation is involved. Bot whitelist
admission and Messaging cleanup must consume the actual current/retired source
with saved original evidence; missing Chat is not successful cleanup. This
observation is not a cross-service lease and introduces no public reparent API.

## Risks and follow-ups

Bot now persists the authenticated Chat request/response bytes and digests in
the same transaction that creates an installation intent. Exact native retry
uses that original accepted observation, rather than requiring a new Chat read
or replacing later whitelist changes. New targets and reinstall generations
require fresh observations. A real-PG old-source relabel RED guards that boundary.
Whitelist rows retain original and current observations separately, bound to
the actual installation ID; overlapping setter rows survive replacement. The
global deletion snapshot freezes both observations before deleting local rows.
Real missing-origin and missing-snapshot REDs distinguish these guarantees.
Migration10 has first-writer DOWN, old-RR and empty default-RC controls.

Review82 found that a PREPARING installation has accepted source bytes but no
whitelist row yet. The root snapshot now also freezes intent-owned original
observations for pending and removed-whitelist targets; absent current whitelist
evidence remains absent. A causal PG RED and mixed pending/installed-then-removed
control assert exact source bytes/hashes and independence from later source rows.

This component does not yet qualify public install/whitelist/enable initiation
or cross-process Chat consumption. The existing whitelist owner path and enable
permission path still need their verified Gateway/Bot initiation cut after an
actual positive HTTP proof. Messaging must consume preserved original lineage
for covering receipt adoption; the private receiver and worker remain open.

Unknown legacy unqualified in-flight adds require explicit release reconciliation,
not a fabricated fence receipt. Role cleanup must preserve Owner/system roles.
Current service contract scopes unpin to saved current whitelist chats; historical
all-pins coverage cannot be invented from absent data. Do not compact unfinished
payloads or invent retention expiry. Terminal Space purge must not silently erase
global deletion inputs or substitute for downstream completion receipts.

### Attachment lifetimes and batch cleanup (c49/c50)

Allocate a durable local attachment UUID before uncertain native Add; preserve it for exact retry, overlap, disable and requalification, and rotate on remove/readd. Freeze current rows plus admitted/pending obligations by attachment identity, permitting two same-chat targets. Additive Bot migration011 retains original source IDs or explicit unqualified origin and actual installation association; it cannot prove historical Space causation. Messaging35 then resolves receiver-owned coverage for the whole per-chat batch and performs at most one uncovered effect in one transaction. Critical PostgreSQL oracles cover pending/no-whitelist lifetime, stable overlap, remove/readd rotation, distinct frozen original/current obligations, legacy capture, concurrent first capture and safe DOWN. This implementation remains open; component365 public admission is separately frozen.

The Bot metadata implementation has actual PG/race proof: causal missing-lifetime
and collapsed-snapshot RED 9.618s, initial four-case GREEN 17.010s, and migration
controls GREEN 18.989s. The final affected matrix passed in 47.471s, including the
old lifecycle writers on schema011 and predecessor DOWN10 empty-RC/old-RR
controls. DOWN11 waits for a real first writer and refuses its retained commit;
an old Repeatable Read snapshot is rejected before destructive locks. Earlier
broader regression failures from stale test migration lists are preserved and
their two lifecycle cases passed after updating the fixture (7.674s). These
results qualify local metadata and frozen snapshots only; Messaging batch
coverage, protected cleanup delivery and the recovery worker remain unfinished.

Purge-before-root is a separate uncovered original criterion (BE-170 c57/c58).
Bot must capture exactly eligible retirement inputs before erasing Space-owned
rows; historical attachments alone must not manufacture targets. Its own purge
receipt does not prove Role or Messaging cleanup. Messaging's existing full
purge evidence expires after its own 30-day window, and no qualified late compact
proof provider currently exists. Missing evidence cannot become zero-target
completion. No new retention clock, perpetual actor history or payload compactor
is implemented by this metadata slice.

The next Messaging batch-store slice has causal PG/race RED 3.639s: a new
global child after completed uninstall removed a later human repin. Receiver-owned
first-effect coverage now preserves that pin with count0/revision0, including
without a caller hint. A mixed batch executes one DELETE/revision and retains
direct origin links independently for old/new targets; concurrent global and
uninstall have one effect. Four new component cases passed in 15.772s and the
whole affected Bot-pin group in 43.845s. Current Chat source facts in these tests
are controlled PG rows read through production SQL, not actual protected Chat
transport. Private Messaging ingress, production datasource wiring, producer
delivery/recovery, purge proof coverage and full BE-170 remain open.

Independent review of batch slice404 found that attachment descriptors retained
caller-owned protobuf pointers despite the frozen full body. A deterministic
actual-PG source-loader barrier reproduced repair of the authenticated stale
incarnation and a nil-response panic after the wait (RED 7.785s). Binding now
decodes its own canonical body before building attachment descriptors. Both
mutations leave the original stale source rejected with zero effects and no
panic (PG/race GREEN 9.316s). This correction does not qualify private ingress or
the unfinished producer/worker/Purge work.

BE-170 c64 clarifies that an explicitly supplied current observation must name
the receiver-owned first effect's effective incarnation (saved current source,
otherwise qualified original source). Projection identity remains unchanged;
different incarnation is a coverage conflict, never an uncovered fallback.
Actual PostgreSQL tests on immutable1547 plus one test overlay reproduced both
covered-only and mixed-batch acceptance (RED 8.052s). The correction rejects both
with no pin, revision, receipt or decision effects (GREEN 8.369s), while a new
read operation/revision for the same incarnation and historical adoption without
current observation still succeed without a live datasource. This isolated
correction excludes parallel private-receiver work and does not resolve Purge or
the post-30-day proof gap.

The private Messaging receiver now has an actual transport proof against official
Messaging35 and Chat25 UP migrations, real Redis, inbound mTLS and protected
HTTPS JWKS. A readonly Chat LOGIN is reopened and verified independently before
current-source queries. Ordinary generated Messaging returned UNIMPLEMENTED in
the causal receiver RED (12.976s); the dedicated production capability/handler
passed the first positive (15.967s), expanded transport matrix (17.977s), and
readonly/current-source controls (14.993s). A request held behind the actual CHAT
advisory lock expires without effects; the same immutable body with fresh JWT
recovers and exactly replays. Actual Chat DELETE with retained native head is
qualified explicitly, rather than treating NotFound as completion (15.017s).
Store regression controls passed in 19.310s. The final production receiver with
the c64 coverage guard passed its coherent transport run in 14.581s, with scoped
vet/build successful. An initial main build lacked the
standard errors import; it was corrected and scoped vet/build passed.
These signed decisions are controlled Bot assertions, not a proof of an actual
Bot root journal or public owner admission. Producer delivery/recovery, global
Space/Role retirement coverage and post-30-day provider gaps remain unfinished.

The Bot Messaging consumer now constructs one attachment batch from the frozen
root only, verifies retained Chat request/response hashes and actual installation
association, and seals deterministic child bytes before issuing a fresh service
credential. Its validator binds every response target/digest/revision/covering
tuple, participant, request and evidence hash before the durable receipt CAS.
The separate mTLS client has five all-or-none BOT_MESSAGING_CLEANUP settings and
never substitutes the ordinary Messaging client.

An actual cross-process source fixture passed with official Bot001-011,
Messaging35 and Chat25, real Redis and protected mTLS/HTTPS-JWKS. The participant
committed before an intentionally lost ACK; Bot retained PENDING plus exact
request bytes. After a human repin, a restarted service rejected a corrupted
target digest, then accepted the original real receipt under a fresh JTI without
repeating effects (caller 5.756s, provider 27.106s). Role and Space effects stayed
pending, so full root completion was explicitly refused. Owner admission and
the Chat point-in-time observation are controlled fixture inputs here; earlier
public Gateway admission and Chat transport proofs retain their separate scope.
This does not complete the production recovery worker, uninstall batching,
global Space retirement, P3 coverage or late proof-provider criteria.

The production recovery worker now atomically claims a due root with SKIP LOCKED,
retains a monotone 30-second claim generation and runs a bounded 25-second pass.
Failed work releases only its still-live generation into a two-second retry wait;
it never clears frozen requests/receipts or a successor's claim. Stored effect
request and receipt bytes are checked against their retained SHA before recovery,
and completed Role/Messaging outcomes are revalidated as typed receipts before
being skipped. Actual PG corruption of either byte column reproduced acceptance
on e200 (RED 8.580s); the final digest/claim/stale-release group passed in 14.530s.

The actual producer/participant handoff was then executed through
ProcessNextBotDeletion: after lost ACK and rejected bad receipt, the next due
claim replayed the exact Messaging child and saved its original receipt while
preserving a later human repin (caller 7.726s/provider 47.048s). Missing Role and
global Space cleanup retained RETRY_WAIT rather than whole-root completion.
Recovery reinstantiates BotGRPC/BotStore on the same live Pool; it is not an OS
process restart. Native402e Empty ACK cannot settle SPACE_CLEANUP: c69/c70 require
the separately implemented target terminal fence and typed completion result.

For c69/c70, 247 owns the additive Space wire and 186 owns its receiver/store,
global marker, every-Add guard and actual schema transition. The wire is seeded
from full canonical69/1a01, preserving SetEntryPolicy, member_admission and402e
Add/Remove. New native-only ApplyBotInstallationRemoval request uses tags1-9/13
(10-12 reserved); its typed receipt mirrors existing bindings through15 and
requires global_native_adds_fenced=true at16, with no unsupported count fields.
Old native Empty cancellation is not that receipt.

Buf compatibility against the full69 Space descriptor and scoped lint passed.
The remote Go plugin returned403; local Linux pinned protoc-gen-go1.36.5 and
grpc1.5.1, protoc3.21.12 and Dart plugin25.0.0 generated coherent mirrors.
Go descriptor/golden and Bot/Space builds passed. The first golden expectation
omitted the existing sha256: string prefix; only that test was corrected, with
its failed log retained. Independent Dart bytes match the same manually derived
vector; analyzer exits0 with27 generated deprecation infos, not zero diagnostics.
This qualifies schema/codecs only. Receiver terminal-fence/receipt persistence
and actual Bot producer/worker composition remain unfinished.

The Bot typed Space producer and worker dispatch now use that exact wire. They
rebuild the original native cancellation from the frozen owner root, seal the
outer child before issuing a fresh credential, and require the exact typed Space
receipt with matching root/request/full nested cancellation hashes and terminal
fence=true before saving completion. Historical completed Space children are
revalidated as well. Legacy/unqualified source and Empty/no-fence responses stay
unavailable; the protected mTLS constructor sets an explicit CleanupSpace alias
and never uses the ordinary Space client. Component/negative/constructor checks
and vet/build pass. Actual protected native46 receiver composition is not yet
executed, so this producer remains an unfinished runtime integration slice.

Uninstall now has separate migration012 journals, a persisted verified manager
decision and frozen attachment source in the cancellation transaction. A failed
snapshot rolls cancellation back before remote effects; concurrent admission
joins one installation operation. Existing claim fencing, exact request sealing,
typed Role/Messaging validation and startup recovery apply in this separate
namespace. Its native Empty result has an explicitly local observation envelope
with genuinely empty response bytes and cannot build or satisfy global Space
terminal authority. Ordinary helper calls have been removed from this path.
Controlled Role lost-ACK recovery rejects a changed-purpose response, retains
the original decision/request, completes the remaining cleanup, preserves another
Space and permits a new installation. This reinstantiates services on the same
Pool; it does not prove an OS restart or real Role/Space transport. Existing
uninstall semantic tests now use explicit native admission and typed controlled
participants, preserving whitelist/membership/message-body assertions. The first
expanded run failed four old raw/legacy fixtures; those failures remain saved.
Actual multi-service uninstall composition, native46 global completion, Purge and
post-retention proof remain separate unfinished acceptance work.

Intermediate b3 coherent affected PG/race run: store62.513s, grpc39.697s,
24 top-level/72 total test passes, one explicit pre-existing optional Messaging
handoff skip (no provider configured). It includes DOWN012 old-RR rejection,
empty default-RC rollback, observed first-writer lock wait/preserved row, and
the historical DOWN008 empty control after official reverse-order DOWN012.
Bot short checks:220 total test passes/231 explicit integration skips; scoped
vet and full Bot build pass. These are component boundaries, not multi-service
Uninstall acceptance or full BE-170 readiness.

The accepted native ACK boundary was then corrected before review readiness:
b3 still returned Unavailable for missing Role despite a durable native ACK.
The canonical ordinary-uninstall test reproduced that error in3.702s. The
successor atomically stores the explicitly local ACK, retires local rows and
marks the native intent CANCELED, returning Empty acceptance with a separate
RETRY_WAIT journal. Typed cleanup is never falsely marked complete. Pending
cleanup retains admission exclusion under the existing Space/Bot guards; its
exact frozen target participates in global discovery even after local source
retirement. The global snapshot copies the retained original/current Chat bytes,
not subsequently changed source rows or arbitrary historical installations.

Successor coherent PG/race proof: store79.239s/grpc52.435s,25 top-level /
73 total test passes and the same one explicit optional Messaging handoff skip.
The new local-retirement fault oracle rolls the ACK/retirement transaction back,
retains CANCELING and the sealed native request, then retries that exact request
and commits CANCELED plus accepted Empty with typed work still RETRY_WAIT.
Bot short:220 passes/232 explicit integration skips; scoped vet/full build pass.
The skips do not prove actual Role/Messaging/native transport composition,
public Gateway uninstall, OS restart, Purge or post-retention completion. Those
acceptance requirements remain open despite this bounded component proof.

Next actual lower native46 consumption uses separate immutable Space source
55b549b531cb9d8ea30423cf513a594ff830c3fa (serving harness atop9c11), built in247's
own private Linux runner.186 owns only that serving fixture and real database
postconditions.247 owns the Bot caller test: actual BotPG PREPARING before the
production protected Add, committed owner-root revoke, real worker typed global
removal, lost ACK, bad receipt and exact fresh-JWT recovery. The provider checks
its own terminal marker/receipt/canceled head and denies historical, higher
generation and new-Space Add. No borrowed Role44 DSN, live peer process, default
Role or fake completion JSON authority. Initiating owner facts remain a
controlled lower fixture; public Gateway/Auth/Role/Messaging/full task proof is
not inferred from this native-only composition.

This actual handoff completed: caller50.842s and provider39.445s, both terminal0
with no skips. The caller uses official Bot001-012 migrations and the production
Add/global-removal worker. A lost committed response and a corrupted fence flag
leave the Space child pending; exact-body/fresh-JWT replay persists the genuine
typed receipt. The Role child remains pending and the root RETRY_WAIT. The
provider independently verifies its terminal row, saved receipt, canceled head,
no membership and all three Add denials. Recovery reinstantiates service/store
over the SAME live Pool, not an OS-process restart. The integration client uses
manually configured equivalent mTLS; the production constructor was tested
separately. Scoped caller vet passes. Own temporary credentials and all test
PG/Redis/listener processes were removed after both processes completed.

The next bounded review includes the previously unreviewed seven-path producer
delta dcb51f4..ceb4925 at current blobs, its current overlap with independently
reviewed86bc Uninstall changes, this new caller and the source-owned55b provider
test branch. The complete executed archive proves inputs; it does not request a
whole-parent semantic audit. Actual protected ordinary Uninstall, full Role and
public composition, Purge and the post30day evidence gap remain open.

The separate ordinary-Uninstall Messaging lower handoff now also passes:
caller10.509s/provider45.783s, no skips. Actual protected Messaging receives
space_uninstall with exact native installation/attachment scope, commits once,
and loses its ACK. After a human repin, a corrupt receipt is rejected and the
worker recovers the saved exact receipt without deleting that later pin.
Bot012 retains the typed journal; the native intent is locally CANCELED, Bot
stays live, no global root exists, and unavailable Role keeps RETRY_WAIT.
Initiating manager facts and native Empty are explicitly controlled here;
this proves actual Messaging purpose-specific transport/recovery, not public
Uninstall or actual Space cancellation. Recovery uses SAME-Pool service reset
and manually equivalent mTLS client. Provider uses actual official Chat SQL
head/incarnation and readonly LOGIN, real Messaging migrations/PG/Redis/JWKS.
The first provider attempt failed before RPC because the runner flattened the
repository layout expected by its source-owned repoRoot helper; the saved
failure is setup evidence, not a product RED. The successful run uses the
canonical src/backend layout and identical production source.

Actual ordinary-Uninstall Role lower handoff also passes: caller8.815s and
source-owned eef1f09 Role25/Space44 provider124.599s, no skips, scoped vet0.
The production signed current manageBots decision succeeds before the lower
initiating facts are committed. The actual Role cleanup receiver commits an
exact native space_uninstall receipt; lost ACK and a corrupted global_delete
purpose leave the Bot child pending. Fresh-JWT/exact-byte worker replay then
completes Bot012, keeps Bot live/no global root and permits a new installation.
The native cancellation remains an explicit controlled Empty seam and owner
facts are not public Gateway/Auth proof. This fixture has no Bot-created Role
definitions/assignments, so the genuine count0 receipt proves protected
transport and journal recovery, not nonzero Role deletion effects. Those store
effects retain their separately reviewed proof. Provider serving source remains
qualified Space44; no Role current46 or complete public composition is inferred.
Recovery is SAME-Pool service reset/manual equivalent mTLS. Provider stop-file
teardown completed, removed credentials and left the private engine empty.

Minimal c57 retired-source uncertainty safeguard (separate from missing P3
proof/snapshot/retention adapters): real PG/race RED5.900s shows successful local
Space Purge erases an actual installation and a later owner DeleteBot falsely
starts COMPLETE from zero remaining sources. Bot013 adds only an ordinary
Bot-level unresolved bit, without Space/chat/actor history or a retention clock.
Purge marks actual eligible associations under Space->sorted Bot locks before
erasure, except obligations already owned by immutable global/pending-uninstall
snapshots. Owner revoke still commits; zero-target admission and final global
completion remain pending while that bit is set. Final completion locks the
actual Bot before its root, matching admission and excluding the first marker
write. DOWN requires RC and serializes against all Bot writers before refusing
to lose any true bit. No clear/proof/provider/compactor is invented. Retired
source snapshots, exact own downstream P3 coverage, unfinished-journal purge
handling and the post30day missing provider remain original OPEN work.

The strengthened unchanged-final-test oracle on exact43ed plus its sole test
overlay reproduces false COMPLETE in9.562s. Stable final production/test source
passes the affected PG/race matrix:22 top-level/32 total test passes,
store147.864s/grpc81.374s, with only two explicit optional ordinary Role/Messaging
handoff skips (provider environments absent; their earlier transport evidence
retains its own source scope). The direct revocation control12.251s proves live
token lookup before owner admission, disabled status/token denial afterward,
persisted immutable decision/root PENDING, refused completion and RETRY_WAIT
with identical bytes/SHA. Already-verified initiating facts are controlled here,
not public Gateway/Auth proof. Bot short220 passes/238 test integration skips
plus2 package skips; scoped vet/full build0. First4.832s missing-column failure
was the existing test helpers omitting new013; both migration lists now include
it, and the failure log is retained as fixture setup evidence.

The original183 fixture-only d793241 successor now closes the separate nonzero
Role target-selection seam. Actual caller6.657s/provider50.630s pass without
skips, scoped caller vet0. No provider SELFTEST runs. After real Bot Create and
PrepareInstallation, the exact persisted Bot/actor/Space/installation/add-op/
generation tuple configures the private fixture. The real Bot012 root/worker
then sends its signed cleanup; lost ACK, corrupt purpose and fresh-JWT exact
replay retain the original receipt counts1 created role/2 assignments. Only
after Bot validates/persists that receipt does the provider independently check
its own committed request/receipt, removed target rows, preserved human/system
Member/Owner controls and one immutable operation. JSON is setup/synchronization,
never cleanup authority. Bot stays live/no global root and replacement admission
remains possible. Source d793241 uses the same qualified Space44/a36 input;
native cancellation/initiating facts remain controlled, recovery SAME Pool and
manual equivalent mTLS. This closes the nonzero producer/receiver effect risk;
it does not qualify public/current46/Purge/post30day/full-BE170 composition.

The optional `BOT_GATEWAY_NATIVE_UNINSTALL=1` public fixture now characterizes
the existing HTTP DELETE route with a second actual native installation. The
primary install/enable/owner-only RPC/deleted-Chat retry controls remain intact.
The second installation has an empty whitelist (no Messaging-effect claim).
After its exact persisted installation configures the reviewed d793241 fixture,
the real client JWT passes Gateway, Bot mTLS/delegated verification, live Java
Auth floor and actual Role manage-bots decision. Stale epoch DELETE is denied;
current DELETE and its retry return204. Bot independently checks its immutable
ordinary journal COMPLETE, native cancellation/local CANCELED, live Bot and no
global root. The saved production Role receipt reports1 role/2 assignments;
the source-owned provider independently verifies target removal and preserved
human/system/Owner controls. Fixture JSON only configures and synchronizes.

This is test-only characterization of existing production behavior, so the
first real execution may pass without an invented product RED. Role25/Space44
and the independently built a36/8cd/d793/abd dependencies are historical scoped
fixtures; this does not qualify current46/all-writer activation. Gateway/Bot
replay controls use miniredis, while the provider fixtures use real Redis.
Connections are manually configured equivalent mTLS, not production main
startup. Full public DeleteBot, Purge/P3/retention and full BE170 remain open.

PURGE-HANDOFF-v1 (BE170c88-c91) now has an agreed finite wire target. Space's
Bot-only GetBotPurgeHandoff relays only accepted COMPLETE original Role typed
retirement and Messaging purge entries before aggregate completion; missing
entries remain absent. Original request/receipt hashes use their fixed producer
FQN plus NUL and exact deterministic bytes. Messaging's original request is
the FULL Messaging PurgeSpaceRequest wrapper, not its inner common request.
The separate Bot-only GetBotPurgeChatCoverage observes exact sealed-manifest
membership for at most1000 sorted unique chats; it confers no purge completion
authority. The transport requests bind a fresh read request_id and full body.
No actor/human identity, caller pages, new clock, permanent personal digest or
retention exception is added. Canonical Settings/native/batch fields are kept.
Actual relay/coverage receivers, positively proved local journal retirement,
P3-native drain and independent global terminal obligations remain required.

The finite Messaging coverage receiver now implements its read-only portion:
exact own SEALED manifest/current irreversible source binding and ordered
membership, with no new database schema or evidence clock. Private signed Bot
capability/body snapshot, real mTLS/HTTPS-JWKS/Redis and post-SQL-wait expiry
checks are exercised independently from the Bot consumer. The store matrix
also verifies wrong binding/unsealed/restorable rejection, own full-receipt
retention erasure without fabricated completion, and caller-array mutation
while blocked on an actual SQL table lock. The remote missing-method RED is
an unregistered RPC, not a claim about prior cryptographic enforcement.
Original ordinary pin cleanup remains on the same finite dedicated listener;
its affected transport regression must stay green. Space relay, native drain,
qualified local journal adoption and independent global terminal work remain.

The coverage receiver distinguishes trusted JWKS/Redis dependency failures
from invalid credentials. Actual production-cache hard expiry followed by
HTTPS 503/malformed JWKS, and stopped real Redis, return Unavailable before
the held SQL source can be entered. Unknown kid and replay remain
Unauthenticated; fresh approved JWKS recovers. Existing pin cleanup retains
its prior error mapping. The causal missing-classification RED was 75.639s;
the corrected real dependency matrix passed in 87.156s, using the production
61-second cache expiry without a substituted clock or resolver. Runtime
compatibility, scoped vet and Messaging build also pass. This is a receiver
availability proof, not qualified Bot P3 adoption or full cleanup completion.

BE-170 c95/c96 allocates a separate minimal RegisterBotGlobalTerminal wire:
protocol, Bot UUID and committed server decision UUID only. The decision is
also the signed request identity. Its receiver-owned receipt binds those IDs,
receipt UUID, original registration time and GLOBAL_NATIVE_ADDS_FENCED outcome.
It supplies only the all-Add terminal fence; every per-Space cleanup obligation
remains independent. The global reducer must require this obligation even
when no Space target exists. No actor, Space, owner credential, root hash,
source counts or new erasure authority enters this wire. Receiver/catalog,
producer/reducer and actual zero-target/replay/concurrent Add proofs remain
separate implementation requirements. Existing known46 projection cannot
authorize legacy personal-data erasure; its actual trigger gap stays open.

Bot014 implements independent root-terminal journal slots without a Space FK.
The zero-target causal PG RED3.419 observed COMPLETE after successful atomic
disable/token denial/root persistence; the pending/reducer gate now passes.
Exact minimal Space request sealing, strict typed receipt/CAS, fresh signing,
lost ACK/corrupt result recovery and independent Role-pending are exercised with
real Bot PG and a controlled signed receiver. They do not qualify native48
transport or Role's unallocated root-only API. Actual migration controls cover
retained historical root backfill, empty RC/old RR DOWN and an actual first
BeginBotDeletion writer blocking DOWN. No old evidence erasure or new clock is
authorized; ordinary accepted-cancellation semantics remain separate.

ROLE-BOT-TERMINAL-v1 (BE-170 c103/c104) allocates a distinct Role-only minimal
wire after original183/44 interface agreement. Request protocol/Bot/root tags
1-3 match the minimal assertion, but its signed RPC/audience are Role-specific.
The own receipt carries protocol/participant=role/Bot/root/receipt/time/outcome
tags1-7 and only BOT_ACTOR_ROLE_MUTATIONS_FENCED. It cannot settle Space's slot
or any per-Space cleanup. Codec and descriptor preservation do not activate
the receiver, qualified Role25 projection, old kernel guards, migrations or
unsupported BotActor methods. Legacy erasure/action-stop boundaries remain.

The Bot Role producer now implements the separate typed caller and Role-only
receipt CAS. Actual PG/RSA controlled-receiver RED5.131 observed zero Role
calls; GREEN5.329 delivers the exact accepted root with fresh Role credentials.
The expanded matrix exercises lost ACK, wrong participant, independent Space
completion, exact Role replay, both-results-only root completion and no remote
calls for an unqualified historical root. Shared private terminal persistence
keeps fixed participant selection, typed validation, owned bytes, retained hashes
and post-SQL-wait claim checks. Missing Role dependency is retryable Unavailable.
These are controlled receiver tests, not actual Role26 or native48 serving proof.

The separate minimal Space48 handoff now exercises real Bot PG014 accepted-root
and local revocation through the production worker, signer and typed receipt CAS
against immutable corrected Space48 source d667 plus provider0d5b. The actual
private mTLS/HTTPS JWKS/Redis/PG18 receiver commits before a simulated lost ACK;
a corrupted decision receipt is rejected, then a fresh credential recovers the
stable own result. A fourth worker pass revalidates saved bytes without another
Space RPC. The independent ROLE row stays PENDING and the root RETRY_WAIT.
The provider independently checks the four-column minimal marker, stable RPC
replay, and generation1/2 Add denial in different absent Spaces before lookup.
Caller race8.745s and provider race32.525s pass without skips. This is zero-target
fence evidence with controlled verified-owner admission, a manually configured
equivalent mTLS client and service reinstantiation on the same Bot Pool. It does
not prove public Gateway/Auth admission, actual Role26, per-Space cleanup, P3
drain/retention or complete DeleteBot. No live provider handles are transferred.

Active P3 consumer work is separate from the frozen minimal-fence proof.
The first local validator checks canonical original Role retirement and FULL
Messaging wrapper bytes with fixed producer domains, their exact receipts and
the separate sealed-manifest membership observation. It owns deep snapshots,
keeps absent/uncovered evidence pending, and never mints a participant receipt.
It is not yet connected to a journal transition or PurgeSpace. The next steps
are to capture the finite eligible local sources under the irreversible Space
guard, read downstream proof outside locks, then re-enter the same decision,
generation, manifest and claim guard for explicit P3-covered retirement. The
live-intent FK must not be removed without that positive retained state and
missing-intent rejection. Native drain remains the separately agreed c90
receiver requirement; no non-LIVE ordinary cancellation bypass is implied.

Bot015 now decouples the accepted journal from its disposable live intent while
introducing a durable, finite pending snapshot before any local erasure. The
original exact9088 pending-uninstall oracle failed with FK23503 in5.506s;
snapshot capture now returns recoverable pending without changing that journal
or issuing a purge receipt. A separate old9088 application with the new SQL pair
first incorrectly committed purge (RED16.329s). The new PUBLIC-execute-revoked
INVOKER receipt-insert guard rejects its unprepared receipt and rolls back all
projection erasure (GREEN5.910s). This SQL guard is local Bot scope and reuses no
stopped function/privilege artifact. The accepted completed journal may outlive
its retired live projection, but DOWN refuses restoring its former FK.

The current pending source also freezes native-drain inputs for accepted targets
whose live intent is already CANCELED. The causal missing-input PG RED5.235s now
passes with exact original Remove bytes/raw SHA, preserved cancel identity and
separate once-persisted outer P3 operation. Duplicate active/accepted references
to one installation must agree; no legacy tuple is fabricated. Retry reuses
the exact source bytes. These are input snapshots, not remote drain receipts.

Current affected metadata matrix passes32.715s: pending/CANCELED/completed-journal
controls, exact-original coverage validation, default-RC/old-RR DOWN, actual first
PurgeSpace writer versus DOWN, and a fresh nonsuperuser LOGIN proving INVOKER
trigger execution without a function EXECUTE grant, denied ALTER and P0001 for
unprepared receipt insertion. The first negative SQL probe referenced a nonexistent
column and is retained as a test-setup failure, not product RED. All nonempty
handoffs still remain pending: protected c89 reads/claim CAS, native49 drain,
positive journal retirement, bounded retention cleanup and full P3 completion
are the next original implementation steps. No component result closes BE-170.

The next c89 read-consumer stage now implements due SKIP LOCKED P3 claims,
private immutable request/source inputs and source-bound observation CAS.
After Space->sorted Bot->row waits it rechecks the same irreversible decision,
manifest, complete source set and current lease. A stale generation, expired
wait or changed/extra source cannot write. The consumer signs only the exact
Space relay or Messaging manifest-read method/body/read ID using the existing
protected clients. No human token/default Role or ordinary metadata is used.

Coverage is an explicit local P3_COVERAGE observation, not a participant receipt.
The worker persists an immutable validated prefix of <=1000-chat batches, so
a bounded pass resumes extra chats without dropping them or rereading accepted
originals. It semantically validates retained observations even when their SHA
matches. Native drain and positive retired-journal transition are still absent;
all nonempty sources remain PENDING and no Bot purge receipt is emitted.

Final current real-BotPG/race component matrix25PASS0skip:
store23.188s/grpc18.858s. It covers SKIP LOCKED, reclaimed generation fencing,
private copies, caller-proof mutation while blocked on the actual Space guard,
lease expiry after that wait, changed/extra frozen source refusal, and controlled
RSA pure-service receiver requests. Zero/one/1001-chat sources exercise outage,
missing Role original, explicitly uncovered membership, saved first-thousand
prefix before a second-batch failure, exact resume and no repeated RPC after
complete observation. A digest-consistent invalid typed saved proof fails
before RPC. This is not actual mTLS/Space49/Messaging provider composition.
The new read-only adapter follows the already frozen canonical contract;
no production causal RED is invented from an absent previous method. Earlier
claim21.276s and worker8.280s/16.146s successes retain their exact earlier inputs.
Actual protected provider handoff, native49 receipt adoption, current physical
source/journal version revalidation for retirement, applicable retention cleanup
and full P3/BE-170 completion remain original implementation work.

The prepacket released-lease control caught a typed-error edge in085336b:
Retry clears claimed_until but leaves PENDING, so the same generation's second
attempt scanned SQL NULL into bool. Exact085336b plus the identical final test
failed in3.711s. COALESCE to false now yields ErrDeletionClaimLost after the
same guard wait, with no write/authority relaxation. The final full affected
matrix again passes25 tests with zero skips (store24.224s; grpc timing in raw
packet), followed by vet/build0. Earlier short proof remains applicable except
this one SQL expression and appended regression assertion; no final whole-short
rerun is claimed solely from those unchanged cases.

## Native P3 request and typed-result persistence

The next component uses the frozen native49 DTO without changing authority or
completion policy. After exact c89 coverage, the producer seals each native
request before transport, preserving the actual original cancellation and its
separate P3 child operation. The existing dedicated Space client receives a
fresh pure service credential for each attempt. Exact typed receipt validation
and claim/source CAS save owned result bytes; saved results avoid another RPC.
No local source, accepted journal or Bot participant is completed by this arm.

The final affected Linux race matrix passes34 tests including subtests, zero
skips (Store15.676s, grpcsvc7.527s). Real BotPG checks include coverage-before-
seal,15 malformed receipt cases, original accepted CANCELED identity, and
caller byte mutation while blocked on the actual Space guard. A controlled
RSA receiver proves sealed bytes exist before RPC and no SQL Space guard spans
the call. Lost ACK and wrong typed result remain pending; a fresh JWT saves the
same exact result, and the next pass makes no fourth RPC. This is controlled
receiver proof, not actual native49 transport or a successful local retirement.
An earlier callback assertion used BYTEA's hex text as JSON and failed setup;
the corrected assertion decodes UTF-8 JSON and both failures/results remain in
the packet. No missing-method failure is labeled a product causal RED.

The remaining representation question is explicit local P3-covered retirement
of accepted targets after disposable live intent removal. It must preserve
ordinary accepted-ACK semantics and pending global roots, remove unnecessary
completed Space/actor/chat scope, and never fabricate remote receipts or an
expiry. Original specialist251 receives the exact source-backed finite question;
actual native49 provider composition proceeds independently from immutable
provider input, without borrowing a running process.

The optional provider branch is now exercised by the actual BotPG caller from
`TestPurgeNativeActual49RelayAndTypedResultRecovery`, using immutable Space
provideracc3fff9 in the same caller-owned private Linux namespace. Actual LIVE49
Add precedes the provider's own irreversible P3/controlled Role-first decision;
the caller then obtains the real protected c89 original and native typed result.
After the real receiver commits, a transport decorator loses the ACK, then
corrupts the P3 binding on the next result. Both leave local work pending. A
fresh JWT for the same saved request recovers its exact result; the fourth pass
makes no RPC. Provider independently verifies its own saved child/native source
before writing a synchronization signal. Final caller58.95s/provider59.76s pass
with zero skips. The first attempt failed only because the standalone caller
binary used a relative migration directory from a different cwd; a source-owned
runtime.Caller path fixes that harness input, with the failed logs preserved.

This covers actual native49 mTLS/JWKS/Redis/PG transport and Bot durable typed
CAS. TLS is manually configured equivalently to the separately tested production
constructor. Empty chats require Role-only coverage; upstream Role/fences are
provider-controlled, not actual remote Role/Chat/Messaging admission. There is
no public Gateway/Auth ingress, OS-process Bot restart or local journal retirement
claim. The caller explicitly asserts local PENDING and absence of a Bot P3
receipt; that remaining transition is the next original implementation stage.

## Bot016 local target retirement (current implementation checkpoint)

Canonical BE-170 c122/c123 requires an explicit local result, not a fabricated
remote receipt. Bot016 seals expected opaque target/effect membership, retires
only positively covered rich target projections, and commits the own Bot purge
receipt in the same transaction. Ordinary journal completion can then use its
own exact local result after the disposable intent is gone. Other global targets
and both independent root-terminal obligations stay pending as required.
The first frozen native cancellation is reused; divergent accepted IDs, unseen
late chat lineage, missing effects/targets and expired claims fail closed.

Meaningful causal checks are preserved: first-cancel identity RED8.214s to
GREEN8.022s, and positive worker retirement RED4.765s to GREEN4.825s. Real PG
retirement matrix8/8 passes27.045s; RC/RR and DOWN first-writer controls pass
11.448s. The finite semantic correction run passes24 cases with two explicit
optional handoff skips, apart from one new negative fixture which initially
omitted mandatory added_by_profile_id; its corrected focused run passes.
Earlier missing016 fixtures and obsolete unproved-Purge expectations remain in
the failed raw logs, not relabeled as product REDs. Final short checks pass257
with275 explicit skips; vet/build exit0. These skips do not prove integration.

The subsequent immutable2c074ab actual native49 composition passes: caller58.67s
and provider59.44s, zero skips. It proves positive local retirement/own Bot purge
receipt, exact replay, no fourth native RPC, removed intent and a still-live Bot.
The earlier2d6369b caller failed only at a new postcondition expecting `active`
instead of canonical `live`; its source and failed log remain separate.

Independent review then found a final global-reducer gap: after qualified
membership accounting and both terminal slots, an old second query still
required every effect to have remote `COMPLETE` state. Legitimate explicit local
results could therefore never complete a global root. A real PG causal test
with two positively retired targets and both typed terminal results reproduces
the failure; missing target/effect, unfinished other target and pending/missing
terminal controls remain negative. The correction removes only this redundant
state-only query and keeps the qualified accounting and independent gates.
The unchanged causal test fails its positive branch in4.520s (package23.782s),
while its five negative branches already pass. After the fix all six branches
pass22.900s; the broader local/terminal regression matrix passes29 tests
including subtests in82.304s, zero skips. These terminal receipts are controlled
typed results saved through production seal/validation/CAS, not new actual
remote Role/Space delivery proof. The prior native49 composition remains a
separate source-bound result.

Full evidence retention/compaction, nonempty remote Messaging coverage,
historical unresolved-source resolution and full BE-170 completion remain open.
