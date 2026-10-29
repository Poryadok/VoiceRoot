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
