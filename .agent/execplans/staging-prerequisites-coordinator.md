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

- [ ] Real Session test: fixed create/CAS/policy submit, exact private body and
  one-use command permission; only its response can reach DONE.
- [ ] Lost reply, claim/source drift, wrong UID/content, pending replay and
  checkpoint failure leave the durable operation uncertain without later writes.
- [ ] Executable run_prepare produces all plans and validates all children before
  any effect; current issuer/source/node and baseline are rechecked.
- [ ] Capture and historical loaders bind the complete operation to subsequent
  Realtime and rollout targets and protected PASS ancestry.
- [ ] Joined parent/children/real Session fixture and independent final review.

Tests run in an owned native Linux container with synthetic Kubernetes/TT wire
responses. They are not a live STAGING qualification. Ordinary test-first work
is followed by an independent high-risk review. Manual AST refresh is deferred
under root's shared-resource instruction until the named main refresh finishes.

## Progress

- Dependencies GIS d064206, P3 ece2c11 and OCI 39d37a7 integrated normally. OCI
  collector has a separately reported deadline fix pending; it is not final.
- Reviewed custody foundation retained. Next checkpoint is the actual executor,
  not another standalone authority scaffold.

## Risks

Missing STAGING issuer files and unresolved real runtime evidence fail before
effects. The candidate GIS migration/verifier version mismatch is assigned to
the existing integration owner and must be fixed before production admission.
