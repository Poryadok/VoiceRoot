# Окружения и выкат

Согласуется со стеком в [MICROSERVICES.md](MICROSERVICES.md) и [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) (k3s / Kubernetes, ClusterDNS). Политика релизов, canary и откат — [OPERATIONS.md](OPERATIONS.md).

---

## Окружения (стенды)

| Окружение      | Назначение                     | Инфраструктура                                                                                                                                               |
|----------------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **local**      | Разработка одного разработчика | Docker Compose: infra (Postgres, Redis, NATS JetStream) или **полный core стенд** — `make compose-app-up` / `--profile app` ([README.md](../README.md), [PLAN.md](PLAN.md)). Порты: `GATEWAY_PORT` (рекомендуется **18080**), `WEB_PORT` (**9080**), `POSTGRES_PORT`, `REDIS_PORT`, `NATS_PORT`, `NATS_HTTP_PORT`. Object storage: **MinIO** (compose, `--profile app`) или Cloudflare R2 через `*_R2_*` в `.env`; staging/prod — только R2. |
| **staging**    | Интеграция, регрессия, демо    | **k3s** (лёгкий Kubernetes), версии близки к prod.                                                                                                           |
| **production** | Пользователи                   | **Kubernetes** 1.35 (self-managed или managed: Yandex Managed Kubernetes, Hetzner и т.д.).                                                                   |

Облачный **dev**-кластер не используем. При необходимости общего dev-окружения — кластер по тому же шаблону, что staging.

**Namespace** в Kubernetes: по умолчанию `voice-staging`, `voice-prod`; иные имена — только если зафиксированы в репозитории инфраструктуры или Helm-чартах.

---

## Поток артефактов

```
Разработка (feature branch)
        → PR → CI (lint, test, build image)
        → merge в master
        → образы в registry (GHCR или выбранный registry)
        → автоматический деплой на staging
        → проверка / ручной регресс
        → прод: тег релиза + workflow с approval (или ручной выкат) → production
```

- **Staging**: образы микросервисов (Go matrix + Auth + **Developer Portal** в [`ci.yml`](../.github/workflows/ci.yml)) пушатся в **GHCR** при каждом push в `master` (теги `:<git_sha>` и `:latest`). Деплой в namespace `voice-staging` выполняет workflow **[`Staging deploy`](../.github/workflows/staging-deploy.yml)** (`kubectl apply` к манифестам в [`deploy/staging/`](../deploy/staging/)): **ручной** запуск (`workflow_dispatch`, ввод тега образа, по умолчанию `latest`); **авто** после успешного `CI` на push в `master` — только при `STAGING_DEPLOY_ENABLED` = `true` (см. раздел ниже). Пока кластер или kubeconfig не готовы — только ручной выкат или отключённый автодеплой.
- **Ограничение staging (историческое):** ранее выкатывался только Gateway. **Текущее:** workflow **Staging deploy** применяет полный app stack [`deploy/staging/`](../deploy/staging/) через `scripts/staging/render-and-apply.sh` (все сервисы + ConfigMap upstreams). Требуются `voice-app-secrets` (см. `secret.example.yaml`) и образы всех сервисов в GHCR. Опционально: `STAGING_SMOKE_ENABLED=true` → `scripts/staging/smoke-staging.sh`.
- **Production**: деплой только с **явным шагом** (approval в GitHub Environments, ручной запуск job или утверждённый релизный тег) — без автоматического «всё, что в master, сразу в prod». Workflow **[`Production deploy`](../.github/workflows/prod-deploy.yml)** — только `workflow_dispatch`, environment **`production`** (approval), обязательный **`image_tag`** (git SHA или semver; **без** default `latest`). Скрипт [`scripts/prod/render-and-apply-prod.sh`](../scripts/prod/render-and-apply-prod.sh); манифесты — [`deploy/prod/`](../deploy/prod/) (сейчас skeleton, расширять по мере cutover).

Версионирование образов: тег по **git SHA** `master` для непрерывного staging; для prod — тег **semver** (`v1.2.3`) или тот же SHA, зафиксированный в релизном манифесте.

---

## GitHub Actions: registry, staging-кластер

Имена — ориентир для настройки в GitHub (значения секретов в репозиторий не коммитить).

| Что | Где в GitHub | Назначение |
|-----|----------------|------------|
| `GITHUB_TOKEN` | встроенный | Push образов в GHCR из job `gateway-image` (в `CI` выдано `packages: write`). |
| Образ gateway | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/gateway:<git_sha>` и тег `latest` (см. `ci.yml`). |
| Образ developer-portal | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/developer-portal:<git_sha>` и тег `latest` (job `developer-portal` в `ci.yml`; build-args из `VOICE_GATEWAY_INGRESS_HOST`). |
| Variable **`VOICE_DEVELOPER_PORTAL_INGRESS_HOST`** | Settings → Secrets and variables → **Actions** → Variables | FQDN Developer Portal (Ingress host, OAuth callback origin). Подставляется в [`deploy/staging/developer-portal.yaml`](../deploy/staging/developer-portal.yaml) при деплое. По умолчанию — из [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) (`scripts/staging/load-staging-domains.sh`). |
| Environment **`staging`** | Settings → Environments | Окружение для job деплоя; при необходимости включить required reviewers / wait timer. |
| Secret **`STAGING_KUBECONFIG`** | Environment **staging** → Environment secrets | Kubeconfig для staging **k3s**, целиком в **base64** (одна строка: `base64 -w0 kubeconfig` на Linux или эквивалент на macOS/Windows). Workflow декодирует в `~/.kube/config`. В поле **`clusters[].cluster.server`** должен быть URL API, **доступный из интернета** (например `https://95.31.10.177:6443`), не `127.0.0.1` и не `https://0.0.0.0:6443` — иначе `kubectl` на GitHub runner не подключится. Подготовка одной строки для секрета: [`scripts/staging/prepare-kubeconfig-secret.sh`](../scripts/staging/prepare-kubeconfig-secret.sh) или [`prepare-kubeconfig-secret.ps1`](../scripts/staging/prepare-kubeconfig-secret.ps1). Локальная проверка шагов workflow без записи в кластер: [`scripts/staging/kubectl-apply-dry-run.sh`](../scripts/staging/kubectl-apply-dry-run.sh) (нужны `kubectl` и рабочий kubeconfig). |
| Variable **`STAGING_DEPLOY_ENABLED`** | Settings → Secrets and variables → **Actions** → Variables | Ровно `true` — разрешить **автоматический** деплой после успешного `CI` на push в `master` (событие `workflow_run`). Пока переменная не задана или не равна `true`, автодеплой не запускается; остаётся **`workflow_dispatch`** в `Staging deploy`. |
| Variable **`VOICE_GATEWAY_INGRESS_HOST`** | Settings → Secrets and variables → **Actions** → Variables | Публичный **FQDN** для маршрутизации к Gateway. **Текущий стенд:** `voice.comrade.click` (см. [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults); в CI задаётся этой переменной). Манифест [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml): `Ingress` с `ingressClassName: traefik`, два ресурса — HTTP (`entrypoints: web`) и HTTPS (`websecure` + `tls`). На проде — другой FQDN, **тот же файл**, другие переменные. Пока переменная **пустая**, шаг **Apply gateway Ingress** в workflow пропускается (локальные скрипты берут default из `domains.defaults`). |

**Смена временного домена staging:** отредактируйте [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) (три строки `VOICE_*`), синхронно обновите GitHub Variables `VOICE_GATEWAY_INGRESS_HOST` и `VOICE_DEVELOPER_PORTAL_INGRESS_HOST`, DNS в Cloudflare, затем `render-and-apply.sh` + Ingress (или Staging deploy workflow).
| Variable **`VOICE_GATEWAY_TLS_SECRET`** | optional | Имя Secret типа `kubernetes.io/tls` в namespace приложения для блока `tls` у HTTPS-Ingress (по умолчанию `voice-gateway-tls`). Создайте Secret на кластере **до** включения HTTPS-Ingress (см. ниже). |
| Variable **`VOICE_K8S_NAMESPACE`** | optional | Namespace, где лежат `Service`/`Deployment` `voice-gateway` (по умолчанию `voice-staging`). Для прод-выката — например `voice-prod`, без смены шаблона Ingress. |

**Маршрутизация Gateway (Traefik):** манифесты без привязки к имени стенда: [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml) — два `Ingress` (`voice-gateway-http`, `voice-gateway-https`), бэкенд — `Service` `voice-gateway`, порт **8080**. Traefik маршрутизирует по **имени хоста** (`spec.rules[].host`): на одной ноде может быть много приложений с разными FQDN, если DNS указывает на тот же вход (IP ноды / балансера перед Traefik, обычно те же NodePort **HTTP/HTTPS**, что выдаёт Helm-релиз Traefik в кластере).

**DNS:** запись **A** или **AAAA** на публичный адрес входа к кластеру (для текущего стенда зона **comrade.click** в Cloudflare: поддомены **`voice`** и **`developers`** → тот же IP ноды, что и у остального трафика на этот k3s, см. [STAGING_SERVER.md](STAGING_SERVER.md)).

**Cloudflare и TLS:** для **`voice.comrade.click`** используется режим **Flexible SSL** (HTTPS между клиентом и Cloudflare, **HTTP** между Cloudflare и origin). Публичный HTTPS к API обеспечивает Cloudflare; на стороне кластера достаточно маршрута на entrypoint **`web`** (HTTP до NodePort Traefik). Secret и Ingress `websecure` на origin **не обязательны** для такой схемы; их имеет смысл добавлять при переходе на **Full** / **Full (strict)** или прямой HTTPS до ноды.

**TLS на origin (в кластере), если нужен HTTPS-Ingress (`websecure`):** Secret типа `kubernetes.io/tls` в namespace приложения, например:

```bash
# self-signed с CN под FQDN; в проде часто cert-manager / ACME или Origin Certificate в Cloudflare
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -subj "/CN=voice.comrade.click"
kubectl create secret tls voice-gateway-tls -n voice-staging --cert=tls.crt --key=tls.key
```

**Сценарии (аналогично миграциям: чисто / с наследием / откат):**

| Ситуация | Что делать |
|----------|------------|
| **Чистый кластер** только под Voice (или вы первые настраиваете ingress) | Поднять Traefik по инструкции вашего дистрибутива k8s, выставить DNS на ноду, создать TLS Secret при необходимости, задать `VOICE_GATEWAY_INGRESS_HOST`, прогнать деплой или [`apply-ingress.ps1`](../scripts/gateway/apply-ingress.ps1). |
| **На ноде уже есть другие проекты** | Не менять чужие namespace. Убедиться, что выбранный **FQDN** не занят чужим `Ingress` (`kubectl get ingress -A`). Добавить только ресурсы Voice (namespace, Deployment, Service, при необходимости Ingress). Порты NodePort у Traefik уже слушают ноду — новый трафик идёт по **другому hostname**, конфликта с другими приложениями нет при уникальном FQDN. |
| **Терминация TLS на внешнем прокси** | **Текущий стенд:** Cloudflare **Flexible SSL** для `voice.comrade.click` — до origin идёт HTTP; достаточно entrypoint **`web`**. Ingress `voice-gateway-https` и TLS Secret на ноде можно не применять, пока не понадобится HTTPS до origin (Full / Full strict или прямой доступ). |
| **Снятие Voice с сервера** | Удалить Ingress по имени: `kubectl delete ingress voice-gateway-http voice-gateway-https -n <namespace>`; затем `kubectl delete deployment,svc -n <namespace> -l app=voice-gateway` (или по именам `voice-gateway`); при необходимости `kubectl delete namespace voice-staging`. YAML в репозитории содержит плейсхолдеры — для `delete -f` сначала подставьте значения или не используйте `-f` с сырым файлом. Чужие namespace не трогать. |

**Ручное применение Ingress** (без ожидания CI): [`scripts/gateway/apply-ingress.ps1`](../scripts/gateway/apply-ingress.ps1) с `-IngressHost` из [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) или подстановка плейсхолдеров в YAML и `kubectl apply -f -`.

**Pull из GHCR в кластере:** если пакет/образ приватный, в namespace `voice-staging` создайте `docker-registry` secret (учёт GitHub с `read:packages`) и задайте **`VOICE_IMAGE_PULL_SECRET`** при `scripts/staging/render-and-apply.sh` — скрипт пропатчит `imagePullSecrets` на Deployments. Или добавьте вручную в Pod template ([`deploy/staging/gateway-deployment.yaml`](../deploy/staging/gateway-deployment.yaml) и др.).

**Developer Portal на staging:** CI пушит `developer-portal:<git_sha>`; auto-deploy подставляет тот же SHA. После deploy проверьте `kubectl get deployment voice-developer-portal -o jsonpath='{.spec.template.spec.containers[0].image}'`.

**Проверка деплоя из GitHub после настройки секрета:** в репозитории **Actions** → workflow **Staging deploy** → **Run workflow** (при необходимости укажите тег образа; по умолчанию для ручного запуска — `latest`). Убедитесь, что job завершает шаг **Apply staging manifests** без ошибок `kubectl`.

**Теги образов и автодеплой:** auto-deploy после CI на `master` использует **`head_sha`** CI run. Ручной `workflow_dispatch` по умолчанию — **`latest`**, что может рассинхрониться при partial failed docker matrix push; предпочитайте **git SHA** из зелёного CI. Workflow проверяет наличие `gateway:<tag>` в GHCR перед apply.

**Sanity после изменений CI:** один раз **Actions → CI → Run workflow** → profile **`full`** (все тиры, все сервисы). Список required checks для branch protection — [`.github/ci/branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md).

---

## План первого промышленного выката (скелет)

Порядок первого выката:

1. **Registry** и секреты для pull в кластере.
2. **Staging k3s**: namespace, секреты приложения, манифесты/Helm для Gateway + зависимости (PostgreSQL, Redis, NATS — в кластере или управляемые сервисы).
3. Поднять **минимальный вертикальный срез** (Auth + Gateway + один сценарий), проверить health и smoke-тесты.
4. Включить **CI** из [TESTING.md](TESTING.md): сборка и пуш образов, деплой на staging.
5. **Production**: кластер, бэкапы БД, мониторинг (Prometheus/Grafana из [MICROSERVICES.md](MICROSERVICES.md)), затем первый релиз по политике [OPERATIONS.md](OPERATIONS.md) (canary, rollback).

Миграции БД при выкате — строго по разделу «Миграции БД» в [OPERATIONS.md](OPERATIONS.md).

---

## Конфигурация по окружениям

- Различия local / staging / prod — через **переменные окружения** и Kubernetes ConfigMaps/Secrets, не через разные ветки кода.
- URL внешних API (Paddle, FCM, R2 и т.д.) — отдельные credentials на staging и prod.

### Firebase / FCM (Web, Android)

- Dev: `src/frontend/lib/firebase_options.dart` — placeholder; override via `--dart-define` or regenerate with FlutterFire CLI.
- Staging/prod: store `google-services.json` (Android) and Firebase web config in CI secrets; set `FCM_*` env on Notification service (see `src/backend/notification/internal/fcm/`).

### Product analytics (ClickHouse + Analytics service)

| Переменная | Где | Назначение |
|------------|-----|------------|
| `CLICKHOUSE_DSN` | `voice-analytics` | Native DSN (`clickhouse://user:pass@host:9000/voice`) |
| `ANALYTICS_ID_HASH_KEY` | `voice-analytics` | HMAC-соль для хеширования account/profile ID (Secret, не коммитить) |
| `NATS_URL` | `voice-analytics`, telemetry publishers | JetStream ingest |
| `GATEWAY_ANALYTICS_SAMPLE_RATE` | Gateway (optional) | Доля REST-запросов для `analytics.gateway.request` (default `0` = off) |

**Compose dev:** `make compose-app-up` поднимает `clickhouse` + `analytics` + `admin` (порт `ADMIN_PORT`, default **9081**). Staff token: `GATEWAY_STATIC_TOKENS_JSON` → `compose-staff-token`. Smoke: `VOICE_RUN_LIVE_COMPOSE=true go test ./... -run TestComposeAnalytics_live` в `src/backend/gateway`.

**Staging rollout:**

1. Применить [`deploy/staging/infra.yaml`](../deploy/staging/infra.yaml) (StatefulSet `voice-clickhouse`) или указать managed ClickHouse в `CLICKHOUSE_DSN`.
2. Применить DDL из [`docker/clickhouse/init/001_events.sql`](../docker/clickhouse/init/001_events.sql) (idempotent).
3. Заполнить `CLICKHOUSE_DSN` и `ANALYTICS_ID_HASH_KEY` в `voice-app-secrets` ([`secret.example.yaml`](../deploy/staging/secret.example.yaml)).
4. Деплой `voice-analytics`; Gateway upstreams уже включают `analytics` в [`configmap-app.yaml`](../deploy/staging/configmap-app.yaml).
5. Grafana: datasource ClickHouse + dashboards `voice-analytics-*.json` ([`deploy/observability/grafana/`](../deploy/observability/grafana/)); plugin `grafana-clickhouse-datasource`.
6. **Backfill** (опционально): replay JetStream с `DeliverAll` за N дней — только по runbook, с лимитом объёма; иначе старт с нуля.

**Admin UI:** `src/admin` — `/analytics/product`, `/analytics/funnels`, `/analytics/export` (staff JWT).

### APNs / VoIP (iOS)

- Notification service reads APNs credentials from env (`APNS_*` — see `src/backend/notification/internal/apns/config.go`).
- **Staging:** copy [`deploy/staging/secret.example.yaml`](../deploy/staging/secret.example.yaml) → `secret.yaml`; set `APNS_KEY_ID`, `APNS_TEAM_ID`, `APNS_PRIVATE_KEY` (Auth Key .p8 PEM), `APNS_BUNDLE_ID`, `APNS_VOIP_TOPIC`, `APNS_PRODUCTION=false` for sandbox devices.
- **Production:** `APNS_PRODUCTION=true`, separate VoIP topic/key if required; enable Push Notifications + Background Modes (remote notifications, VoIP) in Xcode.
- **Compose dev:** token registration E2E (`apns_e2e_live_test`, `voip_e2e_live_test`); alert delivery on device requires staging secrets above.
- Live delivery tests: `src/frontend/test/apns_e2e_live_test.dart`, `voip_e2e_live_test.dart` (opt-in `VOICE_RUN_LIVE_INTEGRATION=true`).

## Developer Portal — production OAuth (bots)

PKCE OAuth for the Developer Portal is enabled in local compose (`developer-portal` service, port `9082`). Production requires matching Auth and portal configuration.

### Auth Service (Java)

| Variable | Purpose |
|----------|---------|
| `AUTH_OAUTH_DEVELOPER_PORTAL_ENABLED` | `true` to enable OAuth for portal |
| `AUTH_OAUTH_DEVELOPER_PORTAL_CLIENT_ID` | OAuth client id (default `voice-developer-portal`) |
| `AUTH_OAUTH_DEVELOPER_PORTAL_CLIENT_SECRET` | Optional; PKCE public client may leave empty |
| `AUTH_OAUTH_DEVELOPER_PORTAL_REDIRECT_URIS` | Comma-separated HTTPS callback URLs (e.g. `https://developers.voice.app/callback`) |
| `AUTH_OAUTH_PUBLIC_API_BASE_URL` | Public Gateway URL used in authorize links |
| `AUTH_OAUTH_AUTHORIZATION_CODE_TTL` | Authorization code lifetime (ISO-8601 duration, default `PT60S`) |

Compose reference: [`docker-compose.yml`](../docker-compose.yml) `auth` service; template [`.env.example`](../.env.example) (`DEVELOPER_PORTAL_OAUTH_*`).

### Developer Portal build

| Build arg / env | Purpose |
|-----------------|---------|
| `VITE_VOICE_API_BASE` | Public Gateway URL (baked at build time) |
| `VITE_OAUTH_CLIENT_ID` | Must match `AUTH_OAUTH_DEVELOPER_PORTAL_CLIENT_ID` |
| `VITE_OAUTH_DISABLED` | `true` — paste-JWT fallback UI (dev only) |

### Production checklist

1. Register OAuth client in Auth with **HTTPS** redirect URIs only.
2. Build and deploy portal static assets with matching `VITE_*` values.
3. Ensure Gateway `GATEWAY_CORS_ALLOWED_ORIGINS` includes the portal origin.
4. Verify PKCE flow: authorize → callback → `POST /api/v1/auth/oauth2/token` with `code_verifier`.
5. Bot runtime (bot token, webhook) is independent of portal OAuth; portal OAuth is for **developer account** login only.

### Staging Kubernetes (voice-auth + portal)

Staging manifests wire Developer Portal OAuth on **voice-auth** via ConfigMap `voice-app-config` ([`deploy/staging/configmap-app.yaml`](../deploy/staging/configmap-app.yaml)) and env on the Auth Deployment ([`deploy/staging/services.yaml`](../deploy/staging/services.yaml)):

| ConfigMap key / Auth env | Purpose |
|--------------------------|---------|
| `AUTH_OAUTH_PUBLIC_API_BASE_URL` | Public Gateway URL in authorize links (must match `https://${VOICE_GATEWAY_INGRESS_HOST}`) |
| `AUTH_OAUTH_DEVELOPER_PORTAL_ENABLED` | `true` on staging |
| `AUTH_OAUTH_DEVELOPER_PORTAL_CLIENT_ID` | OAuth client id (`voice-developer-portal`) |
| `AUTH_OAUTH_DEVELOPER_PORTAL_REDIRECT_URIS` | HTTPS callback(s), e.g. `https://${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}/callback` |
| `AUTH_OAUTH_DEVELOPER_PORTAL_CLIENT_SECRET` | Optional Secret override (PKCE public client may omit) |

Portal Deployment + Ingress: [`deploy/staging/developer-portal.yaml`](../deploy/staging/developer-portal.yaml). **CI** (job `developer-portal` in [`ci.yml`](../.github/workflows/ci.yml)) builds and pushes the image on every push to `master` with `VITE_VOICE_API_BASE=https://${VOICE_GATEWAY_INGRESS_HOST}` and `VITE_OAUTH_CLIENT_ID=voice-developer-portal`. **Staging deploy** applies the portal manifest with the same image tag (`git SHA`) as other services.

Manual build (local or one-off):

```bash
docker build -f src/developer-portal/Dockerfile src/developer-portal \
  --build-arg VITE_VOICE_API_BASE=https://<VOICE_GATEWAY_INGRESS_HOST> \
  --build-arg VITE_OAUTH_CLIENT_ID=voice-developer-portal
```

`scripts/staging/render-and-apply.sh` applies the portal manifest when present; ingress host from `VOICE_DEVELOPER_PORTAL_INGRESS_HOST` (repo Variable, passed through staging-deploy workflow). Add DNS **A/AAAA** for that host to the staging ingress node. **Prod** portal Ingress is not in-repo yet — reuse the staging template with `voice-prod` namespace and production FQDNs.

---

## Bot Service — production rollout (bots)

### Skeleton manifests

| Path | Purpose |
|------|---------|
| [`deploy/prod/namespace.yaml`](../deploy/prod/namespace.yaml) | `voice-prod` namespace |
| [`deploy/prod/services.yaml`](../deploy/prod/services.yaml) | `voice-bot` Deployment + Service (2 replicas); extend before full prod cutover |
| [`deploy/templates/network-policy-voice-bot.yaml`](../deploy/templates/network-policy-voice-bot.yaml) | Ingress NetworkPolicy: gRPC **9090** only from `voice-gateway` |
| [`deploy/templates/migrate-bot-db-job.yaml`](../deploy/templates/migrate-bot-db-job.yaml) | One-shot golang-migrate Job for `bot_db` |

Ensure `voice-app-secrets` includes `BOT_DATABASE_URL` (`postgres://…/bot_db`) and Gateway ConfigMap upstreams include `"bots":"voice-bot:9090"` (staging reference: [`deploy/staging/configmap-app.yaml`](../deploy/staging/configmap-app.yaml)).

### `bot_db` migrations (staging / prod)

1. Create database `bot_db` on cluster Postgres (init script [`docker/postgres/initdb.d/01-init-databases.sh`](../docker/postgres/initdb.d/01-init-databases.sh) includes it).
2. Apply SQL from [`src/backend/migrations/bot_db/`](../src/backend/migrations/bot_db/) **before** scaling `voice-bot` (or on first deploy):

**Local compose:** `make compose-migrate-bot`

**Kubernetes Job (template):**

```bash
# From repo root; namespace voice-prod or voice-staging
NS=voice-prod
kubectl create configmap voice-bot-db-migrations -n "$NS" \
  --from-file=src/backend/migrations/bot_db \
  --dry-run=client -o yaml | kubectl apply -f -

sed -e "s|__K_NAMESPACE__|${NS}|g" \
    -e "s|__MIGRATE_IMAGE_TAG__|v4.18.1|g" \
    deploy/templates/migrate-bot-db-job.yaml | kubectl apply -f -

kubectl wait --for=condition=complete job/voice-migrate-bot-db -n "$NS" --timeout=120s
```

Re-run only when new migration files ship; use a new Job name or delete the completed Job before re-apply.

### gRPC mTLS and NetworkPolicy (prod hardening)

**v1 (current):** `BOT_GRPC_GATEWAY_ONLY=true` — Bot rejects gRPC without Gateway metadata (`x-voice-internal`) or bot-token context. Staging and compose use plaintext gRPC on port **9090** inside the cluster.

**Prod hardening (incremental):**

1. **NetworkPolicy** — apply [`deploy/templates/network-policy-voice-bot.yaml`](../deploy/templates/network-policy-voice-bot.yaml) with `__K_NAMESPACE__=voice-prod` so only `voice-gateway` pods may connect to `voice-bot:9090`. Requires a CNI that enforces NetworkPolicy (Calico, Cilium, etc.).
2. **mTLS between services** — not wired in application code yet; options: service mesh (Linkerd/Istio), or gRPC server TLS on Bot with Gateway as client. Until then, rely on NetworkPolicy + `BOT_GRPC_GATEWAY_ONLY`. Document chosen CA/cert rotation in infra repo when enabled.

```bash
sed "s|__K_NAMESPACE__|voice-prod|g" deploy/templates/network-policy-voice-bot.yaml | kubectl apply -f -
```

### Staging webhook E2E (opt-in)

Compose covers webhook delivery via `host.docker.internal` ([`compose_bots_slash_live_test.go`](../src/backend/gateway/compose_bots_slash_live_test.go)). For **staging**, run the gateway opt-in test (not in default CI):

```bash
cd src/backend/gateway
VOICE_STAGING_API_URL=https://voice.comrade.click \
VOICE_STAGING_WEBHOOK_PING_URL=https://<public-echo>/ping \
go test -run TestStagingBotsWebhook_live -count=1 .
```

`VOICE_STAGING_WEBHOOK_PING_URL` must be reachable **from the staging Bot pod** (tunnel, echo service, or request bin) and return `{"content":"pong"}` to the Bot webhook POST. Localhost URLs will fail.

---

## Связанные документы

- [STAGING_SERVER.md](STAGING_SERVER.md) — инвентарь физического staging-хоста (SSH, k3s, Traefik; без секретов)
- [OPERATIONS.md](OPERATIONS.md) — SLO, деградация, canary, rollback, миграции
- [TESTING.md](TESTING.md) — состав CI
- [CONTRIBUTING.md](CONTRIBUTING.md) — ветки и merge в `master`
- [REPOSITORIES.md](REPOSITORIES.md) — монорепо и имена репозиториев
- [PLAN.md](PLAN.md) — дорожная карта продукта и инфраструктуры


