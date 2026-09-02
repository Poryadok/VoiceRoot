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

Сверка спека↔код (2026-08-17): федерация вне scope. Решения и снятые stale-пункты — `tmp/feature-audit/synthesis.md`. Telegram-parity UX audit (2026-08-28): tracker `tmp/telegram-ux-audit/AUDIT.md` (local tmp; DOC **closed**, open CODE/product → секции ниже). **PLAN.md не трогать** — часть «shipped» завышена (spaces без delete/transfer, checkout-заглушка, стикеры отсутствуют).

### Telegram-parity UX audit — open backlog (2026-08-28)

DOC-аудит закрыт (0 open DOC). Невыполненное разнесено по domain-файлам (ID из `AUDIT.md` для трассировки):

| Куда | Что |
|------|-----|
| [backend.md](todo/backend.md) § High § Telegram-parity audit | R3-A04–A06, A12, A14–A16; stickers/GIF wire; Realtime `message.delivery_ack` NATS; mention `profile_id` |
| [client.md](todo/client.md) § High § Telegram-parity audit | R2-A03–A04 shell/mobile IA; R2-A05 strip done (Batch 9); R3-A11 composer a11y done (Batch 9) |
| [product-roadmap.md](todo/product-roadmap.md) § Telegram-parity audit | Product policy DEFER: strip unread-on-back, pin cycle, archived notifications, Location tab, header idle/DND |
| [design.md](todo/design.md) § Critical Penpot · v3 | R4-04-L04 §3.6b panel GAP frames (implementation track) |

### Топ дыр (после аудита)

| Куда | Что |
|------|-----|
| [backend.md](todo/backend.md) § Critical Subscription | Checkout = `checkout.paddle.test`; CloudPayments `Unimplemented`; JWT `subscription_tier` без `AUTH_NATS_URL` |
| [backend.md](todo/backend.md) § Critical Space | Нет `DeleteSpace` / `TransferOwnership` / tree pin / `GetAuditLog` |
| [backend.md](todo/backend.md) § Critical Messaging | `ForwardMessage` shadow-ban bypass; `MODERATION_GRPC_ADDR` absent on staging k8s |
| [backend.md](todo/backend.md) § High Chat | Стикеры/GIF **в v1** (0 Flutter); view-count; `DeleteChat` |
| [ci.md](todo/ci.md) § Critical | FCM/APNs, Alertmanager, staging DoD, TOTP/Resend/Paddle secrets |
| [admin.md](todo/admin.md) § High | Analytics search/voice dashboard types (backend first) |
| [client.md](todo/client.md) § High | Телефон/OTP, password-reset UI; черновики; mobile shell R2-A04 remainder |

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
