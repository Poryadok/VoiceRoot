# Production full stack

Kubernetes manifests for `voice-prod` namespace. Mirrors [`deploy/staging/`](../staging/) for first production cutover.

## Prerequisites

1. k3s (or managed Kubernetes) cluster with kubectl access ([DEPLOYMENT.md](../../docs/DEPLOYMENT.md))
2. GHCR images built by CI on `master` for all Go services, `auth`, `developer-portal`, `web`, `admin` (tag `:<git_sha>`)
3. Secrets from [secret.example.yaml](secret.example.yaml) → `secret.yaml` (do not commit), or GitHub secret `PROD_APP_SECRETS_YAML`
4. Postgres init + golang-migrate Jobs (`scripts/staging/apply-migrate-jobs.sh` — reused with `VOICE_K8S_NAMESPACE=voice-prod`)

## Image tag and GHCR pull

**Production deploy** requires an explicit `image_tag` (git SHA or semver; no `latest` default).

Before any registry lookup, the image gate validates the entire production
profile from `staging-image-catalog.json`, plus `nats-hub-config-renderer`.
An optional stack lock must use the plain mapping format in
[stack.lock.example.yaml](stack.lock.example.yaml): version1, exactly one
registry/tag and every production image exactly once. Duplicate/unknown fields,
YAML aliases/collections, incomplete or nonproduction image sets, mutable tags
and refs differing from the applied registry/tag are rejected. The verifier
does not silently fall back when an explicitly requested lock is missing.

After all manifests are found, the workflow preserves the verified plan as a
`production-stack-lock-<run_id>-<attempt>` artifact and passes its validated tag
and service scope to apply. `images-only` requires a nonempty set of distinct
production service names; malformed JSON, unknown names and an empty scope fail
before registry lookup and before cluster effects. All production images stay
required even for selective rollout so the preserved lock describes a complete
release; staging-only game integration and deferred Federation are excluded.
This gate validates the declared SHA/semver tag policy, not registry tag
immutability or deployed workload state.

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
