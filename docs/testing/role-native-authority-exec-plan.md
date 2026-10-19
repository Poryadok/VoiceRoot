# ExecPlan: native Role member authority

## Purpose

Finish BE010-ROLE-QUALIFIED-AUTHORITY: a protected Space admission prepares an
inert, immutable Role receipt; only actual Space publication makes its exact
contribution eligible. Role decisions use qualified membership, independent
source floors and canonical Owner writer evidence. Unresolved legacy evidence
must not silently discard restrictions or become fresh authority.

## Context

The product canon is `docs/microservices/role-service.md`, the Role/Space parts
of `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`, and PLAN A2. Execution follows
`docs/TESTING.md`. The accepted source protocol is task comments c48-c51
(ROLE21 ruling, artifact SHA256
`fb94d7549b221307965750ae5a8f7c772f52c5bf76e401612fa36842421e9e9c`).
Settings is the distinct finite service:space contract c64-c66 of BE-010,
artifact `059ca840f39133248321e1b63249995f05b8860c8f02fd192f6a30eab5da8b97`.

The source1 contract remains exact catalog16/17/20. Catalog21 requires explicit
scope2, owner state2, exact function/trigger qualification and the same pure
qualifier as native readers. A version integer is not qualification. Federation
is deferred; no separate deployed decoder is an implementation prerequisite.

## Scope

Owned files are Role admission, runtime/interceptor, main, session-floor and
native readers/writers, Role migration21, and the shared authoritysource pure
codec/qualifier/catalog helper. Space production source and state2 projection
belong to producer186; Space public admission belongs to69; Creator assembly
belongs to342. Preserve the accepted Bootstrap runtime delta9eface. F13 and
blocked Auth identity/recovery/function-DDL work are outside this continuation.
121 alone publishes, runs combined CI and merges. This checkpoint is unfinished
implementation, not a request to merge a disabled foundation.

## Milestones

- [x] Actual protected admission method dispatch and strict service/body guard.
- [x] Inert PG receipt/preparation, replay, independent floors and P30D cleanup.
- [x] Native qualified reader uses shared RoleStateV2 facts and qualifier.
- [x] Explicit bootstrap/transfer owner evidence; no historical tuple backfill.
- [x] Current Auth Role-floor client and delegated session-first runtime seam.
- [x] Explicit Role/Space scope2 selector; source1 semantics preserved.
- [x] Actual SpacePG PREPARING to RolePG receipt to actual Space publication.
- [x] Exact protected approval42 and settings branches use qualified authority.
- [x] Strict source21 negative matrix and SDK independent clock/source parity.
- [x] Actual GetMemberRoles/GetEffectiveMask/hierarchy at21 require observations;
  the protected Space GetMemberRoles caller collects them outside Role locks.
- [x] Delegated actor and target must have current membership in the same fenced
  Space; membership eligibility is separate from possessing a role/grant.
- [ ] Finish native public target ACL, assignment/revocation and derived-reader
  cutovers with durable authorized-operation references.
- [ ] Complete actual delegated Auth/session/receiver ordering proof.
- [ ] Independent review of the coherent changed source, current candidate
  reconciliation, combined CI and native consumer acceptance via121.

## Detailed Steps

1. Run the current source21 parity/negative test and preserve source1 controls.
   The owner catalog includes current contributions, immutable receipt witnesses,
   preparations, independent floors and owner links; every current mutation must
   advance the existing Role clock. Native reads use the same catalog validator.
2. Change actual native public readers and hierarchy inputs together. Collect
   fresh authenticated Space observations outside locks. At catalog21, missing
   observations are unavailable, never a legacy-reader fallback. Keep raw tuple
   inventory explicitly named and isolated from authorization/export.
3. Cut over writers using exact observed bindings and finite recorded source
   sets. Obtain local revision R, release, refresh owner observations, compare
   their independent vectors, then CAS R and re-evaluate locally before commit.
   No remote call under Role locks, no mutation before final CAS, no denial after
   a successful commit. Retain Bot/all-system guard and Owner transfer boundary.
4. Re-run actual signed transport with native Space and Role databases. Exercise
   wrong source/receipt/body/issuer, replay, source removal, expiry and local
   schema/guard corruption. Publish the exact immutable source/proof for review.

## Validation

All Go executables run inside private Linux Docker. Use the cached Go modules
and build volumes, Docker socket for owned testcontainers, GOPROXY=off,
TESTCONTAINERS_RYUK_DISABLED=true and host override host.docker.internal.

The actual cross-owner test is:

    cd src/backend/role
    go test -tags role_space_native . -run '^TestMemberAdmissionPositiveProtectedReceipt$' -race -count=1 -timeout=150s

Its Space fixture is built from exact producer source
`5e0fc4f551dd9dd4659d5a6da1b7393608cf0cff`, not a peer worktree. Before Linux
execution, Windows host preparation from the owning worktree is:

    rtk git archive --format=tar -o tmp/swarm/native-space-5e0fc4f551dd9dd4659d5a6da1b7393608cf0cff.tar 5e0fc4f551dd9dd4659d5a6da1b7393608cf0cff src/backend

`testdata/build_native_space_fixture.sh` builds the tracked stdin-driven fixture
inside that immutable Space module. It constructs no authority response,
receipt or verified private capability. RolePG uses all official migrations;
SpacePG creates the historical Space at34 then applies actual forward39.
Real mTLS, HTTPS two-key JWKS and actual Redis protect both listeners.

## Progress

Observed critical protected Ensure RED4.235 (method denied), then RED4.217
(nil native observer). The actual cross-owner path passed race65.706s after a
test-only stderr-buffer race was corrected; the first run's27.975 failure is
not a pass. Extended actual transport passed race60.891s with both fixed
permission branches, Remove/replay, changed body conflict and inert historical
Ensure after removal.

The unresolved legacy-restriction discriminator failed on old behavior2.715s.
An interim3.731s negative-only pass was withdrawn: its fixture lacked SDK16.
After adding positive controls, the full official16-to21 qualified suite passed
race44.491s plus vet. Source2/native shared projection initially failed44.304s
(unsupported exact21), then passed24.035s; the subsequent expanded dirty/future/
disabled-clock/receipt-guard/RLS/missing-table matrix is a separate pending run.
Shared package suite passed race1.963s. Shared immutable codec/qualifier5de and
scope/catalog helper0b491b7 are source-only prerequisites, not activation proof.

Expanded source2/native SDK plus seven catalog negatives passed race8.834s and
vet. Actual public GetMemberRoles with the two-stage owner observation/local CAS
passed race22.874s in the real cross-owner test. Its fixture uses the accepted5s
budget; an earlier2s fixture budget expired (55.306s test failure), not a pass.
Actual schema21 raw-reader/hierarchy rejection failed5.702s, then passed
race6.740s after native routing. Space service reads are independent of Gateway
delegated activation: a dedicated runtime RED0.411 exposed that accidental
coupling and the branch was split while retaining exact principals.

The next real transport oracle intentionally remains RED31.006s: authenticated
Gateway Owner CreateRole succeeds, but current legacy AssignRole ACKs a raw
tuple and the qualified reader reports unresolved provenance. It must gain its
accepted durable operation witness; do not replace that witness with request ID
or invent a second journal to bypass blocked generic-ledger prerequisites.

Containment now rejects native Assign/Revoke before the raw write while that
accepted core is unavailable. Its no-effects transport oracle first failed
86.230s, then a subsequent run exposed a genuine5s CreateRole deadline failure
38.661s. Exact catalog validation now batches fixed trigger and function checks
without caching, changing versions or weakening their definitions. The entire
protected protocol passed race46.162s, with source21 negatives7.116s plus vet.
This is containment evidence, not a successful interactive mutation.

The actual derived Voice call initially failed23.336s because the handler had
not collected native membership. It now obtains the authenticated Space
observation before the local fence. The same RolePG/SpacePG/mTLS/realRedis
protocol passed race25.868s with the published Member's positive Voice grant;
focused Voice handlers passed race1.264s and vet. The session-first negative
advances the checker floor7 to8 and proves no native observer call occurs for
the stale delegated credential. The checker is a controlled runtime fixture,
not an assembled Java Auth deployment.

Following catalog batching, source1 actual protected transport passed
race12.275s and both exact20 PostgreSQL controls passed race17.996s. The first
shared-package run failed at setup because its offline cache lacked compress
v1.17.9; after fetching that dependency in Linux, the shared authoritysource
suite passed race1.944s and vet. Graphify AST update was attempted on this owned
checkout; it produced no output for the bounded window and only its verified
own parent/child processes were stopped. No graph-update PASS is claimed.

## Decisions

Owner facts originate only from actual Bootstrap or finalized ownership writer,
and must agree with fresh Space ownership plus eligible membership. Legacy raw
Owner has no migration-generated origin. A current managed Member definition
is the fixed community baseline; today's configurable default grants no ambient
rights. READY default proofs join and recompute the complete immutable receipt.
Native positive membership can qualify explicit grants but never a READY default.

## Risks And Follow-Ups

ROLE-LEGACY-SAME-CORE-v1 c55 confirms that native legacy Assign/Revoke remain
no-effects Unavailable until the SAME durable operation core is genuinely ready.
This is an actual delivery dependency, not a separate legacy journal permission.
Provider213 source cb90413 and identity214 source ff5db163 are reviewed adapters;
neither establishes current deployed capability or the stopped172 owner channel.

After checkpoint5c432f6, additive Bot wire d51b65 was consumed as948796021.
The accepted BE170 c10 service:bot CheckPermission branch is separately fixed to
SPACE_MANAGE_BOTS with no node/mixed claims. ROLE_BOT_LIFECYCLE_ENABLED=true
requires the exact additional bot HTTPS JWKS endpoint; default trust is unchanged.
Actual protected RolePG/SpacePG Owner allow, Member deny, replay/node/cross-purpose
negatives failed27.273s before implementation and passed race80.196s afterward.
An earlier42.954s failure expired the fixture's2s Ensure call, before the Bot
oracle; it is not the feature RED. The fixture now consistently uses the accepted
5s calls. Runtime/principal suites passed race17.359s/1.019s plus vet.
Cleanup ApplyBotInstallationRemoval is not enabled by this decision-only change;
44's Role25 correction and source qualification remain separate.

Review82 c57 identified nested unknown-field admission and expiry after the
last override/Voice epoch query. Corrective19d8dfa7 adds recursive message/list/
map rejection and final freshness checks after queries and read-only fence
commit. Nested controls passed while unknown requests failed on old code0.031s;
realPG override/epoch cases failed16.824s and delayed COMMIT cases15.473s. Final
four-case PG/race26.699s, full principalrace1.022s, vet and native Role build
passed. Same review c2 / parentc59 independently CLEARed only this corrective
scope; later Bot/ACL changes still require their own review.

The global caller/target membership ACL uses EligibleMember from the same pure
qualifier, with local independent floors. It does not infer membership from
role count or grant an implicit Member role to a native episode. Actual signed
Gateway Owner and current Member read controls passed while an outsider actor
incorrectly read definitions (RED42.641s); a nonmember target also returned
success (RED26.578s). Both now deny under the final Role fence/CAS, with actual
RolePG/SpacePG/mTLS/race protocol24.327s and source2 parity6.727s plus vet.
The new local ACL helper also rejects another Space's transaction: realPG
positive-same-fence/negative-wrong-fence RED5.331s -> GREEN9.805s, vet0. Shared
pure-qualifier suite passed race2.105s and vet. An earlier fixture build used a
nonexistent EnsureBinding account field; that diagnostic is not a behavior RED.

Chat-scoped effective reads collect an exact protected Space tree observation outside Role locks and revalidate it under the same revision/CAS and expiry budget. Actual RolePG/SpacePG RED52.628s preceded implementation; final expanded race32.792s plus vet/build passed, including node removal and three invalid scopes. Intermediate31.689s dependency failure before the chat oracle is retained, not a feature RED. Historical Space5e remains a component fixture, not current all-writer readiness under Space c76-c78. No unsupported node is promoted from an unscoped observation. Role25 adapter99ae37a has scoped
review144 CLEAR but is not yet imported/qualified by this native cohort; adding
its version to a list would not establish source2/writer/late-Bot readiness.

The complete native caller/writer/derived-reader cutover is still required.
Public delegated transport enablement requires a real current Auth-floor client;
accepted client tests do not establish an assembled Auth production deployment.
Space all-writer closure, source2 projection, consumer acceptance and combined
runtime/CI remain open until their actual evidence exists. No whole-task CLEAR,
activation, final acceptance or delivery custody is inferred from these tests.

Chat review82 c64 found that nested Store freshness checks precede the owning coordinator COMMIT. Exact signed interceptor-to-handler regression delays only after successful final COMMIT while the original observation was fresh; old code returned success (RED31.614s). The chat read path now rechecks the original vector after that COMMIT, without changing mutation outcomes. Native21 RolePG/SpacePG/race37.658s, scoped vet and build passed. The listener fixture explicitly targets21; this is not future25 qualification.

Reviewed44 Role25 source8f9+99 was imported as328314057+16145fa56. Dedicated cleanup now validates exact native21 dependencies plus all four Bot tables/five immutable guards and fixed function body inside its operation/Space transaction before binding or receipt writes. Dirty-schema/disabled-Bot-guard/disabled-contribution-clock tests failed11.947s on the imported adapter and passed with its existing suite race60.606s plus vet/build. This cleanup catalog does not advertise native member/source2 serving at25, activate Bot actors, or open the private RPC by itself.

Role25 cleanup transport ccb3a63 admits only enabled service:bot/full body/operation-bound request IDs on the protected mTLS listener; ordinary dispatch is denied. Actual TLS positives/negative replay, issuer, RPC, body, nested unknown and raw identity controls failed0.328s before implementation, then runtime race17.866s/principal1.020s plus vet/build passed. The real Role25 store/handler transport receipt/retry/conflict/catalog-no-effects component passed race6.851s (miniredis explicitly).

## Current Space44 Consumer Continuation

The concrete producer successor a36cc5e504fab412f6d48dd5233ed199b9b5d243
now supplies both seams below (independent423 SOURCE CLEAR c106). The unchanged
UNKNOWN denial oracle passed race48.770s with Unavailable; actual admin
InitializeNativeMembership, independent nonowner LOGIN and full native25/44
receipt/publication/mask/chat/42/settings/Bot decision/removal/Redis cleanup
protocol passed race47.912s. The initialized positive and UNKNOWN negative use
the same immutable source44 pin and actual API. The prior d410 RED remains
preserved; it is not relabeled as positive. No bare SQL initialization or
package-store adapter was introduced. This closes that component composition
gap, not full all-writer45/current/core or247 public initiation readiness.

The named247 consumer also needs a portable caller-owned Linux serving harness.
Before export implementation, an explicit export run completed the actual44
positive but failed its required ready.json postcondition (runner32155 exit1);
the ready artifact was absent. Export is opt-in only after the complete real44
positive, with a fresh directory, short-lived fixture files, separate Bot-only
fixture signer, independent PostgreSQL identifiers and a bounded consumer stop
file. It must not export Auth attestations, bearer tokens or readiness/private
capabilities. The longer test-process lifetime does not extend authority TTLs.
The actual standalone signed consumer probe and stop/teardown proof are still
required for this new harness; no portable serving PASS is claimed by compile.

The existing Role25 consumer now has an executable denial fixture compiled
inside immutable Space d410305b2e74993966e07aca5a486c16766d9e9a. It uses
the official Space34-to44 migrator, an actual independent nonowner LOGIN and
the real protected getter. The administrator verifies that this Space has no
registry row; the fixture does not initialize it, wrap the getter with a
replacement guard, manufacture observations or issue readiness capabilities.

The signed Bot decision through real Role25 and Space44 mTLS/Redis returned
allowed=true/error=nil for that UNKNOWN registry (causal RED17.071s). The
fixture checks actual schema44, serving session_user and absent registry before
the decision. An earlier same-boundary run without the diagnostic fields also
failed and is retained. The changed harness retains its real native39/Role25
positive, race44.714s plus scoped vet/build, and does not relabel it Space44 PASS.

The producer186 confirms two concrete current gaps: the actual getter must call
RequireScopedNativeMembershipReadiness inside its owning transaction; and the
administrative initializer needs an exported own-RC/same-fence entrypoint
InitializeNativeMembership. Its existing InitializeScopedNativeMembership is
not callable from this consumer with a genuine scoped store. Both are existing
producer-owned implementation, not permission to add a Role guard wrapper,
bare SQL initializer, synthetic scope or alternative authority. Actual positive
44 composition and247 portable serving handoff remain unavailable until an
immutable producer successor supplies both seams. Continue from this exact
denial test, then add the real admin cutover/known positive control under the
same independent app LOGIN. Do not bypass172/current core or duplicate320's
protected report/history consumer. This is an unfinished existing task checkpoint.

The native25/state2 extension uses the same immutable21 projection and pure qualifier; Bot terminal fences restrict mutations and confer no mask or binding. Existing cleanup effects invalidate the existing Role clock; SDK clock remains independent. Actual21-to25 native/source parity initially failed5.465s, then passed7.330s. State1 remains exact16/17/20; intermediate22-24/future/dirty/disabled Bot and contribution guards remain unavailable. Qualified Ensure/Remove catalog guards precede receipt/floor writes in their own transaction; dirty-schema regressions failed11.535s before that implementation. The native21 and native25 full transport protocol (including chat expiry and actual Redis cleanup receipt on25) passed race73.669s, source21/25 parity15.963s before the final receipt-guard addition; exact final-source verification follows separately. Full Assign/core, all-writer and owner readiness remain open.
