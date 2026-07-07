# Branch protection — required status checks (post path-filter CI tiers)

Update **Settings → Branches → master → Branch protection → Require status checks** after merging path-filter CI.

## Tier 1 — require on every PR (path-filtered; skipped when docs-only)

| Check | Job | Notes |
|-------|-----|-------|
| Protobuf | `protobuf` | Skipped when protos unchanged |
| Compose config | `compose-config` | Skipped when compose paths unchanged |
| Flutter | `flutter` | Skipped when frontend unchanged |
| golangci | `golangci` | Skipped when no Go services in matrix |
| Backend Go | `backend-go` | Matrix; one check per service or aggregate via rules |
| Backend Go pkg | `backend-go-pkg` | When `run_pkg` |
| Auth | `backend-auth` | Skipped when auth unchanged |
| Developer Portal | `developer-portal` | Skipped when portal unchanged |
| Docs-only gate | `ci-skip-gate` | Only on docs-only PRs |

GitHub **skipped** jobs do not block merge by default. Either enable **"Require branches to be up to date"** with checks that always run, or accept that path-filtered jobs skip cleanly.

## Tier 2 — master push (not PR merge gates)

`flutter-android-smoke`, `flutter-windows`, `flutter-ios`, `flutter-web-integration`, `compose-e2e`, Docker push jobs — run on push to `master` after merge. Do **not** add as PR required checks unless you want full tier 2 on every PR.

## Tier 3 — do NOT require on PR / master

- `local-ci-parity` — cron 02:00 UTC or `workflow_dispatch` → `tier3-only` / `full`
- `backend-go-integration` — same triggers
- `compose-e2e` on schedule only (tier 3 path)

## Post-merge sanity (one-time after path-filter rollout)

**Actions → CI → Run workflow** → profile **`full`** — all tiers, all services, full matrix.

## Related

- [TESTING.md](../../docs/TESTING.md) — tier table
- [DEPLOYMENT.md](../../docs/DEPLOYMENT.md) — staging deploy, image tags
