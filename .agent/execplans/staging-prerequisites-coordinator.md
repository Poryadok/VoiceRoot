# Protected staging prerequisite coordinator

## Purpose and context

Implement the finite pre-capture operation described in
`staging-full23-prerequisites.md`, `docs/DEPLOYMENT.md`, and the protected root
Session contract. One admitted root operation prepares release policy, P3
assets/publisher and GIS material before normal capture and Realtime admission.
The existing root CLI, global operation lock and TT Session remain the authority.

## Scope and decisions

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
- [ ] Joined parent/children/real Session fixture and independent final review.

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
- Current existing-object path remains HOLD. Protected history and typed capture
  loaders are implemented but historical/C2 qualification is still incomplete;
  no C2 or live readiness claim is made. Independent parent review is pending.

## Risks

Missing STAGING issuer files and unresolved real runtime evidence fail before
effects. SQL32 source guards are integrated. C2 receipt ancestry and the actual
caller composition remain required work before the overall deliverable is ready.
