# Local affected-check planning

`python scripts/ci/local_source_snapshot.py --base <full-base-commit> --head <full-HEAD-commit> --head-ref <branch>`
prints a local plan using the same filters and Go dependency resolver as the PR
planner. The base must be an ancestor of HEAD. The plan includes committed
changes since the base and the current tracked and non-ignored untracked files.
It stages into temporary Git indexes and does not modify the real index.

The local source identity contains HEAD, a Git tree and a SHA-256 digest of raw
working-tree file names, modes and bytes. The snapshot is repeated around
planning, and a change causes the command to fail. Paths whose raw bytes differ
from the Git-normalized blob are also selected, so filters and line endings
cannot silently narrow a local plan. This conservative rule can select more
checks on a checkout with platform-specific line endings. Gitlinks and symbolic
links are rejected because their external contents are not captured.

This command only produces a plan. It does not run checks, capture toolchain or
test receipts, account for ignored files that influence a build, isolate Docker,
or establish a passing preflight. Revalidate source inputs around actual
execution and use the hosted merge-candidate gate; do not use this plan as a
merge or release approval.

The Go matrix resolver requires `jq`; a missing or failing `jq`, invalid filter
output, or a failed Docker-consumer parse stops planning. It accepts CRLF output
from Windows `jq` without dropping selected services. Install a working `jq`
before running the local planner.

For a Flutter run captured with `flutter test --reporter json`,
`python scripts/ci/flutter_test_events.py --events <log> --suite <absolute-test-file>`
checks the expected suite, visible test starts and completions, runner outcome,
and skips, then reports counts and a hash of the original event bytes. Repeat
`--suite` for every selected file. Skips are rejected unless `--allow-skips` is
explicitly supplied, in which case they are listed. This parser does not run
Flutter, bind the stream to source or toolchain identities, establish the
process exit code, or replace the full unit/widget file inventory receipt.

For an isolated Flutter diagnostic, run
`python scripts/ci/local_flutter_execution.py --head <full-HEAD-commit> --output <new-directory-outside-checkout> --flutter <flutter-executable>`.
By default it discovers the full `test/` inventory; repeat `--suite` with a path
relative to `test/` to diagnose a smaller selection. Known `@TestOn('browser')`
suites run separately on Chrome; unsupported platform annotations fail closed.
It copies Git-visible
source bytes into the new directory, uses a private pub cache and temporary
directory, runs locked dependency resolution, the repository SQLite prefetch,
analysis and JSON-reported tests, rechecks captured files after commands, and
rejects package-cache changes after dependency preparation.
Logs and `diagnostic.json` are retained there. The command reports `incomplete`
and exits nonzero even when those checks pass: SDK, package, generated and
ambient inputs are not fully bound, opt-in live skips need classification, and
the complete affected-check preflight and Docker scheduler are not integrated.
Do not use this diagnostic as a passing preflight or PR handoff receipt.

The local source snapshot, Flutter execution diagnostic and Flutter event
parser, including their fixture tests, are explicit local-only CI inputs. A PR
touching only those files still runs the CI script test shards and the current
diff contract; it does not select the broad hosted component matrix solely for
those paths. The hosted affected planner calls the snapshot only for local
plans. Changes to the planner, workflow, filters, Makefile, or an unknown CI
helper continue to widen the hosted checks.
