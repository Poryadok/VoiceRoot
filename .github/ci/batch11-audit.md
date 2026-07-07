# Batch 11 CI/CD audit (2026-07-07)

Audit of TODO [`docs/TODO.md`](../../docs/TODO.md) lines 109–130 after implementation.

## Checklist

| TODO item | Status | Notes |
|-----------|--------|-------|
| path-filters: SQL migrations | **done** | `src/backend/migrations/**` in `compose` |
| path-filters: postgres init | **done** | `docker/postgres/**` in `compose` |
| path-filters: all workflows | **done** | `.github/workflows/**` in `global` |
| Docker build on PR | **done** | `push: false` verify for Go/auth/devportal on PR |
| Path-filter cross-service deps | **done** | `resolve-go-matrix.sh` S2S map |
| Branch protection required checks | **partial** | [branch-protection-checklist.md](branch-protection-checklist.md) — **GitHub Settings manual** |
| staging-deploy vs path filters | **done** | GHCR `gateway:<tag>` manifest check before apply |
| Sanity dispatch full | **partial** | Documented in DEPLOYMENT.md — **manual** `workflow_dispatch` full |
| compose-migrate-all :5432 | **done** | TESTING.md § compose-e2e |
| Developer Portal on staging | **partial** | Deploy script applies portal + SHA tag; **cluster verify manual** |
| Rollout wait portal | **done** | Strict `rollout status` for developer-portal |
| Prod deploy workflow | **done** | `prod-deploy.yml`, `render-and-apply-prod.sh`, `deploy/prod/README.md` |
| Staging DB migrations | **done** | `apply-migrate-jobs.sh` bot/story/moderation/subscription |
| compose-e2e-live Flutter drift | **done** | `compose-e2e-live.yml` env 3.41.7 / 1.26 |
| compose-e2e coverage | **done** | CI `compose-e2e` → `compose-e2e-smoke.sh` (all features) |
| Staging k8s vs CI matrix | **done** | story/subscription/moderation/analytics/federation Deployments + upstreams |
| imagePullSecrets | **done** | `VOICE_IMAGE_PULL_SECRET` patch in render-and-apply |
| Observability in deploy | **done** | Opt-in `VOICE_APPLY_OBSERVABILITY=true` |
| Unified tool pins | **done** | `ci.yml` + `compose-e2e-live.yml` env block |
| Manual deploy `latest` risk | **done** | DEPLOYMENT.md |

## Potential bugs / risks

### High

1. **Staging deploy GHCR check uses `docker manifest inspect` without `setup-docker` step** — may fail on runner if Docker daemon unavailable. Add `docker/setup-buildx-action` or use `crane` action if failures occur.

2. **`patch_image_pull_secrets` uses JSON patch `add` on path that may already exist** — re-apply can fail or duplicate if path exists. Prefer strategic merge patch or check before patch.

3. **Migrate Jobs re-run**: script deletes incomplete jobs but **won't re-run** succeeded jobs when new migration files land. After migration version bump, operators must `kubectl delete job voice-migrate-*` manually.

### Medium

4. **S2S deps map is one-hop only** — e.g. `file` change won't pull `story` (story→file). Acceptable trade-off; document if expanded.

5. **Smoke E2E runtime** — 16 gateway + 15 flutter tests on master push may exceed 15 min on cold cache. Monitor CI duration; split or reduce if flaky.

6. **`compose-e2e` job lost dedicated Phase 17-only fast path** — replaced by broader smoke; tier 2 duration increases vs old Phase 17-only.

7. **New staging services** need `STORY_DATABASE_URL`, `MODERATION_DATABASE_URL`, `SUBSCRIPTION_DATABASE_URL` in secrets — deploy fails at migrate Job if keys missing.

8. **Analytics/federation** are health-only stubs — gateway analytics REST works; federation not exposed via gateway (by design).

### Low

9. **Flutter/gateway test rename incomplete** — only `stories` renamed to feature names; most `phase*` filenames remain in `full_flutter` manifest. Follow-up rename pass recommended.

10. **`e2e-manifest.sh` awk parser** — fragile if YAML structure changes; no schema validation.

11. **Prod skeleton** — only bot in `deploy/prod/services.yaml`; full prod cutover still requires mirroring staging manifests.

12. **Branch protection** — skipped path-filter jobs won't block merge unless repo settings account for it.

## Recommendations

1. Run **CI workflow_dispatch profile `full`** once after merge.
2. Update **GitHub branch protection** per [branch-protection-checklist.md](branch-protection-checklist.md).
3. On staging: verify `kubectl get pods -n voice-staging` after deploy with new services + portal image SHA.
4. Add `docker/setup-buildx-action` to `staging-deploy.yml` before manifest inspect.
5. Schedule follow-up: rename remaining `phase*` live tests to feature names; add migrate job versioning strategy.

## Files touched (summary)

- `.github/ci/path-filters.yml`, `e2e-features.yml`, `branch-protection-checklist.md`
- `.github/workflows/ci.yml`, `staging-deploy.yml`, `compose-e2e-live.yml`, `prod-deploy.yml`
- `scripts/ci/resolve-go-matrix.sh`, `compose-e2e-smoke.sh`, `compose-e2e-live.sh`, `e2e-manifest.sh`
- `scripts/staging/render-and-apply.sh`, `apply-migrate-jobs.sh`, `rollout-app-tier.sh`
- `scripts/prod/render-and-apply-prod.sh`
- `deploy/staging/services.yaml`, `configmap-app.yaml`, `secret.example.yaml`
- `deploy/templates/migrate-*-db-job.yaml`
- `docs/TODO.md`, `TESTING.md`, `DEPLOYMENT.md`
