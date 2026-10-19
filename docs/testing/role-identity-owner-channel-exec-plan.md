# ExecPlan: Role identity owner channel

Task `BE010-IDENTITY-ERASURE-OWNER-CHANNEL`; developer Рудеус-172,
gpt-6.1-sol/medium, focused context. Base
`f3feccef198b444699e66c960b84dc9c77ba4bc0` plus ordinary Auth deadline merge
at preparation `53ab5d981f8bc5a631c920e2d508acb23af7f753`, branch
`codex/role-identity-owner-v1-172`. Consumers are Role identity erasure214,
generic Role ledger183 and recovery215. This is an enabling owner capability,
not another account deletion feature or a shipment claim.

## Purpose

Role can remove private actor and target history only after the actual owner
proves an irreversible identity boundary and complete historical coverage.
Role must also deny new operations for currently ineligible identities without
treating a temporary archive as irreversible erasure. Restored Role history
must be reconciled against independently current owner cuts before serving.

## Context

Sources: `AGENTS.md`, `.agent/AGENTS.md`, `.agent/PLANS.md`, PLAN A2 and the
account-lifecycle boundary in A4; `features/auth-and-contacts.md`,
`features/multi-profile.md`, `features/user-profile.md`, Auth/User/Role service
docs, `DATA_MODEL.md`, `DATA_STORES.md`, `ARCHITECTURE_REQUIREMENTS.md`,
`TESTING.md` and `CONTRIBUTING.md`. Auth stays Java; User and Role stay Go.

Auth has a canonical account deletion deadline and a 30-day restore window.
The separate unintegrated Auth moderation producer206ec949 has opaque lifetime
subjects, lifecycle revision, deletion cycle and deadline, but only
Messaging/Moderation caller permission and no sealed Role restore proof. Its
package, `automod.proto` and listener belong to156 and are not this write scope.

User `store/account_lifecycle.go` records an inactive overlay and Search
deletion. `ListByAccountID` excludes archived profiles. DeleteProfile archives
the profile; no standalone profile erasure grace is defined. Existing SDK/User
authority returns scoped current facts, not complete historic account coverage.

Consumer214 at `ff5db163c333d24b3a125ae97f5ac83a70ed7492` has local
`identityerasure` interfaces only. It requires canonical typed Account/Profile
selectors, positive lifecycle revision, exact already-due EraseAt and complete
authenticated owner/checkpoint/revision coverage. Its SQL sink must recheck DB
time. Consumer215 at `f416de715842f14d4e0cea6d135bf9c37e3dbc47` requires an
independent CURRENT recovery watermark bound to target/generation/stream/cut,
digest/keyset/owner revision. There is no agreed Auth/User wire yet.

## Scope

In: separate explicitly Role-authorized Auth/User owner capability, local
durable minimized historical identity selectors, exact lifecycle and current
eligibility observations, immutable complete cuts and supported recovery proof,
consumer adapters, bounded real transport/storage/crash/restore tests and docs.

Out: broad BE019/BE020 implementation, existing Moderation caller expansion,
Search authority reuse, raw-ID permanent tombstones, client disclosure, new
profile grace, public deployment/provisioning and production activation.

## Milestones

- [x] Atomically claim the actual eligible owner-channel task; inspect immutable
  producer/consumer sources and coordinate156/183/214/215/225 write seams.
- [x] Request genuine architecture/security review162 for named historical
  completeness and independent CURRENT restore-cut uncertainty (TT c2).
- [x] Acquire own WT36 from actual develop; no remote probe or pool expansion.
- [x] Persist162's final source-backed contract and birth-lock addendum.
- [ ] Freeze actual consumer field agreement; current private protobuf is draft.
- [ ] Add a meaningful failing real-SQL/transport oracle for each owner behavior.
- [ ] Implement coherent owner storage, typed protected transport and adapters.
- [ ] Verify actual SQL and transport, crash, partial-cut and rollback negatives.
- [ ] Freeze source and portable evidence, obtain independent exact-head review,
  normal publication/CI through121 and supported integration/delivery handoff.

## Detailed steps

1. Implement the accepted separate Role-purpose contract TT c4-c9. Auth locks
   canonical DB lifecycle/deadline and durably records a
   purpose HMAC selector before raw erasure; User profile archive means current
   eligibility deny and NOT_ERASABLE, not a new deadline. Profile erasure needs
   explicit irreversible User transaction or terminal parent-account proof.
2. Record owner-held HMAC(account)/HMAC(profile) association atomically at profile
   birth before hard deletion. Baseline existing live and archived rows with
   explicit incompleteness for unprovable legacy hard deletes. Role submits its
   actual raw target inventory; no IDs are reconstructed or enumerated from
   hashes. Use accepted162 purpose-separated keys, all retained versions and
   causal CURRENT vector cuts; no accepting fallback for missing owner material.
3. Select additive migration numbers only after actual develop/peer readback.
   Current reservations:172 AuthV28/canonical29,290 canonical User22;156 reserves
   ModV29/canonical30, preserving ModV27/canonical28.225 owns additive Space floor methods in auth.proto and Auth floor
   classifier/verifier/registry, with no User migration. Prefer separate new
   owner DTOs and namespaces; coordinate any adjacent registration hunks before
   editing.156's moderation package/proto/listener remain untouched.
4. Add tests from the final contract before production implementation. Use real
   owned Linux PostgreSQL with no host ports and actual bounded mTLS/JWKS/replay
   composition. Fail for missing behavior, then implement the smallest coherent
   transaction/transport slice. Helper/fake-only green is insufficient.
5. Connect214's private Authority adapter and215's exact restore boundary.
   Network work occurs outside ledger transactions. Owner and consumer DB-time,
   revision, completeness and permanent-terminal checks remain authoritative.
6. Update owning canon and this plan after each meaningful result. Regenerate
   only actual changed DTO mirrors, check wire compatibility, and update AST
   graph after source edits. Preserve immutable exact-head evidence and account
   every owned command/resource before guarded lease return.

## Validation

Required future oracles, none reported PASS:

- Actual Role-only mTLS and signed method/body/audience binding; wrong caller,
  borrowed Moderation purpose, replay and wrong CA/SAN cannot read owner proof.
- Real owner SQL distinguishes LIVE, archived, unknown and irreversible state.
  Deadline comes from locked owner lifecycle, with equality checked by DB time.
- Historical target-only Role rows include primary, secondary, archived and
  hard-deleted profiles. Missing legacy coverage stays incomplete/unavailable.
- Restore before the canonical account deadline remains owner-controlled;
  irreversible cuts, older revisions and stale LIVE never reopen erased state.
- Mixed/incomplete pages, cuts, counts, digests, owner generations, duplicate or
  missing selectors and expired observations fail closed. True initialized
  empty genesis differs from a rolled-back empty database.
- Crash at each journal/cut/receipt step is idempotent. No reciprocal RPC holds
  a database transaction or lock. Successful receipts mean committed cleanup.
- Auth-only, User-only, Role-only and all-old restore cases require independently
  current material. Missing retained keys or incomplete recovery cannot open.
- Minimized records and logs contain no display/content, reversible mapping or
  raw-ID permanent tombstone. General staff cannot compute purpose keys.

Checks will use repository Go1.26/Java25 and appropriate scoped vet/lint/codegen
contracts in owned Linux Docker. Exact commands and case counts will be recorded
when the final contract fixes files and fixtures. Hosted CI, deployed network,
shared-stand recovery and final acceptance remain separate gates.

## Progress

Current implementation and runtime evidence supersede the historical preparation
notes below. Genuine162 c4-c9/c12-c15 is accepted. WT36 lease
`860fd1af6cd5718f32a4f33c868e7e8f`, holder `rudeus-172-identity-owner`.
Realtime645d9c and Auth deadline73437b1 full task/source/CI/delivery custody are
already transferred to121; neither remains172's integration responsibility.
Owner-channel source is uncommitted WIP. Auth selector5 and User selector4
runtime tests passed after missing-implementation RED. Draft protobuf lint and
local Go/Dart generation passed; new Go PB compiled with no tests.

Native Auth5-case RED against native26 failed on the missing new keyset function,
all5 executed/no skips. V28/canonical29 is now authored and applies successfully.
First post-DDL run failed before behavior assertions: required password_hash was
missing in the fixture. Both normal and invalid-partial-capture inserts were
corrected; Linux runner85905 finished0, all5nativecases PASS/0skip at01:33:24Z.
V28/canonical29 exact byte SHA256 is
60304ed338dac5ab3d07def9d319dbc4142db018e5d2be05d20a7ca623441122.
The additional real psql catalog fixture
`scripts/ci/role-identity-auth-catalog-negative.sql` passed8negatives plus exact
keyset retry/rotated birth, and rolled back its entire transaction.
Own PG435cc297/networkeae742d7 has no host ports; no shared stand claim. After any
DDL edit use a fresh database, never Flyway repair. Next run immutable156 V27
compatibility on another owned database, then integration with290 User22/all births, Auth issuance
preflight split, protected transport and actual consumer adapters. Missing
independent CURRENT material and legacy history remain explicit unavailable.
Graph69809 completed; final source still requires update. Concrete fixture
CodeGraph call cancelled after31seconds without result; actual source verified.

## Decisions

- Separate explicit Role authority preserves existing Mod-only authorization.
- Profile archive is reversible backend retention, not irreversible erasure.
- History cannot be reconstructed by live-only lists or UUID-as-account guesses.
- Mutual consistency of stale backups does not prove current owner truth.
- Parent supplies genuine specialist capacity;172 does not spawn/follow up ants.

## Risks and follow-ups

Final CURRENT vector-cut and purpose-key contract is accepted162 c4-c9; missing
actual independent current capture remains an implementation/evidence gap.
Unknown legacy hard deletions are a real completeness gap, not permission
to manufacture a clean genesis. Activation requires complete owner material and
agreed183/214/215 integration. Production KMS provisioning, backup authority,
deployments and shared DEV/STAGING recovery are not granted by local green
tests. No partial or disabled foundation may be presented as feature completion.

## Final162 decision accepted (supersedes preliminary pending notes)

TT c4-c9, immutable artifact role-identity-owner-contract-v1.txt SHA2567000fb5b7cb52db50d10ad9482d18ffb5b5e9638634532b6bf912c5cf64683ba. No routine product question remains. Separate Auth Role and User-only parent-handle methods, User Role-only observations/history coverage and separate listener namespaces. Exact fresh-after-lock Auth deadline and all-writer guard; independent existing repository defect tracked AUTH-RESTORE-DB-DEADLINE. User archive is NOT_IRREVERSIBLE. Capture User own purpose account/profile HMAC association plus distinct opaque Auth handle at EVERY birth, parent RPC outside transaction, local lifecycle recheck and journal/domain atomicity. Missing legacy association stays INCOMPLETE; due raw cleanup cannot wait for keys or Role ACK. Owner append-only revision/cuts, retained complete keysets and causal Auth-to-User import bind the independently current Role/Space recovery vector.172 supplies owner components, not183/215 independent Role journal or native sink. Conditional selector GC requires Role complete acknowledgement plus supported backup destruction/horizon and no dependent cuts. Full Q1-Q14 matrix is authoritative, including real SQL/transport/crash/restore.

Current migration reservations:172 Auth FlywayV28/canonical000029; canonical User000022 transferred to290 by accepted162 c20-c24 and172 c25;156 Mod bootstrapV29/canonical000030, existingV27/000028 preserved;225 no migrations. Mod recovery144/163 uses separate private purpose/DTO and may reuse canonical User lifecycle facts only after exact API freeze; no Role selectors/keys/journal or new Mod raw mapping archive. Actual producer API is not implemented or published yet.

Birth-lock addendum162 c12 summary/c13-c15 full accepted. Read complete19305-byte artifact; preserved owncopy role-identity-birth-lock-addendum-v1.txt SHA256 a87cf9566c1ba2ceac9f73ab9f6e8d078c01498e3fb1c6647f260f2b3fe5ab83. Implement preflight outside Auth account transaction and callback-needed process monitor, private final signer without User I/O. Includes convertGuest, OAuth exact original-code origin then final floor/consume/full bindings, verified-email issuance and verify2FA (NOT enrollment-only enable2FA). Preserve B1-B12 real callback barriers, one-time/security gap tests and existing refresh failure semantics. No owner-channel production implementation/PASS yet; separate native Auth deadline regression is currently RED-confirmed and GREEN executing. User cannot call Auth from inside its local birth transaction. Actual issuer bean156 must be reused by reference with exact method coordination, no second Auth key namespace.

## Active implementation checkpoint 2026-10-19T01:00Z

Preparation branch superseded by codex/role-identity-owner-v1-172, actual develop f3feccef198b444699e66c960b84dc9c77ba4bc0 plus ordinary merge73437b1 Auth deadline at53ab5d981f8bc5a631c920e2d508acb23af7f753. A1 bug exact734 has independent265 SOURCE CLEAR (parentc5/reviewfunctionc2), final15Linux tests/0skip, portable restored bundle, normalSSH publication. Full custody handoff to121 offered, not yet accepted at this checkpoint. Realtime full custody already transferred121; old preparation statements do not retain it. No ownPG/network exists now.

Separate new identityerasure/v1/identity_erasure.proto authored with service suffixes and existing buf STANDARD lint PASS. It is a draft, not an agreed/published producer version; no auth.proto/user.proto grant is added.271 separately owns admission observation methods under their explicit purpose. Auth/User selector implementations are private new namespaces, disjoint from all existing secret families. Chosen canonical codec is HMAC-SHA256 of ASCII voice-role-identity-selector-v1 then NUL/owner/NUL/kind/NUL then16canonical UUID bytes; User domains separate account/profile and Auth. Versioned owner keys are32-byte supplied secrets; all retained versions required by owner history are checked, missing key failsclosed, and input/output aliases cannot mutate retained keys. No production KMS/key provisioning is claimed by this helper.

Runtime RED before helper implementation: UserGo compiled4cases, all4fail with explicit pending implementation, runner75917 terminal1. AuthJava compiled and executed5cases with pending implementation failures/errors, runner47176 terminal1 (actual count in log). These are new-API missing behavior proofs, not a claimed vulnerability reproducer. User helper GREEN4 actual cases terminal0; Auth GREEN17747 active. Runtime uses owned Linux networknone, Maven25/Go1.26, no host listeners. Helpers are an intermediate layer of full task, not completion/readiness. New owner storage, native guards, protected transport, full birth choreography, actual214/215 adapters and Q1-Q14/B1-B12 proofs remain to implement. No stable consumer API published yet. Graph69809 terminal0; later final update follows final source, no duplicate live updater.

## Current build continuation

The preceding timestamped checkpoint is historical; the Progress section above
is authoritative for completed proof and121 custody. Auth17747 finished GREEN5,
User vet finished0. Native catalog GREEN5 log is durable TT attachment
a4d92f34e91a428cdfb829d380565b30, parentc17. Compatible156V27 run72182 stopped
before tests: full Java recompilation with protoc.skip did not include generated
source roots. This is a recipe failure, not a compatibility result.

The real JDBC birth test now exercises both repository INSERT methods with DB
capture enforcement enabled. Its overload intentionally has no capture yet for
runtime RED. Corrected generation uses cached public Linux protoc4.29.1 and
grpc-java1.68.1 from own read-only tools volume. Windows bind generation69496
was deliberately stopped during build via its verified own Maven PID19;
terminal1, no runtime result. Exact input snapshot is in own
rudeus172-owner-auth-native-source Linux volume, source hashes verified.
Native-volume RED runner31652 now compiles671main/165test Java sources including
the new private protobuf. No duplicate active build remains. Logs/XML are copied
to WT36/tmp/rudeus172. After actual RED, implement the two captured INSERT paths,
repeat the focused test then all native cases including fresh156V27 compatibility.
Do not claim producer readiness from this intermediate catalog/birth stage.

Birth integration completed: missing-capture runtime RED1/1error/0skip, then
native GREEN6 at01:50:23Z. Fresh database with exact156V27
da32e423267b418f1856eced453bac369a6d3faa13f2bb5a49a66b4b79cec880 applied28
Flyway migrations and passed6/0skip at01:52:24Z. Psql8negative fixture also passed
with V27; it rolled back all data. Durable c19 attachments include proof manifest
9d9876666aa24fd3cad337726b584e20 and compatibility log0c6eff0f7d897f850b561344f4a79d70.

Before owner-read implementation, a stronger committed-journal oracle exposed
an eligibility commitment gap: savepoint branches for suspended versus guest
produce the SAME lifecycle revision/state but distinct eligibility. Original
V28 emitted identical digests (actual RED1failure/0error/0skip at02:00:17Z).
The journal also omitted the eligibility column (separate runtime RED1error).
V28/canonical29 now retain eligible and hash its canonical byte, with null framed
separately for keyset-only records. New exact DDL SHA256
d45a6d138a1856fffc7fa79834d1c3b175c6fa5ae1f8d768a5f20b19204e670e;
fresh owned auth_owner_v2 runs8cases in99013, no Flyway repair. Earlier6case proof
remains bound to old60304ed3 DDL, not silently relabelled current.

User canonical core decision162 c20-c24 is accepted, full29187-byte artifact read
and preserved as user-canonical-association-contract-v1.txt SHA256
88868ecf9b107ba4cf994f5f70ef68f94007524a79831bb6565ecc0a02b7c982.
Unwritten User22 transfers to290 (actual172 c25, direct290 ACCEPT);290 owns ONE
canonical identityassociation core/keyfamily/DDL/guards/NOW capture.172 retains
Auth parent/typed proof, sole shared profile.go all-birth integration, parent
terminal import and Role adapter. Removed only172 unfrozen User Role-domain
selector draft after actual consumer search; historical helper4PASS/vet source
is preserved externally, not a current canonical implementation. Null parent is
allowed only for retained NOW baseline; new post-cutover births need typed
nonnull parent. Role missing parent/prehistory stays INCOMPLETE. Permanent minimal
profile-HMAC reservations contain no account join and retain required lookup keys.
Current compile seam is being frozen directly290; no peer wait or second registry.

## Native Auth checkpoint before protected store

Final current DDL d45a6d13 native8PASS/0skip at02:12:02Z and exact156V27
compatible8PASS/0skip at02:10:24Z. Strong actual same-revision eligibility digest
RED is fixed; the compatible fixture uses real owner ban/unban and preserves
legacy suspension. SQL8negative fixture passed/rolled back on BOTH new schemas.
New User key/core ownership is transferred290 (c25), no duplicate User helper
remains. All Maven/SQL/codegen jobs are terminal. Graph35862 remains active,
no duplicate AST updater. Sole own PG/network/source+cache volumes remain useful
for next protected Auth parent store/native transport work; no shared stand.
No current full producer contract/readiness/source CLEAR/integration claim.

## Parent observation and isolated principal checkpoint

AuthIdentityOwnerStore now resolves one exact requested account using preloaded
owner keys, its own READ COMMITTED transaction, current account/lifetime locks,
and DB time after serialization. It rejects ambient caller transactions and
mismatched loaded key fingerprints. Unknown history stays INCOMPLETE; a known
terminal result binds the canonical deletion cycle/deadline and owner cut.
This single-parent observation is not a global CURRENT recovery certificate.
Actual native runtime RED was five cases (two failures, three errors), followed
by parent5 + selector5 PASS and a separate V27-compatible parent5 PASS. Portable
logs/XML and hashes are in tmp/swarm/rudeus172/owner-auth-parent/proof.json.

Normal merge 8b68aeee incorporates local develop candidate304b49dc, without
claiming hosted develop or CI acceptance. The native source snapshot was then
refreshed before principal checks. Isolated identityAuthority verifier policy
accepts only Role ObserveAccounts and User ResolveAccountHandle/ObserveAccountHandles.
It does not grant floor, admission, ownership-proof or delegated-user methods.
The dedicated interceptor factory rejects unknown and legacy methods before
creating a handler; existing constructors preserve their existing surfaces.
Actual exact-baseline missing-factory REDs precede native GREEN30 (five new,
fourteen existing verifier, eleven existing interceptor cases), zero skips at
2026-10-19T02:47:44Z. An earlier mixed stale snapshot compile failure is retained
as diagnostic output and is not the accepted baseline RED.

The prior Graph35862 and portable bundle restore are terminal/successful. New
CodeGraph628/636/646/650 queries were cancelled after31 seconds without an answer;
actual owned source supplied the implementation evidence. A new AST update is
required for this checkpoint. Next are protected handler/listener configuration,
typed User parent consumer, and all-birth lock choreography using290's canonical
IdentityTx API. No listener, full producer contract, source CLEAR, birth/runtime
activation or environment acceptance follows from these preparation checks.

## Parent adapter and mutual TLS preparation

The parent adapter now checks verified service:user, protocol1, canonical nonnil
UUID selectors/request ID, unknown fields, and body request ID equality with the
verified principal metadata context before reading the owner. Native adapter3 +
identity5 + existing interceptor11 passed19/0skip at02:53:18Z. Storage failures
return a coarse UNAVAILABLE response without internal identity/key/database text.

Separate AUTH_IDENTITY_AUTHORITY_ENABLED transport configuration rejects partial
settings and requires TLS_CERT_FILE, TLS_KEY_FILE and TLS_CLIENT_CA_FILE under the
same AUTH_IDENTITY_AUTHORITY_ prefix. GRPC_LISTEN defaults to :9094; an ephemeral
port is allowed only for explicit local/test profiles, always with mTLS. Optional
JWKS_CA_FILE supplies owner HTTPS trust. The existing S2S_JWKS_URLS_JSON supplies
exact role/user HTTPS endpoints and the canonical S2S cache duration settings.
Configuration4 passed/0skip at03:00:23Z after missing-class RED. TLS certificate
content is additionally validated by the actual TLS builder before server start.

Actual network-none Linux native mTLS3 passed/0skip at03:05:03Z after transport
implementation RED. A signed User call reaches the fixture owner once; replay,
wrong caller/raw/duplicate metadata/changed body, wrong server CA/SAN, missing
client certificate and plaintext do not reach it. This bounded transport proof
uses a direct RSA resolver and fixture observation: it does NOT establish HTTPS
JWKS fetching, real PostgreSQL+transport composition, Role ObserveAccounts serving,
keyset bootstrap, Spring production wiring, birth/runtime activation or fullQ1.

Preparation43566cc source was normally SSH-published by121 at03:05:16.603UTC,
without custody transfer. Its incremental bundle was verified and restored into
the prior portable2f15 bare repository as restore435; the complete prerequisites
are preserved there. Graph25293 completed106322nodes/221547edges, with the inherited
frontend Linux header warning. Later transport changes need another AST update.
Parent/adapter/config/TLS raw logs and XML remain outside the lease with SHA256
manifest at tmp/swarm/rudeus172/owner-auth-parent/proof.json. Full source review,
delivery and physical-copy acceptance remain OPEN;162 BE045 c28 NO_CANDIDATE
does not authorize a C+P30D lease promise, invented endpoint or retimed cycles.

Next selector bootstrap uses dedicated AUTH_ROLE_IDENTITY_SELECTOR_KEYS_DIR,
AUTH_ROLE_IDENTITY_SELECTOR_KEYSET_REVISION and
AUTH_ROLE_IDENTITY_SELECTOR_ACTIVE_VERSION. All three are absent or configured
together. Files v<positive-version>.key contain exactly32 raw bytes, with every
required retained version present. Keys load once outside SQL and remain an
immutable local snapshot; they are not workload RSA signing keys or Mod keys.
Registration must preserve every retained database fingerprint and fail closed
on stale/replaced/missing versions before capture enforcement is enabled.

Native selector settings4 + existing selector5 passed9/0skip at03:16:19Z after
missing-class RED. Bootstrap3 real PostgreSQL cases passed/0skip at03:19:26Z:
idempotent registration, both repository birth paths, retained-key loss/change
rejection without journal advance, raw uncaptured birth rejection and no ambient
transaction joining. Spring+realPG1 passed/0skip at03:26:18Z after exact missing-
configuration-class REDv2. It covers startup before cutover, both captured births
through the real AccountRepository bean after cutover, and failed startup when
already-required owner keys are omitted. A first testcompile inference diagnostic
was corrected separately and is not the accepted RED. Memory persistence does
not activate JDBC owner capture, and workload signing/Mod packages remain untouched.

Dependency merge c455560b incorporates c850's unrelated frontend/CI changes and
was normally SSH-published121 at03:32:33UTC. The Auth mutation source was unchanged
by that merge. Combined native compatibility uses exact156 ae4298ca migrations:
V27 SHA256 da32e423267b418f1856eced453bac369a6d3faa13f2bb5a49a66b4b79cec880,
V29 bdbb92dde4e56b2671832c6edbb95eca81b5221426a498c32f68a3572426d49e,
V30 bb84293efe66359f0b7c9fc85d25bce4ee9fc58bca16e3cd8aafbf9b4c4c05a9.
Role8 passed/0skip at03:35:28Z on all30 migrations. Parent5 subsequently passed.
Bootstrap initially exhausted its unchanged3s transaction budget before the first
SQL registration statement; the failed log/XML remain preserved. One unchanged-
source retry using an owned Linux-native copy of the existing Maven cache passed3
at03:53:14Z, then Spring passed1 at03:55:58Z. All accepted17 cases have zero skips;
no SQL timeout was relaxed. The cache-I/O explanation remains an inference.
156 confirms the independent opaque Role/Mod cycle UUID namespaces bind the
same canonical account deletion transition/deadline, without cross-purpose UUID
equality or authority borrowing. Source review/activation remains OPEN.

## Canonical User transaction integration preparation

Normal merge5701967f consumes290's immutable compile/API pin70c386c4. It includes
the private identityMutationTx bridge and strict owned Version22 catalog, but NOT
the unpublished User22 migration or its native fixtures. This is source preparation;
it establishes no executable schema, full-core readiness or birth activation.

The sole172 profile.go integration routes withSearchProjectionTx and its callers
through beginIdentityMutationTx. MarkAccountRegular, notification preferences and
free-tier freeze also enter that bridge before profile writes. Freeze reads the
ordered current roster only after acquiring I and applies its writes in the same
transaction; the existing primary/created_at order and free-cap rule are retained.
There is no old-schema fallback.290 retains the four adjacent SDK/verification/
Search entrypoint files and canonical core; no parallel edits are made there.

Linux compile-only completed successfully for internal/store and
internal/identityassociation with -run '^$' (no tests executed). A first RO Windows
module-cache run was stopped while scanning; a second cached-image attempt lacked
required modules with GOPROXY=off and failed setup. The accepted compile used the
existing host module ZIP cache as a read-only file:// proxy and extracted modules
into the owned Linux build volume, with network none. No source, versions or
runtime checks were relaxed; these attempts establish no PostgreSQL behavior.

## Auth owned-catalog runtime gate

An actual Linux PostgreSQL RED1/0error/0skip at04:18:14Z proved that disabling
auth_role_identity_account_guard still allowed owner observation. The new gate
checks six owned tables/column constraints, six normalized function bodies and
security attributes, three enabled triggers, and the exact successful V28
history entry/checksum. Access-share locks precede account/lifetime/head locks.
Metadata loads from an immutable resource outside SQL; observation statements
now also have the existing2s bound, with the3s transaction budget unchanged.

Initial catalog6/0skip passed on pureV28 and exact156 ae4298 V27/V29/V30. A second
actual RED1/0error/0skip at04:24:05Z showed a failed duplicate28 record hiding
behind its valid sibling. The corrected gate requires exactly one owned history
row and validates its success/checksum. Final catalog7/0skip passed04:25:50Z on
the combined schema. The negative fixtures restore every altered definition or
history record in finally blocks. Unrelated additive tables remain accepted;
no fixed global migration version or Federation catalog rewrite is introduced.
The unchanged owner-store positive/race suite passed5/0skip at04:26:47Z on a
separate combined-schema database with the final gate (12 final accepted cases).

Raw logs/XML remain outside the lease in owner-auth-parent. Catalog-specific
source review, protected production serving, bounded connection acquisition and
full producer readiness remain open; this gate is not independent CURRENT proof.

Before runtime qualification, real PostgreSQL barriers must demonstrate I-before-
profile/allocator lock order for these entrypoints on the coherent immutable22 DDL.
All births additionally require typed Auth preflight outside I and canonical birth
registration in the same transaction. Auth issuance lock choreography, private
serving, Role history/current completeness and whole-copy acceptance remain open.
