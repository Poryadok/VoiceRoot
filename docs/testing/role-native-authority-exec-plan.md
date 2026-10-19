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

The complete native caller/writer/derived-reader cutover is still required.
Public delegated transport enablement requires a real current Auth-floor client;
accepted client tests do not establish an assembled Auth production deployment.
Space all-writer closure, source2 projection, consumer acceptance and combined
runtime/CI remain open until their actual evidence exists. No whole-task CLEAR,
activation, final acceptance or delivery custody is inferred from these tests.
