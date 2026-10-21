# Source-only release policy deployment wiring

## Purpose and context

Close PL-A4-02 / PR875 static missing-configuration finding (TT c26/c27).
The accepted runtime contract is `release-capability-runtime-contract.md`;
`docs/TESTING.md`, `docs/CONTRIBUTING.md` and `.agent/PLANS.md` govern work.
Authoring base is 249d3cf06620bf3dfdc13c94556a55a36fa48d37, not a final release.
Developer 273 owns this source stage; 121 consumes reviewed source/configuration.
No live deployment, image qualification, secrets, new runtime semantics or
entrypoint changes are authorized. No final candidate policy is checked in.

## Input and consumer contract

A required closed JSON input carries approved `free-invite-alpha`, protected
candidate SHA, a complete immutable ConfigMap descriptor with exact UTF-8 v2
policy bytes and SHA256, and four owner mappings. Policy values retain the
accepted six disabled capabilities and finite fourteen intents. Caller mappings
provide absolute read-only policy directories and snapshot file paths. The
ConfigMap is a required external immutable object, not created by this stage.
Its real existence/content/permissions and image binding remain live HOLD.

Snapshot files must be direct children of `/tmp` on the container writable
layer, without a covering volume or read-only root filesystem. Current four
images directly exec one service/JVM; a process exit therefore restarts the
container with a fresh writable layer. Do not use a persistent emptyDir/PVC or
PodUID-only filename: a container restart retains the Pod. Runtime exclusive
0600 creation remains authoritative. Actual image filesystem validation is
separate live proof; this source stage checks the declared contract only.

Production validates/render inputs before any cluster command. Staging uses
the captured preservation compiler and source-plan renderer, never executes a
target helper. Old source without the new placeholder contract remains readable
by preservation; new source requires the input, with no development fallback.
No protected preservation gate or TLS/NATS/image value is relaxed.

## Scope and milestones

- [x] Causal RED: all eight existing service containers lack explicit wiring.
- [x] Strict pure input validator and semantic negative/positive fixtures.
- [x] Both manifests, production preflight and protected staging consumers.
- [x] Nine offline static checks pass; source package frozen for review/intake.
- [ ] Independent source review and accepted 121 publication handoff.

## Steps and validation

Write unittest fixtures first; demonstrate missing wiring and missing helper
failures separately. Add the stdlib-only validator/renderer, then placeholders
and actual consumer integration. Python tests decode rendered YAML with the
repository's existing PyYAML testing dependency. Include invalid/missing source,
hash, profile, mutable object, unresolved token, snapshot persistence/writeability
and Auth explicit environment negatives. Mock subprocess effects only; no kubectl
or product runtime is executed. Run focused unittest, affected preservation
tests and shell syntax checks; CI execution/publication belongs to 121.

## Progress and decisions

- c27 accepted through existing-owner TT checkpoint; fresh WT31 lease acquired.
- c29 separately accepted before editing the outer production caller. Missing
  and malformed input reached the first mock infrastructure child on RED;
  GREEN validates before dispatch, with valid-fixture ordering preserved.
- Nine final static tests pass with zero skips in an owned network-none Python
  container. No Voice service, provider, cluster, runtime or hosted CI was run.
- The affected existing 33-test subset has 31 passing methods and two failing
  methods (three assertions/subcases): compiler full/app-only both reject
  `rollout_compile_unenrolled_deployment`, and Voice media projection lacks its
  four SPACE_COMMUNICATION fields. All three failures were independently
  reproduced on untouched authoring base 249d3cf. They were not weakened or
  fixed by this scope; the broader subset is explicitly NOT PASS.
- Windows cannot import the existing Linux-only preservation module because
  `os.O_NOFOLLOW` is absent; that harness error is not causal RED. Linux fixtures
  reproduced the owner-environment failure before consumer integration.
- Graphify was unavailable on PATH; a full-path process was started before the
  user exclusion was received, then stopped by its exact owned handle with exit
  1 immediately on receipt. No freshness PASS and no further Graphify execution.
- No test/code delegation: queen alone dispatches workers; independent review
  will use an existing atomic review function after source freeze.
- Policy bytes are generated only after final protected candidate freeze. A
  source edit changes candidate SHA, so old 4ea5/fcdf artifacts cannot be embedded.

## Risks and follow-ups

Qualified source/image tuples, actual immutable policy mounts, writable private
snapshot readbacks and an authorized rollout/resource owner remain absent.
Static GREEN cannot claim startup/runtime/18-group/deployment readiness.
The protected staging root caller has not been authorized here to transport a
live descriptor or supply image/object authority: new source fails closed until
that separately authorized input arrives. No root allowlist or transport gate
was relaxed. Caller input preparation and source wiring do not supply authority.
If protected caller transport or a runtime entrypoint needs other-owner changes,
preserve the exact continuation; do not invent authority or bypass root gates.
