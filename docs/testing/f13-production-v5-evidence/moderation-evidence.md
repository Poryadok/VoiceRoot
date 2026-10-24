# Moderation deadline fixture repair — PR875

Status: source reviewed CLEAR; final Go 1.26.9 verification GREEN. No repair commit or push.

Root base: `38330e20`; owned WT47 merge HEAD: `84bfa34931cacd37c3ab074a37e2fb63f27cf16f`.

Only changed source: `src/backend/moderation/internal/grpcsvc/automod_scan_outcomes_test.go`.
Final file SHA-256: `5fbc9e9e92d193045605018056d799f3653543d7b9fa8d017efd5fc5042cb4bc`.
Final Git blob: `d547529e2d194457da0a2ae5445f1d4ef1bdc59c`.
Patch: `fix.patch`, SHA-256 `0e6ccdfa539cae43d43e95fcfda41b8aa4ec127899217e2ad2a3f73ae0f88f48`.

## Cause and bounded change

Hosted job 120739621827 (run 40049585842, Go 1.26.9) failed the budget case with expected counter 1, actual 0. The original test starts a real 2 ms parent deadline before registry/service setup. Scheduling delay can expire that deadline before invocation; production correctly prioritizes `skipped_canceled` over `skipped_budget`. The hosted log does not identify the alternate counter directly.

The test now uses `testing/synctest` for deadline arithmetic, preserving its original 1 s / 80 ms / 2 ms budgets and all exact counter, recorder call, child cancellation and parent isolation assertions. A separate expired case explicitly waits for `parent.Done()` and asserts `DeadlineExceeded` before invocation. Production code and metric policy are unchanged. Canonical contract: `docs/microservices/moderation-service.md`, scan outcome budget rules.

## Evidence

- `hosted-120739621827.log`: downloaded original hosted failure.
- `controlled-red.log`: Go 1.26.9 controlled overlay reproduction, **not an unmodified RED**. The original test plus an explicit wait for the budget parent to expire reproduces expected 1 / actual 0. Command: `go test ./internal/grpcsvc -run '^TestAutoModScanOutcomes_BoundedAndSkipped/budget$' -count=1`; failed as intended, 0.011 s.
- Final verification ran from an isolated Linux source snapshot with the exact file hash above, delegated to `join_test_runner`: `go test -race ./internal/grpcsvc -run '^TestAutoModScanOutcomes_BoundedAndSkipped$' -count=500 -timeout=180s` PASS (1.771 s); `go test -short -race ./... -count=1 -timeout=300s` PASS; `go vet ./internal/grpcsvc` PASS. Image `golang:1.26.9-bookworm`, 2 CPU / 4 GiB; owned runner removed.
- `receipt.json` SHA-256 `6e03e895cba6ba9e4e713e84525747e4f7fc28da86cc9dc04e8eefd93d913517` records exact commands, source binding, environment and log hashes. Initial runner setup attempts lacked sibling module replacement paths; the final private snapshot included them. These were setup failures, not a source regression.
- `green-race-count500-retry2.log` SHA-256 `910042877ea791610bf17e50466d1846ae8cf9f2c1a7f54bbb73b465e5c06d21`.
- `green-short-race.log` SHA-256 `efbdf5a490cd6ffeeeba2c5f9f7d83b1e2ce9d415216d0e0ce99848dd9fadc1f`.
- `green-vet.log` SHA-256 `a4ed4c2c830adf807283a578ea7ee8b2f659416a86da69815d6147237e7d7f36`.

`graphify update .` completed successfully; generated HTML was skipped because the graph exceeds its visualization limit. `git diff --check` passed; the tracked change remains the single test file.

Independent source review by `bot_installation`: CLEAR. Review specifically confirmed explicit expiration synchronization, original budgets, exact outcome counts, parent isolation, and test-only scope. Reviewer performed no runtime tests.

No Auth/Chat stopped scenario or local full Compose was run. No source-evidence carrier was edited. Hosted acceptance of this repair is not yet claimed.
