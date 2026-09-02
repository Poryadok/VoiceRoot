# VOICE_IMAGE_TAG audit (batch 33a)

Audit date: 2026-09-02. Policy: **no `:latest` fallback** for Voice app images; local/CI apply requires explicit git SHA (or semver on prod manual deploy).

## Apply entrypoints (fail without TAG)

| Script | TAG handling |
|--------|----------------|
| `scripts/staging/render-and-apply.sh` | `${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}` |
| `scripts/prod/render-and-apply-prod.sh` | `${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required for production}` |
| `scripts/staging/apply-app-manifests.sh` | required |
| `scripts/prod/apply-app-manifests.sh` | required |
| `scripts/staging/apply-infra.sh` | required |
| `scripts/prod/apply-infra.sh` | required |
| `scripts/staging/deploy-changed.sh` | required |
| `scripts/staging/rollout-user-space-tier.sh` | required per-image ref |

## Verify scripts (TAG or stack lock)

| Script | TAG handling |
|--------|----------------|
| `scripts/staging/verify-staging-images.sh` | `STACK_LOCK_FILE` **or** `VOICE_IMAGE_TAG`; errors if both empty |
| `scripts/prod/verify-prod-images.sh` | same |

Intentional: CI passes `STACK_LOCK_FILE` from artifact; manual verify can pass `VOICE_IMAGE_TAG` alone.

## CI workflows

| Workflow | `image_tag` input |
|----------|-------------------|
| `.github/workflows/staging-deploy.yml` | **required** (`workflow_call` + `workflow_dispatch`) |
| `.github/workflows/prod-deploy.yml` | **required**; explicit error if empty (no latest default) |

## Documentation

- `docs/DEPLOYMENT.md` — § `VOICE_IMAGE_TAG` (required)
- `deploy/staging/env.example`, `deploy/prod/env.example` — copy-paste env for local apply
- `deploy/staging/README.md`, `deploy/prod/README.md` — apply examples with required TAG

## Remaining `:latest` references (acceptable)

- Third-party images in `docker-compose.yml` (LiveKit, MinIO) — not Voice GHCR app images
- `kubectl-apply-dry-run.sh` — fixed to `<git-sha>` in usage/error text (was `:latest` example)

## Out of scope (separate ci.md items)

- Prod selective deploy / stack.lock parity with staging
- Prod smoke alias to staging script
