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

## Shared rollout helpers

[`render-and-apply-prod.sh`](../../scripts/prod/render-and-apply-prod.sh)
uses the same operational helpers as staging, with the production namespace,
image selection and manifests supplied by the production wrapper:

| Production path | Shared helper |
|-----------------|---------------|
| `full` | [`rollout-app-tier.sh`](../../scripts/staging/rollout-app-tier.sh), after production infra and app manifests |
| `app-only` | [`apply-migrate-jobs.sh`](../../scripts/staging/apply-migrate-jobs.sh), then production app manifests and [`rollout-subset.sh`](../../scripts/staging/rollout-subset.sh) |
| `images-only` | [`deploy-changed.sh`](../../scripts/staging/deploy-changed.sh), using `CHANGED_SERVICES` |
| Optional observability | [`apply-observability.sh`](../../scripts/staging/apply-observability.sh), after the selected apply path |
| Full infra, missing app Secret with supplied production upload | [`ensure-app-secrets.sh`](../../scripts/staging/ensure-app-secrets.sh), called by [`apply-infra.sh`](../../scripts/prod/apply-infra.sh) |

[`map-prod-env.sh`](../../scripts/prod/map-prod-env.sh) supplies the shared
helpers' environment names. Nonempty `PROD_APP_SECRETS_YAML_B64`,
`PROD_APP_SECRETS_YAML` (a local manifest path) and `PROD_STAFF_TOKEN` map to
their corresponding `STAGING_*` names, replacing any existing values for those
aliases. Nonempty `VOICE_K8S_NAMESPACE`,
`VOICE_IMAGE_PULL_SECRET` and `VOICE_APPLY_OBSERVABILITY` take precedence over
their `VOICE_PROD_*` aliases. Existing cluster app Secrets are preserved by the
production infra wrapper; it invokes the shared upload helper only when that
Secret is missing. Keep production credentials separate from staging material.

The entrypoint requires `VOICE_IMAGE_TAG` before invoking any child helper and
stops when a child helper returns failure. Reusing these helpers does not prove
a production rollout or smoke passed. Production-specific smoke, immutable
selective delivery and the `full` mode's User/Space subset choice remain separate
checks in the [CI inventory](../../docs/todo/ci.md).

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

In `full` mode, `needs_user_space_rollout=true` selects the shared User/Space
subset after infra and application manifests are applied. The default remains
the full ordered app tier; `needs_full_rollout=true` takes precedence even when
both flags are true. Use the full tier for first bootstrap or changes requiring
the complete dependency-order/Auth repair sequence. The subset delegates changed
images to the existing ordered User/Space helper and preserves Space's desired
replica count. It is a rollout-selection option, not a change to the workloads'
`Recreate` strategy or a guarantee of uninterrupted service. `app-only` retains
its existing subset selector; `images-only` retains selective image updates.
Run `bash scripts/ci/prod-user-space-rollout_test.sh` for isolated dispatch,
namespace and failure-propagation checks. Live readiness and deployment
qualification still require the authorized protected rollout window.

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
