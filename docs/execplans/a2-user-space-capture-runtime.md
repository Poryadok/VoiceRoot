# ExecPlan: A2 — User Space Capture Runtime

## Purpose
Prepare PR #1048 for review and merge to `master`: connect the dedicated User Space capture runtime, prove the complete Space → User → Ban flow in an isolated Docker/Compose environment on the final PR head, and record verified DEV/STG rollout prerequisites in TTenrec. Merge comes before rollout. Production rollout is out of scope because no release has occurred.

## Context
- TTenrec task: `A2-USER-SPACE-CAPTURE-RUNTIME`.
- PR #1048 currently points at `9abae6ae3e40e51939e47a7be6bda0b3baebb6e3`, is open and draft, and has 70 successful plus 16 skipped checks at the last inspection; no review decision is recorded.
- PR is broad (about 280 changed files, including public game/GIS work) and must not be merged solely on the strength of A2 checks.
- Authoritative worktree is `.treehouse/Voice-451d38/48/Voice`, clean at the inspected PR head. Preserve other active leases and untracked slot 63 host-session files.
- Existing T40 Compose run used synthetic custody and is not evidence of qualified DEV/STG source readiness.

## Scope
- Before merge: review dedicated capture integration and fail-closed behavior, targeted tests, full isolated Compose Ban scenario, final CI/review, and TTenrec rollout prerequisites.
- After merge: separate DEV then STG source qualification, deployment, and acceptance using recorded evidence and owners.
- Excluded: production rollout/acceptance; treating host-custody prototypes as production controls; automatic `Bootstrap`, `ActivateGuards`, or generated provenance.

## Milestones
- [x] PR head, CI, base, task state, and active worktrees/leases inspected.
- [ ] Verify runtime contract: only dedicated Space capture, complete mTLS/JWKS/replay settings, readiness/cancel/shutdown, qualified canonical identity-association source, retained key/source provenance; fail closed on any qualification error.
- [ ] Run User and affected package tests; run full isolated Docker/Compose Ban scenario on final PR SHA and record SHA, image digest, command, result, and synthetic fixture limitations.
- [ ] Record DEV/STG prerequisites, evidence gates, and accountable owners (or explicit unassigned blockers) in TTenrec.
- [ ] Independent review and required CI green on final head; merge PR with merge commit before rollout.

## Detailed Steps
1. Continue only in the authoritative PR worktree. Recheck PR head, `origin/master`, CI, and leases before changes; do not import stale handoff commits without review.
2. Compare runtime/admission code and tests against the User capture contract. Keep capture closed unless the pinned source, complete keyset, actual database identity, mTLS/JWKS/replay, and custody qualification are verified. Do not add automatic source bootstrap, guard activation, or provenance minting.
3. Inspect the actual T40 test and fixture instructions. Run it only against a confirmed isolated Compose project and synthetic fixture; it must assert HTTP Ban and the resulting Space membership/ban effect. Never point it at DEV/STG.
4. Update TTenrec to state merge-before-rollout and enumerate only documented prerequisites: physical source identification; schema/baseline and identity-association qualification; complete writer inventory; custody and controlled guard opening; environment-specific TLS/JWKS/replay secrets; service readiness and full Ban acceptance. Include named owners where established; otherwise record an unassigned owner/blocker and follow-up task rather than guessing.
5. Run targeted tests and all required PR CI on final head. Obtain independent review, resolve findings, rerun affected checks, and merge with a merge commit. Record the merge SHA; start rollout only after merge and separate DEV/STG gates.

## Validation
- [ ] `go test -short ./...` from `src/backend/user`.
- [ ] Relevant source-custody/runtime package tests pass; run race checks where required by existing CI/PR.
- [ ] Confirm exact location and run `go test -tags=live -run '^TestT40ComposePublicClientPartyThreeMatchesAndChatMembership$' -count=1 .` from its owning package against isolated Compose. Record image digest and fixture limits.
- [ ] Verify fail-closed incomplete configuration and unqualified source, mTLS identity/CA/server-name, runtime shutdown, and successful end-to-end capture/Ban.
- [ ] All required checks green; independent review accepted; merge SHA recorded.
- [ ] TTenrec has DEV/STG owners and evidence gates or explicit linked blocking tasks.

## Progress
- [x] ExecPlan saved in the authoritative PR worktree.
- [x] Repository/task/PR/worktree state inspected; no files changed in main checkout.
- [ ] Runtime and integration test review, isolated Compose run, tracker update, final review/merge remain.

## Decisions
- Merge precedes rollout. Docker/Compose is a useful pre-merge integration gate, while synthetic custody does not qualify a real DEV/STG source.
- DEV testing before merge is optional, not a required gate.
- Host-level custody prototypes are not accepted as production controls absent qualified environment evidence.
- Production rollout is excluded because production has not been released.

## Risks And Follow-Ups
- PR #1048 has a broad cross-feature diff; its full scope needs independent review.
- The last inspected PR head has no recorded review decision; check again before any merge.
- T40 must be rerun on the final head; a previous synthetic-custody pass is historical evidence only.
- DEV/STG rollout is blocked until source qualification, writers, custody/guard procedures, credentials, owners, and acceptance evidence are established.

## Execution update — 2026-11-01
- User: `go test -short ./...` passed in `src/backend/user`.
- Space: `go test -short ./...` failed in `src/backend/space/internal/nativeinstaller` runner tests on Windows: expected child process exit classifications were reported as `driver_exit_unknown` / exit code `-1`. The PR diff in this package only changes `window_integration_test.go`; hosted PR CI was green at inspection. This local platform-specific result is retained for review, not silently counted as a pass.
- Controlledgame: `go test -short .` passed.
- User race checks for `internal/sourcecustody` and `internal/identityassociation` are still running; record their final result before closing this checkpoint.
- T40 has not been run. The isolated Compose project is absent (`docker compose ls` showed no project); the retained fixture files contain local synthetic material but no Compose service declaration. Numerous unrelated containers are running, so do not reuse them. Reconstructing the fixture requires locating or restoring its exact Compose definition and proving project/resource isolation first.
- PR remains draft at `9abae6ae3e40e51939e47a7be6bda0b3baebb6e3`, with green hosted checks but no recorded independent review. Do not merge until the Compose gate and independent review are satisfied.
