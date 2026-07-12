# Staging full stack (core services + bots)

Kubernetes manifests for `voice-staging` namespace. Gateway-only deploy is legacy; use the full stack for product smoke.

## Prerequisites

1. k3s cluster with kubectl access ([DEPLOYMENT.md](../../docs/DEPLOYMENT.md))
2. GHCR images built by CI on `master` for changed services; unchanged images promoted from previous SHA (tag `:<git_sha>` only — no `:latest` in CI)
3. Secrets from [secret.example.yaml](secret.example.yaml) → `secret.yaml` (do not commit)
4. Postgres init + golang-migrate Jobs (`scripts/staging/apply-migrate-jobs.sh` for `bot_db`, `story_db`, `moderation_db`, `subscription_db`)

## Image tag and GHCR pull

Auto **Staging deploy** uses CI **`head_sha`** and optional **`stack.lock.yaml`** artifact (per-image built vs promoted). Manual **`workflow_dispatch`** requires explicit **`image_tag`** (git SHA). Optional: `changed_services`, `needs_full_rollout`, `needs_user_space_rollout` for subset rollout.

**`DEPLOY_MODE`:** `full` (infra + migrations + ordered rollout) | `app-only` (migrations + app manifests + subset rollout) | `images-only` (selective image update only).

If GHCR packages are private, create a `docker-registry` secret in `voice-staging` and set `VOICE_IMAGE_PULL_SECRET` when running `render-and-apply.sh` (patches all Deployments).

Optional: `VOICE_APPLY_OBSERVABILITY=true` runs [`scripts/staging/apply-observability.sh`](../scripts/staging/apply-observability.sh) after app tier (not raw `kubectl apply -f deploy/observability/`). Standalone workflow: [`.github/workflows/staging-observability-deploy.yml`](../.github/workflows/staging-observability-deploy.yml).

## Apply

```bash
# From repo root (bash):
export VOICE_IMAGE_REGISTRY=ghcr.io/your-org/voiceroot
export VOICE_IMAGE_TAG=<git-sha>   # required
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
| `flutter-web.yaml` | Flutter web SPA + Ingress (`VOICE_WEB_INGRESS_HOST`) |
| `admin.yaml` | Moderation Admin + Ingress (`VOICE_ADMIN_INGRESS_HOST`, OAuth `voice-admin`) |

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

Workflow [CI](../../.github/workflows/ci.yml) on push to `master`: path-filtered **selective build** (`staging-images-push`) + **promote** unchanged images (`staging-images-promote` from `github.event.before` / `HEAD^`) → **`staging-stack-lock`** artifact.

Jobs **`developer-portal`**, **`web`**, **`admin`**, **`backend-auth`** run on PR (verify) and push to `master` only when paths change (`run_*` flags from `resolve-staging-matrix.sh`).

Workflow [Staging deploy](../../.github/workflows/staging-deploy.yml) applies manifests via `scripts/staging/render-and-apply.sh` when **`STAGING_DEPLOY_ENABLED=true`** after successful CI on `master`, or manually via **`workflow_dispatch`** (required `image_tag`).
