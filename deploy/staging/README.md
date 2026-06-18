# Staging full stack (phases 0–10 + phase 16 bot)

Kubernetes manifests for `voice-staging` namespace. Gateway-only deploy is legacy; use the full stack for product smoke.

## Prerequisites

1. k3s cluster with kubectl access ([DEPLOYMENT.md](../../docs/DEPLOYMENT.md))
2. GHCR images built by CI on `master` for: `auth`, `gateway`, `user`, `social`, `chat`, `messaging`, `realtime`, `file`, `voice`, `space`, `role`, `search`, `matchmaking`, `notification`, `bot`
3. Secrets from [secret.example.yaml](secret.example.yaml) → `secret.yaml` (do not commit)
4. Postgres init: on first boot run `docker/postgres` init against the cluster Postgres (`bot_db` included in `01-init-databases.sh`), then run migrations — locally `make compose-migrate-bot`; on staging apply `src/backend/migrations/bot_db` against `bot_db`

## Apply

```bash
# From repo root (bash):
export VOICE_IMAGE_REGISTRY=ghcr.io/your-org/voiceroot
export VOICE_IMAGE_TAG=<git-sha>
export STAGING_KUBECONFIG=~/.kube/config   # or use CI secret

scripts/staging/render-and-apply.sh
```

Optional smoke after deploy:

```bash
export VOICE_STAGING_URL=https://voice.tastytest.online
scripts/staging/smoke-staging.sh
```

## Files

| File | Purpose |
|------|---------|
| `namespace.yaml` | `voice-staging` namespace |
| `configmap-app.yaml` | Shared env (GRPC upstreams, NATS, Redis) |
| `secret.example.yaml` | Template for R2, JWT, FCM, APNs |
| `infra.yaml` | Postgres, Redis, NATS, LiveKit, ClamAV |
| `services.yaml` | All application microservices |
| `gateway-deployment.yaml` | API Gateway + Service |

## CI

Workflow [Staging deploy](../../.github/workflows/staging-deploy.yml) applies namespace, configmap, infra, services, and gateway when `STAGING_DEPLOY_FULL_STACK=true`.
