# ExecPlan: preserve attachment intents during migration 26 rollback

## Purpose and context

`MSG26-DOWN-FIRST-WRITER` reproduces a committed attachment intent lost while
`000026_attachment_send_intents.down.sql` waits for its destructive table lock.
The Messaging service contract requires rollback refusal while **any** saved
intent exists; drain and export are operator prerequisites, not automatic cleanup.
Sources: `docs/microservices/messaging-service.md` lifecycle preflight section,
`docs/OPERATIONS.md` migrations, `docs/PLAN.md` A4, `docs/TESTING.md`,
`docs/DATA_MODEL.md`, `docs/DATA_STORES.md` and the task's real PG16 interleaving.

## Scope and decisions

Only migration 26 DOWN, its store integration regressions and this evidence plan
change. UP, runtime send/reconciliation behavior, other migration owners and live
databases remain outside this slice. Acquire a write-excluding table lock before
the guard, retain it through DROP in one atomic statement, and require READ
COMMITTED: a lock cannot refresh an existing repeatable-read snapshot.

## Milestones and validation

- [x] Read the owning contract and atomically claim the task; fresh isolated
  Treehouse branch from local develop `87483dbc0cb55b61a96337512d3307063b2fd0a6`.
- [x] Add tests before production changes and preserve real Linux PG16 RED.
- [x] Apply the minimal lock/isolation correction and obtain GREEN.
- [x] Run affected store tests under race, vet, lint and inspect the final diff.
- [ ] Publish immutable evidence/source, independent review and accepted delivery
  handoff; source verification does not imply stand rollout or acceptance.

The first-writer test holds an actual valid INSERT uncommitted, observes the DOWN
backend blocked on the table lock, commits the writer and checks both refusal and
the complete retained binding. The old-snapshot test pins an empty RR snapshot,
commits the first intent on another connection and requires refusal. Controls
cover every retained state and empty DOWN followed by exact UP.

`MSG26_TEST_DATABASE_URL` opts into an owned private PG server; each test creates
and drops only its own uniquely named database. Without that variable tests use
the existing Testcontainers helper. No host listeners or shared environment.
Run from `src/backend/messaging`: `go test -race -count=1 -run
'^TestAttachmentIntentDowngrade' ./internal/store`, then affected existing tests,
`go vet ./internal/store` and repository-pinned lint. Record exact stdout and
source-byte hashes with the final evidence. Reviewers self-claim the published
function; the author does not self-review or spawn agents.

## Progress and risks

The first-writer and old-RR tests both failed with an unexpected successful DOWN
on unchanged production SQL; controls for all three retained states and empty
DOWN-UP passed (`-race`, package 5.743s, no skips). The corrected DO now owns the
isolation check, exclusive lock, original retained-intent guard and DROP.
Linux Go1.26.5 PG16 focused race GREEN: 3 top-level / 4 subtests, 14.448s, no
skips; affected store short tests 0.023s, vet exit0, pinned golangci-lint2.6.1
reports 0 issues. Native runners use 1 CPU, 2GiB and GOMAXPROCS=2; the dedicated
PG pool cap is4. Red archive SHA256 `d0aeda4a0185957db28487f10e0184bc73ea293ff7ad750f2e6e4b392dc6ead0`,
green archive `9329dc1c0b0e0464742907e78ad4941620e5544d173d5154ee0c499c3f8f0a6b`.
Existing full-file migration execution uses a single SQL query; the DO statement
is also atomic without an executor transaction.
An initial incomplete-archive/setup attempt and an unsupported cgroup diagnostic
failed before tests and are not behavioral RED evidence. A concurrent optional
short-test exec was interrupted when the original container completed its vet;
the separate final short runner exited0. No deployment or historical data export
is authorized by this fix. Independent source review and current integration CI
remain separate delivery gates. Graphify refresh is tracked as an auxiliary
bounded process; it is not source or runtime correctness evidence.
