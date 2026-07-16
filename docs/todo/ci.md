# TODO — CI

[← Индекс](../TODO.md)

CI/CD + выкат: GitHub Actions, promote/deploy, k8s secrets, observability на кластере, manifests.

## Critical

### Secrets & alerts (**Вы**)

- [ ] **Секреты staging k8s** — `voice-app-secrets` по [`deploy/staging/secret.example.yaml`](../../deploy/staging/secret.example.yaml): JWT, Postgres URLs, R2 (`FILE_R2_*`, `USER_R2_*`), FCM/APNs для Notification, **Analytics** (`CLICKHOUSE_DSN`, `ANALYTICS_ID_HASH_KEY`) ([`DEPLOYMENT.md`](../DEPLOYMENT.md)).
- [ ] **Observability: канал алертов** — Secret уведомлений (Telegram bot или email) для Alertmanager; без него P1-алерты уходят в null receiver ([`deploy/observability/README.md`](../../deploy/observability/README.md)).

### Observability staging

Проверки на живом кластере после `apply-observability.sh`; спека — [observability.md](../features/observability.md).

- [ ] **Loki: все поды** — приложение + infra пишут в Loki (`kubectl get pods -n voice-observability` — Running).
- [ ] **Трассировка `request_id`** — E2E: отправка DM на staging → цепочка Gateway → gRPC → NATS → `ws_fanout` в Loki ([`TESTING.md`](../TESTING.md) § Debug by request_id).
- [ ] **Grafana smoke** — Overview: targets UP; дашборды Overview / Tier-0 / Infra / Logs открываются.
- [ ] **Prometheus scrape** — `gateway_http_requests_total` растёт при трафике на staging. 
- [ ] **P1 алерты** — правила активны; тестовый firing → сообщение в канал (не null receiver).

### DNS & cluster prep (**Вы**)

- [ ] **DNS staging FQDNs** — Cloudflare **A** для `app`, `admin`, `livekit` (плюс уже `voice` / `developers`) на IP ingress-ноды; для `livekit` — **DNS only** (grey cloud). Firewall: **30881/TCP**, **30882/UDP** на ноде. GitHub Variables: `VOICE_WEB_INGRESS_HOST`, `VOICE_ADMIN_INGRESS_HOST`, `VOICE_LIVEKIT_INGRESS_HOST`, `VOICE_APPLY_OBSERVABILITY=true`, `STAGING_SMOKE_ENABLED=true`, secret `GRAFANA_ADMIN_PASSWORD`, `LIVEKIT_API_KEY`/`SECRET` в `STAGING_APP_SECRETS_YAML`.

### Pipeline gates

- [ ] **path-filters: `scripts/staging/**`, `scripts/prod/**`** — не в `global` / `compose` / `staging_infra` ([`path-filters.yml`](../../.github/ci/path-filters.yml)); PR только с deploy-скриптами → `code=true`, но tier-1 jobs skipped, **`ci-gate` проходит без проверок**; push в `master` → promote all + deploy с пустым `CHANGED_SERVICES` (rollout фактически no-op).


## High

### Prod ingress & universal links

- [ ] **Prod universal links** — реальные AASA + `assetlinks.json` на `voice.gg` (сейчас Gateway — dev placeholders).
- [ ] **Well-known на prod** — Gateway отдаёт валидные `/.well-known/apple-app-site-association` и `assetlinks.json` для целевого домена.

### Deploy workflow

- [ ] **Sanity selective CI** — `workflow_dispatch` CI → `full`; первый master push с selective promote — проверить GHCR bootstrap; при необходимости `STAGING_FORCE_FULL_ROLLOUT=true` + manual deploy `deploy_mode=full`.
- [ ] **`PROD_SMOKE_ENABLED` / `PROD_STAFF_TOKEN`** — GitHub Variables/Secrets для prod smoke.

### Pipeline & promote

- [ ] **`staging-stack-lock` не требует success `staging-images-push` / `staging-images-promote`** — `if: always()` + `changes.success`; при partial failed promote lock artifact и deploy всё равно стартуют (verify на deploy ловит missing, но run красный поздно).
- [ ] **`deploy-staging` не гейтит failed image jobs** — `if` проверяет только `staging-stack-lock.result == 'success'`; failed `backend-auth` / `web` / promote не блокируют workflow_call явно (частично спасает `needs:` + verify).
- [ ] **`rollout-user-space-tier`: JSON patch `add` `SPACE_GRPC_ADDR`** — повторный rollout может упасть, если env уже в pod template ([`rollout-user-space-tier.sh`](../../scripts/staging/rollout-user-space-tier.sh)); idempotent patch или `set env` + restart.
- [ ] **Migrate Jobs: skip после первого success** — новые SQL в `src/backend/migrations/**` не применятся без ручного `kubectl delete job voice-migrate-*` ([`apply-migrate-jobs.sh`](../../scripts/staging/apply-migrate-jobs.sh)); стратегия version/bump или force re-run.
- [ ] **Drift check только `staging-go-services.txt`** — в `staging-stack-lock` нет проверки [`staging-image-catalog.json`](../../scripts/ci/staging-image-catalog.json) vs `deploy/staging/` / CI jobs.
- [ ] **Док-дрифт `compose-e2e` триггера** — [`TESTING.md`](../TESTING.md) tier 2: «backend/frontend/compose»; в [`ci.yml`](../../.github/workflows/ci.yml) убран `run_go` — Go-only push на `master` **не** гоняет `compose-e2e` (только `compose` / `frontend` / `global`).
- [ ] **Tier 2 не блокирует PR** — `compose-e2e`, platform Flutter smokes только master / `full`; регрессии после merge ([`branch-protection-checklist.md`](../../.github/ci/branch-protection-checklist.md)).
- [ ] **Двойная сборка Flutter web на master** — tier 1 `flutter` (analyze+test) + job `web` (`flutter build web` + Docker); дедуп только для `flutter-windows` ([`ci.yml`](../../.github/workflows/ci.yml)).
- [ ] **`compose-e2e` без Go-only триггера** — ускорение master CI; cross-service регрессии только nightly / `full` / compose-path changes (связано с Tier 2 не блокирует PR).


## Common

### Manifests & rollout

- [ ] **[Deploy/Infra] `scripts/ci/staging-image-catalog.json` points all `k8s_manifest` entries at `deploy/staging/`** — prod drift vs `deploy/prod/` is not catalog-checked (Batch 11 covers `staging-go-services.txt` drift only). Paths: `scripts/ci/staging-image-catalog.json`, `scripts/prod/verify-prod-images.sh`.
- [ ] **`rollout-user-space-tier`: JSON patch `add` `SPACE_GRPC_ADDR`** — повторный rollout может упасть, если env уже в pod template ([`rollout-user-space-tier.sh`](../../scripts/staging/rollout-user-space-tier.sh)); idempotent patch или `set env` + restart.
- [ ] **Migrate Jobs: skip после первого success** — новые SQL в `src/backend/migrations/**` не применятся без ручного `kubectl delete job voice-migrate-*` ([`apply-migrate-jobs.sh`](../../scripts/staging/apply-migrate-jobs.sh)); стратегия version/bump или force re-run.
- [ ] **`apply-app-manifests` всегда scale auth 0→1** — даже selective deploy; downtime Auth на каждый app apply ([`apply-app-manifests.sh`](../../scripts/staging/apply-app-manifests.sh)).
- [ ] **Prod reuse staging ops scripts** — [`render-and-apply-prod.sh`](../../scripts/prod/render-and-apply-prod.sh) → `rollout-app-tier.sh`, `deploy-changed.sh`, `apply-observability.sh`, `ensure-app-secrets.sh` (алиасы `PROD_*` → `STAGING_*`).
- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier`.
- [ ] **`VOICE_IMAGE_TAG` required** — убран fallback `:latest` в [`render-and-apply.sh`](../../scripts/staging/render-and-apply.sh) / prod; локальный apply без TAG падает (документировать в DEPLOYMENT или env example).
- [ ] **Prod smoke = alias staging** — [`smoke-prod.sh`](../../scripts/prod/smoke-prod.sh) → [`smoke-staging.sh`](../../scripts/staging/smoke-staging.sh), `STAGING_STAFF_TOKEN` из `PROD_STAFF_TOKEN`; нет отдельных prod acceptance checks.
- [ ] **Prod deploy без selective / stack.lock** — [`prod-deploy.yml`](../../.github/workflows/prod-deploy.yml): нет `changed_services`, `needs_user_space_rollout`, artifact lock; `verify-prod-images` требует **все** образы catalog на TAG; `images-only` → `deploy-changed.sh` без `CHANGED_SERVICES` = no-op.
- [ ] **Prod `full` mode всегда `rollout-app-tier.sh`** — нет user/space subset rollout как на staging; single-node Recreate strategy остаётся.

### Pipeline bugs

- [ ] **`.github/ci/batch11-audit.md` устарел** — статусы 2026-07-07, не отражает selective CI 2026-07-12; обновить или удалить после сверки с этой секцией.

### Tech debt

- [ ] **Promote bootstrap** — первый push в пустой GHCR, force-push, squash-merge → `BASE_TAG` без образов, CI падает на promote (нужен bootstrap full build или `STAGING_FORCE_FULL_ROLLOUT=true`).
- [ ] **`apply-app-manifests` всегда scale auth 0→1** — даже selective deploy; downtime Auth на каждый app apply ([`apply-app-manifests.sh`](../../scripts/staging/apply-app-manifests.sh)).
- [ ] **Дедуп frontend Docker build** — отложено (admin/developer-portal: npm build + docker build).
- [ ] **Prod reuse staging ops scripts** — [`render-and-apply-prod.sh`](../../scripts/prod/render-and-apply-prod.sh) → `rollout-app-tier.sh`, `deploy-changed.sh`, `apply-observability.sh`, `ensure-app-secrets.sh` (алиасы `PROD_*` → `STAGING_*`).
- [ ] **Prod placeholders** — [`deploy/prod/domains.defaults`](../../deploy/prod/domains.defaults) `*.voice.example.com`; secrets checklist только в README, не в ops TODO Critical.
- [ ] **`staging-stack-lock` параллельно с auth/web/admin/portal** — lock пишется до push frontend-образов; auto-deploy ждёт эти jobs + verify — ок для happy path, не для отладки artifact mid-pipeline.
- [ ] **`rollout-user-space-tier` downtime `voice-space`** — scale 0→1 на каждый user/space deploy; альтернатива — полный `rollout-app-tier`.
- [ ] **`VOICE_IMAGE_TAG` required** — убран fallback `:latest` в [`render-and-apply.sh`](../../scripts/staging/render-and-apply.sh) / prod; локальный apply без TAG падает (документировать в DEPLOYMENT или env example).
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

- [ ] **Helm/Kustomize + GitOps** — отложено; ordered rollout остаётся в bash на runner.
- [ ] **Пересмотреть цель continuous full-stack deploy** — selective deploy есть; GitOps позже.
- [ ] **`e2e-manifest.sh` / smoke runtime** — awk-парсер YAML хрупкий; 16+ gateway + 15 flutter smoke на master — риск >15 min / flake ([`.github/ci/batch11-audit.md`](../../.github/ci/batch11-audit.md)).


**Промпт-якорь:** `CI/CD + deploy from docs/todo/ci.md` + приоритет/подсекция.
