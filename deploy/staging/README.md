# Staging full stack (core services + bots)

Kubernetes manifests for `voice-staging` namespace. Gateway-only deploy is legacy; use the full stack for product smoke.

## Prerequisites

1. k3s cluster with kubectl access ([DEPLOYMENT.md](../../docs/DEPLOYMENT.md))
2. GHCR images built by CI on `master` for changed services; unchanged images promoted from previous SHA (tag `:<git_sha>` only — no `:latest` in CI)
3. Secrets from [secret.example.yaml](secret.example.yaml) → `secret.yaml` (do not commit)
4. Postgres init + golang-migrate Jobs (`scripts/staging/apply-migrate-jobs.sh` for all Go-owned databases: `bot_db`, `chat_db`, `file_db`, `matchmaking_db`, `messaging_db`, `moderation_db`, `notification_db`, `role_db`, `search_db`, `social_db`, `space_db`, `story_db`, `subscription_db`, `user_db`, `voice_db`)

## Image tag and GHCR pull

Auto **Staging deploy** uses CI **`head_sha`** and optional **`stack.lock.yaml`** artifact (per-image built vs promoted). Manual **`workflow_dispatch`** requires explicit **`image_tag`** (git SHA). Optional: `changed_services`, `needs_full_rollout`, `needs_user_space_rollout` for subset rollout.

**`DEPLOY_MODE`:** `full` (infra + migrations + ordered rollout) | `app-only` (migrations + app manifests + subset rollout) | `images-only` (selective image update only).

An ordinary data-preserving `full` deploy reconciles the fixed NATS consumers on the accepted PVC-backed hub and checks that the new Realtime image can inspect the exact durable with its existing NATS leaf credential before any application rollout. This check does not bind the live push consumer and cannot prove its SUB or ACK grants. Newly added grants must first be activated in the externally managed bootstrap and Realtime credentials according to [`deploy/nats/acl-intent.yaml`](../nats/acl-intent.yaml), then proved with the permitted/denied operation checks in [the NATS rotation runbook](../../docs/DEPLOYMENT.md#nats-jwt-and-leaf-activation). After that proof, set staging Environment variable `VOICE_NATS_ACL_PROOF_SHA` to the SHA-256 of the reviewed `deploy/nats/acl-intent.yaml`; a missing or stale value stops full deploy before app rollout. Updating `STAGING_NATS_SECRETS_B64` alone does not rotate an existing Secret because the restore step deliberately skips a complete set. The controlled external Secret-key rotation is separate from deploy. Keep `VOICE_NATS_FRESH_INSTALL=false`; no NATS stream, PVC, or namespace reset is needed.

For an authorized full NATS root rotation on staging, run [Staging NATS root rotation](../../.github/workflows/staging-nats-root-rotation.yml) from `master` on the trusted `voice-staging` self-hosted runner. Select `activate` and enter a new generation token (`rYYYYMMDD` plus up to eight lowercase letters or digits). The staging Environment secret `STAGING_NATS_ROTATION_SECRETS_B64` must contain the gzip-compressed, base64-encoded rotation Secret List prepared outside this repository. Upload a separately issued, short-lived `proof.creds` as base64 to `STAGING_NATS_PROOF_CREDS_B64` just before the run; delete that GitHub secret after the run. The workflow uses protected mode-0600 temporary files, checks the exact-master Realtime image in GHCR, proves anonymous pull or verifies the configured `VOICE_IMAGE_PULL_SECRET` against that image, and requires a live ACL proof plus verified cleanup before restarting leaves. For rollback, select `rollback` and leave generation empty; no rotation bundle or proof credential is read. Rollback first removes and verifies any generation-scoped temporary proof Job, NetworkPolicy, and Secret left by a lost runner. This maintenance can interrupt NATS and discard its existing state. It does not reset the namespace or apply other services. After a successful run, set staging variables `VOICE_NATS_ACL_PROOF_SHA` and `VOICE_NATS_ACL_PROOF_GENERATION` to the reviewed intent digest and active generation before ordinary `full` or `app-only` deploys. See [the NATS rotation runbook](../../docs/DEPLOYMENT.md#nats-jwt-and-leaf-activation) before dispatch.

If GHCR packages are private, create a `docker-registry` secret in `voice-staging` and set `VOICE_IMAGE_PULL_SECRET` when running `render-and-apply.sh` (patches all Deployments).

Optional: `VOICE_APPLY_OBSERVABILITY=true` runs [`scripts/staging/apply-observability.sh`](../scripts/staging/apply-observability.sh) after app tier (not raw `kubectl apply -f deploy/observability/`). Standalone workflow: [`.github/workflows/staging-observability-deploy.yml`](../.github/workflows/staging-observability-deploy.yml).

## Apply

Copy [env.example](env.example) and set `VOICE_IMAGE_TAG` to a green CI git SHA (required; no `:latest`).

```bash
# From repo root (bash):
export VOICE_IMAGE_REGISTRY=ghcr.io/your-org/voiceroot
export VOICE_IMAGE_TAG=<git-sha>   # required
export STAGING_KUBECONFIG=~/.kube/config   # or use CI secret

scripts/staging/render-and-apply.sh
```

The deployment order is database first: `apply-migrate-jobs.sh` applies each Go-owned database's SQL directory through a ConfigMap and a `voice-migrate-<service>-db` Job before application rollouts. Jobs are skipped when the completed Job and stored SQL content hash still match. The migration Jobs consume the corresponding database URL through a `voice-app-secrets` Secret reference; the URLs are not rendered into Job manifests or logs.

For Voice specifically, create `voice_db` and apply `voice_db/000001_room_lifecycle` through ConfigMap `voice-voice-db-migrations` and Job `voice-migrate-voice-db`. Run this migration before Deployment `voice-voice`. Voice reads `VOICE_DATABASE_URL` from `voice-app-secrets`; the Voice migration Job reads `POSTGRES_PASSWORD` from the same Secret and constructs its DSN inside the container without printing it.

Voice uses `/health` for liveness and `/ready` for database/schema readiness.
The R22.2 lifecycle path remains source-disabled: coordinator and lifecycle
handlers are not registered yet.

Optional smoke after deploy:

```bash
export VOICE_STAGING_URL=https://<VOICE_GATEWAY_INGRESS_HOST>
scripts/staging/smoke-staging.sh
```

## Files

| File | Purpose |
|------|---------|
| `env.example` | Required `VOICE_IMAGE_*` env for local/manual apply |
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
