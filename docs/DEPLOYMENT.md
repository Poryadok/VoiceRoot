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

- **Staging**: после зелёного CI на `master` — **автоматический** деплой ([PLAN.md](PLAN.md), фаза 0). Пока workflow не готов — деплой вручную или `workflow_dispatch`.
- **Production**: деплой только с **явным шагом** (approval в GitHub Environments, ручной запуск job или утверждённый релизный тег) — без автоматического «всё, что в master, сразу в prod».

Версионирование образов: тег по **git SHA** `master` для непрерывного staging; для prod — тег **semver** (`v1.2.3`) или тот же SHA, зафиксированный в релизном манифесте.

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

- [OPERATIONS.md](OPERATIONS.md) — SLO, деградация, canary, rollback, миграции
- [TESTING.md](TESTING.md) — состав CI
- [CONTRIBUTING.md](CONTRIBUTING.md) — ветки и merge в `master`
- [REPOSITORIES.md](REPOSITORIES.md) — монорепо и имена репозиториев
- [PLAN.md](PLAN.md) — дорожная карта продукта и инфраструктуры


