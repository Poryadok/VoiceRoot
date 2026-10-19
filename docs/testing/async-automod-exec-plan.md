# ExecPlan: account-scoped asynchronous spam moderation (BE-045)

## Current develop integration checkpoint

Independent source review of Auth snapshot/census `fc364aa` returned scoped
SOURCE CLEAR, with a runtime source-binding qualification: the old census tar
was a baseline before overlays, not the complete executed source. That gap was
corrected by a bounded repeat from the immutable fc364aa Git archive, without
overlays. All 522 source files matched archive hashes before and after native
execution; exact eight tests passed with zero failures/errors/skips. The archive
SHA-256 is `a382abd835871d839561a7682043c3c877d5db78d6d14133309863f14946f026`;
the unchanged source manifest SHA-256 is
`96caa12ec9dde87d288a0e138ecb6f3c89eea63349ab21a40683a23f48f358d5`.
Actual Maven log/manifests/receipt/XML are retained as ignored exact-proof
artifacts. Review-function c3 records the correction; the old baseline is not
retrospectively relabeled as exact execution proof. Evidence-only recheck is
separate from unchanged source review. The lifecycle producer `46d6f24` also
received independent bounded SOURCE CLEAR; delivery/activation remain open.

Normal merge input `0a0024900b5bb3ba2f6fee391615f0f4fa0864e3` preserves
the separate Role/Space Auth floor methods and reviewed strict restore deadline.
The combined issuer inventory retains all six service issuers while each RPC
keeps its own exact caller allowlist. Moderation retains both scan metrics and
the private report authority/privacy fields; liveness still mounts metrics and
configured readiness still requires P5. Auth Go/Dart bindings were regenerated
from the merged canonical proto, rather than selecting either stale descriptor.

The frozen source tree `f1c6752127db76472a3821172019c127614667e4` passed
Linux Moderation `go test -short ./...` and `go vet ./...`. The native Maven
compile and 40 tests passed first, with two fixture errors from unavailable
Docker. The identical compiled candidate then passed all 42 selected principal,
Role-floor and Space-floor tests with zero failures/errors/skips using real
PostgreSQL16/Redis inside an owned private DinD engine. Nested published ports
existed only in its Linux namespace; there were no Windows host ports/listeners.
The XML reports are retained in ignored checkpoint evidence. Both runners,
engine, private inner network and engine volume were removed after terminal
results. This proves the bounded merge, not P1-P5 or full feature acceptance.

## Account-core v2 implementation checkpoint

The private lifecycle wire now declares the Auth-only receiver RPC and strict
opaque fact/typed receipt. Validation rejects unknown/duplicate/noncanonical
fields, mismatched receipt generation/event/subject, revision overflow, missing
required cycle/deadline and equal-revision changed fact hashes. A due DELETED fact stays
nonterminal; a positive PURGED fact carries no invented erased revision/cycle.
Newer/terminal peer receipts remain inputs to CURRENT owner-state comparison,
not direct ACK authority. Actual receiver/publisher transactions remain open.
The independent 120-byte vector is pinned in the Auth service doc.

Linux wire/client race tests passed (1.037s/1.148s), scoped vet and buf lint
passed. All 318 executed source files matched the frozen source-tree archive
before/after the final observation (the first manifest was captured during the
compile, not relabeled as a pre-run observation). Archive SHA-256 is
`ebdd3a422e591c23722181f77679894c2dcabe988bffb3e4a8d9b1adc0d63023`;
raw logs/manifests/receipt are retained under ignored lifecycle-wire proof.
This is protocol validation proof, not method activation or physical cleanup.

C19 transactional producer checkpoint: Auth V30 / canonical version31 capture
every owner account creation/lifecycle update and physical removal through SQL
triggers, with an opaque compact current projection and two durable recipient
intents. Session-only updates do not manufacture another lifecycle event.
Restore changes the exact revision/state/cycle atomically; rewriting an existing
DELETED timestamp cannot extend its cycle. Positive terminal insertion removes
the nonterminal projection and independently preserves both terminal intents.
No account/profile/content identifier is present in the producer tables.

Native PostgreSQL regression first failed on the absent producer (RED 8.444s).
The same compiled test then passed after applying the new SQL, together with the
existing census/store and two-connection restore-race tests: five tests, zero
failures/errors/skips. The producer test injects a real outbox write failure and
checks owner restore/projection rollback, exact recipient replacement, distinct
next deletion cycle and terminal delivery surviving physical account removal.
All 29 Flyway migrations applied; Flyway and canonical up-SQL bytes are equal.
The guarded down migration refused to discard populated lifecycle authority,
leaving its tables/rows intact. Actual wire delivery, recipient application,
finite lifecycle catch-up and P1-P5 remain open; SQL intent is not a cleanup ACK.

P0 composition checkpoint: main now loads one local issuer and starts its
certificate-only JWKS and private9467 listener before database connection. The private listener
registers only `ModerationEvidenceService` with workload mTLS, bounded replay
verification and lazy Auth/JWKS dependencies. It reuses the same issuer/store/
Auth client for report validation; the compatibility signer loader is no longer
used by main. Configured public RPCs and `/ready` remain closed, while `/health`
and private discovery remain available. The old ungated expiry loop is not
started in this configured boot. Only actual bound local listeners produce P0;
no P1–P5 receipt is invented. Coordinator/census and gated expiry remain open.
Linux runtime TLS tests, readiness/generation tests and existing health tests
pass under `-race` (1.442s), including a valid signed private request while the
Auth peer and domain DB are unavailable: it reaches the closed boot gate.
Dedicated not-ready-publishing ClusterIP Service templates and both Moderation
readiness probe paths are source-only; no rollout/network/current-tail proof.

Restriction-sync checkpoint: a finite queue worker now requires this boot's P2
private-repair capability before any peer call. Auth RPCs run outside account
transactions with a two-second deadline. Only an exact typed account/subject/
revision/ban acknowledgement can clear the matching durable intent, with the
captured generation checked again under the account lock and before commit.
An ahead reply is compared to CURRENT local revision under that lock: a newer
local committed request remains queued; genuine remote-ahead divergence closes
the gate without bumping local history. Closed-boot delivery and the ordinary
in-flight-ahead case were reproduced as RED. Three real PostgreSQL worker tests
and the existing complete-ban-set/stale-CAS test pass under Linux `-race`
(3.148s). Main startup/census integration remains open; an empty batch or a
successful sync call is not itself a P3/P4 receipt.

Private lifecycle mutation boundary: provision, subject binding, lifecycle
refresh and effective snapshot now carry the captured boot check through the
account transaction, including after account-lock acquisition and immediately
before commit. Entry-only checks reproduced four committed-after-close REDs
(8.563s). All four rollback oracles plus signed evidence RPC and restriction
worker regressions pass under real Linux PostgreSQL `-race` (9.684s). Legacy
internal helpers remain for local authoritative participants/tests; the private
RPC service exclusively uses the checked entry points.

Auth local-issuer/bootstrap checkpoint: native RED reproduced the SDK-off local
issuer depending on a User endpoint. A complete key pair now creates exactly one
issuer with no SDK/User client; all twelve configuration tests pass, preserving
partial-setting failures. The same issuer signs only the fixed Moderation
snapshot purpose through a dedicated method; eight crypto contract tests pass.
The standalone certificate-only HTTPS publisher serves current/next public keys
before domain readiness, without JWT/Redis/User dependence. Two actual HTTPS
tests reject anonymous, wrong-CA and wrong-host clients and partial settings;
two existing outbound mTLS tests also pass after the shared TLS loader refactor.
All 24 distinct scoped tests pass with zero skips. XML evidence exported before
removing the owned native Maven runner; no Windows listeners or stand changes.
Actual Auth session/census and public readiness composition remains open.

Auth listener checkpoint: inheriting the proof listener's server-only TLS was
reproduced as native RED. The moderation authority is now registered solely on
dedicated9093 with mandatory workload mTLS and request-bound principals; existing
proof/floor policy remains separate. Native tests: 3 new transport tests, 5
existing transport tests and 1 existing moderation-principal test pass with zero
failures/skips. Actual Linux TLS rejects no client certificate and wrong CA;
a valid but unauthorized Gateway principal is PermissionDenied before dispatch,
and a valid Messaging principal reaches the fixture. This is private transport
proof, not the assembled public/session bootstrap or deployment topology.
An initial incomplete archive extraction was a setup failure, not RED; the
complete fixture was recopied only after archive completion before actual RED.

The complete c23/c24 offline recovery contract is persisted in
`async-automod-recovery.md`. Ordinary live startup has no export/sink dependency;
declared restore still requires protected current-operation evidence and actual
recoverable history. The production typed adapter/verifier remains to implement.

Privacy-only retained-subject helper checkpoint: naive binding admitted an
already due DELETED snapshot (real PostgreSQL RED). The dedicated helper now
checks fresh DB time, exact lifecycle/state/deadline and permanent terminal
fences under caller-owned account then subject locks. It permits pre-deadline
DELETED only for privacy recovery and leaves `RequireLiveSubjectTx` unchanged.
Expired captured authority is unavailable, never a permanent terminal assertion;
older DELETED cannot overwrite a newer local LIVE restore. Rechecking after
intervening work is required before commit. Focused PostgreSQL race proof covers
rollback at deadline, unchanged send denial, immutable same-revision deadline,
stale revision and terminal replay. Actual report recovery worker remains144.

Terminal-seal checkpoint: the old client accepting `complete=true` without a
seal was reproduced as RED against the immutable pre-change client. Strict Go
client/codec race tests pass (1.626s/1.191s). Auth now produces one bounded
repeatable-read terminal union with persisted generation/high-water/count/hash;
Java native PostgreSQL18 proof passes two tests, zero failures/skips (20.395s),
applying the branch's 28 Flyway migrations through V29. V28 belongs to the
separate Role owner-channel author and must be assembled before deployment;
this disposable database was never a shared environment. Canonical counterpart
is auth_db000030. Native source first exposed the assembled report-target proto
include omission in Auth POM; adding both User and common report_target files
restored compilation. This is scoped proof, not full restore/census/readiness.
The first concurrent-source Go attempt was a build setup failure and is not RED;
the actual baseline RED was obtained only after that process had terminated.

The c17 origin-identifier leak was reproduced against the unactivated v1
producer. New inserts now create a domain-hashed immutable account-only v2 core
and a separate local `automod_message_coverage` bridge. Core delivery does not
need that bridge. Physical primary/companion deletion removes the bridge while
preserving core bytes, source cursor and eligibility. The format marker cannot
be reset; no live v1 migration or rehash is claimed.

Linux Docker evidence: codec race tests pass (1.040s), three real-PG producer
race tests pass (9.937s), and signed private RPC race proof passes (2.628s).
The latter rejects legacy v1 and nested unknown origin bytes before ingestion;
the codec rejects unknown fields recursively, incomplete aliases and unsupported
authority. Buf lint and diff whitespace checks pass. Graphify AST update6697
produced no output within the bounded operation and was canceled (terminal1);
no AST success is claimed.

The missing FROZEN cleanup was reproduced as a second privacy RED. Five real-PG
producer/Space/managed-purge race tests now pass (5.521s): exact manifest cleanup,
orphan selector, rollback of the fence on cleanup failure, unaffected foreign
chat, local cleanup before external owner wait, and recovered old-link cleanup
without rebuilding old links or erasing new post-restore coverage. Cores remain
countable. Purge completion also cleans and verifies zero links by manifest.
The local restore helper requires independently recovered lifecycle history and
never opens a gate itself. Default test schema includes migration31.

Still open: remaining c17 E1-E14 assembled race/restore/receipt proofs and actual
startup invocation, c13 sealed terminal authority/startup coordinator, every producer's
verified admission, private delivery/retention, all sanction-writer composition,
and assembled activation/review/CI. These scoped passes do not certify Space P3
or full BE-045 readiness.

## Purpose

Successfully committed messages create private keyed evidence. Five matching
messages in ten minutes, or one message containing three links, cause a one-hour
account mute. A second independent hit for that pattern within thirty days causes
one permanent account sanction. Failed sends and retries cannot create strikes.

## Context

Implementation owner: Rudeus-156. Immutable starting develop:
`c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c`. TT BE-045 c2 owns the original
numeric policy; c9 and its attached autonomous contract v1 resolve P1/P2/P3 under
the user's subsequent delegation. The appended contract distinguishes those
sources. Historical specialist source observations are not current runtime proof.

Owning specifications: reports, Moderation, Messaging, Bot, Auth, DATA_STORES,
CONTRACT_MATRIX and ARCHITECTURE_REQUIREMENTS. Current source has a synchronous
profile-based CheckMessage detector, private Messaging insertion transactions,
signed service-principal helpers and a raw unversioned Auth status writer.
The existing raw writer and producer identity paths must be reconciled before
activation; merely adding another consumer is insufficient.

## Scope

In: all enabled insertions including forward commentary, attachments, ghost
messages and Bot/game events; verified account/subject ownership; private atomic
outbox, ordered consumer, account-wide read-only admission, idempotent sanctions,
durable event/restriction journals, Auth revision/deletion precedence, rotation,
expiry and terminal snapshot/restore gates. Auth stays Java. Public message events
gain no private account IDs, digests, text or URLs.

Out: report-count thresholds, Space-local report authority, shadow-ban routing
changes, inactive scheduled producer activation, federation, rollout and final
acceptance appointment. Independent source review is mandatory for ACL, privacy,
concurrency and migrations. No host test executables/listeners or GitHub probes.

## Milestones and progress

Exact projection acknowledgement checkpoint: a Linux client regression first
failed because equal revision without accepted account/subject/state was treated
as convergence. The client now validates the complete accepted projection; a
typed remote-ahead result must be compared with CURRENT local authority before
classifying restore divergence. The same source passed real Linux PostgreSQL 18
and Java 25 JDBC tests after all 27 Flyway migrations, including stale/equal
revision handling, deleted lifecycle, imported terminal fence and overdue purge
(one test, zero skips). The initial test's attempt to change an Auth-owned purge
deadline was correctly rejected; the fixture now asserts that rejection and uses
an actual terminal import and independently overdue account. This is scoped
projection evidence, not P0-P5 runtime or whole-vertical acceptance.

JWKS transport checkpoint: the actual Java HTTPS bootstrap regression failed
with the anonymous default client against a certificate-required issuer. The
configured workload keypair/CA client then passed the same real TLS fixture;
missing certificate and wrong CA fail, partial material and automatic issuer
configuration cannot downgrade. Existing configuration tests also pass (ten
tests total, zero skips, Linux Java 25). HTTPS endpoint identification remains
explicit; this fixture does not claim a separate wrong-host deployment check.
The own Java runner and JDBC database were removed after exporting Surefire XML.

Subject-only terminal participant checkpoint: both mapped and missing-mapping
Purge paths originally returned success without invoking the newly registered
subject participant (real PostgreSQL RED). Both now insert the opaque terminal
fence and execute the callback inside the same transaction, preserving the
mapped-account participant order. Participant failure rolls back its mutations
and the terminal fence; an unknown subject never invents account authority.
The regression and existing first-bind/purge race tests pass on private Linux
PostgreSQL. Report-specific participant wiring and complete boot import remain
part of the coherent activation work.

Messaging commit checkpoint: the real producer test observed a successful
message with zero private evidence rows (meaningful RED). New insert-result
tracking and a private hook on ordinary, attachment, game-event and protected
game-create transactions now allocate the subject sequence/source time and
immutable keyed payload atomically. The deferred database coverage guard closes
forgotten/old writers after monotonic activation. Exact same-ID replay remains
non-inserting even after delivery removes its journal row. Real PostgreSQL tests
cover injected journal rollback, terminal denial, key identity/omission/retirement
guards, twelve concurrent same-account sends, metadata-only sends, and one-event
forward/commentary commits. Three tests passed with the Linux race detector.
Verified ingress, full producer activation/dispatch, retention and current-boot
terminal/cursor reconciliation are still open; no completed vertical is claimed.

Moderation issuer transport checkpoint: direct/raw-header entry to the bootstrap
handler originally returned 200 (RED). The private publisher now requires a
verified workload certificate, serves only the fixed JWKS path, and has no peer
RPC/database/public-ready dependency. Actual Linux HTTPS tests passed with the
current/next keys and denied anonymous clients, wrong CA and wrong hostname;
partial TLS cannot start a listener. The owned listener closes on cancellation.
Main P0-P5 integration and private discovery manifests remain open.

- [x] Confirm actual TT owner156, fresh base and exclusive lease22.
- [x] Read c9; ordinary policy choices no longer require a user response.
- [x] Persist exact owning API/schema and policy contracts before production code.
- [x] Observe meaningful RED on existing pre-commit CheckMessage behavior.
- [ ] Implement private evidence/projection and ordered transactional detector.
- [ ] Implement Auth subject/lifecycle and revision-fenced restriction projection.
- [ ] Implement Bot owner authority and verified producer admission on every path.
- [ ] Wire workers, retention/purge/readiness and durable sanction events.
- [ ] Execute T1-T12, real PostgreSQL rollback/concurrency, Go/Java/proto checks.
- [ ] Independent review, immutable artifacts and accepted ant121 CI handoff.

## Detailed steps

1. Add a private `voice.automod.v1` protobuf contract. Auth owns subject lookup,
   complete terminal snapshot and restriction reconciliation; Bot owns owner
   resolution; Moderation owns evidence ingestion and send restriction lookup.
   Every surface requires mTLS plus fresh Phase-0 exact method/request credentials.
2. Start with the current CheckMessage regression: uncommitted text must never
   create a strike or reject a send merely because it matches a pattern. Keep
   existing-state mute/permanent-ban denial and fail-closed dependency errors.
3. Use one Messaging insert hook in the actual transaction, after chat/Space
   locks and before commit, to allocate the account sequence and private event.
   Exact replay must return its prior result before fresh owner resolution.
4. Under the Moderation account lock atomically consume inbox/cursor, samples,
   strikes, mute episode, sanction, event outbox and desired Auth revision. Cover
   reordered delivery, transaction failures and restart/ACK loss with real PG.
5. Add protected Auth/Bot authority and route all manual/automated restriction
   mutations through one revisioned projection; preserve legacy unknown bans.
6. Complete producer/client/listener wiring, content expiry receipts, local
   deadline purge and protected terminal snapshot readiness. No inactive partial
   implementation is a completed deliverable.

## Validation

T1-T12 below define observable outcomes. Run focused tests first in owned Linux
Docker; real testcontainers require the Docker socket and no published host ports.
Go package checks, race/migration checks, Maven tests and protobuf generation/lint
follow affected surfaces. Compile/setup failures are not meaningful RED. Existing
evidence remains valid only for unchanged covered inputs. Runtime rollout, hosted
CI and acceptance remain separate from local evidence.

## Decisions

One author owns tests and implementation. Use exact private APIs, never public
`message.sent` enrichment or unrestricted content lookup. The complete c9 policy
is recorded below as an autonomous delegated decision, not attributed retroactively
to c2. Preserve the unknown-suspension Auth restriction during migration.

## Current evidence and continuation

Shared boot-gate checkpoint: a new process initially admitted effects and a
properly signed RPC bypassed the closed startup barrier (two meaningful REDs).
The gate now enforces fresh boot/generation, ordered phase receipt references,
separate P2 private repair versus P5 effects, immutable phase proof identity and
permanent invalidation of captured checks after close. Evidence ingestion checks
the captured token after peer resolution and under its account transaction,
including immediately before commit. Real PostgreSQL rollback at that final
check leaves no inbox/action/cursor change; a new ready boot cannot revive the
old token. Focused gate/decision tests and RPC/store cases passed with the Linux
race detector. These tests use explicit test phase receipts; they do not prove
the still-open coordinator's sealed snapshots/census or network readiness.
The latest own Graphify AST attempt was bounded and cancelled through its own
session after no output; terminal exit 1 and disappearance of own PIDs 51404/
54804 were verified, peer PIDs 54412/53644 remained untouched. No AST PASS claimed.

BE-045 c17 resolves the newly discovered Space P3 linkage boundary under existing
autonomous delegation. The complete [account-core/erasure refinement](async-automod-space-erasure.md)
requires additive minimized protocol v2 and a separately erasable local coverage
bridge from original insertion. Account-purpose PERSONAL samples remain c9
eligible after Space deletion; account terminal purge dominates. Implement this
before activation, reject unsupported v1/unknown bytes, and execute E1-E14 alongside
c9/c13. The earlier unactivated v1 producer checkpoint is implementation evidence
only; it cannot authorize silently rehashing any hypothetical live v1 stream.

The additional startup/restore risk is resolved by BE-045 c13 in
[the startup and restore contract](async-automod-startup-restore.md). It preserves
c2/c9 and adds P0-P5 method/worker gates, readiness-independent private network
routes, sealed terminal manifests/boot receipts, exact payload-bound projection
ACKs, finite subject census and an independently recovered erasure/history cut
for declared restore. A larger remote revision must be compared to CURRENT local
authority before distinguishing an old in-flight request from lost local history.
No reciprocal RPC may hold an account DB lock. Implement N1-N14 and R0-R5 there
before activation; these are required checks, not reported successes.

Contract checkpoint `31d8d23586efa0012552d0aede6115af20510965` was published by
ant121. It is not a runtime or integration-ready head. Linux PG18 focused tests
prove ordered buffering, non-overlapping sample use, atomic rollback on an
injected sanction-event outbox failure, exact replay/conflict handling and HMAC
alias continuity. The original CheckMessage regression observed one unwanted
auto_mod_log insert before message commit (meaningful RED); its active-path
replacement is still required. Pure engine window/boundary/expiry tests pass.

Exact Auth V27/canonical000028 SQL executed on PG18 passes revision dominance,
equal-revision conflict/idempotence, legacy-suspension preservation, deletion
precedence and terminal restore fencing. Java compile/private-listener tests
remain pending. AuthAuthorityReader's exact migration gate advances with the
new migration; verify both supported migration engines before activation.

Moderation schema allocation is coordinated: report intake000005 (ant144),
automod000006 (this task), report threshold000010 (ant158). Shared machine issuer
is `service:moderation/automod`; shadow sanctions require machine_case_id and the
threshold owner's FK. Fresh Auth binding can provision a common subject before
the first message, without inventing a Messaging source epoch or cursor.

Activation-critical work still open: complete private verified ingress, atomic
Messaging evidence on every enabled insert path, Bot owner lookup, real workers,
all manual/appeal/revoke/expiry revision convergence, terminal snapshot restore
readiness, retention/deadline purge including minimal sanction snapshots and
report-owned transaction participants. No partial store/proto checkpoint closes
these requirements or authorizes rollout.

## Risks and follow-ups

Cross-service admission is not an atomic cross-database transaction. Persisted
message evidence remains actionable even when asynchronous enforcement lags.
Existing raw metadata ingress, Auth status writes, anonymous machine issuer,
ordered gaps, HMAC rotation and account erasure are activation-critical seams.
Record any new concrete source incompatibility here and resolve it within c9;
do not reopen settled ordinary policy questions.

## Accepted contract v1

The following source is the specialist c9 artifact, retained verbatim as the
decision record. Owning service sections define the final RPC names.

BE-045: autonomous completion of c7 policy edges, contract v1
Actor Рудеус-162; architectural/security specialist; gpt-6-astra/high.
Consumer and implementation/delivery owner: Рудеус-156.
Authority: subsequent user delegation relayed by root: «но вообще решай вопросы сам, я ж говорю работай автономно». The following P1/P2/P3 choices are agent decisions under that delegation. They are NOT claimed to be choices previously made by the user in c2.
Supersedes the P1/P2/P3 decision-only hold in BE-045 c7 and its attachment. Retains the source evidence and technical safeguards there unless explicitly refined below. Source evidence baseline remains 1740fa52a7c9f5e1b0eaca2d89ef2ed6bb7de990; implementation must reconcile its fresh immutable base.

1. EXPLICIT ACCEPTED c2 (UNCHANGED)
Same normalized pattern on the same Auth account in at least five successfully persisted messages in a rolling ten-minute window qualifies. Three or more links on one successfully persisted message qualifies. First hit in the rolling thirty-day strike window gives a one-hour mute; second hit for that account/pattern within thirty days gives a permanent account sanction. Profile switching does not reset state. First ten successfully persisted sends after a timed mute expires are inspected by the same detector without immunity. Failed/muted sends do not count. Processing is asynchronous after commit and never rejects an already committed message retroactively. Only keyed pattern evidence, never copied raw bodies or URLs, is retained for automod. Permanent action is perm_ban plus moderation.sanction_applied, not shadow_ban.

2. P1 DECIDED: NON-OVERLAPPING EVIDENCE, SOURCE-TIME STRIKES, APPLICATION-TIME MUTES
For each account/pattern, process committed messages in the Messaging-owned account sequence from c7. A qualifying hit consumes the pattern's previously unconsumed eligible samples through its trigger sequence. Those samples cannot support a second hit. A later hit needs new committed evidence after that per-pattern consumed-through sequence. This is an evidence-reuse rule, not a new numeric threshold or a grace period.
For event at source time t: remove unconsumed samples strictly older than t minus ten minutes; include the boundary and this event. If link_count >= 3 OR the unconsumed repeat count >= 5, emit exactly one hit, consume all pattern samples through this sequence, and clear that repeat bucket. If both predicates hold, still emit only one hit. A link-flood event independently qualifies on its own; a second distinct link-flood message can be the second hit even before async mute enforcement is observed. Do not add a blanket active-mute/backlog exemption.
Consequences: six identical ordinary messages committed before consumption -> one hit/mute; ten within the rolling window -> two disjoint hits/one permanent sanction. A retry of any of these messages -> no additional count. Different patterns retain independent hit histories but share the account enforcement lock.
Hit time is the immutable source recorded_at of its triggering message, not arrival time or processing time. The second hit qualifies when its time is no more than thirty days after the preceding hit; strictly later starts a fresh strike window. Reset is logical and never depends on the background GC schedule. Source time is monotonic per-account DB time at the sequence serialization point, as c7 specifies. Client timestamps are never used.
Mute begins at the Moderation transaction's application timestamp and expires one hour later. Effective account mute is the maximum of effective mute expiries across patterns. A genuinely new first hit for a different pattern may extend that effective expiry; replay of an existing hit cannot. A permanent sanction always wins over mute expiry. Cross-service UTC clocks must be synchronized; tests explicitly control source and application clocks.
Post-mute counter belongs to the effective account mute episode. Start with zero when that effective mute expires, and count the first ten new committed records whose source recorded_at is at or after that expiry, ordered by source sequence. A new effective mute starts a new episode; delivery retry does not. Delayed pre-expiry records are processed normally but never counted as post-expiry sends. Existing-state checks linearize before message commit: an already admitted in-flight send can commit; it is then evaluated like every other committed send. No claim of atomic cross-database admission is made.
Rationale: a permanent sanction must not be manufactured by reusing the same five messages twice. Actual independent offending evidence remains actionable immediately, including backlog. Source time preserves the accepted rolling windows under delay; application time provides the full accepted one-hour future-send mute.

3. P2 DECIDED: AUTHOR COVERAGE AND TRUSTED BOT OWNER LIABILITY
Cover every enabled successful Messaging insertion: ordinary send, forward, attachment/caption, ghostOnly, and Bot/game-event publication. No Bot exclusion and no content-scope immunity. Inactive scheduled handlers remain inactive; any future producer must use the same insertion/evidence contract before activation.
Ordinary/delegated-user sends target the authenticated Auth account that owns the actual sending profile, verified at admission. All profiles of that account share the same detector. Bot-originated sends target the Bot's owning Auth account at admission. This includes the owner's ordinary messages and every Bot it owns in the same account/pattern namespace. Bot ownership transfer affects future admitted messages only; committed evidence stays with its captured owner and ownership revision. This liability choice is newly made under the user's delegation, not inferred from actor_profile_id or a client field.
Implement a narrow protected Bot-owned ResolveBotModerationOwner operation. Messaging presents exact bot_id, actual sender_profile_id, installation/scope when present, and operation/request binding from its verified Bot ingress. Bot reads its own bot/installation rows and checks actor correspondence, live status, allowed scope and ownership revision. Return only owner_account_id, a positive ownership revision, and the bound identifiers; authenticated TLS server response and exact Phase-0 caller/request binding establish authority. Never accept an owner_account_id supplied by a bot client, Gateway body, arbitrary profile lookup, game payload or spoofable header. Bot author identity and installation membership remain independently verified by the existing publication path.
Before a NEW commit, Messaging also checks the resolved owner's current Auth lifecycle and existing moderation enforcement. Owner deletion, missing mapping, changed/invalid binding, unavailable Bot/Auth authority or owner mute/ban denies that new send without a counter increment. Current owner resolution is repeated for a new admission, not used as an indefinite cache. Exact replay of an already committed operation returns its saved result without another owner resolution, liability change or evidence event. A transferred Bot cannot reinterpret previously committed work. No synthetic Auth account or synthetic human moderator is created.
Text projection v1 is the complete persisted user-visible text: main content and any separately stored authored forward commentary in a fixed field order, each length-prefixed. Normalize each textual field using the existing Unicode ToLower(TrimSpace) behavior; do not collapse internal whitespace, rewrite URLs, strip markup or add confusable folding. Pin this projection/version in golden tests. Count http:// plus https:// occurrences across those normalized text fields; the accepted >=3-link rule counts their total, including mixed schemes. No URL strings leave Messaging in automod evidence.
If all normalized text fields are empty, mark eligibility=NO_TEXT and send no pattern digest/link count. Do not equate five unrelated photos/stickers/locations with five equal empty messages. Nonempty captions are eligible as text. E2E is always eligibility=E2E with no digest/link count/ciphertext inspection; the authoritative chat/mode gate controls this, not an unchecked request flag. Content types and attachments are not themselves spam patterns in this slice.
Every successfully committed record, including E2E and NO_TEXT, consumes one position in the first-ten post-mute counter. Inspection result distinguishes ELIGIBLE, E2E, NO_TEXT and EXPIRED_EVIDENCE; no metric claims ciphertext was inspected. Because the same detector continues after ten, sending ten non-text messages creates no bypass. Existing account mute/ban applies to E2E and Bot sends too.
Rationale: account-scoped policy is enforceable only with a real owner authority; Bot developers own the output they authorize. Empty-text equality must not punish unrelated media. E2E exclusion is explicit canon, while metadata-only counter advancement preserves c2's successfully-persisted-message wording.

4. P3 DECIDED: MINIMIZED RETENTION AND TERMINAL PURGE
The retention choices below are autonomous policy/engineering decisions. They reuse the accepted windows as bounds but are not extra c2 detection thresholds. No new legal-hold exception is created.
Use an Auth-owned random, immutable moderation_subject_id for each account lifetime, delivered through a purpose-bound lifecycle/subject lookup to Messaging and Moderation. It is not public, is never reused and is included with the verified raw account binding in every new evidence envelope. Auth separately provides lifecycle_revision, deletion_fence_id for a deletion cycle, deletion_at/purge_at, and terminal PURGED. Account UUIDs remain cross-service references, never foreign keys. The subject identifier is a private anti-replay scope, not an alternate public identity or a way to reconstruct an erased account.
Live-account retention table:
- Pending content-derived evidence (keyed digest/link fact) expires at source recorded_at plus thirty days. A send event already beyond that deadline is not allowed to create a new hit. Expiry is an explicit privacy/degradation outcome, never PASS or a false claim of inspection. This prevents unbounded punishment from arbitrarily old offline backlog.
- Applied detection samples may be removed when contiguous source progress proves no later admissible ten-minute window needs them. Hard upper bound is sample recorded_at plus thirty days plus ten minutes, derived from maximum admissible delivery lateness and the counting window. All rows remain only keyed evidence.
- Strike/pattern state is logically effective for thirty source-time days. Physical GC occurs once source progress closes that comparison window; hard upper bound is last_hit_at plus two times thirty days, derived from the thirty-day hit window plus thirty-day delivery admissibility. Retaining a keyed row for this derived bound does NOT extend the strike window. Delete expired digest aliases with the same rule.
- Applied inbox event IDs/hashes, verbose action/evidence receipts and delivered private producer bytes are removed no later than thirty days after application/acknowledgement; producer digest payload still obeys its earlier source-time expiry. Retain only an account-scoped contiguous cursor and bounded pending gaps after that, so a pruned old event cannot be reapplied. Retired lower sequences return an explicit ALREADY_RETIRED no-op; no promise of old-payload mismatch diagnostics is made after its hash expires.
- Content-free pending sequence receipts may remain while a live account exists until the consumer acknowledges them. Keep only subject/account routing, source sequence/time, original envelope hash and terminal outcome; drop message/chat/profile IDs and content-derived fields at payload expiry. Compact acknowledged ranges into the cursor. No permanent raw per-message history is created.
- Effective sanctions and pending appeals retain only the fields needed for enforcement/appeal while the account is live. Verbose automod logs and resolved/revoked evidence follow the thirty-day post-resolution bound. Do not retain message bodies, URLs or copied message evidence in an automod appeal; use minimized sanction/hit receipts. Normal report evidence is outside this slice and retains its existing policy.
Producer evidence bytes and their hashes are immutable. At content-evidence expiry the source writes a separate durable terminal-expiry receipt referencing the original event/sequence/hash, then erases expired payload bytes. It never rewrites the old event with different fingerprints. The consumer checks the deadline even if an original payload arrives before the expiry receipt. Under the account lock, either the previously valid event was already applied or the sequence becomes EXPIRED_EVIDENCE once; neither branch can later change into the other. It advances sequence and first-ten metadata counters but creates no hit. Thus privacy expiry closes gaps without fabricated moderation success. Missing not-yet-expired sequences remain buffered; they are not skipped merely because later events exist.
Key rotation: Messaging alone owns versioned HMAC keys. Derive/account-scope the keyed value with moderation_subject_id and normalized text, with versioned domain separation and length-prefixing. Moderation receives digests only. During rotation produce old+new digests in one immutable envelope; alias them to one logical pattern and one count. Retire an old key only after every live state depending on it has either received its new alias or passed the derived retention horizon, and all its admitted old-epoch evidence has been delivered/expired. The upper horizon is computed from the table, never a guessed short TTL. An invalid/partial ring fails startup. On compromise, stop new affected evidence production/admission rather than silently reset strikes; replace the key under a documented operator recovery using the same alias/expiry boundaries. This is a finite operational recovery path, not a request for a new product decision.
At confirmed account deletion, Auth supplies the authoritative deletion cycle and purge_at (the canonical thirty-day recovery deadline). Messaging and Moderation persist that deadline locally and schedule owner-authority reconciliation and cleanup; the producer immediately denies new ordinary and Bot sends for the deleted owner. Restore before purge is explicit Auth authority and preserves unexpired sanctions/strikes; it never clears a ban. A restored account's newer lifecycle revision cancels only the matching nonterminal deletion cycle. Purge cannot be undone. Per [c19](async-automod-unknown-deadline.md), an old schedule alone cannot prove that a newer valid restore did not win.
The physical-erasure target remains purge_at: each participant atomically erases raw account/profile/message links, pending digest payloads, samples, strike aliases, personal log details and raw Auth-sync request bytes for that subject. With winning positive owner terminal authority established, cleanup does not wait for a missing evidence sequence or ACK. A total partition hiding deletion or a winning restore cannot satisfy both unconditional at-deadline physical erasure and preservation of an indistinguishable live account; this is an explicit remaining failure-model acceptance limitation, never an allowed deadline extension, synthetic terminal fence or global reset. A missed deadline is recorded as such and positive-authority cleanup on recovery is late cleanup. Preserve only a permanent opaque terminal subject fence and non-personal sanction/appeal snapshot (random sanction/case ID, type, times, outcome) when needed for an existing case; no raw identity, content or reversible mapping survives actual terminal cleanup. Existing appeals may be resolved against this minimal snapshot; no status change can recreate identity. Terminal receipt explicitly dominates pending gaps. Hashes derived from raw payload/account data are also removed at purge; opaque subject/cycle identifiers are sufficient for the fence.
Every producer, consumer and Auth reconciliation verifies terminal subject state BEFORE lower-sequence replay, pending operation or retry handling. A replay containing an old raw account ID cannot recreate a mapping: its bound subject is terminal. New evidence cannot omit/change that subject because the signed request hash, source receipt and current lifecycle authority bind it. Reject source epoch/account-subject remapping; do not bootstrap a stream from an arbitrary late event. Existing retained pre-activation data cannot be backfilled into new strikes without the new authority and deadline checks.
Offline/backup restore MUST import the protected Auth terminal-subject snapshot and apply local purge before serving or replaying queues. Absence/unavailability of this snapshot keeps that instance unready. Participant receipts are asynchronous proof, not a dependency that can delay erasure. Purge across replicas is idempotent, does not wait for all consumers, and does not retain a raw-ID escape hatch. This applies to payload stores, retries, logs and restored backups; the implementation must not claim closure from database cleanup alone.
Rationale: content evidence has a bounded lifetime; older pending work becomes explicit expired evidence instead of retaining private derived data forever. A permanent opaque fence prevents backup/replay resurrection without retaining the erased identity. The accepted thirty-day recovery lifecycle takes precedence over pending moderation work.

5. TECHNICAL CONTRACT RETAINED AND MADE CONCRETE
Use the private Messaging transactional outbox -> protected Moderation ingestion RPC selected in c7. It remains asynchronous committed-message processing. Do not add raw account/digest fields to shared message.sent and do not grant Moderation arbitrary GetMessage access. Named methods can be introduced additively with semantic equivalents of IngestCommittedMessageEvidence, GetAccountSendRestriction, ResolveBotModerationOwner, ResolveModerationSubject and ReconcileModerationRestriction; owning service/proto docs must pin their final exact names and caller matrices before implementation.
The per-account source sequence includes EVERY covered successful commit, including ghostOnly and ineligible text records; client idempotency replay creates none. It is allocated by a transactional counter under the existing chat->Space locks followed by the account counter lock. Persist row, evidence bytes and sequence in the same commit on every insertion path. Account subject/owner revisions are captured before that commit from verified authorities. Preserve hash-bound event identity across retries; use fresh Phase-0 request credentials for each transport attempt.
Moderation's shared account advisory lock serializes ordered inbox processing, distinct-pattern state, manual apply/revoke, appeals, expiry and reconciliation. One local PostgreSQL transaction commits inbox/cursor, keyed samples, consumed-evidence watermark, strike, effective mute/post-mute state, machine-issued perm_ban, auto_mod_log, immutable sanction-event outbox and desired Auth restriction revision. No swallowed errors and no post-commit-only event publication. A partial local commit cannot produce a sanction; duplicate delivery cannot extend a mute or emit another sanction. Account-wide permanent sanction identity must be deterministic/idempotent across concurrent qualifying patterns; a reason/trigger association can accumulate without duplicate active automod bans.
Automated sanctions use explicit machine issuer service:moderation/automod; human issued_by is nullable under a constrained issuer-kind schema. No fabricated moderator UUID. Existing admin APIs and appeals expose the actual sanction identifier through their existing flows, without disclosing pattern digests.
Auth remains Java. Add a protected moderation-owned revisioned RESTRICTION projection, not another blind account.status assignment. Exact subject/account, desired moderation_banned boolean, positive revision and request hash are bound. Auth stores revision/hash atomically with projection. Lower revision is a stale no-op; equal revision/equal bytes is an idempotent receipt; equal revision/different bytes conflicts. Deleted/erased state and other Auth-owned security restrictions dominate both adding and removing this restriction. Clearing it is not unconditional activation. A terminal subject returns durable PURGED and never recreates its account.
Moderation's worker recomputes ALL effective account bans, writes a new complete desired revision only when required, and retries the persisted intent after commit. All manual/revoke/appeal/expiry writers touching this account restriction must join that revision path before activation. The legacy raw-header/unversioned SetAccountStatus route cannot remain an alternative writer that defeats the fence. There is no cross-database transaction claim. Timed-out old requests arriving after a newer ban are rejected by Auth's revision even if the Moderation lock was released.
Migration must seed Auth's moderation projection through a protected effective-ban snapshot/revision, serialized with the same Moderation account lock. Never reinterpret every preexisting suspended status as a moderation ban or clear it because a new projection is empty. Preserve an unresolved preexisting suspension as an Auth-owned legacy restriction until its provenance is reconciled; automatic moderation-clear cannot remove that independent restriction. Record any actual legacy ambiguity as migration evidence and resolve through existing authorized account-status administration, not invented account reactivation. Activate every new producer only after its subject identity and consumer baseline are provisioned; do not auto-initialize sequence progress from the first arbitrary received event.
Synchronous send admission is read-only existing restriction/lifecycle enforcement: no content, no detector, no strike or post-mute increment. A committed local permanent sanction denies new sends while Auth reconciliation is pending. Existing mute denies E2E/Bot sends as well. Authority/read failure denies NEW admission without counting failure; exact replay of an already committed send returns its prior result. Existing ShadowBan routing remains separate and must not leak an ordinary public message event; its committed messages still produce private automod evidence.

6. FINITE TEST ORACLES (c7 N1-N14 REMAIN; THESE REMOVE THE OPEN EXPECTATIONS)
T1: five same-pattern plaintext commits in ten minutes -> one hour mute; sixth -> same hit only; tenth with two nonoverlapping groups inside the window -> one perm_ban. Five with oldest strictly outside ten minutes -> no hit. Both repeat and link predicates on one event -> one hit.
T2: two distinct >=3-link messages -> second hit/perm_ban; two http plus one https qualifies. Message retry/changed idempotency body cannot manufacture evidence.
T3: same account changes profile, Bot or chat -> same pattern history. Different accounts -> no correlated digest or shared strikes. Missing/wrong Bot owner/profile/installation authority denies before commit; transfer charges old committed sends to old owner and fresh sends to new owner.
T4: five different captionless photos -> zero hits; identical nonempty captions qualify; E2E ciphertext never hashed. All three classes advance post-mute count only for actual successful commits; ten ineligible sends do not disable later text detection.
T5: delayed ordered/reordered messages use source-time windows; a hit just beyond thirty days starts a fresh first strike; the inclusive boundary produces the second. Mute expiry is application timestamp plus one hour. Duplicate ACK loss never moves expiry. New distinct pattern hit may extend effective mute.
T6: out-of-order n+1 buffers; n applied/expired terminal receipt closes the gap once. Same sequence/hash replays; same sequence/different bytes conflicts while receipt retained; a retired lower sequence cannot apply after receipt GC.
T7: data at exact source+thirty-day evidence deadline is expired, not sanctioned. Expiry receipt vs original payload race yields one terminal outcome, correct source progress and explicit EXPIRED_EVIDENCE metric, never a false scan/pass.
T8: inject failure at EACH local transactional write; restart after commit/ACK loss; verify exactly one hit/sanction/outbox action and unchanged state on rollback. Concurrency spans two profiles, patterns and consumers.
T9: rotate key during sample/strike window; dual digest counts once; old delayed event and current alias share history. Key removal before required horizons rejects startup/retirement rather than resetting state.
T10: old clear-restriction request delayed until after newer ban -> Auth ignores it; revoke one of multiple bans does not activate; deletion beats both ban and clear revisions. Restore before purge retains unexpired restriction; post-purge cannot restore.
T11 (c19 refinement): known winning terminal plus source gap/pending evidence/missing ACK -> independent local cleanup; opaque fence rejects retries. Unknown deletion or unseen winning restore while every authority path is partitioned -> no guessed terminal/global reset/physical deadline PASS; record unresolved or missed deadline and deny affected authority, then reconcile and clean on recovery. A stale DELETED timer cannot terminalize a possibly restored live account. Restored old backup stays unready until independently current terminal history is imported and cleanup applied. See U1–U14 in the c19 owning contract.
T12: every enabled insertion path (including forward commentary, attachments, ghostOnly and Bot game) produces one private record on real insert, none on failure/replay. Wrong ingress credentials, metadata duplicates, stale epoch, raw identity, invalid TLS/JWKS or replay never reach domain writes. Shared public message stream/logs/analytics contain no new account/digest/plaintext leakage.

7. IMMEDIATE IMPLEMENTATION CONTINUATION
P1/P2/P3 are decided under the new delegation; no question to the user and no decision-only hold remains. Рудеус-156 may resume a fresh isolated checkout on the actual current immutable integration base. Before production code, persist this explicit-c2 versus autonomous-decision distinction and exact RPC/schema/retention contracts in docs/features/reports.md, docs/microservices/moderation-service.md, messaging-service.md, bot-service.md, auth-service.md and the affected DATA_STORES/CONTRACT_MATRIX/architecture sections. Use a focused ExecPlan and coherent transaction seams, not an isolated disabled component.
Deliver the complete activated path with the producer authority, Bot owner check, account lifecycle/terminal snapshot integration, private atomic outbox, protected consumer, existing-state send gate, Auth revision fence, and durable sanction events. This is a cross-service vertical with one implementation owner; ordinary implementation returns to the standard ant. A genuine source incompatibility found on the refreshed base is an implementation finding to resolve, not a reason to reopen these product decisions.
Validation is scoped RED/GREEN and independent source review against T1-T12/N1-N14, affected Go/Java checks and protobuf checks in owned permitted Linux/Docker runners. No Windows test executables, host listeners, GH probes or shared DEV/STAGING operations are authorized by this decision. Deployment/tester/resource gates remain separate. No tests or source implementation have been performed in this specialist continuation.
Resource closeout: no stand claims, no worktree lease, no running subprocess or tool operation, no changed source files. Only this local decision artifact and TT handoff are written. c7 remains historical investigation evidence; this document is its policy/contract supersession.
# Finite recovery artifact inspection

## Auth census and terminal linearization checkpoint

The private account census is finite (10,000 sorted bindings), nonterminal and
Moderation-only. The real native caller test first failed when Messaging could
enumerate it (RED 2.881s), then passed with exact method allowlists. Java and Go
share the independently framed census vector in the startup/restore contract.

A real two-connection PostgreSQL race reproduced the old RR snapshot returning
a permanent terminal subject while a valid pre-deadline restore held the owner
row lock (RED 17.38s). Auth now settles finite due work under current owner locks,
rechecks fresh DB time/state, and seals only its durable terminal ledger using
protocol2. The native Linux Maven run passed all eight scoped principal,
store/census, protected transport and restore-race tests with zero skips/errors/
failures. Surefire XML is preserved in the ignored checkpoint evidence directory.
Go wire/client race tests passed (1.046s/1.137s); protobuf lint passed.

These are source/fixture proofs, not P1-P5 completion or physical P30D acceptance.
The [c19 owner lifecycle producer and partition correction](async-automod-unknown-deadline.md)
and [c20 bridge-feasibility review](async-automod-identity-bridge-feasibility.md)
remain required owning contracts. Transactional lifecycle delivery, exact
recipient cycles, positive terminal catch-up and actual activation remain open.

The first file-verifier stage checks a bounded immutable bundle against an
independently supplied expected operation, deployment generation, environment,
store identities and required artifact kinds. It rejects duplicate/unknown JSON
fields, unsafe or symlinked inventory paths, unlisted files, incorrect lengths
or hashes, and mixed operations/stores. Artifact bytes are retained from the
verified read, so later file replacement cannot change the inspected input.

This stage returns inspected artifacts, not a `VerifiedRecoveryCut` or a boot
phase receipt. Native quiescence/drain, lineage/current-tail verification,
domain replay and participant checks remain separate required steps. Neither a
valid bundle nor an operator-provided `complete` flag opens the gate.

Actual Linux RED reproduced hash-only acceptance of a foreign operation,
environment/generation/store, missing coverage, unsupported method, duplicate
JSON fields, extra files, duplicate artifact kinds and unclean paths. Strict
inspection now passes these negatives plus nested duplicates, truncated/hash
and size/schema errors, mixed methods/stores, parent/absolute/Windows paths,
file/directory symlinks, and immutable byte/metadata readback. Scoped `-race`
PASS 1.190s and `go vet` PASS. This is artifact inspection proof only; no
restore operation, current-tail proof or P5 completion is claimed.

Recipient cycle binding slice (BE-045 c19, in progress): retain Auth's exact
`deletion_cycle_id` in Moderation's subject binding, including report privacy
projection. A DELETED binding requires a nonzero cycle and microsecond deadline;
LIVE has neither. Equal revision requires equal state, cycle and deadline.
Retained rechecks compare that entire tuple using fresh database time; a newer
LIVE restore clears the previous cycle without resetting detector state.
Migration 9 adds the nullable cycle column for existing rows without inventing
legacy cycles; pre-activation recovery must refresh any legacy DELETED binding.
First demonstrate the old implementation discarding a changed transported cycle
at the same revision, then verify database binding, retained recheck and restore
negatives in the private Linux PostgreSQL fixture. This slice does not provide
the receiver, lifecycle publisher, boot coordinator or physical P30D proof.

The bounded [whole-copy/key/owner-time review](async-automod-whole-copy-custody.md)
(BE-045 c28-c32, Astra162) selected no candidate under the named faults. Local
cleanup, all-copy destruction by the deadline, and restored-copy scrubbing are
three different evidence levels. Current restic/PVC/config/dump custody and a
paused sole owner do not supply the latter two by a readiness flag or key TTL.
The physical P30D acceptance criterion stays open under unchanged policy; this
does not stop the separately testable lifecycle/recipient/current-owner ACK work.

Cycle slice evidence: the old binding accepted a different transported cycle
at the same revision/deadline (Linux PostgreSQL RED 0.835s). A second negative
showed that even a greater revision could extend the same cycle (RED 12.558s;
that run also recorded a teardown timeout, with no database left afterward).
Binding, provision, refresh and retained checks now preserve the exact cycle;
the database also rejects rewriting the deadline of the same nonnull cycle.
The final scoped PostgreSQL race run passed in 58.252s without failed/skipped
cases; all Moderation short tests and `go vet ./...` passed. The frozen source
tree is `57091a7e9e8ab7b9a4ac6bee73198d97fab8e214`; portable before/after,
archive, raw JSON/log and receipt evidence is in the task checkpoint.

The broader 61.117s probe remains FAIL: the existing legacy `CheckMessage`
path still creates an offence for uncommitted text. Its regression remains a
required vertical fix, not a cycle-test PASS. Legacy DELETED bindings without
a cycle remain unavailable to ordinary equal-revision binding; the boot
coordinator still needs an explicit fresh-Auth repair without deleting/recreating
their detector stream. Receiver, producer dispatch/current-owner ACK, lifecycle
repair and real P1-P5 activation remain open. The own Graphify AST update was
canceled at terminal exit 1 after no progress; no AST verification is claimed.

Develop integration checkpoint (immutable input `c850c2e4d56b2130cf739dbcddde8faf6d72a663`):
preserve the existing Moderation caller restrictions and shared issuer together
with the incoming fixed Space-admission method and capability. Regenerate the
Auth/User Go and Dart mirrors; buf lint and scoped Go tests/vet pass. The initial
User root check lacked its exact root Compose fixture; its same-source rerun
passes after supplying that tracked file. Other successful package results are
retained, not relabeled as one uninterrupted full run.

Auth compilation and the final aggregate XML report 64 cases with no failure,
error or skip. The first run had eleven resource-fixture errors: archive mtime
zero prevented Maven from copying TLS test resources. Copying those resources
with corrected timestamps leaves all source bytes unchanged. One remaining
two-second mTLS fetch timed out at one CPU; the same compiled two-case class
passes at two CPUs without changing TLS policy or timeout. This is an explicitly
qualified aggregate, not a single clean 64-case execution. Source tree
`5320d6de601a0bf891301e7967bcec17415d7204`, archive and native before/after
inventories are preserved in the portable merge receipt. Graphify was canceled
at terminal exit 1; no AST pass is claimed. These checks do not activate P1-P5
or satisfy physical P30D acceptance.

Independent cycle review identified a separate migration-9 DOWN race: the
emptiness check must exclude concurrent writers through the destructive DDL.
Keep the existing source review open and fix that guard with a two-connection
negative before claiming the cycle slice clear.

Rollback-guard correction: independent reviews 160 and 323 exposed the same
check/write race in Moderation migration 9 and Messaging migration 31. Both DOWN
scripts now explicitly begin a transaction and take writer-excluding table locks
before checking retained authority. Messaging locks message-trigger admission
before its history tables. Locks remain held through the destructive DDL. The
two-connection regressions hold an uncommitted first cycle/key while DOWN starts,
observe the actual PostgreSQL lock wait, then commit the writer. The old scripts
returned success and discarded that commit (RED 0.895s and 3.336s). Corrected
scripts refuse rollback and preserve the committed value plus guard triggers;
empty-authority rollback remains valid. Scoped Linux PostgreSQL race results:
Moderation three cases PASS 2.366s, Messaging two cases PASS 4.571s, no skips.
Independent exact-delta rechecks are still required; this does not clear the
whole vertical or the physical-deadline criterion.

Next activated-vertical slice: register the Auth-only lifecycle receiver on the
existing private P0 listener. Raw callers must be unauthenticated; a correctly
signed Auth call before P2 must be unavailable without touching a database or
waiting for public readiness. After P2, the receiver must consume the locally
verified current Auth authority generation, persist monotonic opaque facts and
exact hashes, and update any existing binding under account-before-subject locks.
A due DELETED fact cannot create a terminal fence. A positive PURGED fact must
commit all mapped and subject-only participants with its permanent fence before
acknowledging. Unknown-subject facts must fence later stale binds without
inventing raw identity. Local state ahead of an incoming fact must not be
overwritten or acknowledged with a fabricated newer hash. Test the real missing
private route first, then transactional replay/conflict/restore and participant
rollback. Migration 13 is reserved for this receiver; Messaging 32 belongs to
the accepted BE184 consumer. Actual coordinator/current-owner ACK and P1-P5
activation remain required consumers of this slice.

Receiver checkpoint: the real missing P0 route produced Unimplemented (RED
1.016s); the Auth caller matrix rejected that exact method (RED 0.213s). Both
are now wired narrowly. A further unknown-binding negative accepted a changed
same-cycle deadline (RED 0.343s), now refused in the opaque projection as well
as the raw binding. The final frozen tree is
`050bf64ab32585dd9812c51232174c2203f226d5`; all 4241 Git/archive/native
before/after bodies match. Linux PostgreSQL/RPC/P0 race results: 18 top-level
plus 6 subcases PASS, zero skip, 21.539s for the integration package and 2.113s
for the private-runtime package. Principal/wire/client short-race checks pass
40 top-level plus 135 subcases, with one explicit integration skip. Moderation
all-short and vet, and scoped shared-package vet, pass. Real migrations 1-13
apply cleanly, metadata version 13/dirty=false; the fixture database was removed.

Keep the qualifications: the first final driver stopped before tests because
its path list used CRLF, then the pathname fixture was corrected. An overly broad
non-short shared-package run reached an unavailable nested Docker fixture and
failed; it is retained as infrastructure failure, not a TLS/Redis runtime pass.
The scoped short run reports its skip explicitly, while the separate P0 TLS and
PostgreSQL/RPC cases execute. Initial receiver iteration corrected typed conflict
normalization and a stale test callback capture. Graphify was canceled at exit 1.
Portable receipts include these failures. This checkpoint still has no actual
Auth lifecycle publisher/current-owner ACK, bootstrap coordinator, public P1-P5
activation or physical P30D acceptance. The broader legacy CheckMessage failure
above also remains open.

Auth publisher next: reuse the one Auth service issuer and permit only the exact
lifecycle RPC for recipients Messaging and Moderation. Capture a bounded batch
of complete current/terminal intents outside network calls. Send outside every
database transaction; after response, lock the actual current Auth account (or
immutable positive terminal ledger), compare current generation, revision and
canonical payload, then CAS only the requested event intent. A newer owner
transition leaves its new intent pending. A peer ahead of CURRENT owner state,
changed same-revision hash, or unowned terminal response is restore divergence,
never an ACK or a reason to increment/terminalize local state. Test the fixed
signer purpose first, then real PostgreSQL transitions during network latency,
strict receipt negatives and actual mTLS transport. The Scheduled terminal worker
also needs the verified own-authority bootstrap barrier before restored due work;
its present JDBC-only activation is not a complete startup/restore solution.

M6 downgrade correction: the original guard inspected machine sanctions only,
so existing permanent terminal history and a concurrently committed first fence
could both be discarded. The downgrade now takes ACCESS EXCLUSIVE locks on all
owned state and sanctions in an explicit transaction before checking any history.
Any detector, receipt, event, restriction or terminal row refuses downgrade;
unused state may still be removed while preserving human sanctions. Actual Linux
PostgreSQL race proof: two failing baseline cases (2.659 s), then three passing
cases (4.036 s), including the concurrent first terminal and empty downgrade/up.
Scoped go vet passed. This corrects rollback safety only, not P0-P5 activation or
the open physical P30D criterion.

Auth lifecycle delivery component: fixed shared-signer purpose test first failed
with the missing method, then passed with exact Messaging/Moderation audiences.
The Java codec matches the independent 120-byte Go hash and rejects malformed
facts/receipts. Seven native PostgreSQL delivery tests passed (14.637 s, no skips),
including a concurrent uncommitted owner transition, newer-intent preservation,
unowned terminal/ahead response denial, terminal-ledger ACK and generation-check
rollback. Four actual private mTLS/configuration tests passed, including anonymous
and wrong-CA refusal before the handler. These component checks do not yet wire
the scheduled publisher, finite catch-up, Auth own-authority bootstrap barrier or
public issuance gate. Exact frozen-source verification follows before review.

P1 source-stage checkpoint uses the existing c13/c23-c24 operational boundary:
an operator-installed private read-only startup context explicitly selects
LIVE_CURRENT or DECLARED_RESTORE and binds environment, deployment generation
and expected store incarnation/authority generation. Absence or malformed input
never selects live mode. Auth V32 / canonical auth_db33 adds an immutable local
Mod-purpose store incarnation, distinct from the authority generation and from
Role-purpose metadata. This UUID identifies the connected store only; it cannot
prove currentness or detect an undeclared rollback. Intact LIVE_CURRENT still
requires finite native integrity checks. DECLARED_RESTORE must remain closed
without the independently operation-bound actual recovered domain cuts from
c23/c24. Private authority, scheduled terminal work and lifecycle publication
must capture this boot's verified authority check and recheck before commit;
opening P1 does not open public P4 or supply physical-deadline acceptance.

The production configuration now injects the closed current-boot authority into
the store and terminal worker; the old JDBC-only worker first failed its native
mock oracle (one failure, 4.320 s). Seven native PostgreSQL/gate/context tests
then passed (21.703 s), including altered native catalog denial and generation
closure rolling back terminal ledger/mapping changes. Two actual mTLS tests
passed (11.318 s): the private listener is reachable but denies authority before
P1, and terminal cleanup completes while a separate publisher peer RPC waits.
Final source binding/review follow. No new product scope is added after the user
master-priority override. Preserve this checkpoint separately from the reviewed
downgrade components; restore verification, finite catch-up and public P4 are
still unfinished, and the total-partition/all-copy physical criterion remains
open under c19/c20/c28. Sole master integration/CI belongs to 121.

Preservation boundary after the master override: keep the already-authored P1
prototype on a separate source-stage branch; the original branch's reviewed
components and downgrade correction remain independently consumable. P1's
quiescent local catalog check is not a function-DDL barrier: table AccessShare
locks alone do not qualify concurrent CREATE OR REPLACE FUNCTION or rolling
schema cutover. That named race still needs a production fence and finite oracle
before P1 activation, alongside the already-open restore/P4 requirements. No
claim that the present prototype is a complete serving-readiness solution.
