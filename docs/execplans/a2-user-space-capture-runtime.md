# ExecPlan: A2 — User Space Capture Runtime

## Purpose
Prepare PR #1048 for review and merge to `master`: connect the dedicated User Space capture runtime, prove the complete Space → User → Ban flow in an isolated Docker/Compose environment on the final PR head, and record verified DEV/STG rollout prerequisites in TTenrec. Merge comes before rollout. Production rollout is out of scope because no release has occurred.

## Context
- TTenrec task: `A2-USER-SPACE-CAPTURE-RUNTIME`.
- PR #1048 currently points at `2bf87e7938ee172460776a1e2366e63dbf904ff4`, is open and draft, and hosted CI run `41339581848` passed; no review decision is recorded.
- PR is broad (about 280 changed files, including public game/GIS work) and must not be merged solely on the strength of A2 checks.
- Authoritative worktree is `.treehouse/Voice-451d38/48/Voice`. A GIS keep-group review fix and its tests are in progress. Preserve other active leases and untracked slot 63 host-session files.
- Existing T40 Compose run used synthetic custody and is not evidence of qualified DEV/STG source readiness.

## Scope
- Before merge: review dedicated capture integration and fail-closed behavior, targeted tests, full isolated Compose Ban scenario, final CI/review, and TTenrec rollout prerequisites.
- After merge: separate DEV then STG source qualification, deployment, and acceptance using recorded evidence and owners.
- Excluded: production rollout/acceptance; treating host-custody prototypes as production controls; automatic `Bootstrap`, `ActivateGuards`, or generated provenance.

## Milestones
- [x] PR head, CI, base, task state, and active worktrees/leases inspected.
- [x] Review runtime contract: dedicated Space capture RPC only; complete pinned mTLS/JWKS/replay, Auth parent, custody binding and keyset; startup waits for qualified custody; a held grant verifies physical source and identity state before installing the guarded store; failure/cancellation keeps User admission closed.
- [x] `go test -short ./...` passes in User; `go test -short .` passes in Controlledgame.
- [ ] Repeat the full isolated Docker/Compose Ban scenario on the final PR head after the GIS review fix and record images, command, result, and synthetic fixture limitations. It passed on `cca6c12abafd5c64f3dbf2c39c84bf6b9606fe5d` before that fix.
- [ ] Complete the remaining source-custody/runtime race checks or establish their coverage in required CI.
- [x] Record DEV/STG prerequisites, evidence gates, and explicitly unassigned source/custody owners in TTenrec.
- [ ] Independent review and required CI green on final head; merge PR with merge commit before rollout.

## Detailed Steps
1. Continue only in the authoritative PR worktree. Recheck PR head, `origin/master`, CI, and leases before changes; do not import stale handoff commits without review.
2. Compare runtime/admission code and tests against the User capture contract. Keep capture closed unless the pinned source, complete keyset, actual database identity, mTLS/JWKS/replay, and custody qualification are verified. Do not add automatic source bootstrap, guard activation, or provenance minting.
3. Inspect the actual T40 test and fixture instructions. Run it only against a confirmed isolated Compose project and synthetic fixture; it must assert HTTP Ban and the resulting Space membership/ban effect. Never point it at DEV/STG.
4. Update TTenrec to state merge-before-rollout and enumerate only documented prerequisites: physical source identification; schema/baseline and identity-association qualification; complete writer inventory; custody and controlled guard opening; environment-specific TLS/JWKS/replay secrets; service readiness and full Ban acceptance. Include named owners where established; otherwise record an unassigned owner/blocker and follow-up task rather than guessing.
5. Run targeted tests and all required PR CI on final head. Obtain independent review, resolve findings, rerun affected checks, and merge with a merge commit. Record the merge SHA; start rollout only after merge and separate DEV/STG gates.

## Validation
- [x] `go test -short ./...` from `src/backend/user`.
- [ ] Relevant source-custody/runtime package tests pass; run race checks where required by existing CI/PR.
- [ ] Repeat T40 Compose Ban with `VOICE_T40_COMPOSE_PUBLIC_SESSIONS=1` on the final head. The prior A2-enabled pass used actual Auth-provisioned profiles in the same User source and synthetic custody; see execution update below.
- [x] Review/test fail-closed incomplete config, capture-only protected listener, mTLS/JWKS/replay transport and runtime lifecycle through User `go test -short ./...` on the current PR head. Qualified live source and end-to-end capture/Ban are not verified.
- [ ] All required checks green on the final PR head; independent review accepted; merge SHA recorded.
- [x] TTenrec records DEV/STG evidence gates and explicitly unassigned source/custody ownership blockers.

## Progress
- [x] ExecPlan saved in the authoritative PR worktree.
- [x] Repository/task/PR/worktree state inspected; no files changed in main checkout.
- [ ] Finish and re-review the GIS keep-group fix, repeat A2-enabled Compose Ban on its final head, finish required CI, and merge.
- [x] Current execution and tracker comments c73, c77, c78, and c79 recorded in TTenrec; linked blocker `A2-T40-COMPOSE-SOURCE-QUALIFICATION` is unassigned for human appointment. Isolated Compose project stopped with volumes/artifacts preserved.

## Decisions
- Merge precedes rollout. Docker/Compose is a useful pre-merge integration gate, while synthetic custody does not qualify a real DEV/STG source.
- DEV testing before merge is optional, not a required gate.
- Host-level custody prototypes are not accepted as production controls absent qualified environment evidence.
- Production rollout is excluded because production has not been released.

## Risks And Follow-Ups
- PR #1048 has a broad cross-feature diff; its full scope needs independent review.
- The last inspected PR head has no recorded review decision; check again before any merge.
- T40 must be rerun on the final head; a previous synthetic-custody pass is historical evidence only.
- The T40 source uses synthetic custody and an already guarded fixture; it provides integration evidence only. DEV/STG writer and custody qualification remain separate rollout prerequisites.
- Before deploying the additive GIS keep-group change to DEV or STG, quiesce old GIS writers and requests, drain in-flight Chat calls, reconcile pending legacy `new_chat` operations and uncertain outcomes using independently verified Chat receipts and an approved recovery procedure, and verify zero pending operations before new instances serve. Do not blindly resend legacy replacement requests. A changed `AddOnly` request has a different owner request hash and method. Record evidence or establish that the predecessor never ran in that environment; the TTenrec task and rollout follow-up contain this gate.
- GIS validates lease, consent, policy and binding before the remote Chat mutation, so Chat can commit after that authority changes. G09 does not specify whether a permanent keep-group Chat mutation is a governed session write or where authorization linearizes. Resolve and test `A2-GIS-KEEP-GROUP-LEASE-BOUNDARY` before DEV/STG GIS rollout; the T40 pass does not establish G09.
- DEV/STG rollout is blocked until source qualification, writers, custody/guard procedures, credentials, owners, and acceptance evidence are established.

## Execution update — 2026-11-02 (independent review and GIS correction)
- Independent review found that an older standalone keep-group consent snapshot could complete after a newer one and replace its Chat roster, removing a later consenter. A deterministic integration regression reproduced the failure before the fix.
- The correction sends the existing additive Chat membership operation for both new groups and existing parties. The regression and existing keep-group tests pass, including a race run; a Chat database regression verifies that a later smaller addition and receipt replay preserve both members. Review of this change found no new blocker in that scope.
- Old pending replacement operations cannot transparently retry as additive operations because their owner request hash and method differ. The DEV/STG quiescence and reconciliation gate is recorded in TTenrec before rollout. GIS consent validation and Chat mutation remain separate calls; the expiry/authority boundary gap is tracked in `A2-GIS-KEEP-GROUP-LEASE-BOUNDARY` and is a DEV/STG rollout gate.
- The final-head T40, final CI, and the independent review recheck remain required after this code change.

## Execution update — 2026-11-02 (A2 Compose Ban passed)
- On PR head `2bf87e7938ee172460776a1e2366e63dbf904ff4`, a network-isolated copy of the stopped source showed `guards_active=true` and `baseline_cut=2`. The earlier fixture request incorrectly specified `GuardCutover=true`. Following supported recovery, a signed request with `GuardCutover=false` opened the already guarded synthetic source.
- Corrected the local User trust bundle to include the actual Phase0 CA and started Space with the A2 User capture overlay. Auth's T31 bootstrap created two profiles through A2 User; both account/profile pairs were verified in the same `user-source-db/user_capture_db`.
- `docker compose ... run --rm --no-deps -e VOICE_T40_COMPOSE_PUBLIC_SESSIONS=1 t31-session-event-runner go test -v -count=1 -run '^TestT40ComposePublicClientPartyThreeMatchesAndChatMembership$' .` passed in 9.14s. User logged `CaptureSpaceMemberAccountBindings` OK and Space logged `BanMember` OK; the test checks the HTTP Ban and persisted Space result.
- Image IDs: User `sha256:1ceb6ef129fd07226c92a86fad649768bb7a99ab9dc8e4de060834857ce64dc1`; Space `sha256:888c29597321614ef97a27e1899041e7d1fc3f266bc8004d829855aff2a8d0b6`; custody `sha256:781bcee453a666b7752b4868ac2765488782ab756fdf339b0e345a48a86db9f3`; runner `sha256:c01d669321c72461439d8850600f07d59217fdddb47dae41c38233e31e075698`. The fixture uses synthetic custody and GIS bindings, and is not DEV/STG qualification.
- TTenrec parent comment `c80` and linked fixture issue comment `c1` record the pass and correct the earlier unconfirmed writer-grants hypothesis. Hosted PR checks show 70 passed and zero failed; independent review is still required.

## Execution update — 2026-11-02 (custody fixture diagnosis)
- The pinned physical metadata in the synthetic open fixture matches the retained source DB: system identifier 7703065912053952555, database user_capture_db, OID 16388; the control DB identifier also matches.
- Corrected the startup order: A2 Auth first, then A2 User (which publishes JWKS while admission awaits custody), then user-custody. The mTLS /challenge endpoint then returned HTTP 200. The earlier conclusion that the endpoint did not serve was wrong.
- A focused test confirmed openPinnedPostgres constructs a lazy pool without connecting, so a closed source database does not prevent the custody operator from starting. The diagnostic test was removed; no production code or test changes remain.
- The signed local fixture /open request returned HTTP 503. The control row is now CLOSED / RECOVERY_REQUIRED; the physical source still has datallowconn=false. Custody logs only report source custody unavailable, so the exact guarded activation/open failure is not yet identified.
- Do not submit another /open, manually change the physical database/catalog, activate guards, or issue provenance. First identify the failed validation and use a supported recovery procedure, if one exists. Preserve the isolated project volumes and artifacts.
- Full T40 remains incomplete; this is not evidence of User capture acceptance, and the fixture still does not qualify DEV/STG custody.

## Execution update — 2026-11-02 (tracker recovery)
- TTenrec comment `c73` records the current PR SHA, successful hosted CI run, T31 bootstrap, the T40 test's explicit skip without its opt-in flag and subsequent HTTP 503 failure with the flag, the missing A2 runtime in the User container used for that attempt, and the synthetic custody `RECOVERY_REQUIRED` result. It also records that no guard/database/provenance protection was bypassed and Compose was stopped with volumes retained.
- The task description already lists DEV/STG prerequisites and evidence gates and states source/custody owners are unassigned; no owner was guessed. Those task criteria are recorded. Specific owner assignments remain a rollout blocker.
- T40 Ban did not pass. It returned `moderation service unavailable` / `complete account ban authority unavailable`; this attempt cannot count as A2 Space → User capture → Ban acceptance because its User container did not run the A2 capture overlay.
- Hosted CI run `41339581848` is successful on PR head `2bf87e7938ee172460776a1e2366e63dbf904ff4`. PR #1048 remains draft/open with no review decision. Do not merge until the actual A2-enabled Compose acceptance, independent review, and final required checks pass.

## Execution update — 2026-11-02 (A2 custody challenge and failed open)
- Restarted the retained isolated Compose project in the required order: A2 Auth, A2 User (JWKS listener published while capture admission awaited custody), then user-custody. The mTLS challenge request returned HTTP 200.
- openPinnedPostgres creates lazy pools without connecting; a focused diagnostic test passed before any product-code change and was removed.
- The signed synthetic fixture open request returned HTTP 503. The handler deliberately returns a generic error; its logs only say source custody unavailable, so the failing phase (preparation, guard cutover, or final open) is not distinguished by current operator diagnostics.
- Read-only control/catalog checks show CLOSED / RECOVERY_REQUIRED and source datallowconn=false. Do not retry open until supported recovery and the rejected validation are understood. No database flag, guard, bootstrap, or provenance was manually changed.
- The isolated project containers are stopped; its volumes and artifacts remain. No product source or test changes remain. Full T40 is incomplete and has no passing image digest/result.
- This confirms the prior T31 bootstrap used postgres/user_db in base User, whereas A2 User is configured for user-source-db/user_capture_db; those profiles cannot be used as same-database acceptance evidence.

## Execution update — 2026-11-02 (supported closed recovery and qualification checks)
- Ran the built-in recover-activation command against this isolated project only. It completed successfully and returned the control state to CLOSED / RECOVERED_CLOSED; source user_capture_db remains datallowconn=false. This is a fencing recovery only and does not authorize capture.
- Read-only checks reconfirmed control/source system IDs and database OID match the fixture, custody_app exists with LOGIN disabled, no other non-superuser LOGIN role has CONNECT to user_capture_db, and there are no sessions or prepared transactions for the source/writer.
- The existing source-custody code requires the writer role to satisfy additional source-database confinement checks before guarded cutover. Because user_capture_db remains closed, those database-local grants/schema checks are not safely inspectable through the supported reader path. The current fixture files do not document a supported writer privilege provisioning procedure for this independently managed source.
- No second signed open, manual catalog/grant changes, guard activation, bootstrap, or provenance issuance was performed. Only the isolated source/control DB containers were restarted for read-only checks; they are stopped again. Volumes and artifacts are preserved.
- This remains the concrete Compose blocker: establish the supported source initialization and writer-role qualification for the A2 User database, then provision T31 profiles through Auth while A2 User is active and rerun T40. No full Ban acceptance exists yet.

## Execution update — 2026-11-01 continuation

- Rechecked PR #1048: current head remains `2bf87e7938ee172460776a1e2366e63dbf904ff4`, base `master`, draft/open, no reviews.
- Compose: a new isolated project `voice-a2-t40-1048-fresh-20261101` provisioned a fresh main Postgres volume successfully (`compose migrate OK`). Repeating `up` against that already-provisioned volume correctly triggered the creation-custody HOLD; no guard was bypassed and no volume was removed.
- Compose bootstrap: Auth HTTP health was green and its mTLS identity-authority listener was observed on port 9094. The Auth certificate chain needed a test-only CA bundle; the bundle now validates the Auth certificate. This did not yet resolve User startup.
- A focused diagnostic test confirmed that User capture config, identity keyset/digest, lazy custody client, User JWKS listener, and Auth mTLS readiness can all initialize with the local fixture. The running User container still exits with the generic `qualified User capture source unavailable`; exact failure within qualified grant/physical-source binding is not yet established. The full T40 Ban test has therefore **not** passed.
- The local `go test -tags=integration -count=1 ./internal/sourcecustody` run on Windows failed at the 10-minute test timeout; multiple Docker/Testcontainers cleanup and RPC operations stalled. Several source-custody tests also failed. Treat this as incomplete local evidence, not a product pass. The focused `TestActivationOperatorCloseFinishesAfterCallerDisconnects` run passed previously.
- Hosted CI attempt 2 for run `41339581848` is in progress; attempt 1 had failed `flutter` (`encrypted-opus.ogg`, `ciphertext_format`) and `backend-go-integration-pr (user)` (`InitializeClosedSource`, source custody unavailable).
- The new Compose stack has been stopped without deleting volumes. Historical stacks and artifacts remain preserved.
- TTenrec is still unavailable per user instruction; no new tracker update was made during downtime.
- No product source changes were made in this continuation. Temporary diagnostic test code and a temporary port overlay were removed.
- Current completion gaps: determine why User exits at qualified grant/source validation; complete the actual Docker/Compose T40 Ban test; resolve/verify hosted CI attempt 2; obtain independent review. Do not merge while any of these gates remain unresolved.

## Execution update — 2026-11-01
- User: `go test -short ./...` passed in `src/backend/user`.
- Space: `go test -short ./...` failed in `src/backend/space/internal/nativeinstaller` runner tests on Windows: expected child process exit classifications were reported as `driver_exit_unknown` / exit code `-1`. The PR diff in this package only changes `window_integration_test.go`; hosted PR CI was green at inspection. This local platform-specific result is retained for review, not silently counted as a pass.
- Controlledgame: `go test -short .` passed.
- User race checks for `internal/sourcecustody` and `internal/identityassociation` were started but produced no output for more than seven minutes; the process remained alive and mostly idle, so it was interrupted. Race validation is incomplete and needs a bounded rerun/investigation when a suitable environment is available.
- T40 has not been run. The isolated Compose project is absent (`docker compose ls` showed no project); the retained fixture files contain local synthetic material but no Compose service declaration. Prior task evidence further requires actual Auth-provisioned profiles in the same User database whose qualified capture is called by Space; the available process fixture uses a separate test-created User source and fixture Auth, so it cannot substitute. Numerous unrelated containers are running, so do not reuse them. Reconstructing the fixture requires locating/restoring the exact Compose definition and proving the shared database, actual Auth setup, project/resource isolation, and synthetic custody limits first.
- Original code head was `9abae6ae3e40e51939e47a7be6bda0b3baebb6e3`; subsequent commits only add this ExecPlan. PR remains draft, hosted checks for the current documentation head are running, and no independent review is recorded. Recheck the live PR head before proceeding; do not merge until the Compose gate, current-head CI, and independent review are satisfied.
