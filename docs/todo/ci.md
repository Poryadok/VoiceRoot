# TODO — CI

[← Индекс](../TODO.md)

CI/CD + выкат: GitHub Actions, promote/deploy, k8s secrets, observability на кластере, manifests.

## Critical

### Blockers

Стабильные ID для отложенных пунктов (агент без secrets/DNS/кластера):

1. Staging k8s secrets (`voice-app-secrets`)
2. Auth secrets (TOTP/NATS/Resend)
3. Billing secrets (Paddle/CloudPayments)
4. FCM/APNs secret values (**Вы**) — env names + `services.yaml` mounts fixed (#62); нужны human values в `voice-app-secrets`
5. Observability alert channel secret (Telegram/email для Alertmanager)
6. DNS staging FQDNs + firewall + GH variables (секция DNS & cluster prep **Вы**)

### Secrets & alerts (**Вы**)

- [ ] **Секреты staging k8s** — `voice-app-secrets` по [`deploy/staging/secret.example.yaml`](../../deploy/staging/secret.example.yaml): JWT, Postgres URLs, R2 (`FILE_R2_*`, `USER_R2_*`), FCM/APNs для Notification, **Analytics** (`CLICKHOUSE_DSN`, `ANALYTICS_ID_HASH_KEY`) ([`DEPLOYMENT.md`](../DEPLOYMENT.md)).
- [ ] **Auth secrets** — `AUTH_TOTP_ENCRYPTION_KEY` (без `DEFAULT_DEV_KEY`); `AUTH_NATS_URL` → `NatsSubscriptionTierStore` иначе JWT `subscription_tier=free`; `RESEND_API_KEY` иначе `NoopMailSender`.
- [ ] **Billing secrets** — Paddle live (не `checkout.paddle.test`); CloudPayments когда RPC появится. Не шипить test checkout URL.
- [ ] **Observability: канал алертов** — Secret уведомлений (Telegram bot или email) для Alertmanager; без него P1-алерты уходят в null receiver ([`deploy/observability/README.md`](../../deploy/observability/README.md)).

### Observability staging

Проверки на живом кластере после `apply-observability.sh`; спека — [observability.md](../features/observability.md).

- [ ] **Loki: все поды** (**Вы**) — приложение + infra пишут в Loki (`kubectl get pods -n voice-observability` — Running). — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **Трассировка `request_id`** (**Вы**) — E2E: `scripts/staging/smoke-request-id.sh` (DM → Loki chain Gateway → gRPC → NATS → `ws_fanout`); runbook [`TESTING.md`](../TESTING.md) § Debug by request_id. — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **Grafana smoke** (**Вы**) — Overview: targets UP; дашборды Overview / Tier-0 / Infra / Logs открываются. — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **Prometheus scrape** (**Вы**) — `gateway_http_requests_total` растёт при трафике на staging.  — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)
- [ ] **P1 алерты** (**Вы**) — правила активны; тестовый firing → сообщение в канал (не null receiver). — **отложено: блокер #1,#5,#6** (нужен живой staging + secrets/алерты)

### DNS & cluster prep (**Вы**)

- [ ] **DNS staging FQDNs** — Cloudflare **A** для `app`, `admin`, `livekit` (плюс уже `voice` / `developers`) на IP ingress-ноды; для `livekit` — **DNS only** (grey cloud). Firewall: **30881/TCP**, **30882/UDP** на ноде. GitHub Variables: `VOICE_WEB_INGRESS_HOST`, `VOICE_ADMIN_INGRESS_HOST`, `VOICE_LIVEKIT_INGRESS_HOST`, `VOICE_APPLY_OBSERVABILITY=true`, `STAGING_SMOKE_ENABLED=true`, secret `GRAFANA_ADMIN_PASSWORD`, `LIVEKIT_API_KEY`/`SECRET` в `STAGING_APP_SECRETS_YAML`.

## High

### Prod ingress & universal links

- [ ] **Prod universal links** — реальные AASA + `assetlinks.json` на `voice.gg` (сейчас Gateway — dev placeholders).
- [ ] **Well-known на prod** — Gateway отдаёт валидные `/.well-known/apple-app-site-association` и `assetlinks.json` для целевого домена.

### Deploy workflow

- [ ] **`MODERATION_GRPC_ADDR` on staging/prod k8s** — `docker-compose.yml` sets `MODERATION_GRPC_ADDR: moderation:9090` for messaging; `deploy/staging/configmap-app.yaml` omits it → shadow-ban/spam-mute checks silently disabled (`messaging/main.go` `PlatformMod` nil). Add `MODERATION_GRPC_ADDR: voice-moderation:9090` to shared configmap + prod mirror; verify in smoke.
- [ ] **Sanity selective CI** — `workflow_dispatch` CI → `full`; первый master push с selective promote — проверить GHCR bootstrap; при необходимости `STAGING_FORCE_FULL_ROLLOUT=true` + manual deploy `deploy_mode=full`. — **отложено: нужен live GHCR/staging**
- [ ] **`PROD_SMOKE_ENABLED` / `PROD_STAFF_TOKEN`** — GitHub Variables/Secrets для prod smoke.

### Pipeline & promote

- [x] **Tier 2 не блокирует PR** — `compose-e2e`, platform Flutter smokes только master / `full`; регрессии после merge ([`branch-protection-checklist.md`](../../.github/ci/branch-protection-checklist.md)). Док drift закрыт: `compose-e2e` без `run_go`, триггеры admin/developer-portal (parallel/admin-ci-polish).
- [ ] **Двойная сборка Flutter web на master** — tier 1 `flutter` (analyze+test) + job `web` (`flutter build web` + Docker); дедуп только для `flutter-windows` ([`ci.yml`](../../.github/workflows/ci.yml)).
- [x] **`compose-e2e` без Go-only триггера** — master CI: cross-service smoke только при compose/frontend/global/admin/developer-portal; Go-only push не гоняет compose-e2e (parallel/admin-ci-polish). Full/nightly — schedule / `workflow_dispatch`.


## Common

### Manifests & rollout

- [ ] **Prod reuse staging ops scripts** — [`render-and-apply-prod.sh`](../../scripts/prod/render-and-apply-prod.sh) → `rollout-app-tier.sh`, `deploy-changed.sh`, `apply-observability.sh`, `ensure-app-secrets.sh` (алиасы `PROD_*` → `STAGING_*`).
- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier`.
- [x] **`VOICE_IMAGE_TAG` required** — fallback `:latest` убран в apply-скриптах; локальный apply без TAG падает; документировано в [`DEPLOYMENT.md`](../DEPLOYMENT.md), [`deploy/staging/env.example`](../../deploy/staging/env.example), [`deploy/prod/env.example`](../../deploy/prod/env.example); аудит [`voice-image-tag-audit.md`](../../.github/ci/voice-image-tag-audit.md) (batch 33a).
- [ ] **Prod smoke = alias staging** — [`smoke-prod.sh`](../../scripts/prod/smoke-prod.sh) → [`smoke-staging.sh`](../../scripts/staging/smoke-staging.sh), `STAGING_STAFF_TOKEN` из `PROD_STAFF_TOKEN`; нет отдельных prod acceptance checks.
- [ ] **Prod deploy без selective / stack.lock** — [`prod-deploy.yml`](../../.github/workflows/prod-deploy.yml): нет `changed_services`, `needs_user_space_rollout`, artifact lock; `verify-prod-images` требует **все** образы catalog на TAG; `images-only` → `deploy-changed.sh` без `CHANGED_SERVICES` = no-op.
- [ ] **Prod `full` mode всегда `rollout-app-tier.sh`** — нет user/space subset rollout как на staging; single-node Recreate strategy остаётся.

### Pipeline bugs

- [x] **`.github/ci/batch11-audit.md` удалён** — устаревший аудит 2026-07-07; актуальные риски перенесены в эту секцию / [`branch-protection-checklist.md`](../../.github/ci/branch-protection-checklist.md).

### Tech debt

- [x] **Promote bootstrap** — пустой GHCR / squash-merge / force-push: `find_promote_base_sha` больше не abort'ит `changes` (`set -e`); missing manifests → rebuild. Ручной `full` / `STAGING_FORCE_FULL_ROLLOUT=true` остаётся опцией.
- [ ] **Дедуп frontend Docker build** — отложено (admin/developer-portal: npm build + docker build).
- [ ] **Prod reuse staging ops scripts** — [`render-and-apply-prod.sh`](../../scripts/prod/render-and-apply-prod.sh) → `rollout-app-tier.sh`, `deploy-changed.sh`, `apply-observability.sh`, `ensure-app-secrets.sh` (алиасы `PROD_*` → `STAGING_*`).
- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.
- [ ] **`staging-stack-lock` параллельно с auth/web/admin/portal** — lock пишется до push frontend-образов; auto-deploy ждёт эти jobs + verify — ок для happy path, не для отладки artifact mid-pipeline.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier`.
- [x] **`VOICE_IMAGE_TAG` required** — см. Common / Manifests (batch 33a).
- [ ] **Prod smoke = alias staging** — [`smoke-prod.sh`](../../scripts/prod/smoke-prod.sh) → [`smoke-staging.sh`](../../scripts/staging/smoke-staging.sh), `STAGING_STAFF_TOKEN` из `PROD_STAFF_TOKEN`; нет отдельных prod acceptance checks.
- [ ] **Prod deploy без selective / stack.lock** — [`prod-deploy.yml`](../../.github/workflows/prod-deploy.yml): нет `changed_services`, `needs_user_space_rollout`, artifact lock; `verify-prod-images` требует **все** образы catalog на TAG; `images-only` → `deploy-changed.sh` без `CHANGED_SERVICES` = no-op.
- [ ] **Prod `full` mode всегда `rollout-app-tier.sh`** — нет user/space subset rollout как на staging; single-node Recreate strategy остаётся.
- [ ] **S2S deps one-hop в `resolve-go-matrix.sh`** — e.g. `file` change не тянет `story` (story→file); для CI tests ок, для promote/build — только прямой path + gateway ([`resolve-go-matrix.sh`](../../scripts/ci/resolve-go-matrix.sh)).


## Low

### Runtime config (magic numbers)

- [ ] **BOT_WEBHOOK_* retry/backoff** — hardcoded retry/backoff webhook delivery → env.
- [ ] **NATS_* connect/reconnect** — Realtime: connect/reconnect timeouts в конфиг.
- [ ] **pgxpool max conns** — лимиты пула Postgres → env/ConfigMap.
- [ ] **BOT_RATE_LIMIT_* / Gateway rate-limit JSON** — staging defaults в ConfigMap (сейчас только dev bypass).

### Platform sign-off

- [ ] **Windows sign-off** — скилл `voice-project-full-verification`: `compose-config-ci`, `buf-ci`, `flutter-ci` — OK; `backend-test-ci-short` — после `c3598f3` fix [`jetstream_test.go`](../../src/backend/messaging/internal/messageevents/jetstream_test.go) (Flush + EnsureStream) **перепроверить на Windows/Docker**; compose smoke E2E не гонялся. См. [TESTING.md](../TESTING.md) § «Локальные грабли».

### Deferred / polish

- [ ] **Helm/Kustomize + GitOps** — отложено; [ADR 004](../adr/004-helm-gitops-deferred.md); ordered rollout остаётся в bash на runner.
- [ ] **Пересмотреть цель continuous full-stack deploy** — selective deploy есть; GitOps позже.
- [ ] **`e2e-manifest.sh` / smoke runtime** — awk-парсер YAML хрупкий; 16+ gateway + 15 flutter smoke на master — риск >15 min / flake ([`e2e-manifest.sh`](../../scripts/ci/e2e-manifest.sh), [`e2e-features.yml`](../../.github/ci/e2e-features.yml)).


**Промпт-якорь:** `CI/CD + deploy from docs/todo/ci.md` + приоритет/подсекция.
