# Branch protection — required status checks (post path-filter CI tiers)

Update **Settings → Branches → master → Branch protection → Require status checks** after merging path-filter CI.

## Tier 1 — require on every PR (path-filtered; skipped when docs-only)

| Check | Job | Notes |
|-------|-----|-------|
| **CI gate** | **`ci-gate`** | **Always on code PRs** — fails if a required path-filtered job skipped unexpectedly |
| Protobuf | `protobuf` | Skipped when protos unchanged |
| Compose config | `compose-config` | Skipped when compose paths unchanged |
| Flutter | `flutter` | Skipped when frontend unchanged |
| golangci | `golangci` | Skipped when no Go services in matrix |
| Backend Go | `backend-go` | Matrix; `-short` tests |
| Backend Go integration (PR) | `backend-go-integration-pr` | Full `go test` for matrix services |
| Backend Go pkg | `backend-go-pkg` | When `run_pkg` |
| Auth | `backend-auth` | Skipped when auth unchanged |
| Developer Portal | `developer-portal` | Skipped when portal unchanged |
| Admin | `admin` | Skipped when admin unchanged |
| Docs-only gate | `ci-skip-gate` | Only on docs-only PRs |

Require **`ci-gate`** on all PRs with code changes. Path-filtered jobs may still show **skipped** in the UI; `ci-gate` validates the expected set ran.

## Tier 2 — master push (not PR merge gates)

`staging-images-push`, `staging-images-promote`, `staging-stack-lock`, `deploy-staging`, `flutter-android-smoke`, `flutter-windows`, `flutter-ios`, `flutter-web-integration`, `compose-e2e` — run on push to `master` after merge.

## Tier 3 — do NOT require on PR / master

- `local-ci-parity` — cron 02:00 UTC or `workflow_dispatch` → `tier3-only` / `full`
- `backend-go-integration` — same triggers (nightly full matrix)
- `compose-e2e` on schedule only (tier 3 path)

## Post-merge sanity (one-time after selective CI rollout)

**Actions → CI → Run workflow** → profile **`full`** — all tiers, all services, full matrix. First master push: consider `STAGING_FORCE_FULL_ROLLOUT=true` and manual **Staging deploy** with `deploy_mode=full`.

## Related

- [TESTING.md](../../docs/TESTING.md) — tier table
- [DEPLOYMENT.md](../../docs/DEPLOYMENT.md) — staging deploy, stack lockfile
