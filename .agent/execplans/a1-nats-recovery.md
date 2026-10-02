# ExecPlan: Recover staging NATS and resume A1 acceptance

## Purpose

Restore `voice-staging` without deleting the namespace or unrelated cluster
resources, complete the interrupted NATS credential rotation, then continue A1
staging acceptance with verifiable run/SHA, readiness, smoke, and E2E evidence.

## Context

- `docs/PLAN.md` defines A1 acceptance scope; this plan addresses a staging
  infrastructure blocker and does not claim the rest of A1 is complete.
- `docs/DEPLOYMENT.md` defines staging NATS generations, immutable Secrets/PVCs,
  live ACL proof, and state-preserving deploy requirements.
- `docs/TESTING.md` defines the CI and integration checks to run.
- `deploy/templates/nats-realtime-acl-proof.yaml` mounts Realtime credentials
  into a proof Pod whose image runs as UID/GID 65532 (`src/backend/realtime/Dockerfile`).
- `scripts/staging/rotate-nats-root.sh` currently has no continuation path for
  a target generation whose marker is already `rotating`.
- Current evidence: activation run `36697211586` set marker `rotating/r20260930a3`
  but failed the live proof with `hub_connect_failed`; rollback run
  `36698242541` returned to legacy but failed the stale Realtime bootstrap.
  NATS PVCs are retained, and service deployments remain scaled to zero.
- A NATS authentication JWT appeared in an earlier diagnostic tool output. Its
  credential generation must be revoked by a successful rotation; never repeat
  or log its value.

## Scope

- In: make the proof Pod able to read only its projected credentials; add a
  guarded continuation operation for the exact already-rotating generation;
  validate contracts; use it to finish generation `r20260930a3`; then continue
  the authorized staging deployment and A1 tests.
- Out: namespace reset, production changes, changing any other namespace,
  deleting retained PVCs, or changing product privacy behavior.
- The observed file-attachment behavior is already documented: friends may
  receive files when the recipient's `allow_files` audience includes them;
  rejection for a non-friend is an expected privacy gate.

## Milestones

- [x] Add failing contract coverage for proof credential readability and
  continuation of an exact interrupted generation.
- [x] Implement least-privilege proof volume access and guarded continuation.
- [ ] Run relevant contract checks and CI; create and merge the fix through the
  repository's merge-commit workflow.
- [ ] Resume `r20260930a3`, prove live ACL, restart leaves, and verify marker,
  readiness, and retained legacy PVCs.
- [ ] Deploy the exact current master SHA in full mode with smoke and continue
  A1 acceptance; record remaining gaps in the A1 TODO.

## Detailed Steps

1. Preserve the current marker, retained legacy PVC, and `r20260930a3` Secrets.
   Do not reset `voice-staging`.
2. Add a proof template contract asserting `fsGroup: 65532` and group-readable
   (`0440`) projected credential files; demonstrate failure before changing the
   template.
3. Add rotation contract coverage for a continuation that requires marker
   `rotating`, the exact requested generation and source, valid immutable target
   Secrets, a Bound target PVC, source-bound stopped leaves, and fixed workload
   identities. It must reuse existing target resources, run bootstrap and live
   proof before restarting leaves, and leave the marker rotating on failure.
4. Implement the continuation operation and expose it only as an explicit
   staging workflow operation. Keep all mutations limited to `voice-staging`
   NATS and its owned app leaf deployments.
5. Run the focused Bash contract tests, syntax checks, and CI. Review the full
   diff before merge.
6. Refresh the short-lived proof credential without exposing its value, invoke
   continuation for `r20260930a3`, and verify ACL proof, `active` marker,
   generation-bound deployments, rollout readiness, and retained old PVCs.
7. Run the normal full staging deploy on the exact merged master SHA with smoke;
   do not pass a wipe flag or run a namespace reset.
8. Continue manual and automated A1 checks. Report any account action that only
   the user can perform as pending rather than marking A1 accepted.

## Validation

- [x] `bash scripts/staging/nats-live-acl-proof-contract_test.sh`
- [x] `bash scripts/staging/nats-root-rotation-contract_test.sh`
- [x] Shell syntax checks for changed staging scripts.
- [ ] CI for the fix PR is green before merge.
- [ ] Staging NATS marker is `active r20260930a3`; hub and leaves are ready;
  old PVCs remain Bound; no other namespace changed.
- [ ] Full staging deploy run SHA matches merged `master`; rollout, health, and
  smoke checks pass.

## Progress

- [x] Root cause investigation identified stale legacy Realtime ACL; prior
  bootstrap repeatedly failed on a JetStream consumer subject.
- [x] Issued and safely stored generation `r20260930a3`; GitHub environment
  bundle and short-lived proof credential are uploaded without printing values.
- [x] Activation reached the live proof but failed `hub_connect_failed`.
- [x] Legacy rollback was attempted and failed at the known stale Realtime
  bootstrap; no namespace or PVC was deleted.
- [x] Implement proof credential permission fix and safe continuation path;
  both focused contract tests and Bash syntax checks pass in a disposable Alpine
  container. The continuation mock persists Deployment replica changes after
  `kubectl scale`, matching Kubernetes behavior.
- [ ] Document, run CI, create/merge the fix PR, refresh proof creds, and
  continue `r20260930a3` only after the merged workflow is available.

## Decisions

- Keep the already-created generation and PVC. Reuse them through a narrowly
  guarded continuation rather than creating another generation or deleting
  state.
- Require live ACL proof before restarting any leaves or setting the marker
  active.
- Grant the nonroot proof container access through a pod `fsGroup` and group
  read only (`0440`), not world-readable Secret volumes.

## Risks And Follow-Ups

- Staging app leaf deployments are currently stopped while the marker is
  `rotating`; restore them only after the new generation passes its live proof.
- Until activation succeeds, the previously exposed legacy credential has not
  been proven revoked.
- A1 has other open acceptance items; successful staging recovery alone does
  not complete A1.
