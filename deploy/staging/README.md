# Staging full stack (phases 0–10 + phase 16 bot)

Kubernetes manifests for `voice-staging` namespace. Gateway-only deploy is legacy; use the full stack for product smoke.

## Prerequisites

1. k3s cluster with kubectl access ([DEPLOYMENT.md](../../docs/DEPLOYMENT.md))
2. GHCR images built by CI on `master` for: `auth`, `gateway`, `user`, `social`, `chat`, `messaging`, `realtime`, `file`, `voice`, `space`, `role`, `search`, `matchmaking`, `notification`, `bot`, `developer-portal`
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
export VOICE_STAGING_URL=https://<VOICE_GATEWAY_INGRESS_HOST>
scripts/staging/smoke-staging.sh
```

## Files

| File | Purpose |
|------|---------|
| `namespace.yaml` | `voice-staging` namespace |
| `domains.defaults` | Staging public FQDNs (single file to edit on domain rotation) |
| `configmap-app.yaml` | Shared env (GRPC upstreams, NATS, Redis); OAuth URLs templated at apply |
| `secret.example.yaml` | Template for R2, JWT, FCM, APNs |
| `infra.yaml` | Postgres, Redis, NATS, LiveKit, ClamAV |
| `services.yaml` | All application microservices |
| `gateway-deployment.yaml` | API Gateway + Service |
| `developer-portal.yaml` | Developer Portal static site + Ingress (OAuth callback host) |

## Prometheus scrape (observability)

Every app Deployment pod template has `prometheus.io/scrape` annotations. k3s-lite Prometheus discovers them via `kubernetes_sd_configs` (see `deploy/observability/prometheus/scrape/voice-apps.yaml`).

| Deployment | HTTP port | Metrics path |
|------------|-----------|--------------|
| voice-gateway | 8080 | `/metrics` |
| voice-auth | 8080 | `/actuator/prometheus` |
| voice-messaging, chat, user, social, space, role, voice, file, matchmaking, search, notification, realtime, bot | 8080 | `/metrics` |

After changing annotations, re-apply staging and roll out pods:

```bash
scripts/staging/render-and-apply.sh
kubectl rollout restart deployment -n voice-staging -l 'app in (voice-gateway,voice-auth,voice-messaging,voice-chat,voice-user,voice-social,voice-space,voice-role,voice-voice,voice-file,voice-matchmaking,voice-search,voice-notification,voice-realtime,voice-bot)'
```

For **kube-prometheus-stack** (`OBSERVABILITY_PROFILE=full`), use ServiceMonitors instead of pod annotations:

```bash
kubectl apply -f deploy/observability/profiles/full/service-monitors.yaml
kubectl apply -f deploy/observability/profiles/full/prometheus-rules.yaml
```

See [deploy/observability/README.md](../observability/README.md) for the observability stack apply order.

## CI

Workflow [CI](../../.github/workflows/ci.yml) job **`developer-portal`** runs `npm ci`, `npm test`, and `npm run build` on every PR and push to `master`. On push to `master` it also builds and pushes `ghcr.io/<owner>/<repo>/developer-portal:<git-sha>` and `:latest` to GHCR. Staging build-args (`VITE_VOICE_API_BASE`, OAuth client id) come from GitHub Variables — see [DEPLOYMENT.md](../../docs/DEPLOYMENT.md).

Workflow [Staging deploy](../../.github/workflows/staging-deploy.yml) applies the full stack (infra, services, gateway, developer-portal) via `scripts/staging/render-and-apply.sh` when **`STAGING_DEPLOY_ENABLED=true`** after successful CI on `master`, or manually via **`workflow_dispatch`** (optional image tag, default `latest`).
