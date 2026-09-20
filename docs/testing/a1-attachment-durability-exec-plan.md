# ExecPlan: A1 attachment durability proof

## Purpose

Provide an exact-head, hosted-CI proof that an A1 DM attachment is uploaded,
survives independent File and Messaging restarts, remains reachable from fresh
per-chat history, and downloads with its original SHA-256.

## Context

- Docs: `docs/PLAN.md` A1 DoD/verification, `docs/features/file-storage.md`,
  `docs/microservices/file-service.md`, `docs/TESTING.md` CI tiers.
- Existing proof: `src/backend/gateway/compose_file_attachment_restart_live_test.go`
  and `scripts/ci/compose-file-attachment-restart-proof.sh`.
- Contract boundary: File has terminal `DeleteFile`; it has no `RestoreFile` RPC.
  A1 proves authorized terminal deletion, access denial, and retained message
  history; a re-upload receives a new ID. Account restore and backup restore
  are A4/G1 scope.
- Runtime constraint: Windows local Compose/localhost execution is prohibited;
  runtime proof is GitHub-hosted CI on the PR exact head.

## Scope

- In: prove File-only restart against durable DB/blob stores, same-ID fresh URL
  and SHA-256 download, unauthorized/authorized terminal delete, post-delete
  metadata/URL denial, and retained message attachment history.
- Out: File resurrection/RestoreFile, A4 account restore, backup restore,
  retention/GC redesign, or local runtime execution.
- Decision: the release-smoke shorthand “delete/restore” is corrected for A1:
  `DeleteFile` is terminal and re-upload is a new logical ID; restore belongs
  to A4/G1, not this PR.

## Milestones

- [x] RED: the exact-head CI reachability contract failed because the runtime
  proof ran only after a master push.
- [x] GREEN: PR A1-path changes, including the reachability contract itself,
  now run the existing isolated runtime proof on the exact head; static and
  pre-existing runner contracts pass.
- [ ] Hosted CI: exact PR head passes `ci-gate` and the restart-proof job.
- [ ] Merge: merge commit lands on `master`; master ancestry is verified.

## Detailed Steps

1. Inspect the A1/File contracts and the existing restart runner/test.
2. Add focused gateway or runner tests before production edits; run them RED.
3. Implement the smallest scoped change, then rerun the focused checks GREEN.
4. Run static/service checks allowed on Windows; do not start local Compose.
5. Push, open PR, wait for hosted exact-head CI and inspect required jobs.
6. Update this plan and PLAN/TODO only with observed evidence; merge with a
   merge commit and verify `master` contains the merged head.

## Validation

- [ ] Focused Go test for the restart-proof behavior passes.
- [ ] Runner shell test passes when affected.
- [ ] GitHub PR exact-head: `ci-gate` and `a1-attachment-restart-proof` pass.
- [ ] Post-merge master is an ancestor of the merged PR head.

## Progress

- [x] Lease acquired: Treehouse slot 14.
- [x] Docs and current restart proof inspected.
- [x] RED test authored and independently reviewed. The runtime flow itself was
  already fully covered; the missing testable boundary was exact-PR-head CI
  reachability.
- [x] GREEN implementation and local static checks.
- [ ] Hosted CI and merge evidence.

## Decisions

- Treat File deletion as terminal unless a repository-backed restore contract is
  supplied; `DeleteFile` has no inverse RPC.
- Preserve the existing two-phase external runner: the Go test is HTTP-only and
  the runner, not the test, restarts services.

## Risks And Follow-Ups

- Exact runtime evidence depends on GitHub-hosted CI; no local Compose fallback
  is permitted for this Windows task.
- If a file-restore flow is required, it needs a documented contract and should
  be a separately authorized cross-service slice.
