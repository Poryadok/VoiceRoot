# Окружения и выкат

Согласуется со стеком в [MICROSERVICES.md](MICROSERVICES.md) и [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) (k3s / Kubernetes, ClusterDNS). Политика релизов, canary и откат — [OPERATIONS.md](OPERATIONS.md).

---

## Окружения (стенды)

| Окружение      | Назначение                     | Инфраструктура                                                                                                                                               |
|----------------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **local**      | Разработка одного разработчика | Docker Compose: infra (Postgres, Redis, NATS JetStream) или **полный core стенд** — `make compose-app-up` / `--profile app` ([README.md](../README.md), [PLAN.md](PLAN.md)). Порты: `GATEWAY_PORT` (рекомендуется **18080**), `WEB_PORT` (**9080**), `POSTGRES_PORT`, `REDIS_PORT`, `NATS_PORT`, `NATS_HTTP_PORT`. Object storage: **MinIO** (compose, `--profile app`) или Cloudflare R2 через `*_R2_*` в `.env`; staging/prod по умолчанию используют MinIO, внешний S3 provider опционален ([OBJECT_STORAGE.md](OBJECT_STORAGE.md)). |
| **staging**    | Интеграция, регрессия, демо    | **k3s** (лёгкий Kubernetes), версии близки к prod.                                                                                                           |
| **production** | Пользователи                   | **Kubernetes** 1.35 (self-managed или managed: Yandex Managed Kubernetes, Hetzner и т.д.).                                                                   |

Облачный **dev**-кластер не используем. При необходимости общего dev-окружения — кластер по тому же шаблону, что staging.

### Postgres / Redis versions (compose ↔ k8s)

Compose (`docker-compose.yml`) and in-cluster infra (`deploy/staging/infra.yaml`, `deploy/prod/infra.yaml`) both use **PostgreSQL 16** and **Redis 7**. Decision and upgrade policy: [ADR 001](adr/001-compose-postgres-redis-versions.md).

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

- **Staging**: на push в `master` CI **собирает только изменённые** образы (`build_services` из path filter), **promote** остальных с предыдущего SHA (`promote-ghcr-images.sh`), публикует immutable тег `:<git_sha>` (без `:latest`). Артефакт **`stack.lock.yaml`** фиксирует теги всех 23 образов. Деплой: job **`deploy-staging`** в CI (`workflow_call` → [`staging-deploy.yml`](../.github/workflows/staging-deploy.yml)) при `STAGING_DEPLOY_ENABLED=true`; режимы `DEPLOY_MODE=full|app-only|images-only` в [`render-and-apply.sh`](../scripts/staging/render-and-apply.sh). Ручной выкат — **Staging deploy** с обязательным git SHA. Для проверки загруженного Secret без кластера укажите `validate_app_secret_only=true`. При неполном Secret отдельный `mail_only=true` обновляет только `AUTH_RESEND_API_KEY` и `AUTH_RESEND_FROM` существующего `voice-app-secrets`, затем перезапускает только `voice-auth`; он не применяет полный Secret и не запускает остальные сервисы. Эти режимы взаимоисключающие.
- Ручной **Staging deploy** не применяет observability namespace даже при `VOICE_APPLY_OBSERVABILITY=true`; автоматический `workflow_call` сохраняет настройку. Для observability есть отдельный workflow **Staging observability deploy**.
- Both staging deployment and its pre-deploy diagnostic acquire repository source from the public GitHub API tarball endpoint pinned to the requested full commit SHA. The runner verifies the API commit/tree and archive tree before replacing its workspace. It requires HTTPS access to `api.github.com` and `codeload.github.com`; Git transport to `github.com` is diagnostic only. The archive request is unauthenticated.
- **Production**: workflow **[`Production deploy`](../.github/workflows/prod-deploy.yml)** — `workflow_dispatch`, environment **`production`**, обязательный **`image_tag`**, selective inputs `changed_services` / `needs_full_rollout`, optional [`deploy/prod/stack.lock.yaml`](../deploy/prod/stack.lock.example.yaml) для verify; полный стек [`deploy/prod/`](../deploy/prod/) через [`render-and-apply-prod.sh`](../scripts/prod/render-and-apply-prod.sh).

### `VOICE_IMAGE_TAG` (required)

Локальный и CI apply **не используют `:latest`**. Перед `scripts/staging/render-and-apply.sh` или `scripts/prod/render-and-apply-prod.sh` задайте:

```bash
export VOICE_IMAGE_TAG=<git-sha>   # тот же SHA, что в GHCR после зелёного CI
export VOICE_IMAGE_REGISTRY=ghcr.io/<owner>/<repo>
```

Без `VOICE_IMAGE_TAG` скрипты завершаются с ошибкой.

Шаблон env для локального apply: [`deploy/staging/env.example`](../deploy/staging/env.example), [`deploy/prod/env.example`](../deploy/prod/env.example). Аудит скриптов: [`.github/ci/voice-image-tag-audit.md`](../.github/ci/voice-image-tag-audit.md).

### Promote bootstrap (пустой GHCR / первый selective push)

Если в registry нет полного набора образов для `BASE_TAG` (первый push в `master`, squash-merge, force-push, пустой GHCR), job `changes` **не падает**: `resolve-staging-matrix.sh` отходит назад по истории (до 20 коммитов) и для отсутствующих манифестов ставит **rebuild** вместо promote.

Принудительно собрать все образы всё ещё можно так:

1. **Actions → CI → Run workflow** → profile **`full`**.
2. GitHub Variable **`STAGING_FORCE_FULL_ROLLOUT=true`** + ручной **Staging deploy** с `deploy_mode=full` после одного успешного full build.

После появления образов в GHCR selective promote на `master` работает штатно.

Версионирование образов: тег по **git SHA** `master` для непрерывного staging; для prod — тег **semver** (`v1.2.3`) или тот же SHA, зафиксированный в релизном манифесте.

---

## GitHub Actions: registry, staging-кластер

Имена — ориентир для настройки в GitHub (значения секретов в репозиторий не коммитить).

| Что | Где в GitHub | Назначение |
|-----|----------------|------------|
| `GITHUB_TOKEN` | встроенный | Push образов в GHCR из job `gateway-image` (в `CI` выдано `packages: write`). |
| Образ gateway | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/gateway:<git_sha>` (selective push на `master`; unchanged — promote с предыдущего SHA). |
| Образ developer-portal | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/developer-portal:<git_sha>` (job `developer-portal`; build-args из `VOICE_GATEWAY_INGRESS_HOST`). |
| Образ web (Flutter SPA) | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/web:<git_sha>` (job `web`; build-args `VOICE_API_BASE_URL`, `VOICE_LIVEKIT_URL`). |
| Образ admin | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/admin:<git_sha>` (job `admin`; PKCE OAuth `voice-admin`, без staff token в bundle). |
| Variable **`VOICE_DEVELOPER_PORTAL_INGRESS_HOST`** | Settings → Secrets and variables → **Actions** → Variables | FQDN Developer Portal (Ingress host, OAuth callback origin). Подставляется в [`deploy/staging/developer-portal.yaml`](../deploy/staging/developer-portal.yaml) при деплое. По умолчанию — из [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) (`scripts/staging/load-staging-domains.sh`). |
| Variable **`VOICE_WEB_INGRESS_HOST`** | Settings → Secrets and variables → **Actions** → Variables | FQDN Flutter web SPA (Ingress host). Подставляется в [`deploy/staging/flutter-web.yaml`](../deploy/staging/flutter-web.yaml) при деплое. По умолчанию — `app.comrade.click` из [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults). |
| Variable **`VOICE_ADMIN_INGRESS_HOST`** | optional | FQDN Moderation Admin (`admin.comrade.click` в `domains.defaults`). Ingress + OAuth redirect в [`deploy/staging/admin.yaml`](../deploy/staging/admin.yaml). |
| Variable **`VOICE_LIVEKIT_INGRESS_HOST`** | optional | Публичный FQDN LiveKit (`livekit.comrade.click`); signaling Ingress + NodePort RTC 30881/30882. Flutter web build: `VOICE_LIVEKIT_URL=wss://…`. |
| Variable **`VOICE_APPLY_OBSERVABILITY`** | optional | `true` — после app tier вызвать [`scripts/staging/apply-observability.sh`](../scripts/staging/apply-observability.sh) в **Staging deploy**. |
| Variable **`STAGING_OBSERVABILITY_SMOKE_ENABLED`** | optional | `true` — `scripts/staging/smoke-observability.sh` после деплоя. |
| Variable **`STAGING_ALERTMANAGER_NOTIFICATIONS_ENABLED`** | optional | `true` — Alertmanager с Telegram/email (нужен Secret `alertmanager-notifications` в `voice-observability`). |
| Secret **`GRAFANA_ADMIN_PASSWORD`** | Environment **staging** | Пароль Grafana admin (не дефолт `changeme-voice-observability`). |
| Environment **`staging`** | Settings → Environments | Окружение для job деплоя; при необходимости включить required reviewers / wait timer. |
| Secret **`STAGING_KUBECONFIG`** | Environment **staging** → Environment secrets | Kubeconfig для staging **k3s**, целиком в **base64**. `server:` — **`https://127.0.0.1:6443`**. Подготовка: [`prepare-kubeconfig-secret.sh`](../scripts/staging/prepare-kubeconfig-secret.sh). |
| Secret **`STAGING_APP_SECRETS_YAML`** | Environment **staging** → Environment secrets | Base64 Secret manifest `voice-app-secrets` с явным целевым namespace. Для существующего Secret можно передать только изменяемые ключи: до первого изменения кластера preflight проверяет имя/namespace и полный набор обязательных ключей после объединения с текущим Secret, а apply выполняет patch только переданных ключей с проверкой `resourceVersion`. Отсутствующий в кластере Secret требует полного manifest; неполный или некорректный набор останавливает выкат и выводит только имена отсутствующих ключей. MinIO access key/secret хранятся отдельно в `voice-minio-credentials`. Без загруженного manifest локальный путь проверяет `deploy/staging/secret.yaml`, `STAGING_APP_SECRETS_YAML` или существующий Secret. Значения не выводятся. Проверка `validate_app_secret_only` без доступа к кластеру проверяет только формат загрузки и почтовые поля, явно не подтверждая полноту эффективного Secret; при полном деплое выполняется проверка с доступом к кластеру. [`secret.example.yaml`](../deploy/staging/secret.example.yaml) оставлен пустым для local/test и не проходит staging preflight. Нужен действующий ключ Resend и настроенный отправитель для live email; сама проверка наличия ключа не доказывает доставку. |
| Variable **`STAGING_RUNNER_LABELS`** | Environment **staging** → Variables | JSON-массив `runs-on` для deploy jobs. **Рекомендуется для home staging:** `["self-hosted","Linux","X64","voice-staging"]` + runner на ноде ([`setup-github-runner.sh`](../scripts/staging/setup-github-runner.sh)). Пусто → `ubuntu-latest` (github-hosted). |
| Secret **`STAGING_SSH_PRIVATE_KEY`** | Environment **staging** → optional | Только для **github-hosted** runner + SSH-туннель. На home staging с UFW `22` только LAN — **не сработает**; используйте self-hosted runner. |
| Variable **`STAGING_DEPLOY_VIA_SSH`** | optional | `true` — туннель с github-hosted; `false` — локальный API (self-hosted runner); **auto** — туннель только если github-hosted + `127.0.0.1` в kubeconfig + задан SSH key. |
| Variable **`STAGING_SSH_HOST`** / **`STAGING_SSH_USER`** / **`STAGING_SSH_PORT`** | optional | SSH endpoint (defaults: `95.31.10.177`, `pmd`, `22`). |
| Variable **`STAGING_DEPLOY_ENABLED`** | Settings → Secrets and variables → **Actions** → Variables | Ровно `true` — разрешить **автоматический** деплой после успешного `CI` на push в `master` (событие `workflow_run`). Пока переменная не задана или не равна `true`, автодеплой не запускается; остаётся **`workflow_dispatch`** в `Staging deploy`. Для расследования A1 автоматический выкат временно выключен; возвращать `true` можно только после исправления причины NATS и успешной полной приемки staging. |
| Variable **`VOICE_GATEWAY_INGRESS_HOST`** | Settings → Secrets and variables → **Actions** → Variables | Публичный **FQDN** для маршрутизации к Gateway. **Текущий стенд:** `voice.comrade.click` (см. [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults); в CI задаётся этой переменной). Манифест [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml): `Ingress` с `ingressClassName: traefik`, два ресурса — HTTP (`entrypoints: web`) и HTTPS (`websecure` + `tls`). На проде — другой FQDN, **тот же файл**, другие переменные. Пока переменная **пустая**, шаг **Apply gateway Ingress** в workflow пропускается (локальные скрипты берут default из `domains.defaults`). |

**Смена временного домена staging:** отредактируйте [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) (строки `VOICE_*`), синхронно обновите GitHub Variables и DNS в Cloudflare (`voice`, `developers`, `app`, `admin`, `livekit`), затем `render-and-apply.sh` + Ingress (или Staging deploy workflow).
| Variable **`VOICE_GATEWAY_TLS_SECRET`** | optional | Имя Secret типа `kubernetes.io/tls` в namespace приложения для блока `tls` у HTTPS-Ingress (по умолчанию `voice-gateway-tls`). Создайте Secret на кластере **до** включения HTTPS-Ingress (см. ниже). |
| Variable **`VOICE_K8S_NAMESPACE`** | optional | Namespace, где лежат `Service`/`Deployment` `voice-gateway` (по умолчанию `voice-staging`). Для прод-выката — например `voice-prod`, без смены шаблона Ingress. |

**Маршрутизация Gateway (Traefik):** манифесты без привязки к имени стенда: [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml) — два `Ingress` (`voice-gateway-http`, `voice-gateway-https`), бэкенд — `Service` `voice-gateway`, порт **8080**. Traefik маршрутизирует по **имени хоста** (`spec.rules[].host`): на одной ноде может быть много приложений с разными FQDN, если DNS указывает на тот же вход (IP ноды / балансера перед Traefik, обычно те же NodePort **HTTP/HTTPS**, что выдаёт Helm-релиз Traefik в кластере).

**DNS:** запись **A** или **AAAA** на публичный адрес входа к кластеру (для текущего стенда зона **comrade.click** в Cloudflare: поддомены **`voice`**, **`developers`**, **`app`**, **`admin`**, **`livekit`** → тот же IP ноды; для **`livekit`** предпочтителен **DNS only** (grey cloud) из‑за WebRTC UDP).

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

**Проверка деплоя из GitHub после настройки секрета:** в репозитории **Actions** → workflow **Staging deploy** → **Run workflow** (обязательно укажите тег образа — git SHA из зелёного CI run; default `latest` убран). Убедитесь, что job завершает шаг **Apply staging manifests** без ошибок `kubectl`.

**Теги образов и автодеплой:** auto-deploy после CI на `master` использует **`head_sha`** CI run. Job **`staging-stack-lock`** публикует `stack.lock.yaml` (built vs promoted per image); **Staging deploy** скачивает lock при `workflow_call` и `verify-staging-images.sh` проверяет манифесты из lock или полного [`staging-image-catalog.json`](../scripts/ci/staging-image-catalog.json). Job **`staging-images-push`** может быть **skipped** при frontend-only change — достаточно **`staging-images-promote`** + frontend jobs. `scripts/staging/verify-staging-images.sh` не использует `:latest`.

**Sanity после изменений CI:** один раз **Actions → CI → Run workflow** → profile **`full`** (все тиры, все сервисы). Список required checks для branch protection — [`.github/ci/branch-protection-checklist.md`](../.github/ci/branch-protection-checklist.md).

---

## План первого промышленного выката (скелет)

Порядок первого выката:

1. **Registry** и секреты для pull в кластере.
2. **Staging k3s**: namespace, секреты приложения, манифесты/Helm для Gateway + зависимости (PostgreSQL, Redis, NATS — в кластере или управляемые сервисы).
3. Поднять **минимальный вертикальный срез** (Auth + Gateway + один сценарий), проверить health и smoke-тесты.
4. Включить **CI** из [TESTING.md](TESTING.md): сборка и пуш образов, деплой на staging.
5. **Production**: кластер, бэкапы БД, мониторинг (Prometheus/Grafana из [MICROSERVICES.md](MICROSERVICES.md)), затем первый релиз по политике [OPERATIONS.md](OPERATIONS.md) (canary, rollback).

Для будущей активации Voice lifecycle сначала восстановить/мигрировать
`voice_db`, затем проверить PostgreSQL и Redis readiness. Protected coordinator
можно включать только после R22.4–R22.6; R22.3 сам по себе не является activation evidence.

Миграции БД при выкате — строго по разделу «Миграции БД» в [OPERATIONS.md](OPERATIONS.md).

---

## Конфигурация по окружениям

- Различия local / staging / prod — через **переменные окружения** и Kubernetes ConfigMaps/Secrets, не через разные ветки кода.
- URL внешних API (Paddle, FCM, R2 и т.д.) — отдельные credentials на staging и prod.

### Firebase / FCM (Web, Android)

- Dev: `src/frontend/lib/firebase_options.dart` — placeholder; override via `--dart-define` or regenerate with FlutterFire CLI.
- Staging/prod: store `google-services.json` (Android) and Firebase web config in CI secrets; set `FCM_*` env on Notification service (see `src/backend/notification/internal/fcm/`).

### Product analytics (ClickHouse + Analytics service)

Спека фичи: [features/analytics.md](features/analytics.md).

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

### Voice lifecycle storage (source-disabled)

Voice использует `VOICE_DATABASE_URL` для `voice_db` и существующие
`VOICE_REDIS_ADDR`/`VOICE_REDIS_PASSWORD`; новых secrets R22.3 не добавляет.
Миграции `000001_room_lifecycle` и `000002_redis_divergence` выполняются до
Voice rollout. Эти settings не регистрируют bridge/coordinator/handlers и не
открывают public или admin repair route.

### APNs / VoIP (iOS)

Notification Service loads APNs configuration in [`http_sender.go`](../src/backend/notification/internal/apns/http_sender.go):

| Variable | Purpose |
|----------|---------|
| `APNS_KEY_ID`, `APNS_TEAM_ID` | Apple Auth Key ID and Developer Team ID |
| `APNS_AUTH_KEY` | Preferred inline `.p8` PEM value |
| `APNS_PRIVATE_KEY` | Backward-compatible inline `.p8` PEM alias used by the staging/prod secret templates |
| `APNS_AUTH_KEY_PATH` | Optional mounted `.p8` PEM file; used only when neither inline variable is set |
| `APNS_BUNDLE_ID` | Required iOS bundle identifier and regular APNs topic |
| `APNS_VOIP_TOPIC` | Optional VoIP topic; defaults to `APNS_BUNDLE_ID` |
| `APNS_PRODUCTION` | Set exactly to `false` for Apple's sandbox endpoint; any other or unset value uses production |

All of `APNS_KEY_ID`, `APNS_TEAM_ID`, `APNS_BUNDLE_ID` and one auth-key source are required to enable either HTTP sender. Missing or invalid credentials leave the corresponding sender in noop mode.

- **Staging:** copy [`deploy/staging/secret.example.yaml`](../deploy/staging/secret.example.yaml) to `secret.yaml`; its `voice-app-secrets` template wires `APNS_PRIVATE_KEY`. Set it to the Auth Key `.p8` PEM together with `APNS_KEY_ID`, `APNS_TEAM_ID`, `APNS_BUNDLE_ID`, and, when VoIP tokens are used, `APNS_VOIP_TOPIC`; keep `APNS_PRODUCTION="false"` for sandbox devices.
- **Production:** use the production `voice-app-secrets` template, set `APNS_PRODUCTION="true"`, and provide the production key/topic values. Enable Push Notifications and Background Modes (remote notifications, VoIP) in Xcode.
- **Compose dev:** the `notification` Compose service does not set APNs credentials and therefore uses noop senders. It still supports gateway token-registration E2E through [`apns_e2e_live_test.dart`](../src/frontend/test/apns_e2e_live_test.dart) and [`voip_e2e_live_test.dart`](../src/frontend/test/voip_e2e_live_test.dart); run either only with `VOICE_RUN_LIVE_INTEGRATION=true`. Device alert delivery needs configured staging or production credentials.
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
| `ACCOUNT_DELETE_TOKEN_SECRET` | Required dedicated HMAC secret for account-delete restore tokens; at least 32 UTF-8 bytes, stored in `voice-app-secrets`, never reuse JWT/TOTP keys |

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

`scripts/staging/render-and-apply.sh` applies the portal manifest when present; ingress host from `VOICE_DEVELOPER_PORTAL_INGRESS_HOST` (repo Variable, passed through staging-deploy workflow). Add DNS **A/AAAA** for that host to the staging ingress node. **Production** portal Deployment + Ingress: [`deploy/prod/developer-portal.yaml`](../deploy/prod/developer-portal.yaml) (`voice-prod` namespace; substitute production FQDNs via `VOICE_DEVELOPER_PORTAL_INGRESS_HOST`).

### Flutter web staging

Deployment + Ingress: [`deploy/staging/flutter-web.yaml`](../deploy/staging/flutter-web.yaml). **CI** (job `web` in [`ci.yml`](../.github/workflows/ci.yml)) builds and pushes `ghcr.io/<owner>/<repo>/web:<git_sha>` on push to `master` with build-args:

| Build arg | Purpose |
|-----------|---------|
| `VOICE_API_BASE_URL` | Public Gateway URL (e.g. `https://${VOICE_GATEWAY_INGRESS_HOST}`) |
| `VOICE_LIVEKIT_URL` | LiveKit WebSocket URL (e.g. `wss://${VOICE_LIVEKIT_INGRESS_HOST}` or default `wss://livekit.<base-domain>`) |

**Staging deploy** applies `flutter-web.yaml` with the same image tag (`git SHA`) as other services. Ingress host from `VOICE_WEB_INGRESS_HOST` (repo Variable; default `app.comrade.click` in [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults)). Add DNS **A** record in Cloudflare for **`app`** → staging ingress node IP.

Manual build (local or one-off):

```bash
docker build -f src/frontend/Dockerfile src/frontend \
  --build-arg VOICE_API_BASE_URL=https://<VOICE_GATEWAY_INGRESS_HOST> \
  --build-arg VOICE_LIVEKIT_URL=wss://<VOICE_LIVEKIT_INGRESS_HOST>
```

Voice calls on staging: Traefik Ingress for signaling (`deploy/livekit/ingress.yaml`) + NodePort **30881/TCP** and **30882/UDP** on `voice-livekit`; `LIVEKIT_API_KEY` / `LIVEKIT_API_SECRET` in `voice-app-secrets`; `LIVEKIT_URL=wss://<VOICE_LIVEKIT_INGRESS_HOST>` in ConfigMap.

### Moderation Admin staging

Deployment + Ingress: [`deploy/staging/admin.yaml`](../deploy/staging/admin.yaml). **CI** job `admin` pushes `ghcr.io/.../admin:<git_sha>`. Auth OAuth client `voice-admin` (PKCE); **не** вшивать `STAGING_STAFF_TOKEN` в образ. Local compose: `VITE_OAUTH_DISABLED=true` + `ADMIN_STAFF_TOKEN`. DNS **A** for **`admin`** → staging node IP.

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

PG_PASS="$(kubectl get secret voice-app-secrets -n "$NS" -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)"
DSN="postgres://voice:${PG_PASS}@voice-postgres:5432/bot_db?sslmode=disable"
sed -e "s|__K_NAMESPACE__|${NS}|g" \
    -e "s|__MIGRATE_IMAGE_TAG__|v4.18.1|g" \
    -e "s|__DATABASE_URL__|${DSN}|g" \
    deploy/templates/migrate-bot-db-job.yaml | kubectl apply -f -

kubectl wait --for=condition=complete job/voice-migrate-bot-db -n "$NS" --timeout=120s
```

Re-run only when new migration files ship; use a new Job name or delete the completed Job before re-apply.

### `voice_db` lifecycle migration and readiness

`voice_db` is owned by Voice Service. Apply
`src/backend/migrations/voice_db/000001_room_lifecycle.up.sql`, then
`000002_redis_divergence.up.sql`, before rolling or
starting the `voice-voice` Deployment. Local Compose does this through
`compose-db-init`; an operator can also run `make compose-migrate-voice`.

For staging and production, `scripts/staging/apply-migrate-jobs.sh` publishes
ConfigMap `voice-voice-db-migrations`, runs Job `voice-migrate-voice-db`, and
waits for completion. `scripts/prod/apply-infra.sh` reuses that script with the
default `voice-prod` namespace. The application reads `VOICE_DATABASE_URL` from
`voice-app-secrets`; its readiness probe uses `/ready` and checks the lifecycle
schema. Migration completion therefore precedes application readiness.

The same runner applies every Go-owned schema before application manifests:
`bot_db`, `chat_db`, `file_db`, `matchmaking_db`, `messaging_db`,
`moderation_db`, `notification_db`, `role_db`, `search_db`, `social_db`,
`space_db`, `story_db`, `subscription_db`, `user_db`, and `voice_db`. Each
service migration Job reads its database URL through a `voice-app-secrets`
Secret reference. The runner reuses a completed Job only when the stored hash
matches the current SQL files; otherwise it applies the service's migration
ConfigMap and waits for the replacement Job before continuing.

Before a lifecycle schema release, backup `voice_db` together with the other service-owned PostgreSQL databases.
For recovery, restore `voice_db` into an isolated database,
validate migrations through `000002_redis_divergence`, all seven lifecycle tables,
then perform a separately approved cutover; do not restore over the live source.

### gRPC mTLS and NetworkPolicy (prod hardening)

See also [ADR 002: gRPC mTLS scope](adr/002-grpc-mtls-scope.md).

**Current Bot baseline:** `BOT_GRPC_GATEWAY_ONLY=true` remains a narrow legacy
boundary until its Phase-0 principal cutover. It is not a general S2S credential
and cannot justify `x-voice-internal` metadata elsewhere. Compose may use
plaintext gRPC on port **9090** only in an explicitly selected development/test
profile. Phase-0 protected S2S calls require TLS in staging and production; see
[ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md#phase-0-межсервисные-и-edge-principals-target-внедряется-по-сервисам).

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




### Ownership lifecycle principal transport

This remains a disabled public foundation: production Space TransferOwnership
denies before lock/database access. The private durable v2 recovery coordinator
uses the protected clients below only to converge already-persisted journal rows;
no signing, TLS or environment setting activates the public request or v1 saga.
Public ownership activation remains a later gate.

Role keeps ordinary callers on `ROLE_GRPC_LISTEN` (default `:9090`) and rejects
the capability RPC plus every v1/v2 ownership lifecycle RPC there. The additional
listener is TLS-only and routes only `GetOwnershipTransferCapabilities`,
`PrepareOwnershipTransfer`, `FinalizeOwnershipTransfer` and
`AbortOwnershipTransfer`; legacy Apply/Compensate and unrelated RPCs are denied.
Capability advertisement is disabled by default and returns `UNAVAILABLE` until
the process explicitly proves the v1 drain, the exact complete v2 method set and
the retired-space fence. Current deployment settings do not bypass that hold.

| Service | Setting | Purpose |
|---|---|---|
| Space | `SPACE_PRINCIPAL_SIGNING_KEYS_DIR` | Secret mount with exactly two distinct RSA PKCS#8 `<kid>.pem` private keys (at least 2048 bits) |
| Space | `SPACE_PRINCIPAL_ACTIVE_KID` | Active key ID; the peer key remains published for rotation |
| Space | `ROLE_PRINCIPAL_GRPC_ADDR` | Dedicated Role TLS endpoint, normally `voice-role:9091` |
| Space | `ROLE_PRINCIPAL_TLS_CA_FILE` | Optional additional trusted CA PEM; system roots remain available |
| Space | `ROLE_PRINCIPAL_TLS_SERVER_NAME` | Optional expected Role certificate DNS name override; otherwise use endpoint authority |
| Space | `SPACE_ROLE_CLIENT_CERT_FILE`, `SPACE_ROLE_CLIENT_KEY_FILE` | Required mutual-TLS client identity for the Role private listener; mount read-only and scope the certificate to Space |
| Space | `AUTH_PRINCIPAL_GRPC_ADDR` | Dedicated Auth TLS proof endpoint; enabling it requires the Space principal signer |
| Space | `AUTH_PRINCIPAL_TLS_CA_FILE` | Optional additional trusted Auth CA PEM; system roots remain available |
| Space | `AUTH_PRINCIPAL_TLS_SERVER_NAME` | Optional expected Auth certificate DNS name override; otherwise use endpoint authority |
| Space | `SPACE_OWNERSHIP_RECOVERY_INTERVAL` | Positive interval for bounded private journal convergence; defaults to `1s` and starts only when both protected Auth and Role clients are configured |
| Role | `ROLE_PRINCIPAL_GRPC_LISTEN` | Dedicated listener address; default `:9091` when enabled |
| Role | `ROLE_PRINCIPAL_TLS_CERT_FILE`, `ROLE_PRINCIPAL_TLS_KEY_FILE` | Server TLS certificate chain and matching private key secret mounts |
| Role | `ROLE_PRINCIPAL_CLIENT_CA_FILE` | Required PEM CA bundle used to verify mutual-TLS clients on the private listener |
| Role | `S2S_JWKS_URLS_JSON` | Trusted issuer-to-HTTPS endpoint map; include exact keys `space`, `gameintegration`, and `voice` |
| Role | `S2S_JWKS_CA_FILE` | CA bundle for HTTPS issuer endpoints using a private or local CA; Phase0 uses the fixture CA |
| Role | `ROLE_PRINCIPAL_REPLAY_REDIS_ADDR` | Shared Redis for atomic credential replay rejection |

Role's private listener requires a verified client certificate before signed
principal verification. Space, GIS, and Voice each present a distinct client
certificate trusted by `ROLE_PRINCIPAL_CLIENT_CA_FILE`. Configure
`S2S_JWKS_URLS_JSON` with HTTPS URLs for Space's `/.well-known/jwks.json` and
GIS/Voice's `/internal/v1/principal/jwks.json`; the issuer keys must be exactly
`space`, `gameintegration`, and `voice`. Keep each signing private key mounted
only into its issuer. Phase0's GIS and Voice signer settings are
`GAME_INTEGRATION_PRINCIPAL_PRIVATE_KEY_FILE` / `GAME_INTEGRATION_PRINCIPAL_KID`
and `VOICE_PRINCIPAL_PRIVATE_KEY_FILE` / `VOICE_PRINCIPAL_KID`.

Space signs both Role and Auth ownership calls with a fresh `service:space`
principal bound to the exact full RPC, deterministic protobuf request hash and
one `x-request-id`. The Auth client calls only consume and receipt lookup; it
replaces, rather than forwards, incoming authority metadata. A configured Auth
TLS setting without `AUTH_PRINCIPAL_GRPC_ADDR` is a startup error, and TLS has no
plaintext fallback. This client is a disabled coordinator seam and does not
activate the public transfer RPC.

Space publishes only the public keys at `GET /.well-known/jwks.json` on its HTTP
listener. A trusted HTTPS reverse proxy must expose that endpoint to Role; no
HTTP fallback is accepted by the verifier. Provision service-specific keys,
certificates, issuer HTTPS routing and shared replay Redis before activating the
Space dedicated client. Never share Gateway and Space private keys or commit
fixture private keys. Preserve the current+next rotation overlap from
[ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md).

Role validates runtime/TLS configuration at startup. Initial JWKS fetch is lazy
at the first protected request to avoid a Role-to-Space startup cycle; health
alone does not prove ownership transport readiness. Missing/unavailable JWKS,
invalid credentials, or unavailable replay storage deny the call. With the
principal runtime absent, ordinary health/service calls can remain available,
but ownership transfer stays unavailable. A configured Space Role integration
with an absent signer or dedicated client denies transfer before its database
mutation. Public Auth proof confirmation remains a separate activation gate.

## Auth-to-User SDK profile principal

Auth's T14 RS256 signer and request-bound `GetSdkProfileEligibility` client are
implemented and merged in [PR #513](https://github.com/Poryadok/VoiceRoot/pull/513).
That PR reports passing focused Auth contract tests, the Auth Maven suite, and
User gRPC service tests. This is implementation and test evidence only;
deployment and runtime acceptance remain unverified, and no staging or
production deployment is claimed. The game-integration plan still holds
staging rollout until A1 acceptance completes. This section records the
deployment contract and readiness gates. The transport is separate from Auth's
client JWT signer and from Auth's inbound Gateway/Space proof listener on
`:9091`.

| Holder | Setting or Secret | Contract |
|---|---|---|
| Auth | `AUTH_PRINCIPAL_SIGNING_KEYS_DIR` | Read-only secret-mounted directory containing exactly two distinct unencrypted RSA PKCS#8 private keys `<kid>.pem`, each at least 2048 bits. |
| Auth | `AUTH_PRINCIPAL_ACTIVE_KID` | Must name one of the two loaded keys; only that key signs new credentials. |
| Auth | `AUTH_USER_PRINCIPAL_GRPC_ADDR` | User's dedicated TLS listener, normally `voice-user:9094`. |
| Auth | `AUTH_USER_PRINCIPAL_TLS_CA_FILE` | Optional additional trusted CA PEM; JVM system roots remain enabled. |
| Auth | `AUTH_USER_PRINCIPAL_TLS_SERVER_NAME` | Optional expected User certificate DNS name override; otherwise use endpoint authority. |
| User | `USER_AUTH_PRINCIPAL_GRPC_LISTEN` | Auth-only TLS listener, default `:9094`; serves only the allowlisted Auth RPCs. |
| User | `USER_AUTH_PRINCIPAL_TLS_CERT_FILE`, `USER_AUTH_PRINCIPAL_TLS_KEY_FILE` | Matching server certificate chain and private key, mounted read-only. |
| User | `USER_AUTH_PRINCIPAL_REPLAY_REDIS_ADDR` | Shared Redis used by all User replicas for atomic credential replay rejection. |
| User | `S2S_JWKS_URLS_JSON` | Must contain `auth` mapped to the HTTPS Auth principal JWKS endpoint. |
| User | `S2S_JWKS_CA_FILE` | Optional additional CA for the HTTPS JWKS origin. |

The exact signing secret name is `voice-auth-principal-signing`, with keys
`current.pem`, `next.pem`, and `active-kid`. Mount only the two PEM files into
`AUTH_PRINCIPAL_SIGNING_KEYS_DIR`; set `AUTH_PRINCIPAL_ACTIVE_KID` from
`active-kid` (`current` or `next`). Mount keys read-only and never put private
material in a ConfigMap, source control, logs, or review evidence. The Auth issuer is optional
when all three required settings (`AUTH_PRINCIPAL_SIGNING_KEYS_DIR`,
`AUTH_PRINCIPAL_ACTIVE_KID`, `AUTH_USER_PRINCIPAL_GRPC_ADDR`) are absent; in
that state the T14 profile-authority path is unavailable and fails closed. If
any required setting is supplied, all three must be valid or Auth fails startup.
Optional CA/server-name overrides are valid only with the complete required
set. Enabling `auth.sdk-authorization.enabled` requires all three. Invalid,
partial, one-key,
duplicate-key, weak-key, malformed-key, missing-mount, or untrusted-TLS
configuration never falls back to client JWT signing, raw caller headers, or
plaintext gRPC.

Auth publishes only the two public principal keys at
`/api/v1/auth/.well-known/principal-jwks.json`, separate from the client-token
JWKS at `/api/v1/auth/.well-known/jwks.json`. User's `S2S_JWKS_URLS_JSON`
`auth` entry points to the HTTPS URL for the principal endpoint. The principal
JWKS contains the sorted current and next RSA signing keys (`kid`, `use=sig`,
`alg=RS256`) and public parameters only. The existing User verifier cache
remains bounded by `S2S_JWKS_REFRESH_AFTER=30s`, `S2S_JWKS_HARD_EXPIRY=2m`,
and `S2S_UNKNOWN_KID_COOLDOWN=5s`; invalid refresh
does not replace the last-good complete keyset and hard expiry fails closed.

Provision the Auth signing secret and HTTPS JWKS route, User listener
certificate, Auth-to-User network route, shared User replay Redis, and User
issuer map before enabling the Auth caller. Deploy and verify User's issuer
configuration first. Wait for the verifier's JWKS refresh (at most one 30s
refresh interval) before the synthetic proof. Route only Auth to User `:9094`
and User to the Auth HTTPS JWKS endpoint; do not expose the principal gRPC
listener externally. User owns
replay admission using shared Redis `SET NX` at
`user:principal:replay:<hex SHA-256(issuer + NUL + jti)>`, with TTL bounded by
credential expiry and at most 35 seconds. Redis failure, replay, missing keys,
expired JWKS, TLS failure, or malformed proof denies the call.

Rotation must preserve the complete current+next RS256 set across Auth replicas:

1. Provision two distinct keys and publish both public keys at the principal
   JWKS endpoint. Wait for User JWKS refresh and verify a synthetic request
   signed by the peer key before selecting it for live signing.
2. Change `active-kid` in the secret manager and restart Auth replicas to load
   the selected key. Keep both private/public keys during the rollout.
3. After the last old-signing Auth replica stops, retain its public key for at
   least **35 seconds**: principal credentials live at most 30 seconds and User
   permits up to five seconds of future issue/not-before skew.
4. Replace only the inactive key, redeploy the complete two-key set everywhere,
   wait for JWKS refresh, and verify another peer-signed synthetic request
   before a later activation. Never remove an old verification key while a
   credential it signed can still be accepted.

Before T14 transport readiness is claimed, demonstrate valid Auth proof at User
and denials for absent, malformed, wrong-issuer/audience/RPC/request/hash,
replayed and expired proof; verify mismatched profile IDs and zero/stale
revisions also deny. Do not use key values in the evidence.

## Messaging game-signature runtime

These settings enable the protected T15 ingress. Incomplete key or transport
configuration fails startup; when an entire provider set is absent, its new
write route remains fail-closed. Secret files are mounted read-only and never
logged.

| Holder | Setting | Contract |
|---|---|---|
| Messaging | `MESSAGING_TOMBSTONE_KEY_ID`, `MESSAGING_TOMBSTONE_PRIVATE_KEY_FILE`, `MESSAGING_TOMBSTONE_NOT_BEFORE`, `MESSAGING_TOMBSTONE_NOT_AFTER` | Ed25519 PKCS#8 signer and explicit exclusive validity window for moderation tombstones. |
| Messaging | `MESSAGING_TOMBSTONE_JWKS_LISTEN` | Dedicated HTTPS listener for Messaging's public-only tombstone JWKS. |
| Messaging | `MESSAGING_TOMBSTONE_JWKS_TLS_CERT_FILE`, `MESSAGING_TOMBSTONE_JWKS_TLS_KEY_FILE`, `MESSAGING_TOMBSTONE_JWKS_CLIENT_CA_FILE` | Server identity and CA for mandatory client-certificate verification on tombstone JWKS reads. |
| Messaging | `MODERATION_PRINCIPAL_JWKS_URL`, `MODERATION_PRINCIPAL_JWKS_CA_FILE` | Fixed HTTPS moderation principal keyset URL and trust CA. |
| Messaging | `GATEWAY_PRINCIPAL_JWKS_URL`, `GATEWAY_PRINCIPAL_JWKS_CA_FILE` | Fixed HTTPS Gateway principal keyset URL and trust CA for `ApplyGameMessage`. |
| Messaging | `MESSAGING_GATEWAY_PRINCIPAL_TLS_CERT_FILE`, `MESSAGING_GATEWAY_PRINCIPAL_TLS_KEY_FILE`, `MESSAGING_PRINCIPAL_REPLAY_REDIS_URL` | Messaging mTLS client identity and shared replay store for exact Gateway service-principal verification. |
| Messaging | `MESSAGING_PRINCIPAL_TLS_CERT_FILE`, `MESSAGING_PRINCIPAL_TLS_KEY_FILE` | Messaging client identity for moderation JWKS mTLS. |
| Messaging | `MESSAGING_PRINCIPAL_REPLAY_REDIS_URL` | Shared Redis for atomic moderation service-principal replay rejection. |
| Messaging | `AUTH_PRINCIPAL_JWKS_URL`, `AUTH_PRINCIPAL_JWKS_CA_FILE` | Fixed Auth device-status keyset URL and trust CA. |
| Messaging | `MESSAGING_AUTH_PRINCIPAL_TLS_CERT_FILE`, `MESSAGING_AUTH_PRINCIPAL_TLS_KEY_FILE` | Messaging client identity for Auth status JWKS mTLS; snapshots older than four seconds are refreshed or denied. |
| Messaging | `AUTH_GAME_MESSAGE_EXECUTION_PERMIT_URL`, `AUTH_GAME_MESSAGE_EXECUTION_PERMIT_CA_FILE` | Deployment-pinned HTTPS Auth execution-permit endpoint and trust CA. |
| Messaging | `MESSAGING_AUTH_EXECUTION_PERMIT_TLS_CERT_FILE`, `MESSAGING_AUTH_EXECUTION_PERMIT_TLS_KEY_FILE` | Messaging client identity for Auth execution-permit mTLS. |

## Social privacy principals

The standard local/CI Compose app generates its own 30-day credentials through
`social-principal-init`. Named volumes isolate Social signing keys, each TLS
leaf, and the public CA; private files are 0600 for service UID 65532 and mounted
read-only. This bootstrap is not used for staging/production. An incomplete
bootstrap fails rather than replacing existing material. Expired development
credentials require a new disposable Compose project or deliberate recreation
of that project's five principal volumes after stopping its services.

User and Space expose only their privacy decision method to `service:social`
on TLS port 9091. Social publishes the two public signing keys at
`https://voice-social:8443/.well-known/jwks.json`. Its ordinary User connection
remains necessary for `GetProfile` and `ListProfileIDsForAccount`; the migration
denies raw Social authority on ordinary privacy methods, without granting those
other methods access to the protected listener.

Before applying the staging/prod application manifests, provision these
namespace-local Secrets through the environment's secret manager:

For staging, GitHub repository **Settings → Environments → staging → Environment
secrets** stores `STAGING_PRINCIPAL_SECRETS_B64` (base64-encoded gzip of a
Kubernetes Secret List) and `STAGING_PRINCIPAL_CA_KEY_PEM` (the CA private key
for certificate renewal). The deploy restores the bundle when none of its
Secrets exist; a partial set requires manual recovery to avoid rotating keys.
Staging also stores `STAGING_USER_SEARCH_CURSOR_HMAC_KEY` there. The deploy
creates `voice-user-search-projection/cursor-hmac-key` from it when absent.

| Secret | Required data |
|---|---|
| `voice-social-principal-signing` | `current.pem`, `next.pem`: distinct unencrypted RSA PKCS#8 private keys, at least 2048 bits; `active-kid`: `current` or `next` |
| `voice-file-principal-signing` | `current.pem`, `next.pem`, `active-kid` as above |
| `voice-search-principal-signing` | `current.pem`, `next.pem`, `active-kid` as above |
| `voice-social-principal-tls` | `tls.crt`, `tls.key`; certificate SAN includes `voice-social` |
| `voice-user-principal-tls` | `tls.crt`, `tls.key`; certificate SAN includes `voice-user` |
| `voice-space-principal-tls` | `tls.crt`, `tls.key`; certificate SAN includes `voice-space` |
| `voice-file-principal-tls` | `tls.crt`, `tls.key`; certificate SAN includes `voice-file` |
| `voice-user-file-principal-tls` | `tls.crt`, `tls.key`; certificate SAN includes `voice-user` |
| `voice-search-principal-tls` | `tls.crt`, `tls.key`; certificate SAN includes `voice-search` |
| `voice-principal-ca` | `ca.crt`: trusted CA bundle for all six TLS endpoints |

The manifests project only the two `.pem` entries into Social's signing
directory and mount all key/certificate material read-only. Never put private
keys in ConfigMaps, source control or PR evidence. Missing mounts, one key,
identical keys, invalid TLS or partial application config must fail startup.
If replay Redis uses authentication, set the service-specific
`USER_PRINCIPAL_REPLAY_REDIS_PASSWORD` and
`SPACE_PRINCIPAL_REPLAY_REDIS_PASSWORD` from Secrets as well.

Social uses `SOCIAL_PRINCIPAL_SIGNING_KEYS_DIR`, `SOCIAL_PRINCIPAL_ACTIVE_KID`,
`SOCIAL_PRINCIPAL_JWKS_LISTEN`, and `SOCIAL_PRINCIPAL_TLS_CERT_FILE` /
`SOCIAL_PRINCIPAL_TLS_KEY_FILE`. Each protected target uses
`USER_PRINCIPAL_GRPC_ADDR` / `SPACE_PRINCIPAL_GRPC_ADDR`, with matching
`*_PRINCIPAL_TLS_CA_FILE` and `*_PRINCIPAL_TLS_SERVER_NAME` settings.
Consumers use `USER_PRINCIPAL_GRPC_LISTEN` / `SPACE_PRINCIPAL_GRPC_LISTEN`,
their `*_PRINCIPAL_TLS_CERT_FILE` / `*_PRINCIPAL_TLS_KEY_FILE`, and
`*_PRINCIPAL_REPLAY_REDIS_ADDR`. They trust only the configured HTTPS issuer map
in `S2S_JWKS_URLS_JSON` and `S2S_JWKS_CA_FILE`. Cache settings are
`S2S_JWKS_REFRESH_AFTER=30s`, `S2S_JWKS_HARD_EXPIRY=2m` and
`S2S_UNKNOWN_KID_COOLDOWN=5s`; no plaintext fallback exists.

Cutover sequence:

1. Provision keys, CA/certificates and replay Redis; verify each Secret in the
   target namespace without printing its values. Deploy consumers first, with
   9091 enabled, then the Social signer/JWKS and protected client configuration.
   During this direct cutover failed privacy calls deny; no raw-header fallback
   or dual-auth acceptance period is allowed.
2. Apply [the NetworkPolicy](../deploy/templates/network-policy-social-privacy-principal.yaml)
   after replacing `__K_NAMESPACE__`. The staging apply script includes it.
   The CNI must enforce policies; audit additive policies so none grants broader
   access to 9091/8443. Ordinary User 9090 stays reachable for the two explicitly
   unmigrated lookups. Neither network reachability nor a raw marker authenticates
   a privacy caller.
3. Run synthetic Social privacy requests against User and Space, plus absent,
   duplicate, forged, replayed and wrong-bound credentials; prove denied calls
   do not reach stores. Check wrong-CA TLS failure and repeated attempts across
   consumer replicas. Health probes alone are not transport-readiness evidence.
4. If synthetic requests fail, keep the privacy path fail-closed while correcting
   keys, trust or deployment; never roll back into raw-header authentication.

Rotation sequence (all Social replicas must publish the same key set):

1. Publish the active and peer public keys; verify consumer refresh and a
   synthetic request signed by the peer before selecting it for live signing.
2. Update `active-kid` in the secret manager and restart Social deployments to
   reload the active selection. Keep both private/public keys during the rollout.
3. After the last old-signing replica stops, retain its public key for at least
   **35 seconds** (30-second credential plus allowed future issue skew).
4. Replace the inactive key through the secret manager, roll out the complete
   two-key set to every Social replica, and wait for consumer refresh/synthetic
   proof before any later activation. An unavailable or invalid refresh does
   not replace the last good complete set; after two minutes without a valid
   refresh requests fail closed.
## File → User protected retention-owner contract

Provision `voice-file-principal-signing` with `current.pem`, `next.pem` and
`active-kid`; both private keys are distinct unencrypted RSA PKCS#8 keys of at
least 2048 bits. Provision `voice-file-principal-tls` (SAN `voice-file`) and
the User listener certificate (SAN `voice-user`) plus the shared CA. File mounts
its signing/TLS/CA material read-only and publishes HTTPS JWKS on 8443; User
serves the File-only resolver on 9092. Neither port is host-published.

User's issuer map contains both the retained Social URL and
`"file":"https://voice-file:8443/.well-known/jwks.json"`; File calls only
`voice-user:9092` with the CA and exact `voice-user` server name. Replay Redis
is shared but uses the target-specific User File setting. NetworkPolicy permits
only File→User:9092 and User→File:8443, while retaining Social's existing
routes. Before switching active key, publish current+next keys, wait the
credential hard expiry, then retain the retired public key for at least 35 s.

## NATS JWT and leaf activation

The reviewed grants and issuer contract are in
[`deploy/nats/README.md`](../deploy/nats/README.md) and
[`deploy/nats/acl-intent.yaml`](../deploy/nats/acl-intent.yaml). Issue on a
trusted Linux host from the exact release SHA; the issuer writes a protected
four-Secret `secrets.json` restore List plus a separate operator/APP/SYS seed
backup. The disposable fixture is not staging or production issuance material.
Do not issue or persist signing seeds under the shared staging `pmd` UID;
perform issuance on an isolated trusted Linux runner and transport the restore
List by secret-manager stdin, with signing seeds backed up separately. The
existing staging runner briefly decodes the uploaded restore List into a
mode-0600 temporary file during the authorized rotation and deletes it on exit.

Staging and production use an operator-signed APP account for JetStream and a
distinct SYS account with no JetStream entitlement. The hub resolves both JWTs
in MEMORY and accepts workload traffic only through TLS leaf port 7422. Each
actual NATS workload receives one service credential and reaches only its local
`127.0.0.1:4222` leaf; the bootstrap Jobs alone receive `bootstrap.creds`.
Compose is not production evidence for this topology.

Before enabling application leaves, restore the four Secrets, apply the hub,
then run and wait for all four bootstrap Jobs. All 40 fixed durables must exist
and match their filter, delivery subject, ACK and deliver policy; a mismatch is
a stopped rollout requiring explicit durable migration, not an app-side
repair. Run the Bot database receipt migration before starting the Bot fixed
consumer. For Auth, drain the old randomly targeted
`auth_subscription_tier` before moving to the new fixed no-group push durable;
old and new Auth replicas cannot bind it concurrently. Start service leaves
only after successful bootstrap, then verify allowed publish/consume/ACK and
neighboring denial at the exact release SHA.

Staging Bot, Chat, Matchmaking, Space, Notification, and Realtime bind fixed
push durables without a queue group. Keep each deployment singleton during an
image rollout: the staging manifests use `Recreate`, and the apply script
clears Kubernetes' defaulted `rollingUpdate` strategy before applying that
declaration. Do not delete or recreate the durable to work around a subscriber
overlap. Gateway waits for required User gRPC before opening HTTP, so its
staging startup probe allows up to 150 seconds for the configured 120-second
gRPC readiness deadline before liveness checks begin.

Staging account migration is not yet approved: the existing anonymous `$G`
account was observed with 566 consumers, exceeding the issued APP account's
512-consumer limit. Inventory and classify legacy dynamic consumers and prove
state-preserving migration of the eight existing stream messages before
cutover. Do not silently increase the limit, discard consumer state, or start
the JWT hub on empty storage during an ordinary migration.

The explicitly authorized staging-only **known-baseline NATS data reset**
replaces the earlier historical-preservation criterion for that new scenario;
it does not claim the original 566-consumer/eight-message preservation passed.
Only NATS data is reset. Existing operator/account identities, credential Secret
references, TLS, Service/DNS and all non-NATS databases, Redis and files remain.
Retain previous NATS PVCs; use a fresh clean NATS PVC instead of purging them.
An optional `dataPVC` field in `voice-nats-generation` selects independent data
storage while `generation` still selects credential identities. The field must
match `voice-nats-jsdata-dYYYYMMDD` followed by at most eight lowercase letters
or digits. Ordinary manifest rendering preserves this selection; an absent field
retains the earlier identity-bound PVC behavior. Every non-`active` phase blocks
ordinary deployment. Never set this marker by hand to bypass maintenance guards.
The reset operation requires a separately reviewed fenced procedure and known
record/config/consumer-state backup restored into an owned isolated broker.
Broker backup success does not attest current service ACLs or business replay.

For the explicitly authorized staging-only **NATS root rotation**, the owner
accepts an outage and loss of staging NATS streams, consumers, and messages.
That maintenance operation must leave the namespace and all non-NATS resources
and PVCs intact. Stop all 18 actual NATS leaf Deployments before changing the
single PVC-backed hub; retain the previous NATS PVC and four Secrets for
rollback, create a new NATS-only PVC and four immutable generation-named
Secrets, and keep the existing `voice-nats` Service/DNS and TLS server name.
Recreate all four fixed bootstrap Jobs and run the Realtime credential preflight
against the new broker before restarting leaves in dependency order. Normal
staging deploys must reject an in-progress rotation and use the active
generation after cutover. This exception does not authorize a namespace reset
or production rotation.

Run this operation only after the rotation PR is merged and its exact `master`
CI is green. In the protected local `.local/staging-nats` directory, use the
already issued `staging-nats-secret.yaml` (the fixed-name JSON List) and
`scripts/staging/prepare-nats-rotation-bundle.py GENERATION INPUT OUTPUT` to
create the immutable, generation-named List. Keep both Lists and all three
signing seeds in that protected, gitignored directory; never put a seed in a
Kubernetes Secret or GitHub Environment. Choose a new token matching
`rYYYYMMDD` followed by at most eight lowercase letters or digits. The
packager refuses an existing output file and validates all four Secret names,
namespace, keys and encodings before writing a mode-0600 file.

Upload **only** the new versioned List as gzip+base64 to GitHub repository
**Settings → Environments → staging → Environment secrets** under
`STAGING_NATS_ROTATION_SECRETS_B64`. Base64 is transport encoding, not
encryption; do not print the encoded bundle in a terminal or CI log. Keep the
legacy `STAGING_NATS_SECRETS_B64` unchanged for rollback. Do not update app,
Auth, Postgres, MinIO or another namespace's secrets. The trusted staging
runner must be the existing `self-hosted`, `Linux`, `X64`, `voice-staging`
runner. Its manual [Staging NATS root rotation](../.github/workflows/staging-nats-root-rotation.yml)
workflow accepts `activate` plus that exact generation token, or `rollback`
with an empty token. Its `voice-staging-maintenance` concurrency group prevents
ordinary staging deploy from overlapping a rotation. The script derives the
new PVC class/size from the current NATS PVC; it does not touch another PVC.
For the Realtime permissions preflight and live ACL proof Jobs, the workflow
checks that the Realtime image tagged with its exact `master` SHA exists in
GHCR before any cluster mutation. Neither Job uses the previously deployed
Realtime image, which may lack the preflight entrypoint.
It checks anonymous pull with a clean Docker configuration. If that fails, the
staging `VOICE_IMAGE_PULL_SECRET` variable must name a reviewed GHCR registry
Secret in `voice-staging`; the script verifies that Secret can access the exact
image and binds it explicitly to both one-shot Jobs before changing the marker.

Immediately before activation, issue a short-lived `proof.creds` for the new
APP account on an isolated trusted Linux issuer host and upload its base64 bytes as
the staging Environment secret `STAGING_NATS_PROOF_CREDS_B64`. The issuer
limits its lifetime to at most two hours (60 minutes by default). The rotation
preflight verifies the credential signature, issuer, exact scoped ACL and at
least 30 minutes of remaining validity against the versioned Secret List before
writing the marker or creating any Secret/PVC. The account signing seed remains
only in the protected external location; neither the Job nor GitHub receives
it. Delete the GitHub proof credential secret after the run and retain only
sanitized evidence. The workflow does not delete GitHub secrets itself.

After the new hub and four bootstrap Jobs are ready, the script creates a
unique immutable temporary `proof.creds` Secret, a staging-only additive
NetworkPolicy, and a one-shot Realtime proof Job. The Job uses separate local
leaves for the fixed Realtime and proof credentials; only that Job also mounts
the fixed Realtime credential for a direct authenticated hub ACK check. The
script requires the exact sanitized
`NATS_LIVE_ACL_PROOF=PASS generation=<generation> acl_sha=<sha256>` result and
verifies deletion of the Job, NetworkPolicy and Secret before restarting any
leaf or writing `active`. A failure after the cutover begins leaves the marker
at `rotating`. Before leaf restart all 18 leaves remain stopped; a User/Space
startup or cleanup failure can leave some or all leaves running with the new
credentials. Inspect the marker, User override and leaf references before
dispatching `rollback` to restore the retained generation. If the runner is
lost before its cleanup trap runs,
rollback identifies only generation-labeled proof resources, verifies their
names, annotations and namespace, then removes the Job and its Pod before the
temporary NetworkPolicy and credential Secret. It verifies all are absent
before changing the hub, leaves, or marker.
If activation failed after creating its immutable generation Secrets and Bound
PVC, do not rerun `activate` and do not remove those resources. The explicit
`continue-activate` workflow operation is limited to the exact `rotating`
marker generation: it verifies the retained source and target PVCs and Secret
sets, accepts only known source/target hub and leaf references, repeats the
bootstrap and live ACL proof, and restarts leaves only after that proof passes.
The Realtime proof container runs as UID/GID 65532, so its projected service
and proof credential files use group-read mode `0440` with Pod `fsGroup: 65532`.
If continuation fails, leave the marker and resources intact, inspect sanitized
workflow evidence, and choose the recovery path only after verifying the live
resource state.
If that rollback itself stops on a historical bootstrap Job after restoring
the legacy hub, use `diagnose` first. For the interrupted
`r20260930a1 → legacy` incident, `recover-legacy` with generation
`r20260930a1` verifies the rotating marker, ready legacy hub and PVC, exact
Service selector/client port, legacy Secret references and the partial
state observed after run 36660139990: Auth and Social ready at one replica,
User at one unready replica, and the other 15 leaves stopped with no Pods.
On retry it accepts only a contiguous prefix of those same old-reference
leaves, verifies any existing Pods also mount the old credentials/TLS, and
rejects an out-of-order or mixed-generation deployment before mutation.
For the interrupted `r20260930a2 → legacy` recovery, the same operation accepts
either the observed state with all 18 legacy-reference leaves stopped and no
Pods, or a completed rollout with all 18 leaves at one ready and updated replica
on legacy refs. The completed-rollout path also requires exactly one Running,
ready Pod per leaf with legacy credential/TLS mounts and no User override or
override Pod. It skips workload changes and only compare-and-swaps the marker to
`active/legacy` after those checks and a final User Pod check. Partial, mixed,
missing, or unready state fails closed before mutation.
Recovery paths skip bootstrap and proof Jobs and retain NATS PVCs and Secrets.
When leaves are stopped, recovery starts the remaining leaves and temporarily
overrides only User's `SPACE_GRPC_ADDR` to an empty value in its Pod template. User's
space-membership privacy check denies when Space is unavailable. The operation
waits for User and then Space, removes its exact owned override, waits for the
normal User rollout, waits up to 30 seconds for exactly one Running, ready User
Pod without the temporary override, and finally requires all 18 leaves ready
before it compares and swaps the marker to `active/legacy`. A failed step attempts to
remove the temporary override; a later `recover-legacy` run recognizes an
owned override left by runner interruption and resumes its cleanup. Kubernetes
may omit the serialized `value` field for an empty environment variable; the
cleanup accepts only that omitted or exactly empty value with no `valueFrom`
and CAS-tests the observed entry before removal. If all 18 legacy leaves are
already started and the exact owned User override remains, dispatch
`restore-user-cycle` with generation `r20260930a1`. This narrower operation
requires the fully started old-reference state, removes only its owned User
override, waits for restored User and all 18 leaves, then CAS-marks legacy
active; it does not scale stopped leaves, run Jobs, or touch PVCs or Secrets. The
read-only `diagnose` operation reports only booleans and counts for that
override and its Pods; it never prints the environment value. A failed
precondition, cleanup, or rollout leaves the marker rotating for investigation.
Root `activate` and `rollback` now use the same reviewed fail-closed temporary
User override after all 18 leaves are started. Each waits for temporary User,
then Space, removes the exact owned override, waits for the restored User Pod,
and only then permits the marker to become active. A failed step leaves the
marker rotating; cleanup failures retain the owned annotation for diagnosis.
The four bootstrap Jobs and the Realtime permissions preflight fail promptly
when a Job reports `Failed` or `DeadlineExceeded`, with only a bounded failure
category in CI output. Run staging smoke after recovery and
leave A1 ACL proof variables unset unless a separate live proof passes.
After successful proof and cleanup, set both staging Environment variables
`VOICE_NATS_ACL_PROOF_SHA` (the reviewed ACL intent SHA-256) and
`VOICE_NATS_ACL_PROOF_GENERATION` (the newly active generation). Ordinary
`full` and `app-only` deploys require both values to match the reviewed
intent and active generation before mutation. After a verified rollback,
restore `VOICE_NATS_ACL_PROOF_GENERATION` to the retained active generation
and `VOICE_NATS_ACL_PROOF_SHA` to its previously accepted reviewed digest;
never leave either variable attesting to the abandoned target. Keep the old
NATS PVC and Secrets until the replacement has passed NATS and HTTP smoke.

After activation, verify the marker is `active` with the requested generation,
`voice-nats` still selects `voice-nats-pvc-candidate`, the candidate hub mounts
the new PVC/operator/TLS Secrets, all 18 leaves mount new service credentials
and TLS CA, the four generation-annotated bootstrap Jobs and Realtime preflight
are complete, and a staging smoke passes. The next ordinary `full`, `app-only`
or `images-only` deploy reads that marker and must keep the active Secret/PVC
references. A manual `rollback` keeps both generations' assets so a later
attempt remains reviewable; it never deletes a versioned NATS PVC or Secret.

When the owner explicitly authorizes a destructive staging reset, the manual
`Staging deploy` workflow supports a separate clean-install path. It verifies
the requested NATS storage class, image, and operator-confirmed capacity before deleting only the dedicated
`voice-staging` namespace, then recreates that namespace, restores its Secrets,
starts the PVC-backed NATS candidate as the primary hub, and recreates the
current streams and fixed durables before app rollout. This path intentionally
discards every Voice resource and PVC in that namespace, including old NATS
streams, messages and consumers; it does not delete cluster resources, nodes or
any other namespace. The opt-in is manual-only, defaults to false, and requires
a full deploy from `master` using the exact workflow SHA and runs the HTTP smoke. Ordinary full
deployments remain behind the state-preserving migration gate above.

For each target namespace (`voice-staging` or `voice-prod`), the secret manager
must create `voice-nats-operator` with UTF-8 `operator.jwt`, `account.jwt`,
`system-account.jwt`, `account.public`, and `system-account.public`; the hub
TLS Secret is `voice-nats-hub-tls`; and
`voice-nats-service-credentials` has exactly one `<service>.creds` key for
every deployed NATS client. The hub init container validates signatures and
claims, renders the APP/SYS resolver preload into a memory-only file, and runs
`nats-server -t` without emitting configuration output. The hub container sees
only `operator.jwt`, rendered config, and TLS files. Kubernetes `stringData` is
input only. Operator/account/user NKey seeds are never committed, logged, put
into ConfigMaps, or mounted into app pods. Production values are operator
supplied, never copied from fixtures or placeholders.

The exact Secret contract is the same in `voice-staging` and `voice-prod`;
values and signing authorities must be independent across namespaces:

| Secret (`type: Opaque`) | Exact `data` keys | Holder |
|---|---|---|
| `voice-nats-operator` | `operator.jwt`, `account.jwt`, `system-account.jwt`, `account.public`, `system-account.public` | NATS hub renderer; only `operator.jwt` reaches the hub container |
| `voice-nats-hub-tls` | `tls.crt`, `tls.key`, `ca.crt` | NATS hub TLS listener; each leaf reads only `ca.crt` |
| `voice-nats-bootstrap-credentials` | `bootstrap.creds` | Four central provisioning Jobs only |
| `voice-nats-service-credentials` | `analytics.creds`, `auth.creds`, `bot.creds`, `chat.creds`, `file.creds`, `gateway.creds`, `matchmaking.creds`, `messaging.creds`, `moderation.creds`, `notification.creds`, `realtime.creds`, `role.creds`, `search.creds`, `social.creds`, `space.creds`, `story.creds`, `subscription.creds`, `user.creds`, `voice.creds` | Each workload mounts only its own key |

The TLS leaf certificate must verify against `ca.crt` and include DNS SAN
`voice-nats`; its key must match the certificate. `operator.jwt` must name the
distinct SYS account; `account.jwt` is the APP account with JetStream enabled,
and `system-account.jwt` is SYS without JetStream. Each `.creds` file contains
one signed APP user JWT and its user NKey seed, with exact reviewed publish,
subscribe, JetStream INFO and ACK permissions. The Job credential alone may
create the fixed streams and durables. Keep the operator and APP/SYS account
signing seeds in the external secret manager for rotation; they are never
members of these four Kubernetes Secrets.

Staging GitHub Environment secret `STAGING_NATS_SECRETS_B64` holds a base64
encoding of a gzip-compressed JSON Kubernetes `List` containing exactly these
four namespace-bound Secrets, using `data` rather than `stringData`. The
`restore-nats-secrets.sh` step validates names, namespace and key inventory
before creating any Secret. It leaves a complete existing set alone and stops
on a partial set for operator recovery. The separate production secret manager
must supply the same names and keys in `voice-prod`; no staging bundle or
fixture value may be copied to production.

Rotation runbook:

1. Generate a replacement service user in the external secret manager with the
   least-privilege ACL reviewed for that one service; never grant `$JS.API.>`.
2. Issue a replacement user JWT signed by the account, write the replacement `.creds` key, and perform
   a controlled rollout of only that service. Prove its permitted publish,
   subscribe, ACK, and consumer operations and a denied neighboring subject.
3. Revoke the old user JWT only after every replica uses the replacement; retain
   audit evidence without copying credential contents into logs or tickets.
4. Rotate account/operator material only in a maintenance window: create new
   immutable, versioned Secret objects, validate the APP/SYS pair and TLS SAN,
   restart the single-replica hub deliberately, re-run every bootstrap Job, then
   restart leaf workloads. Keep the prior version until the hosted leaf and
   redelivery checks pass; do not claim zero downtime for this RWO hub.

# Object storage

See [Object storage operations](OBJECT_STORAGE.md) for the self-hosted MinIO
default, pinned image/mirror policy, k3s prerequisites, backup/restore, and the
optional S3-provider migration procedure. Its browser-facing signed URL contract
uses the existing Gateway HTTPS host and bucket-specific MinIO ingress routes;
the full staging rollout applies both the ingress and Web-origin CORS setting.
