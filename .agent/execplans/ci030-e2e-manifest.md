# CI-030: deterministic E2E manifest selection

## Purpose and scope

Read the YAML E2E catalog without shell quoting/comment ambiguities and stop
before test execution on malformed selection. Consumers are the existing smoke,
full, restart and A1 runners; keep their scenario lists and ordering unchanged.
Runtime/flake improvement is limited to removing parser subprocess fanout and
invalid/partial selection; no broad live-suite duration improvement is claimed.

## Sources

TT CI-030, docs/TESTING.md (manifest and Compose runners), docs/CONTRIBUTING.md,
.github/ci/e2e-features.yml, scripts/ci/e2e-manifest{,_test,_helper_test}.sh,
all manifest consumers and their existing fixture tests, CI job dependency setup.
CodeGraph warned its index is frozen; current files were confirmed directly.

## Implementation and validation

- [x] Same author adds real shell-entrypoint fixtures: quoted/flow YAML,
  ordering, duplicate keys/items, invalid types, empty/missing section, malformed
  YAML and valid full_gateway_run scalar; confirm meaningful RED in Linux.
- [x] Use pinned existing PyYAML 6.0.3 family and Python3 SafeLoader, reject
  duplicate mapping keys; validate complete selected value before stdout.
- [x] Preserve shell CLI, all existing lists, and make full runner use the same
  parser for its scalar. Smoke parse failures must stop before Go/Flutter work.
- [x] Declare/install dependency in actual CI consumer jobs and document local
  prerequisite. Fix fixtures that copy the parser shell without its Python peer.
- [x] Focused GREEN, existing parser/helper/runner fixture checks, workflow YAML
  and setup coverage, shell syntax and diff checks in owned isolated Linux.
- [ ] Independent source review plus integration/exact-head required CI by a
  real ant; portable source/proof and guarded lease return before handoff.

## Resources and risks

Developer Рудеус-160 owns WT39 lease4c116a5ff7163844c3c596a954df8a30,
branch codex/ci030-e2e-manifest, base actual develop
ab6aa8e224f31f2ec55e93b923a43d9b7867ec76. No shared stand or host listener.
Do not run real Go/Flutter live suites on the host; fixtures must intercept
external commands. Missing Python/PyYAML must fail with an actionable message,
never fall back to the old parser or guess a different test set.

## Progress and decisions

- Initial source reconciliation: shell strips comments before YAML decoding,
  prints quoted identifiers literally and silently ignores non-list values.
  Full runner separately uses awk; smoke logs a parser failure but can proceed
  with a partial nonempty list. No test result yet.
- Full historical live runtime/flake qualification remains a separate actual
  Linux Compose/Flutter proof; this change cannot reduce scenario coverage to
  achieve a timing target.
- Meaningful Linux RED: quoted/flow/scalar parsing, invalid list types/duplicate
  keys and both runners' external-work-before-parser-failure produced 8 failing
  assertions in 6 test functions. GREEN: 7 functions including explicit pinned
  dependency setup in all five CI consumers. All six old list outputs compare
  byte-identical against baseline parser on the unchanged catalog.
- Existing parser/helper/smoke/full fixtures passed. The A1 multi-account fixture
  initially failed because its Python mock swallowed the new parser invocation;
  a narrow real-parser passthrough fixes that fixture without launching external
  commands. All three A1/profile-handoff/restart fixture suites then passed.
  Linux Node24 image supplied Python3.11.2 and read-only cached PyYAML6.0.3;
  network disabled. Shell syntax and CRLF-aware diff checks passed.
- Own initial Graphify update99709 was interrupted (terminal1) before the final
  test-shim change; exact child33708/descendants absent on readback. No graph
  completion claim. Final frozen-source refresh/evidence remains to account for.
