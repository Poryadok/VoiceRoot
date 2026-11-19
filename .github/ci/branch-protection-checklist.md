# Branch protection — required status checks (post path-filter CI tiers)

Update **Settings → Branches → master → Branch protection → Require status checks** to
the names in **Tier 1 only**. The lists below are rebuilt from the live
[`.github/workflows/ci.yml`](../workflows/ci.yml) job graph (66 jobs) and
[gate-policy.md](gate-policy.md), re-derived 2026-11-19 at `origin/master`
`bc9e4909e092f1be63a45c4d81e20b3bf5b9a902`. Card: `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1`
(reference finding [docs/audits/2026-11-19-tt-fleet-audit.md](../../docs/audits/2026-11-19-tt-fleet-audit.md), F6).

## Tier 1 — require on every PR (run on every PR regardless of path filters)

| Check | Job | Notes |
|-------|-----|-------|
| **CI gate** | **`ci-gate`** | `always() && (pull_request \|\| master push)`; rejects missing/skipped/cancelled/failed **selected** jobs — the enforcement point for Tier 1b |
| Current diff contract | `current-diff-contract` | every PR incl. docs-only: exact PR parents, clean merge tree, immutable migration entries |
| Affected check plan | `affected-check-plan` | planner comparison artifact; PR routing widens unclassified paths before job selection |
| Path selector | `changes` | feeds every filter; infra job, require it so a filter failure cannot pass silently |

`ci-skip-gate` runs **only** on docs-only PRs (`needs.changes.outputs.code != 'true'`), so it
must not be a required check — branch protection is per branch, and requiring it would block
every code PR. `markdown-link-check` (workflow **Docs link check**) fires only on `docs/**`
paths and is likewise not required.

## Tier 1b — path-selected PR jobs: do NOT require

**Correction to the first rebuild pass:** `compose-e2e` runs on PRs too (global/`ci_global`/
`flutter_live_suite` legs, next to its schedule and master-push legs), so it was added to the
enumeration below. Re-derived 2026-11-19 at `origin/master`
`bc9e4909e092f1be63a45c4d81e20b3bf5b9a902`.

Two independent reasons, both structural:

1. A path-filtered job shows **skipped** on PRs that do not touch its paths; a required check
   that never ran blocks unrelated PRs. `ci-gate` (Tier 1) recomputes the plan at the candidate
   and rejects missing/skipped/cancelled/failed **selected** jobs (gate-policy), so the selected
   set is enforced without naming it in branch protection.
2. Matrix jobs run under `<job> (<matrix>)` names
   (`ci-script-tests (policy|runtime|staging|proofs)`, `backend-go (<service>)`,
   `backend-go-integration-pr (<service>)`, `flutter-unit-tests (0..3)`,
   `Key-backup golden (<platform>)`, `t31-critical / compose-https-claim-inbox-ack`),
   so a fixed required name cannot express them either.

Enumeration (nothing outside it is a PR validation job):
`protobuf`, `compose-config`, `ci-script-tests`, `f13-historical-replay`,
`match-squad-overlay`, `voice-history-audit`, `compose-e2e`, `flutter-unit-plan`, `flutter-unit-tests`,
`flutter-unit-receipts`, `flutter`, `flutter-music-proof`, `flutter-help-proof`,
`flutter-startup-proof`, `flutter-device-driver`, `flutter-key-backup-goldens`,
`flutter-windows`, `golangci`, `backend-go-pkg`, `backend-go`,
`backend-go-integration-pr`, `backend-go-integration-evidence`,
`backend-auth`, `backend-auth-image`, `developer-portal`, `web`, `admin`,
`t31-critical`, `story-lfp-integration`, `user-association-integration`,
`notification-quota-integration`, `role-space-receipt-integration`,
`realtime-lifecycle-integration`, `chat-lifecycle-integration`,
`chat-auth-java-integration` (global/CI wiring only), `local-docker-isolation`,
`space-native-installer-integration`, `space-autovacuum-integration`,
`messaging-threadlist-integration`, `voice-ledger-integration`,
`bot-membership-integration`, `role-migration-integration`,
`analytics-clickhouse-integration`, `a1-attachment-restart-proof`.

## Tier 2 — master push / platform (never PR-required)

`staging-images-push`, `staging-images-promote`, `staging-stack-lock`,
`minio-server-image-publish`, `minio-mc-image-publish`,
`flutter-android-smoke`, `flutter-ios`, `flutter-web-integration`,
`flutter-windows` (push leg), `compose-e2e` (push leg, on compose/frontend/global/admin/
developer-portal paths, not on Go-only backend diffs), `deploy-staging`.

Publication legs, intentionally not validation: `nats-hub-config-renderer-image`
(PR leg builds with `push: false`; the `master` push leg publishes),
`backend-auth-image` (PR/full-dispatch build verify only, no push leg),
`staging-images-push` and the promote/stack-lock chain.

## Tier 2b — dispatch publication (never a branch-protection check)

`staging-images-dispatch-publish` — matrix of the catalog images, runs only when the
`workflow_dispatch` inputs `publish_tag` and `publish_services` are both set
(durable CI-owned publication at an exact 40-hex master commit). It publishes bytes;
it is not PR validation and must never be required on a PR or on master.

## Tier 3 — never require (cron 02:00 UTC, or `workflow_dispatch` → `tier3-only` / `full`)

`local-ci-parity`, `backend-go-integration` (full `go test` matrix, no `-short`),
`compose-e2e` on schedule, `voice-history-audit`, `a1-e2e`,
`a1-flutter-profile-handoff`, `a1-attachment-restart-proof`,
`analytics-clickhouse-integration`, `grafana-analytics-smoke`,
`full-validation-aggregation` (aggregates the full-tier set through
`scripts/ci/full_validation_gate.py`, only when `full_validation` is selected).

Red classes in these tiers stay registered in [docs/todo/ci.md](../../docs/todo/ci.md);
tier 3 reds are documented debt, not merge blockers.

## Evidence — observed check-name sets (2026-11-19)

| CI run | event | jobs | success/skipped | notes |
|--------|-------|------|-----------------|-------|
| 44499761996 (CI #6053) | `pull_request`, docs-only PR #1172 | 66 | 5 / 61 | ran: `changes`, `affected-check-plan`, `ci-skip-gate`, `current-diff-contract`, `ci-gate` |
| 44576366297 (CI #6056) | `pull_request`, docs-only PR #1173 | 66 | 5 / 61 | same set |
| 44497648425 (CI #6052) | `schedule` | 86 | 22 (5 red debt-family) | 59 skipped |
| 44574745810 (CI #6055) | `workflow_dispatch` `profile=full` | 128 | in flight | matrix-expanded names; `ci-gate`/`full-validation-aggregation` wait on the matrix |

## Post-merge sanity (one-time after selective CI rollout)

**Actions → CI → Run workflow** → profile **`full`** — all tiers, all services, full matrix.
First master push: consider `STAGING_FORCE_FULL_ROLLOUT=true` and manual
**Staging deploy** with `deploy_mode=full`.

## Related

- [TESTING.md](../../docs/TESTING.md) — tier table (kept in sync with this file)
- [gate-policy.md](gate-policy.md) — gate obligations and the `ci-gate` selected-job rule
- [DEPLOYMENT.md](../../docs/DEPLOYMENT.md) — staging deploy, stack lockfile
