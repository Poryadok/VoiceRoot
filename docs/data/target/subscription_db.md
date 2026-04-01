# `subscription_db` — целевая схема

**Сервис:** Subscription ([subscription-service.md](../../microservices/subscription-service.md)). **Шаг порядка:** 7.

`account_id` → Auth; `space_id` → Space (**без FK**).

---

## `subscriptions`

Подписка **Premium** на аккаунт.

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `account_id` | `UUID` | NOT NULL |
| `plan` | `TEXT` | `premium` |
| `billing_period` | `TEXT` | `monthly` \| `yearly` |
| `status` | `TEXT` | `active` \| `past_due` \| `cancelled` \| `expired` |
| `provider` | `TEXT` | `paddle` \| `cloudpayments` |
| `provider_subscription_id` | `TEXT` | NOT NULL |
| `current_period_start` | `TIMESTAMPTZ` | NOT NULL |
| `current_period_end` | `TIMESTAMPTZ` | NOT NULL |
| `grace_period_end` | `TIMESTAMPTZ` | NULL |
| `cancelled_at` | `TIMESTAMPTZ` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (account_id)` при одной активной логике — или частичный уникальный на `status = active`; `(provider, provider_subscription_id)`.

---

## `space_subscriptions`

**Space Pro** на пространство.

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `space_id` | `UUID` | NOT NULL |
| `purchaser_account_id` | `UUID` | NOT NULL |
| `plan` | `TEXT` | `space_pro` |
| `billing_period` | `TEXT` | `monthly` \| `yearly` |
| `status` | `TEXT` | `active` \| `past_due` \| `cancelled` \| `expired` |
| `provider` | `TEXT` | `paddle` \| `cloudpayments` |
| `provider_subscription_id` | `TEXT` | NOT NULL |
| `current_period_start` | `TIMESTAMPTZ` | NOT NULL |
| `current_period_end` | `TIMESTAMPTZ` | NOT NULL |
| `grace_period_end` | `TIMESTAMPTZ` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(space_id)`; `(purchaser_account_id)`.

---

## `billing_events`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `subscription_id` | `UUID` | NULL, FK → `subscriptions(id)` ON DELETE SET NULL |
| `space_subscription_id` | `UUID` | NULL, FK → `space_subscriptions(id)` ON DELETE SET NULL |
| `provider` | `TEXT` | NOT NULL — `paddle` \| `cloudpayments` |
| `type` | `TEXT` | NOT NULL |
| `amount` | `NUMERIC(18,4)` | NULL |
| `currency` | `TEXT` | NULL |
| `provider_event_id` | `TEXT` | NOT NULL |
| `details` | `JSONB` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Ограничение:** ровно одна из `subscription_id` / `space_subscription_id` NOT NULL (CHECK в миграции).

**Индексы:** `(subscription_id, created_at)`; `(space_subscription_id, created_at)`; `UNIQUE (provider, provider_event_id)` для идемпотентности webhook.

---

## `account_entitlements`

Эффективные лимиты и косметика **Premium** на аккаунт (не дублирует счёт в Paddle — это продуктовый слой после webhook).

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `account_id` | `UUID` | NOT NULL |
| `entitlement_key` | `TEXT` | NOT NULL — например `max_upload_mb`, `profile_banner`, `animated_avatar`, `cosmetic_pack_ids` |
| `value` | `JSONB` | NOT NULL |
| `source_subscription_id` | `UUID` | NULL, FK → `subscriptions(id)` ON DELETE SET NULL |
| `valid_until` | `TIMESTAMPTZ` | NULL — NULL = пока активна привязанная подписка или бессрочный грант |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (account_id, entitlement_key)`; `(source_subscription_id)`.

---

## `space_entitlements`

Лимиты и перки **Space Pro** (слоты буста, кастомный URL и т.д. — конкретные ключи в продукте).

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `space_id` | `UUID` | NOT NULL |
| `entitlement_key` | `TEXT` | NOT NULL |
| `value` | `JSONB` | NOT NULL |
| `source_space_subscription_id` | `UUID` | NULL, FK → `space_subscriptions(id)` ON DELETE SET NULL |
| `valid_until` | `TIMESTAMPTZ` | NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (space_id, entitlement_key)`; `(source_space_subscription_id)`.
