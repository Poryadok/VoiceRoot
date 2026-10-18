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

Copy [env.example](env.example) and set `VOICE_IMAGE_TAG` (required).

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
| `env.example` | Required `VOICE_IMAGE_*` env for local/manual apply |
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

## Production smoke

With `PROD_SMOKE_ENABLED=true`, the production workflow runs
[`smoke-prod.sh`](../../scripts/prod/smoke-prod.sh). Before the shared health,
storage, frontend and optional staff checks, it requires `401` from protected
Space, Analytics dashboard and Admin report reads, both without credentials and
with an intentionally invalid bearer. These are read-only GET requests with
normal TLS verification, bounded connection/request timeouts and no redirect
following. No account is created and no real staff token is sent in these
negative probes; optional shared staff checks still use `PROD_STAFF_TOKEN`.

Any other status or transport error fails the smoke before shared checks. This
checks the public authentication boundary; it does not prove valid-session ACLs,
media delivery, deployed release identity or final production acceptance. Real
rollout and version-sensitive verification still require their authorized window.

The offline fixture runs the actual production wrapper with fake curl and a
shared-smoke boundary: `bash scripts/ci/prod-smoke_test.sh` (also included in
`make ci-script-tests`). It makes no network or Kubernetes calls.

## ClickHouse schema

The `full` path runs `scripts/prod/apply-infra.sh`, waits for the ClickHouse
StatefulSet rollout, then invokes `scripts/prod/apply-clickhouse-init.sh`. The
wrapper selects `voice-prod` by default (or the explicit `VOICE_K8S_NAMESPACE`)
and runs the shared staging implementation through Bash; script execute bits
are not required. `app-only` and `images-only` do not apply ClickHouse DDL.

The shared implementation publishes the init ConfigMap, waits for native and
remote authentication, and applies `docker/clickhouse/init/001_events.sql` using
the Pod's local client. Both first bootstrap and subsequent full rollouts apply
the additive, idempotent SQL. Missing SQL, a failed apply, or a missing
`voice.events` / `voice.events_logical` prevents successful completion. This
path targets the bundled StatefulSet; managed ClickHouse needs its own reviewed
operator procedure. Local orchestration regression checks run with
`bash scripts/ci/prod-clickhouse-init_test.sh` without cluster access; they do
not prove a production rollout or actual ClickHouse authentication.
