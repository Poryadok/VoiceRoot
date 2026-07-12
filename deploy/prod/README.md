# Production full stack

Kubernetes manifests for `voice-prod` namespace. Mirrors [`deploy/staging/`](../staging/) for first production cutover.

## Prerequisites

1. k3s (or managed Kubernetes) cluster with kubectl access ([DEPLOYMENT.md](../../docs/DEPLOYMENT.md))
2. GHCR images built by CI on `master` for all Go services, `auth`, `developer-portal`, `web`, `admin` (tag `:<git_sha>`)
3. Secrets from [secret.example.yaml](secret.example.yaml) → `secret.yaml` (do not commit), or GitHub secret `PROD_APP_SECRETS_YAML`
4. Postgres init + golang-migrate Jobs (`scripts/staging/apply-migrate-jobs.sh` — reused with `VOICE_K8S_NAMESPACE=voice-prod`)

## Image tag and GHCR pull

**Production deploy** requires an explicit `image_tag` (git SHA or semver; no `latest` default).

If GHCR packages are private, create a `docker-registry` secret in `voice-prod` and set `VOICE_PROD_IMAGE_PULL_SECRET` (patches all Deployments).

Optional: `VOICE_PROD_APPLY_OBSERVABILITY=true` runs observability apply after app tier.

## Apply

```bash
# From repo root (bash):
export VOICE_IMAGE_REGISTRY=ghcr.io/your-org/voiceroot
export VOICE_IMAGE_TAG=<git-sha>   # required
export VOICE_K8S_NAMESPACE=voice-prod
export DEPLOY_MODE=full            # full | app-only | images-only

bash scripts/prod/render-and-apply-prod.sh
```

Domain defaults live in [domains.defaults](domains.defaults); override via env or GitHub Variables `VOICE_PROD_*_INGRESS_HOST`.

## Files

| File | Purpose |
|------|---------|
| `namespace.yaml` | `voice-prod` namespace |
| `domains.defaults` | Production public FQDNs (placeholder `*.voice.example.com`) |
| `configmap-app.yaml` | Shared env (GRPC upstreams, NATS, Redis); OAuth URLs templated at apply |
| `secret.example.yaml` | Template for R2, JWT, FCM, APNs |
| `stack.lock.example.yaml` | Example immutable tag lock for all catalog images |
| `infra.yaml` | Postgres, Redis, NATS, LiveKit, ClamAV, ClickHouse |
| `services.yaml` | All application microservices (19 services) |
| `gateway-deployment.yaml` | API Gateway + Service |
| `developer-portal.yaml` | Developer Portal static site + Ingress |
| `flutter-web.yaml` | Flutter web SPA + Ingress |
| `admin.yaml` | Moderation Admin + Ingress |

## CI

Workflow **[Production deploy](../../.github/workflows/prod-deploy.yml)** — manual only, `environment: production` (approval), **required** `image_tag`, optional `deploy_mode` (`full` default).

Verifies all catalog images via `scripts/prod/verify-prod-images.sh`, then applies via `scripts/prod/render-and-apply-prod.sh`.

GitHub setup:

| Item | Purpose |
|------|---------|
| Environment `production` | Required reviewers / wait timer |
| Secret `PROD_KUBECONFIG` | base64 kubeconfig |
| Secret `PROD_APP_SECRETS_YAML` | optional base64 full Secret manifest |
| Variables `VOICE_PROD_K8S_NAMESPACE`, `VOICE_PROD_*_INGRESS_HOST`, `VOICE_PROD_IMAGE_PULL_SECRET` | Prod FQDNs and namespace |

See [DEPLOYMENT.md](../../docs/DEPLOYMENT.md) for tag policy and approval flow.
