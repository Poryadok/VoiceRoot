# Окружения и выкат

Согласуется со стеком в [MICROSERVICES.md](MICROSERVICES.md) и [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) (k3s / Kubernetes, ClusterDNS). Политика релизов, canary и откат — [OPERATIONS.md](OPERATIONS.md).

---

## Окружения (стенды)

| Окружение      | Назначение                     | Инфраструктура                                                                                                                                               |
|----------------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **local**      | Разработка одного разработчика | Docker Compose (цель — одна команда: сервисы + PostgreSQL + Redis), см. [PLAN.md](PLAN.md) фаза 0; до готовности compose — ручной подъём отдельных сервисов. |
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

- **Staging**: образ **API Gateway** (минимальный каркас в [`src/backend/gateway/`](../src/backend/gateway/)) собирается и пушится в **GHCR** при каждом push в `master` (workflow [`CI`](../.github/workflows/ci.yml), job `gateway-image`). Деплой в namespace `voice-staging` выполняет workflow **[`Staging deploy`](../.github/workflows/staging-deploy.yml)** (`kubectl apply` к манифестам в [`deploy/staging/`](../deploy/staging/)): **ручной** запуск (`workflow_dispatch`, ввод тега образа, по умолчанию `latest`); **авто** после успешного `CI` на push в `master` — только при `STAGING_DEPLOY_ENABLED` = `true` (см. раздел ниже). Пока кластер или kubeconfig не готовы — только ручной выкат или отключённый автодеплой.
- **Production**: деплой только с **явным шагом** (approval в GitHub Environments, ручной запуск job или утверждённый релизный тег) — без автоматического «всё, что в master, сразу в prod».

Версионирование образов: тег по **git SHA** `master` для непрерывного staging; для prod — тег **semver** (`v1.2.3`) или тот же SHA, зафиксированный в релизном манифесте.

---

## GitHub Actions: registry, staging-кластер

Имена — ориентир для настройки в GitHub (значения секретов в репозиторий не коммитить).

| Что | Где в GitHub | Назначение |
|-----|----------------|------------|
| `GITHUB_TOKEN` | встроенный | Push образов в GHCR из job `gateway-image` (в `CI` выдано `packages: write`). |
| Образ gateway | GHCR | `ghcr.io/<owner_lowercase>/<repo_lowercase>/gateway:<git_sha>` и тег `latest` (см. `ci.yml`). |
| Environment **`staging`** | Settings → Environments | Окружение для job деплоя; при необходимости включить required reviewers / wait timer. |
| Secret **`STAGING_KUBECONFIG`** | Environment **staging** → Environment secrets | Kubeconfig для staging **k3s**, целиком в **base64** (одна строка: `base64 -w0 kubeconfig` на Linux или эквивалент на macOS/Windows). Workflow декодирует в `~/.kube/config`. В поле **`clusters[].cluster.server`** должен быть URL API, **доступный из интернета** (например `https://95.31.10.177:6443`), не `127.0.0.1` и не `https://0.0.0.0:6443` — иначе `kubectl` на GitHub runner не подключится. Подготовка одной строки для секрета: [`scripts/staging/prepare-kubeconfig-secret.sh`](../scripts/staging/prepare-kubeconfig-secret.sh) или [`prepare-kubeconfig-secret.ps1`](../scripts/staging/prepare-kubeconfig-secret.ps1). Локальная проверка шагов workflow без записи в кластер: [`scripts/staging/kubectl-apply-dry-run.sh`](../scripts/staging/kubectl-apply-dry-run.sh) (нужны `kubectl` и рабочий kubeconfig). |
| Variable **`STAGING_DEPLOY_ENABLED`** | Settings → Secrets and variables → **Actions** → Variables | Ровно `true` — разрешить **автоматический** деплой после успешного `CI` на push в `master` (событие `workflow_run`). Пока переменная не задана или не равна `true`, автодеплой не запускается; остаётся **`workflow_dispatch`** в `Staging deploy`. |
| Variable **`VOICE_GATEWAY_INGRESS_HOST`** | Settings → Secrets and variables → **Actions** → Variables | Публичный **FQDN** для маршрутизации к Gateway. **Текущий стенд:** `voice.tastytest.online` (задаётся в этой переменной). Манифест [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml): `Ingress` с `ingressClassName: traefik`, два ресурса — HTTP (`entrypoints: web`) и HTTPS (`websecure` + `tls`). На проде — другой FQDN, **тот же файл**, другие переменные. Пока переменная **пустая**, шаг **Apply gateway Ingress** в workflow пропускается. |
| Variable **`VOICE_GATEWAY_TLS_SECRET`** | optional | Имя Secret типа `kubernetes.io/tls` в namespace приложения для блока `tls` у HTTPS-Ingress (по умолчанию `voice-gateway-tls`). Создайте Secret на кластере **до** включения HTTPS-Ingress (см. ниже). |
| Variable **`VOICE_K8S_NAMESPACE`** | optional | Namespace, где лежат `Service`/`Deployment` `voice-gateway` (по умолчанию `voice-staging`). Для прод-выката — например `voice-prod`, без смены шаблона Ingress. |

**Маршрутизация Gateway (Traefik):** манифесты без привязки к имени стенда: [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml) — два `Ingress` (`voice-gateway-http`, `voice-gateway-https`), бэкенд — `Service` `voice-gateway`, порт **8080**. Traefik маршрутизирует по **имени хоста** (`spec.rules[].host`): на одной ноде может быть много приложений с разными FQDN, если DNS указывает на тот же вход (IP ноды / балансера перед Traefik, обычно те же NodePort **HTTP/HTTPS**, что выдаёт Helm-релиз Traefik в кластере).

**DNS:** запись **A** или **AAAA** на публичный адрес входа к кластеру (для текущего стенда зона **tastytest.online** в Cloudflare: поддомен **`voice`** → тот же IP ноды, что и у остального трафика на этот k3s, см. [STAGING_SERVER.md](STAGING_SERVER.md)).

**Cloudflare и TLS:** для **`voice.tastytest.online`** используется режим **Flexible SSL** (HTTPS между клиентом и Cloudflare, **HTTP** между Cloudflare и origin). Публичный HTTPS к API обеспечивает Cloudflare; на стороне кластера достаточно маршрута на entrypoint **`web`** (HTTP до NodePort Traefik). Secret и Ingress `websecure` на origin **не обязательны** для такой схемы; их имеет смысл добавлять при переходе на **Full** / **Full (strict)** или прямой HTTPS до ноды.

**TLS на origin (в кластере), если нужен HTTPS-Ingress (`websecure`):** Secret типа `kubernetes.io/tls` в namespace приложения, например:

```bash
# self-signed с CN под FQDN; в проде часто cert-manager / ACME или Origin Certificate в Cloudflare
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -subj "/CN=voice.tastytest.online"
kubectl create secret tls voice-gateway-tls -n voice-staging --cert=tls.crt --key=tls.key
```

**Сценарии (аналогично миграциям: чисто / с наследием / откат):**

| Ситуация | Что делать |
|----------|------------|
| **Чистый кластер** только под Voice (или вы первые настраиваете ingress) | Поднять Traefik по инструкции вашего дистрибутива k8s, выставить DNS на ноду, создать TLS Secret при необходимости, задать `VOICE_GATEWAY_INGRESS_HOST`, прогнать деплой или [`apply-ingress.ps1`](../scripts/gateway/apply-ingress.ps1). |
| **На ноде уже есть другие проекты** | Не менять чужие namespace. Убедиться, что выбранный **FQDN** не занят чужим `Ingress` (`kubectl get ingress -A`). Добавить только ресурсы Voice (namespace, Deployment, Service, при необходимости Ingress). Порты NodePort у Traefik уже слушают ноду — новый трафик идёт по **другому hostname**, конфликта с другими приложениями нет при уникальном FQDN. |
| **Терминация TLS на внешнем прокси** | **Текущий стенд:** Cloudflare **Flexible SSL** для `voice.tastytest.online` — до origin идёт HTTP; достаточно entrypoint **`web`**. Ingress `voice-gateway-https` и TLS Secret на ноде можно не применять, пока не понадобится HTTPS до origin (Full / Full strict или прямой доступ). |
| **Снятие Voice с сервера** | Удалить Ingress по имени: `kubectl delete ingress voice-gateway-http voice-gateway-https -n <namespace>`; затем `kubectl delete deployment,svc -n <namespace> -l app=voice-gateway` (или по именам `voice-gateway`); при необходимости `kubectl delete namespace voice-staging`. YAML в репозитории содержит плейсхолдеры — для `delete -f` сначала подставьте значения или не используйте `-f` с сырым файлом. Чужие namespace не трогать. |

**Ручное применение Ingress** (без ожидания CI): [`scripts/gateway/apply-ingress.ps1`](../scripts/gateway/apply-ingress.ps1) с `-IngressHost 'voice.tastytest.online'` или подстановка плейсхолдеров в YAML и `kubectl apply -f -`.

**Pull из GHCR в кластере:** если пакет/образ приватный, в namespace `voice-staging` создайте `docker-registry` secret (учёт GitHub с `read:packages`) и добавьте `imagePullSecrets` в Pod template Deployment (в репозитории при необходимости расширить [`deploy/staging/gateway-deployment.yaml`](../deploy/staging/gateway-deployment.yaml)).

**Проверка деплоя из GitHub после настройки секрета:** в репозитории **Actions** → workflow **Staging deploy** → **Run workflow** (при необходимости укажите тег образа; по умолчанию для ручного запуска — `latest`). Убедитесь, что job завершает шаг **Apply staging manifests** без ошибок `kubectl`.

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

---

## Связанные документы

- [STAGING_SERVER.md](STAGING_SERVER.md) — инвентарь физического staging-хоста (SSH, k3s, Traefik; без секретов)
- [OPERATIONS.md](OPERATIONS.md) — SLO, деградация, canary, rollback, миграции
- [TESTING.md](TESTING.md) — состав CI
- [CONTRIBUTING.md](CONTRIBUTING.md) — ветки и merge в `master`
- [REPOSITORIES.md](REPOSITORIES.md) — монорепо и имена репозиториев
- [PLAN.md](PLAN.md) — дорожная карта продукта и инфраструктуры


