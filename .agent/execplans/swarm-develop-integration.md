# ExecPlan: assemble the Rudeus colony into develop

## Purpose

Collect the completed source results of Rudeus-1 through Rudeus-140 into one
history-preserving `develop` branch. This is the user's explicit 2026-10-18
instruction to finish current work and assemble its results. It does not claim
that incomplete feature slices become complete when merged.

## Context

- Tracker: project `voice`, task `SWARM-DEVELOP-20261018`, endpoint
  `http://192.168.0.109:30990`. Product tasks retain their author ownership.
- Canon: `AGENTS.md`, `.agent/AGENTS.md`, `.agent/PLANS.md`,
  `docs/CONTRIBUTING.md`, `docs/TESTING.md`, and each selected source task's
  accepted feature/service contract. The user's directive overrides the old
  A1-only admission restriction and explicitly selects `develop` as target.
- Sole physical develop writer and integration/CI owner: Rudeus-121. Main
  checkout remains on `master`; its pre-existing dirty files are untouched.
- Initial integration workspace (returned before review): `D:/Git/Voice/.treehouse/Voice-451d38/39/Voice`, own
  lease `4c53ae84671c0eb1384cc4def6a0ec6f`, holder `rudeus-121-develop`.
- Initial local/cached remote develop:
  `4d6e26608934b3350be68b71402aae54662eec51`, an ancestor of cached master
  `698d4f064f032ea74abf3ede2017100b26c8ee69`. There were no develop-only commits.
- Reproducible local inventory under `tmp/develop-integration/`: `tasks.json`
  contains full TT source/check records, `task-index.json` the 670-task index at
  initial revision 1133, `refs.txt` local refs, and `candidates.json` commit-to-TT
  evidence associations. Portable copies/bundle must be preserved before return.

## Scope

Include already frozen colony sources and final successors from current authors,
with ancestor deduplication. Read-only independent review of unreviewed final
ACL/client heads is part of this integration. Exclude unrelated/user branches,
uncommitted source, deployment, acceptance appointment, and blindly closing old
PRs. Preserve actual full-feature/runtime/hosted-CI gaps in TT and the manifest.
No new agents or ordinary implementation tasks may be started.

## Milestones

- [x] Finish BE-102 review, record narrow source CLEAR at exact `0361d8597` (c9).
- [x] Atomically claim integration task and acquire exclusive worktree lease.
- [x] Inspect existing develop history and merge cached master with merge commit.
- [x] Inventory final source heads, associate TT evidence, dedupe ancestors.
- [x] Merge ready dependency groups and resolve conflicts against accepted canon.
- [x] Independently review current final identity/ACL source heads before merge.
- [x] Execute assembled contract/migration/component verification and classify
  actual failures; full global CI remains FAIL, not a release sign-off.
- [x] Regenerate final graph and preserve exact source6739 complete-history bundle,
  actual fresh bare restore and portable manifest/evidence outside the lease.
- [ ] Reconcile live remote develop and publish normally after transport recovery;
  independent review and qualified CI/activation gates remain OPEN.

## Detailed Steps

1. Reconcile author final messages with TT immutable checkpoints and local Git
   objects/bundles. Do not take every local branch as colony work. Heads already
   ancestral to cached master/develop are accounted without redundant merges.
2. Integrate independent reviewed leaves first. Preserve every source commit;
   use `git merge --no-ff --no-stat HEAD -m ...`, never squash/rebase/amend/reset.
3. Assemble shared Role/Space/Chat groups in dependency order. Role Admin17
   precedes producer18 and entry-policy19. Role20 was released: admission DTOs
   and receipt codec are contract-only, with no schema/receiver activation.
   Space nickname26 and audit27/29,
   composite28, entry-policy30, composite audit-ID correction31 require their
   exact final author order. Do not invent or silently renumber migrations.
4. Preserve all additive protobuf fields/RPCs. Resolve source proto conflicts
   first, then regenerate canonical Go/Dart consumer trees as one assembled
   result. Do not choose one generated side over another added field.
5. Integrate BE-102 wire before BE-106 committed journal encoder adapter.
   BE-256 pending-intent safety and ordinary media identity mapping both edit
   `call_providers.dart`: retain their separate imports and distinct logic.
6. For a material conflict, record exact parents/hunks and consult the owning
   author. Original authors review integration-authored behavioral deltas.
7. Run bounded affected checks after coherent groups, then the named global
   full-verification route for this cross-service assembly. Record exact SHA,
   executed tests, skips and failures. Run
   Realtime only on supported Linux; Windows compilation is not execution.
8. Remote transport recovery belongs to Rudeus-135. Its observed proxy TLS reset
   is not repaired by repeat probes. Until actual recovery, preserve local
   develop and verified bundle. On recovery read live develop ref, merge any
   genuine remote additions without rewriting, then ordinary push.

## Validation

- CRLF-aware `git diff --check` and no unmerged paths after each resolution.
- Ancestor checks cover every accepted manifest head at the assembled tip.
- Proto lint/format/breaking and generated Go/Dart parity for assembled additions.
- Relevant Go short/vet/lint plus real no-skip PostgreSQL tests for affected
  migration/transaction groups, using the recovered authorized local runner.
- Relevant Flutter tests/analyze for combined provider/media/client changes.
- Relevant Python/shell/workflow tests for NATS, CI and admin browser changes;
  source review is distinct from Linux JWT/TLS/bootstrap/Realtime runtime proof.
- Preserve hosted CI, media, deployment and appointed acceptance limitations;
  local merge commits prove only source integration.

## Progress

Cached master merged as `0bd6750ff`. Initial independent group merged without
conflicts: BE-120 `472556807`, BE-122 `eadbf70df`, BE-070 `b5fef7ab`, BE-051
`21f5959ba`, BE-050 `f8c713b06`, BE-141 `e78be9ddf`; resulting tip `f99f747d8`.
Author-provided exact final heads and remaining gates continue in TT/manifest.

All frozen final author sources are now included through normal BE106 merge
`8c7488168bd52cd510df872d3a84d4d115b7a054`. The self-contained source order and
67 immutable checkpoint dispositions are in `swarm-develop-manifest.md` beside
this plan. BE211's obsolete contract-gap proposal is explicitly excluded in
favor of the accepted current author37 source. No current author head remains
pending. Full original task scopes remain open where only slices were delivered.

Actual populated PostgreSQL baseline698 Space migrations1..25 -> assembled26..31
PASS: Space/member/audit/outbox rows preserved, local receipt backfilled, new
local insert reserved; owned container removed. Raw exact SQL and output are in
portable `tmp/develop-integration/clean-upgrade-log.json`. This proves clean first
activation only. Migration31's matching-history reconciliation is unfinished;
existing receipt/fence overlap must remain a fail-closed upgrade blocker.

Assembled Messaging/Space short checks and final BE106 affected Voice short
checks PASS. Toolchain, compose config/NATS policy and Buf lint/format PASS.
Supported Docker Linux Realtime short PASS; no native Windows Realtime run.
Flutter analyze has no issues; full tests returned2205PASS/100SKIP/3FAIL. Exact
baseline698 execution reproduces the same three MatchSquad golden failures and
pixel counts, with byte-identical actual image hashes. No golden was changed;
the Flutter gate remains FAIL. Global backend full failure/interruption remains
separate from later isolated PASS; no complete CI success is claimed.

Bot regression: exclusive per-Space SetChatEnabled queued healthy senders behind
one blocked webhook. Existing real-PG test reproduced RED twice; shared lifecycle
guard plus whitelist row serialization changed it to GREEN. Independent author1
review found first-enable insertion contention; correction`b47d81417` adds a
same-Space guarded upsert. A deterministic PostgreSQL test establishes two INSERT
lock waiters before release and passes; author1 gave narrow independent CLEAR.
Lifecycle fencing remains exclusive and drains all external attempts.

Additional real polling failures exposed shipped Bot4 unique-token index treating
legacy empty message tokens as slash identities. Additive Bot7 excludes only
empty tokens; real tokens remain unique. Eight lifecycle cases and a populated
upgrade/uniqueness/unsafe-downgrade rollback test PASS. This new migration delta
still needs independent review; prior author1 CLEAR does not cover it.

Rolea6eaed371 fixes the shared protected CheckPermission dispatch collision:
only MEMBER_MANAGE_NICKNAMES uses the existing strict nickname validator, all
other requests retain the composite creation validator. No generic fallback or
negative trust-boundary assertion was removed. Real TLS runtime and Role full
short/vet/lint PASS; independent review is OPEN. b4d199aed strengthens the Auth
report inventory for the merged observation suite; actual Maven999PASS/zero
skips and report/self-tests PASS. caa8f2632 preserves only Windows SYSTEMROOT
in Matchmaking's isolated child fixture; actual tracked main short/vet/lint PASS.
Portable review packet includes BE044 merge6f6715c50, Bot7c140f416f, Rolea6eaed371,
mechanical ecceaaecb and the CI fixture corrections. No reviewer reactivation is
authorized; the existing independent review gates are not self-cleared.

All verification test handles are terminal. Full serial Bot86675 PASS325 tests,
12 packages, zero skips/failures; the original parallel full attempt remains FAIL.
Portable Linux CI73645 FAIL four targets, then affected supported routes ran:
Notification catalogc652cea3a20unit+preflight PASS, diagnostic4cd763a8c phase0
49PASS/4explicit opt-in skips, Bot7ops6739dcb505unit and6actual PostgreSQL checks
PASS. NATS900 test route55104 returned895PASS/1Bot7FAIL/4root-pinned-sourceERROR;
affected Bot7 failure is corrected by the later exact-query/unit proof. Root pins
equal baseline698Git blobs and deliberately refuse new BE102/BE211 template
contracts; no authority pin changed. R22cached698 route49410 FAIL180 old-scope
violations; no allowlist changed. These remain qualified CI/activation gates.
Terminal: Chat store1053 PASS81/zero skips168.917s; other Chat62295 PASS297/zero
skips, grpcsvc514.277s within10m and original MuteChat cleanup3.11s; Auth48773
Maven999PASS but original wrapper inventory FAIL, later report+self-tests54216
PASS; Role93195 full short PASS; Gateway race97057 PASS; docs7282 all148PASS;
images26494 remaining route PASS after Role4a24 actual image PASS, total22images;
graph25621 PASS before the final ops-only changes; final refresh97485 terminal
PASS102398nodes213485edges (AST158/158, existing frontend header warning retained).
No live test/graph handle remains. All67 candidate dispositions verified at final source
6739dcb50ac37583cf1e8f90605af39767cccc20. The earlier4a24 complete-history bundle
and fresh bare restore PASS. Final source6739 complete-history bundle61145161bytes,
SHA2560d03927720cf2de0a99fd1d5543445a84fa0741b9e83e9268ac7afb7ab8365be,
git verify and actual fresh bare restore exactdevelop PASS. External evidence
has a verified per-file hash index. The final documentation carrier, TT receipt
and guarded lease return are recorded in external RESTORE.md after this commit;
that receipt necessarily names the commit/bundle hashes produced afterward.
No idle polling or new ordinary claims follow closeout. Resume only on concrete
independent review authorization/findings or actual external transport recovery.
Independent review and external transport publication remain explicit gates.

## Authorized review-finding resume

The user authorized one independent reviewer, Rudeus-141. His frozen review of
318bc1bf6/source6739dcb50 records bounded source CLEAR for sections B-G, and a P1
in A: legitimate channel commentary carries PostedAsChat/DisplayChatID but the
store's exact expected companion omitted them. The unchanged existing actual-PG
channel-forward test failed in2.97s; reviewer evidence is separately attributable
at D:/Git/Voice/tmp/swarm/rudeus141-review/channel-forward.jsonl and TTc8/c9.

Existing integration custody resumed in a newly acquired own WT20 lease
ed1d2b69cc7426699a493f97cbe37548, holder rudeus-121-review-fix, physical
D:/Git/Voice/.treehouse/Voice-451d38/20/Voice. develop318bc1bf6 was reconciled
before switching; main dirty files and prior immutable bundles remain untouched.

Frozen correction1a6acf259b571a80340d99d11ae8adb5a34c15e1 changes three files:
the expected companion now includes exact parent PostedAsChat/DisplayChatID;
reflect.DeepEqual and all other identity/content/metadata checks remain. New
strict positive/negative attribution unit was RED on channel before the fix,
then GREEN. Actual API forward18PASS/zero skips covers stored forwarding profile
and destination display on both rows. Strengthened commentary-denial test PASS
and asserts zero target messages/outbox. Store compound6PASS/zero skips retains
rollback and expired-reference reconciliation tests. No equality/assertion was
relaxed and no activation or new product scope was introduced.

Same reviewer141 independently rechecked exact1a6acf259 CLEAR and ran seven
top-level targeted PostgreSQL tests PASS/zero skips, including strict compound,
channel identity/denial zero effects and communication deadline/cancel. His
evidence is attributable to him, not to integration CI. Own final affected
short/vet/lint64968 PASS250 tests/243 explicit PostgreSQL skips; these skips do
not replace the separate actual PostgreSQL proofs. Changed-input Messaging image
PASS (image06a9a5d888a60284a4b20cea524bdadd07ef0d40a46c3c4df101a16e46627641).
Graph39388 terminal PASS102405nodes/213489edges after AST696/696, with the existing
frontend header warning retained. All checks are terminal. Successor bundle/restore,
packet and guarded lease return must be
recorded before dormant closeout. Existing golden/NATS/R22/transport and full
feature runtime gates remain unchanged; no additional reviewer/task/spawn.

## Decisions

### Resumed full-colony wave 2

The user resumed autonomous Swarm delivery and explicitly retained121 as the
only physical develop writer and CI owner. New source intake uses actual TT
handoffs, immutable heads and existing independent reviews; no author report is
called an assembled check. Full self-contained current ledger and proof paths:
`D:/Git/Voice/tmp/swarm/rudeus121-develop/wave2/continuation.md`.

Fourteen reviewed new heads were normally merged throughc08cf1ed, then the
BE234 missing-row panic correction8493fa547, CI019 and PL-A4-07 throughd4abb829.
CI025/BE218 adjacent docs were resolved by exact union; author151 independently
reviewed merge45cf501e, with unchanged Realtime source tree. Main/master and
user dirties are preserved. Own WT36 lease d98699dc810d3f2707902abd371fb6c3,
holder rudeus-121-wave2-integration, is the only integration checkout.

Actual combined Linux Realtime full428PASS/zero skips and focused race12PASS,
vet0 qualify BE218+BE112 together. Actual Portal Node24 build and packaged
nginx assets/SPA under100m/64Mi PASS, peak17874944/failcnt0, no host ports.
Fifteen changed production Dockerfiles PASS at exact backendab6aa8e2; unchanged
prior builds are reused. Missing ffmpeg offline APK dependency failed before
the later ordinary dependency-network build PASS; both outcomes are preserved.

Actual BE234 scanner(nil,nil) panic was corrected without changing shared
scanner/SQL/locks/auth or original tests. Independent146c13 CLEAR8493fa547;
unchanged native Linux tests in owned private DinD yield13PASS/zero skips:
8top+5sub, including7 PostgreSQL scenarios and1rawactor unit. All nested PG
fixtures and own runner/engine/network/volume were cleaned. Changed-input Space
image PASS sha256:1fe4bd722fdf86885e854fb16cfc1a29cf8daeeeeb81512ca8be68831b0fb4af.
Graph94631 terminal0, AST769/769,102423nodes/213614edges, with the existing
frontend header warning; later inputs require separate freshness qualification.

Actual user transport recovery20:52:26UTC allowed exact fetch/ancestry
reconciliation. HTTPS publication was rejected for OAuth workflow scope, not
worked around. Existing-key normal SSH publishedc08cf1ed and8493fa547; actual
master remains698. CI run39165804964 was queued at its snapshot, not PASS.
The156-requested31d8d2358 contract-only BE045 branch was separately published,
without develop integration or task transfer. Current publication/CI is a real
executable stage; the earlier transport-blocked notes are historical.

Next ready group: reviewed CI020d11cba677 plus CI021 test-onlycdd7b4f5 union,
BE008 fixtureeef427a1, BE238 corrected6a1d3b8 (147c9 resolves metric deadline
P2), BE269 setup0fb2970 (149c5) and BE1699b8b69c (149c6). Preserve all strict
production preflight and test assertions; qualify combined ops fixtures and
changed production images. Other pending reviews/actual handoffs are listed in
the ledger. Reconcile TT reservations before any independent audit.

No shared stand operation occurred. DEV/STAGING requires actual resource
list/show, atomic claim/current revision, real observation/revalidation and a
protected rollout/check window; task custody/Markdown is not reservation.
No Windows test.exe/listeners/firewall changes. Golden/NATS/R22 and all broader
feature/provider/acceptance gates remain as recorded, never silently converted
to PASS. BE265 architecture now explicitly limits the current adapter to
EJECT_ONLY; full receipt needs durable refreshed-credential rejection and
settled handshake/cleanup proof. No source change came with that clarification.

Before safe closeout create a complete-history current-head bundle, verify and
actually restore it into fresh bare Git, check accepted-head ancestry, append
portable TT evidence and exact pending CI/review actions, then guarded-return
only own lease after fresh process/container evidence. Existing earlier bundles
remain immutable and do not pretend to cover this resumed wave.

- Preserve existing develop instead of replacing it: user requires history.
- One physical writer/queue; original task checks/ownership stay with authors.
- Pending source review and runtime prerequisites remain visible even if the
  underlying source checkpoint is accepted into the assembly.
- No remote mutation until successful exact-ref reconciliation; cached tracking
  refs are not live remote evidence.

## Risks And Follow-Ups

BE-106/BE-130/BE-131 final immutable successors are now integrated. BE-064
has no new implementation source and its clean preparation lease was returned.
BE-010 final `deff1b0` delivers a bounded policy slice, not the full admission
runtime; Role20 remains unimplemented and is not a missing commit to invent.
Shared generated contracts and Role/Space migrations need assembled verification.
BE-102's source CLEAR excludes actual committed journal emission and Linux
runtime gates. Several product tasks intentionally retain larger open scopes.
GitHub transport remains externally blocked by configured proxy upstream TLS
reset; no hosted CI/publication success is implied by local progress.
