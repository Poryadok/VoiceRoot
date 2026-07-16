# Пробелы и открытые вопросы (документация)

Здесь — **вне статуса реализации** [PLAN.md](PLAN.md). Критерии «готово» по фичам — `docs/features/`; открытые инженерные задачи — в [docs/todo/](todo/).

## Схема

TODO разбит **по домену** (где менять код), не по приоритету. **Critical / High / Common / Low** — секции **внутри каждого файла**.

**CI и deploy — один файл:** pipeline (build/test/promote) и выкат (k8s, secrets, observability smoke) — один контур доставки до staging/prod, поэтому всё в `ci.md`.

| Файл | Домен | Что искать |
|------|--------|------------|
| [ci.md](todo/ci.md) | CI/CD + deploy | GitHub Actions, promote/deploy, k8s, secrets, observability smoke, manifests |
| [design.md](todo/design.md) | Design | Penpot, tokens, frames, design ↔ Flutter parity |
| [client.md](todo/client.md) | Client | Flutter, mobile, a11y, deep links, onboarding, guest UX |
| [backend.md](todo/backend.md) | Backend | Go/Java сервисы, Gateway (server), protos, NATS, compose live tests |
| [admin.md](todo/admin.md) | Admin | `src/admin/`, Developer Portal, staff OAuth |
| [product-roadmap.md](todo/product-roadmap.md) | Product | Сквозные инициативы из плана 20 improvements |

**Не заводить** `critical.md` / `high.md` / `low.md` — приоритет не отдельный файл. Критичный secret → `ci.md` § Critical; Penpot → `design.md`.

## Как пользоваться

| Метка | Кому | Смысл |
|-------|------|--------|
| **Agent batch** | Cursor / агент | Один PR или сессия: общий контекст, TDD, без секретов |
| **Вы** | Человек | Ключи, аккаунт, DNS, юридическое — агент ждёт ввод |

**Порядок работы:** выберите **домен** → внутри файла сверху вниз **Critical → High → Common → Low**. Для агента: `docs/todo/<domain>.md` + приоритет + подсекция (промпт-якорь в конце файла).

**Выполненные пункты:** удалять из нужного domain-файла **целиком** (пункт и пустую подсекцию). **Не** `[x]`, не зачёркивания — только открытое. Чеклисты в `docs/features/` — по своим правилам.

## Куда писать новое

| Тип задачи | Файл |
|------------|------|
| Секрет, k8s, Alertmanager, Loki/Grafana, DNS, rollout manifests | `ci.md` |
| Workflow, path-filters, promote, flaky CI job | `ci.md` |
| Penpot, tokens, screen frames, визуальная parity с Flutter | `design.md` |
| Flutter UI, mobile, a11y, deep link на устройстве | `client.md` |
| gRPC/REST handler, миграция, NATS, proto | `backend.md` |
| Admin UI, moderation queue, developer portal | `admin.md` |
| Новая продуктовая инициатива на несколько сервисов | `product-roadmap.md` |
| Пробел в **спецификации** (`docs/features/`) | сначала спека или вопрос человеку; в TODO — только инженерный хвост |

Внутри файла: `## <Приоритет>` → `### <Тема/сервис>` → пункт. Пересечения — одна строка + «см. `other.md` §…», без копипасты.

## Приоритеты

| Уровень | Смысл |
|---------|--------|
| **Critical** | Блокирует софт-ланч на staging: секреты, observability, обязательные live E2E |
| **High** | Prod/mobile, deep links, регрессия гостей — до или сразу после первых пользователей |
| **Common** | Verification и UX-дыры, не ломают Tier 0 (DM + WS) |
| **Low** | Post-MVP, polish, техдолг |

Критерии фич: [encryption.md](features/encryption.md), [bots.md](features/bots.md), [stories.md](features/stories.md). Observability на staging — [observability.md](features/observability.md) §Definition of Done.

---

## Навигация по доменам

| Файл | Кратко |
|------|--------|
| [ci.md](todo/ci.md) | CI + deploy: pipeline, secrets, observability smoke |
| [design.md](todo/design.md) | Penpot, tokens, design parity |
| [client.md](todo/client.md) | Flutter, a11y, guest/deeplink mobile |
| [backend.md](todo/backend.md) | Service audit по приоритету |
| [admin.md](todo/admin.md) | Admin + Developer Portal |
| [product-roadmap.md](todo/product-roadmap.md) | 20 improvements |
