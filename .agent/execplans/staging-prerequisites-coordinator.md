# Protected staging prerequisite coordinator

## Purpose and context

Implement the finite pre-capture operation described in
`staging-full23-prerequisites.md`, `docs/DEPLOYMENT.md`, and the protected root
Session contract. One admitted root operation prepares release policy, P3
assets/publisher and GIS material before normal capture and Realtime admission.
The existing root CLI, global operation lock and TT Session remain the authority.

## Scope and decisions

This is a source-only checkpoint. The coordinator is not called by the staging
bridge until the outstanding integration, recovery and runtime qualification
below are completed. Existing full and app-only rollouts retain their original
renderer proof and other admission checks; this checkpoint does not provision P3.

Only the parent coordinator and its tests are owned here. Child material,
cryptographic validation, auxiliary OCI and existing bridge/Session files have
separate owners. No live operations, generic root API, arbitrary command/path,
lost-response adoption or implicit existing-object enrollment are introduced.
Static release policy does not assert runtime image qualification. Space
lifecycle remains disabled. Fixed operator inputs are root-private files below
`installed/prerequisite-inputs`; parent qualifies their actual content against
current STAGING anchors rather than trusting a supplied success flag.

## Milestones and validation

- [x] Real Session test: fixed create/CAS/policy submit, exact private body and
  one-use command permission; only its response can reach DONE.
- [ ] Lost reply, claim/source drift, wrong UID/content, pending replay and
  checkpoint failure leave the durable operation uncertain without later writes.
- [x] Executable cold run_prepare produces all plans and validates all children before
  any effect; current issuer/source/node and baseline are rechecked.
- [ ] Capture and historical loaders bind the complete operation to subsequent
  Realtime and rollout targets and protected PASS ancestry.
- [x] Joined parent/children/real Session fixture and independent scoped C1/C2 review.

Tests run in an owned native Linux container with synthetic Kubernetes/TT wire
responses. They are not a live STAGING qualification. Ordinary test-first work
is followed by an independent high-risk review. Manual AST refresh is deferred
under root's shared-resource instruction until the named main refresh finishes.

## Progress

- Fixed submit checkpoint 2f270e9 adds exclusive durable attempts, exact private
  body transport and same-Session successful response custody. Lost replies
  remain PENDING and cannot be retried or completed from matching live state.
- Dependencies GIS/SQL32 cc5f3fc, P3 completion 9e0432e and OCI deadline fix
  390abb6 integrated normally. Their scoped reviews do not approve this parent.
- Cold joined fixture executes actual run_prepare, all three real child plans,
  preflights and verifiers, 15 Kubernetes effects plus one root policy replacement,
  publisher completion, and actual Session capture transition. Real OpenSSL and
  captured services/SQL source are used. Installed helper cohort, Stage baseline,
  candidate image admission, registry content, Kubernetes and HTTPS transport
  are explicitly controlled fixtures, not live authority. Ten native Linux tests
  pass, including missing issuer before effects and committed/lost app CAS reply.
- Protected C2 and C3 now follow the exact previous active PASS to original
  successful effect receipts. The joined fixture exercises real historical
  binding, preserved-only plans and zero writes, and rejects ROLLED_BACK,
  altered intent, foreign UID and active-pointer drift. PASS creation itself is
  modeled; this does not qualify the full transaction or live runtime.
- Independent C1 review found that additional P3 trust roots were not tied to
  current anchors. The complete five-entry trust map now equals the actual
  live CA inputs; an appended unrelated CA fails before effects. Approved OCI
  collector source is compiled from the exact checked bytes, bypassing Python's
  timestamp-based bytecode cache; a forged same-size cache regression first
  failed and then passed.
- The native Linux coordinator, Session, policy and P3 suites pass 67 tests.
  Parent-to-Realtime Secret extension and the full caller/candidate/renderer
  qualification remain separate outstanding integration evidence. Independent
  review of this successor, including C2, remains required; no live claim.
- Reviewed P3 routing checks 363b358 integrated; the affected P3 and parent
  composition rerun passes 18 tests. A further parent test passes for actual
  capture revalidation after a modeled protected Realtime COMPLETE: PENDING,
  before UID/RV, unrelated Secret data and authority changes are rejected.
  It executes the real local receipt/source/Secret validators, not SQL effects;
  the actual Realtime producer join remains outstanding.
- Independent C1 review of c246/current production bytes closed the trust P1;
  C2 static review found no new concrete issue and requested discriminating
  evidence. Two focused C2 tests pass: same/new candidate policy paths, lost
  current claim, a changed hash-qualified publisher source, and a foreign origin
  with all local digest links re-sealed. The new candidate creates only its
  policy ConfigMap and root policy; GIS/P3 remain preserved. Candidate selection
  is still an external fixture boundary, not a CI/image eligibility claim.
  An opt-in test fixture now includes actual Realtime helper/source bytes before
  Session admission; its cold parent smoke passes for the caller owner's join.
- Independent reviewer qualified C1 on c246 and C2 on e36ca61d79f9 with no
  remaining concrete parent finding. Production module is unchanged between
  these snapshots (SHA256 3af62505340c1022d6bbf976b9b563f9ebe77362fb64d3c23d2fa8c13dcb480b).
  Review includes source inspection and author native test evidence; reviewer
  did not independently rerun the suite. Actual caller-to-SQL, full renderer,
  installed package cohort and live eligibility remain explicitly excluded.

## Risks

Missing STAGING issuer files and unresolved real runtime evidence fail before
effects. SQL32 source guards are integrated. C2 receipt ancestry and the actual
caller composition remain required work before the overall deliverable is ready.
