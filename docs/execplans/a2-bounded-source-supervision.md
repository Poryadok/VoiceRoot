# ExecPlan: bounded User source supervision

## Purpose

Make the production custody operator require an independently authenticated,
separately running supervisor before opening User capture. Loss of supervision
permanently revokes that controller incarnation. SQL recovery either verifies a
closed/drained source or remains HOLD; it never authorizes restore or reopening
from an unknown outcome.

## Context

The selected model is bounded custodian-attested continuity in
`docs/todo/backend.md` (Space). `docs/microservices/user-service.md` describes
signed opening, dedicated capture transport and planned supervision. Existing
`identity_meta` locking and migration22 writer triggers already serialize source
facts; `SpaceStore.BanAccount` rechecks its local cohort after remote capture.
Physical identifiers alone cannot detect a clone or undeclared rollback.

## Scope

User sourcecustody operator/runtime, command entrypoint, tests and these narrowly
related docs. No shared package changes, CI scripts, live DEV/STG changes,
automatic guard activation, generic new mutex, physical watchdog or host stop.
WT64 and WT73 are preserved read-only investigation artifacts.

## Milestones

- [x] Own clean treehouse lease and bounded design recorded.
- [x] Reproduce unsupervised production opening with a failing test.
- [x] Add direct mTLS signed supervision and all production admission gates.
- [x] Add a separately runnable supervisor with actual source/control health.
- [x] Verify separate processes, loss, stale responses and recovery/HOLD.
- [x] Reconcile docs; run affected checks, graph refresh and independent review.
- [x] Commit/push and create draft PR; root owns rollout and final acceptance.

## Detailed Steps

1. Test `/open` without an independently configured supervisor: expect 503 with
   admission CLOSED and writer dormant. Confirm current master opens instead.
2. Reuse existing `supervisionSession`, nonce/sequence and Ed25519 response
   protocol over mutually authenticated HTTPS. Supervisor starts with one exact
   independently signed opening decision; its private signing key is distinct
   from the custodian and is never mounted in the controller.
3. Initial supervisor readiness uses the maintenance database on the source
   cluster while the source forbids connections. Verify actual source/control
   identities, exact binding, dormant writer, zero sessions/prepared work and
   control leadership. No startup code enables source access or guards.
4. Gate `/open`, `OpenQualified`, `AcquireAttested` and attested permit checks on
   the live exact decision. Timer expiry is independently advanced and terminal;
   controller pause or late responses cannot renew an expired incarnation.
5. Controller or supervisor loss attempts existing SQL recovery. Source outage,
   lock or prepared work leaves CLOSED/HOLD; transitions and reopen remain
   forbidden until verified recovery and a fresh signed opening. Bound wait and
   join ownership; never equate timeout with stopped work.
6. Test the executable/runtime across separate processes and real PostgreSQL.
   Reuse existing negative protocol tests and add actual transport/health cases.

## Validation

- Focused RED/GREEN operator admission regression on disposable PostgreSQL.
- Focused supervisor transport/config/security tests and separate-process tests.
- Existing affected custody/open/recovery tests, `go test -short ./...`, `go vet`
  and relevant race/integration checks in `src/backend/user`.
- Required hosted CI/review before merge; root coordinates these and deployment.

## Progress

- Lease: `a5d27ae67f9c1e003b7966eafd69cdc6`, holder
  `source-safety-gpt6-1-sol-high`, checkout
  `D:/Git/Voice/.treehouse/supervision-sol/.treehouse/Voice-451d38/1/Voice`.
- Branch `codex/a2-bounded-source-supervision`, base
  `ffad84570c38455ec77f387ad9ec04fd94411004`.
- Ordinary TDD: same author owns tests and implementation; high-risk concurrency
  and credential boundaries require root-arranged independent final review.
- Actual PostgreSQL RED: `/open` without supervision returned 204 rather than
  503. The implementation now denies it before source preparation.
- The first polling regression reproduced a ticker firing before the next
  probe was due. A deterministic RED/GREEN test now verifies that early ticks
  retain the existing deadline without issuing or extending a probe.
- Actual PostgreSQL RED exposed mixed control observations during opening.
  Each health sample now uses a fresh control snapshot; a conflicting source
  observation retries once and must prove the exact signed preparing/active
  decision. Both forced opening interleavings and stable mismatch rejection
  pass against disposable PostgreSQL.
- Separate real `sourcecustody supervise` and User processes passed capture,
  birth and primary operations, then rejected fresh capture on supervisor loss
  while User stayed running. Recovery verified the source closed and NOLOGIN.
- Real supervisor tests passed control-pin mismatch, prepared transaction,
  stalled opener and SQL transport partition cases. Prepared work and partition
  explicitly report HOLD; the partition test confirms PostgreSQL stays running
  until separately reachable SQL can verify recovery. Old decisions do not revive.
- `go test -short ./... -p=1` and `go vet ./...` passed. An earlier parallel
  module run hit an unrelated local miniredis dial timeout; it is retained in
  the ignored receipts rather than counted as a pass. Native PostgreSQL tests
  above are separate evidence; short-mode skips are not integration evidence.
- `go test -short -race ./internal/sourcecustody` passed. Native operator,
  crash/restart, recovered-leader, checked-source and activation-watch regressions
  passed together (626.065 seconds); their manual local fixtures explicitly arm
  synthetic sessions and do not replace the separate production-process tests.
- AST-only `graphify update . --no-cluster` completed: 132712 nodes and 289774
  edges. The generated local graph remains ignored under repository policy.
- Synced `origin/master` by fast-forward to `f7fe4f0b1521`; upstream changes
  were outside User. Root's independent review has no confirmed blocking finding,
  with final clearance tied to the final source fingerprint.
- Implementation commit `cb1ff09a8b12ae2104d741e269549f198b9fd68d`, tree
  `0be375504af1ae1a21575d834d74c235972ba0d3`, pushed with hooks and published
  as [draft PR #1119](https://github.com/Poryadok/VoiceRoot/pull/1119). Root
  independently reviewed that exact source and returned CLEAR. Tracker evidence
  is `A2-USER-CAPTURE-INDEPENDENT-FENCE#comment-c11`. Initial draft backend CI
  jobs were skipped, so no genuine hosted CI PASS is claimed. No merge or live
  rollout has occurred; root retains those gates.
- PR #1119 was then marked ready for genuine CI. Its first selected run exposed
  errcheck/staticcheck issues in the new transport and fixtures; response-body
  cleanup, receiver naming and HTTP constants were corrected. Local User
  `golangci-lint run --timeout=5m` reports zero issues, and focused transport,
  configuration and early-poll tests pass with `-race` (2.783 seconds).
- The affected public Space Ban test passed with real User/supervisor processes
  under `-race -tags spacebanintegration` (120.78 seconds). The companion native
  test first failed during Space fixture setup before starting User because its
  existing membership observation expired; that failure is retained separately
  and only that test is scheduled for an isolated rerun. No deadlines changed.
- Commands, source hashes, full logs and owned container identities are retained
  under ignored `tmp/source-supervision/` in this checkout. Deployment custody,
  real keys and DEV/STG qualification remain the named consumer's work.

## Decisions

- Direct HTTPS avoids an additional monitor database and capability schema.
- One supervisor process admits one signed decision; restart does not authorize
  reuse. Requalification and new decisions remain operator responsibilities.
- Source SQL failure is HOLD, not a requirement to stop a partitioned PostgreSQL
  or the shared host. Undeclared restore under failed custody remains outside
  the selected guarantee; administrator/storage/network custody is real evidence,
  never inferred from signed nonzero digests or a healthy process.

## Risks And Follow-Ups

- Existing unit/synthetic fixtures are scoped evidence only. Production rollout
  requires separately held keys/credentials, complete writer inventory, exact
  source continuity and actual deployed Ban/recovery checks.
- Root owns independent review, CI/source records and DEV then STG qualification.

## Final Native Failure Triage

- The complete Windows-native custody attempt failed at its 20-minute package
  deadline: 164 tests passed, zero skipped, ten failed; the seven mandatory
  production runtime scenarios were not reached in that attempt. This is partial
  failing evidence, not a full package PASS.
- Six failures occurred during Bootstrap/ActivateGuards setup, and one during
  the positive non-superuser physical capture. These paths retain the existing
  two-second catalog/shape/identity-lock budget. Three synthetic stream cases
  failed before their intended permit/pre-read observations; their existing
  real-clock 500ms client and 100ms heartbeat deadlines remain unchanged.
- The terminal Creation test was waiting for a child go build at line 67 when
  the global package deadline expired. The stack does not prove a Pool.Close
  cleanup hang, and no Creation cleanup fix is claimed.
- Source inspection separately found five fatal assertion paths without an
  immediate Controller cleanup fallback. Checked-source, crash-child/restart
  and writer-recovery tests now register that fallback; explicit shutdown and
  recovery assertions remain intact. This defensive fixture correction does
  not explain or erase the complete-run timeout or the ten failures.
- The existing final-fixture lint attempt exited 4 while go/packages loading
  exceeded its context deadline; no issue analysis completed and no zero-issues
  result is claimed for that snapshot. The approved focused ten-case rerun will
  use Windows-native Go with existing caches, serial execution and real clocks;
  no owned Linux harness has been established for this worker.

- The focused Windows-native ten-case attempt completed with FAIL: seven passed,
  three failed, zero skipped (270.743 seconds of test execution). PreReadAndCancels
  reached its first successful capture, then failed before the later commit gate
  with source custody unavailable; CommitWaits reported COMMIT not observed; the
  positive non-superuser physical capture returned identity association evidence
  incomplete. These remain unresolved verification failures, not cleanup fixes.
- Focused receipt source SHA256: `dace5e2e09aab3eaeb7adaa8204c8dbaf0fedd24958c7a4818856de39ab593ca`;
  `final-focused-native10.receipt.json` records the exact command and exit 1.
  Its log retains every disposable container identity and teardown. No repeat or
  timeout increase was performed. Independent review cleared the test-only cleanup
  bytes; exact-commit hosted CI remains required before merge.
