# Current release history source qualification

## Purpose

Qualify the current release source through the real full F13 historical gate
before STAGING, without changing historical event baselines or admitting a
partial Native unit. Consumer: root DEV-to-STAGING release owner.

## Context

CONTRIBUTING lines 30-33 and .github/ci/gate-policy.md require full history
before staging/release. TESTING Native/P3 paragraphs require complete projection,
context, literal A1-A4/B1-B4 applicability and conserved producer provenance.
The predecessor implementation is scripts/ci/f13_production_source.py v10,
Native v4 in f13_native_p3_documentation.py, and f13_f54_history.py.
Read .agent/execplans/pr875-current-native-source.md for immutable prior proofs.

The runtime evidence source is b2b566f1103e29058da0e8b786cbd8f0dd028d83; its tree equals
tested product ec214b28747a4552eb89f4e431fc018aa82f023a. Actual master parents
are f2f5211bfcd09868ae4adc639b8fb3a4734aef54 and that product source. Existing
Native/event authority fixes 2c8bdf9e2f420b724575096beb4fcf5cb4ea00be; therefore
context_exact_event_base is a correct rejection, not a guard defect.
Final H is not frozen: root is integrating separately reviewed deployment and
release-request changes. An exact independent extension review must bind H to
the unchanged Native unit and review every other changed source input.

## Scope

Own WT4, branch codex/release-history-source, lease
0e0e7f98792853d544fa91a8ca805d0a. No DEV/STAGING mutation, product authority
activation, source whitelist, baseline substitution or historical record edits.
Any product defect found must return to root for a new immutable image cycle.

## Milestones

- [x] Identify exact historical event/source boundary and lease isolated worktree.
- [x] Reproduce b2 refusal with exact real parents; retain diagnostic.
- [x] Reconstruct full b2 Native projection/context/compiled closure and
      independently review changed inputs and all eight evidence clauses.
- [ ] Bind that evidence to final H through exact independent extension review.
- [ ] Reconstruct complete current history, source owners, runtime-OR, guarded
      intermediate taint, nested merges and every best base; independent review.
- [ ] Freeze reviewed H, source-only R directly descending from H, then later
      carrier K binding immutable R bytes and source-bound evidence.
- [ ] Implement separately typed successor consistently in producer and all F54
      consumers, with negative tests before implementation.
- [ ] Run actual unmodified producer/final fold and hosted historical gate on the
      real integrated candidate; commit, push, review and merge normally.

## Detailed steps and ownership

Owner Astra handles named historical/security architecture uncertainty.
native_review_gpt6_1_terra_high independently reviews Native source/evidence.
history_review_gpt6_1_terra_high independently reviews producer ownership and
historical reuse boundary. native_evidence_gpt6_1_terra_high owns bounded Linux
current-source runtime tests and actual go-list closure; no live environment.
Ordinary successor implementation moves to Terra once the contract is precise.

Native v4-to-H currently changes 3 projection, 24 context, 20 old compiled paths
and adds files inside dependency directories. Endpoint equality does not imply
runtime applicability. Shared socialprincipal and integrationtest dependencies
require semantic review and fresh affected evidence. Old literal SQL proof is
reusable only with exact inputs/drivers bound and independent applicability.

## Validation

New tests must reject wrong event/parent/source/result, unpinned or forged record,
partial unit, changed mode/blob/deletion, added dependency, incomplete history,
missing owner, changed best bases and guarded intermediate taint. Keep v1-v10
negative behavior. Run scoped Python real-Git suites, full actual F13 fold and
hosted voice-history-audit. Passing private fixtures never establishes actual
master qualification. Graphify refresh remains owned and separate from authority.

## Progress

2026-10-28 initial checkpoint: No source PASS or runtime PASS issued. Historical inventory located
at WT7/tmp/pr998-history-inventory.json is for old 5cb5749e and is diagnostic only.
Fresh source-bound reviewers and Linux evidence owner assigned.

2026-10-28 implementation checkpoint: Independent Native source review accepts
all eight clauses for b2, supported by 86 Linux race tests (no failures/skips),
raw go-list closure, wire/generated-code comparison and exact old SQL-driver
applicability. A deterministic portable archive conserves the evidence bytes;
the large source archive is reconstructible by its recorded Git command/hash.
Supporting extension review for 70a3ca533e23d7454782428304c180e76fad74e1 proves
the Native unit unchanged; this is not acceptance of a later final H.

WT4 integrates the typed Native v5 / production v11 loader and the F54 bridge.
At this earlier checkpoint the source loader required actual event base H;
the continuation design below supersedes that restriction with actual base B.
Both independently qualify the fixed collection base
2c8bdf9e2f420b724575096beb4fcf5cb4ea00be. The producer runs
one continuous collector from that base through K, retaining historical nested
scopes, owners, best bases, runtime-OR and guarded intermediate taint. Independent
review found and corrected a 132-boundary event/collection-base mix-up.

Linux synthetic source/event tests: 12 PASS, including forged CLI binding/event
cases and a real RED-to-GREEN check for old and added canonical proto drift.
The successor conserves all nine context proto tree entries between the runtime
evidence source and H, even when generated Go files remain unchanged. The final
assertion-only test tightening separately passed both mutation variants with
the specific release_context_proto_drift rejection.
Integrated focused bridge tests: 5 PASS. An earlier incomplete container
overlay omitted the changed shell and failed one wiring test; the complete
overlay passed. These are fixture results, not actual release qualification.
The legacy production-source suite is still running. Real Git preview loading
on 70a reconstructs all predecessor Native records, 99 projection paths and
501 compiled paths without mocks. No R/K commit or actual F13 PASS exists yet.

The source-only R must be a direct child of final H. A separate production
source-contract review binds the exact current-input inventory and draft carrier
diff; Native runtime review cannot supply that verdict. Final independent K
code review and actual F13/hosted checks remain required after mechanical R/SHA
pinning. Executed carrier blob identity does not replace independent code review.

2026-10-29 continuation requirement: root will create ordinary full-build marker
commits after the foundation merge, then another cycle after four hours. The
initial exact-H event design would deadlock those events and is not complete.
The marker is .github/ci/full-application-build-request.json; in the accepted
70a unit it is not a projection/context/compiled/dependency-directory member.
The proposed DEV ops helper scripts/dev/media_initial_admission.py is also
outside that unit; final-H membership must still be recomputed. Neither path
receives a source admission allowlist.

The bounded continuation must validate the real event T=[B,S], event SHA/HEAD,
push-before or PR ref, B ancestry in S, and complete S/T tree parity. R remains
a direct child of the reviewed foundation base H. For B=H, foundation is this
event. Otherwise derive F as the first first-parent commit after H on B's real
chain, require F=[H,K], R ancestor K, K later than R, and K/F tree equality.
Revalidate the original strict R-to-K carrier delta against files actually
executed at T; never manufacture an old GitHub event to qualify F.

For continuation, enumerate every new DAG commit in S excluding F's ancestry.
Every new commit must descend F and preserve the complete qualified Native unit,
all immutable R evidence/records, and exact K carrier entries. Compare modes,
types, blob IDs, directory additions/deletions, canonical proto and the whole
Voice tree. Runtime projection/compiled paths may not overlap carriers; any
carrier context path is explicitly fixed to K. This rejects mutation followed
by restoration and transient dependency additions. A newly imported pre-F
branch requires a separately reviewed successor. The requested fresh post-K
marker/ops branches satisfy this bounded ancestry condition.

Bindings must distinguish the static foundation base from actual event base B,
record the real F/K tuple and actual no-renames B-to-S delta digest. Reusing
unchanged-unit evidence grants no authority to outside-unit changes: the single
complete 2c8-to-S producer fold, every original gate, all owner/best-base facts,
runtime OR and guarded intermediate history remain mandatory. Changes to the
verifier/continuation policy require a new independently reviewed successor.

Supporting validation before this continuation change: real 70a H/R/K/T fixture
passed unmocked Native v5/production v11, collection CLI, old predecessor chain
and F54 J reconstruction, including forged before/base negatives. It is not a
real release event or producer PASS. The old production suite finished 68 tests
with 11 errors and one failure, all context_executed_carrier: the draft-overlay
harness had Git HEAD b2 while executing modified carrier bytes. Its immutable
guard correctly rejected that mismatch. A private committed test snapshot now
repeats the suite without changing the guard or user repository history.

2026-10-29 diagnostic checkpoint: the correctly committed private legacy
snapshot passed all 68 production-source tests. The current bounded continuation
suite passes 19 real-Git tests, including intermediate side-branch mutation and
restoration rejection. A real immutable private 70a R/K foundation and two later
marker/ops events passed Native/production binding, collection CLI and F54 J
reconstruction. These are supporting fixtures, not the complete producer fold.

The first unchanged full-profile shell run stopped before producer collection:
the old successor integration fixture only recognized earlier DATA_STORES
versions. Its new source-only route independently loads pinned v5 Native and
production records and checks the unique H projection entry and ancestry.
The next real run exposed the old seven-path Voice-additions assumption; H has
nine reviewed additions, including two reference-inventory generated files.
The fixture now derives the exact added set and entries from immutable H,
retaining old routes for earlier sources. Independent review accepted that
bounded correction. A fresh real v5 run completed 42 tests with one remaining
failure: a test assertion still directly expects the old DATA_STORES blob.
That current-source assertion is being corrected through the same checked H;
historical D1/C2 assertions remain mandatory. All failed logs and private Git
snapshots are preserved. No full producer trace or release PASS exists yet.

Actual local master 1bf5c4361a6469d946601e14afbb12a7ffe2af4e differs from
supporting 70a in 55 paths; a read-only exact membership scan intersects the
Native unit at Makefile. This is not final source review. Final H must receive
an independent exact extension review before R is created; unchanged compiled
inputs alone do not establish context applicability.

The final Native successor fixture correction passed all 42 tests on an exact
private H/R/K/F. Two later events with canonical cycle-1/cycle-2 marker JSON
passed Native/production binding, collection CLI and F54 J; a side-branch
mutation followed by restoration was rejected. Marker schema validation used
the exact external selector from 1bf because 70a predates that helper; this is
not a source CI or full producer claim.

The next unchanged full F13 run passed the same 42 tests and preceding suites,
then stopped at one of 26 Role successor tests. Its current user/common
reference_inventory.pb.go assertion still expected the old inventory blob.
That path is in the accepted Native compiled closure and context. The corrected
fixture retains the historical blob assertion and checks the current blob only
through fully loaded v5 source records and the unique exact H context entry.
The Role authority, runtime-taint facts and actual-head drift denial are intact.
Two separately reproduced production fixture failures are corrected with a real
historical target and an explicit unchanged v10 qualifier rejection of actual
HEAD. The new source dispatch also retains a real foreign-source negative.
These two test files extend the reviewed carrier set from nine to eleven; they
are outside Native projection/context/compiled inputs. Fresh exact-record tests
and the unchanged full F13 run remain required. No producer trace has yet run.

The first full producer on the exact eleven-carrier private fixture reached the
real collector and failed at `exact132_actual_production_outer_scope` after
4,297 seconds. Its sealed trace has root `[2c8,K,true,true]` and an actual first
operation `69cc/2` whose child was `[2c8,011c,true,false]`; that false child
omits nested source provenance needed by 132 and the earlier e675/DC0 path.
The preserved failure is `tmp/release-history-record/full-producer-eleven-132-failure`.
An independently reviewed decision introduces one typed, live-prefix-bound
exception: the exact real 69cc parent opens `[2c8,011c,true,true]`, preserving
the full child map, owners, runtime OR and every nested operation. All other
children retain their existing partition rules. A separately typed 132 scope
must bind that actual root/parent/child chain; old v10 root rules remain intact.
Router policy uses the independently verified actual event base for v11 while
the collection frame retains its distinct 2c8 base; R23 window policy remains
on the actual event base. Synthetic real-Git transport/fold tests do not qualify
the source. Exact pinned-source review, complete child/map/taint tests, full
script checks and a new actual producer replay remain pending; no PASS is claimed.

The next genuine producer trace reached the historical Role duplicate at
9dc/e963/477 and rejected the current target against the older v2 context.
All eleven corrected entries remained unchanged, but five context entries
changed, five member-name inventories grew and six complete directory-entry
maps changed (the sixth contained a changed existing integrationtest blob).
A proposed Role v3 record binds the complete
reviewed Role unit and unchanged historical correction to final H, using the
70a Go evidence only if the whole unit remains unchanged through H. Actual
event qualification and every continuation DAG commit must preserve that unit;
the record grants no Native path or runtime authority. The independent review,
real pinned-source qualification and unchanged producer replay remain pending.
The Role verifier joins the carrier set; no final source or record is pinned yet.

## Decisions and risks

Preserve all old contracts and failed diagnostics. New typed authority must
consume complete source/evidence and the actual producer ledger. The b2
reachable-set inventory has 985 commits since d783ab03; first-parent-only
inventory is unsound.
Record/carrier publication awaits real independently accepted source evidence.
