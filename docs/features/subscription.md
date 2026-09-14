# Subscription — монетизация через подписки

Подписка — один из инструментов монетизации (наряду с другими покупками, которые будут определены отдельно).

Два независимых продукта: личная подписка и подписка спейса.

---

## Премиум профиль — $5/мес (личная, привязана к аккаунту)

Бонусы применяются ко всем профилям аккаунта — см. [multi-profile.md](multi-profile.md).

### Косметика профиля
- Значок Premium ★ рядом с именем во всех чатах
- Анимированный аватар (GIF)
- Баннер профиля (фоновое изображение)
- Кастомный статус
- Эмодзи-статус
- Красивый @username (без суффикса #1234)
- Эксклюзивные стикеры и эмодзи
- Кастомные иконки приложения
- Эксклюзивные темы оформления чатов

### Функциональные улучшения
- **Множественные реакции на сообщение** — до **3** emoji на одно сообщение от одного пользователя (free: **1**); enforcement в Messaging `AddReaction` — [text-chat.md](text-chat.md) § «Социальные механики»
- Приоритетная полоса при скачивании файлов (CDN-приоритет)
- Увеличенный лимит загрузки файлов: **200 MB** (vs 50 MB бесплатно) — см. [file-storage.md](file-storage.md)
- До 1000 спейсов и групповых чатов (vs 100 бесплатно)
- Повышенное качество стриминга: **720p / 30fps** (vs 360p / 30fps бесплатно)
- До 5 профилей на аккаунт (vs 2 бесплатно) — см. [multi-profile.md](multi-profile.md)

### Дополнительные покупки (отдельно от подписки)
- Красивый @username для дополнительных профилей — докупается отдельно

---

## Space Pro — $5/мес (на спейс, платит владелец)

### Лимиты
- Участников в спейсе: 5000 (vs 50 бесплатно)
- Войс-лимит (одновременно в голосовой комнате): 128 (vs 32 бесплатно)
- Каналов внутри спейса (текстовые + войс суммарно): 500 (vs 50 бесплатно)

### Кастомизация
- Баннер спейса
- Кастомные эмодзи спейса

### Поведение при отмене подписки
- Существующих участников не кикает
- Новые участники не могут вступить, пока число не опустится ниже бесплатного лимита (50)

---

## Условия оплаты

- **Годовая подписка**: скидка −20% (оба продукта)
- **Возврат**: не предусмотрен

## Платёжные провайдеры

| Рынок  | Провайдер     | Условия                                                                        |
|--------|---------------|--------------------------------------------------------------------------------|
| Не-СНГ | Paddle        | Merchant of Record, работает без юр. лица, сам разбирается с НДС               |
| СНГ    | CloudPayments | Поддерживает самозанятых; нужно: статус самозанятого + счёт в Tinkoff Business |

Оба провайдера поддерживают рекуррентные платежи (подписки).

## Жизненный цикл подписки

### Состояния подписки

```
active ──(неудачная оплата)──► grace_period ──(7 дней)──► cancelled
  ▲                                  │
  └────────(оплата прошла)───────────┘

active ──(отмена вручную)──► active до конца периода ──► cancelled

cancelled ──(новая подписка)──► active
```

- **active** — подписка активна, все фичи доступны
- **grace_period** — 7 дней после неудачного платежа; фичи продолжают работать; уведомления на 1-й, 3-й и 7-й день
- **cancelled** — фичи работают до конца оплаченного периода (при ручной отмене) или сразу отключаются (после grace period)
- **Пробный период**: нет на старте

Диаграмма выше сохраняет текущие product/storage names. Для межсервисного A7
контракта канонические состояния — `ACTIVE`, `GRACE_PERIOD`, `INACTIVE`; ручная
отмена до конца оплаченного периода остаётся `ACTIVE` с
`cancel_at_period_end=true`; её `downgrade_cycle_id` сохраняется через возможный
failed-payment grace до successor `INACTIVE`, но очищается при
resume/recovery/renewal или новой покупке/`STARTED`.

### Downgrade-сценарий (личная подписка)

При cancel-at-period-end или grace, если у пользователя более 2 non-deleted
профилей, заранее показывается выбор primary + eligible secondary. До
authoritative `INACTIVE` ничего не замораживается. На `INACTIVE` User применяет
выбор/fallback и добавляет остальным отдельный subscription-freeze overlay;
данные сохраняются. Любой последующий `ACTIVE`, включая новую покупку, снимает
только этот overlay. Grace не замораживает профили, а moderation/deletion/другие
disabled-причины подписка никогда не снимает.

### Provider-independent lifecycle contract (accepted target; not implemented)

`Subscription Service` owns one revisioned entitlement aggregate per personal
account and per Space. `active` and `grace_period` grant the same benefits;
manual cancellation sets `cancel_at_period_end` but remains active until the
paid period ends. Only the subsequent authoritative `inactive` revision removes
benefits. Payment recovery or renewal during grace returns the aggregate to
`active` and cancels undelivered reminders. Webhook arrival order is not state
order: adapters use a provider version/authoritative snapshot and never regress
an aggregate from an older event. Every entitled snapshot includes
`entitled_until`; benefits require database time strictly before it, so a lost
renewal/expiry event cannot leave Premium or Space Pro enabled forever.
Provider `paused` is normalized as active-until-period-end only when a verified
snapshot proves that boundary, then becomes inactive; ambiguous pause semantics
are reconciled/quarantined rather than exposed as a fourth state.

The canonical cross-service fact is a complete revisioned
`subscription.entitlement_changed` snapshot. Subscription commits it through a
transactional outbox with a stable UUID and publishes the stored bytes with
`Nats-Msg-Id=event_id`. Auth, User, File, Space, Voice and Notification keep
service-owned transactional inbox/projection state and apply only a newer
aggregate revision. Analytics deduplicates canonical event IDs in ClickHouse.
Duplicate and out-of-order delivery is
therefore expected and safe; a seven-day event history or an in-memory cache is
not bootstrap authority. Full wire, claim, replay and snapshot-reconciliation
rules are frozen in
[subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

Entering a failed-payment grace atomically schedules D1/D3/D7 reminders for the
same grace revision. D1 is due at grace start, D3 after two days and D7 after six
days; recovery/expiry cancels remaining rows and missed old windows are not sent
as a burst. Personal and Space Pro reminders are push + in-app to the payer
account's primary profile; email remains auth-only.

During scheduled cancellation or grace the primary ID reserves one slot and,
when an eligible secondary exists, the user selects the exact pair with one
distinct owned, non-deleted profile not disabled for another reason, but no
profile freezes before final expiry. At personal expiry User applies that pair
even without an online client; if it is missing/invalid, the deterministic
fallback is primary plus the earliest-created eligible secondary. With no
eligible secondary, only the primary slot is selected and no unrelated disabled
state is revived; every other non-deleted profile receives an independent
subscription-freeze overlay so a later moderation-unfreeze cannot exceed the
limit. The choice
is bound to a stable downgrade cycle, so a stale choice cannot refreeze after
renewal. Subscription-owned freeze state is
separate from deletion/moderation state, so activation/recovery unfreezes every
and only subscription-frozen profile.

File retention is account-wide across profiles and applies to exact references,
not a shared blob. Each reference stores immutable `retention_account_id`; a
forward/reuse reference belongs to its acquiring account independently of binary
dedup. A free non-E2E reference starts at `created_at + P90D`;
Premium/grace clears the deadline; downgrade gives every still-live non-E2E
reference `downgrade_effective_at + P90D` instead of deleting old Premium data
immediately. E2E remains `created_at + P90D` in every tier. Renewal clears only a
live entitlement-created deadline and never resurrects expired/GC data. Space
Pro uses the same seven-day payment-failure grace. At final expiry existing
members/resources are not deleted or kicked; new growth is denied while a free
cap is exceeded.

Voice keeps a revisioned Space entitlement projection for its 32/128 room cap;
personal paid quality comes only from Auth's short-lived trusted entitlement
claim. Both carry/fail closed at `entitled_until`, so stale grace/expiry delivery
cannot preserve paid admission or quality forever. At that boundary Subscription
alone owns `ResolveEntitlementAtBoundary(minimum_revision=observed_revision)`;
timeout/unavailable denies only the paid uplift for the current decision and
triggers reconciliation.

Account deletion immediately makes personal entitlement inactive and schedules
idempotent provider auto-renew cancellation; provider outage cannot restore local
access. Explicit restore inside the canonical 30-day window may expose only
provider-verified paid time with `cancel_at_period_end=true`; it never silently
resumes recurring billing. Space Pro purchased by the deleting account is not
transferred: renewal/reminders stop, while the Space keeps verified already-paid
caps only until period end. At `P30D` each service independently erases raw
account/purchaser IDs by an opaque deletion fence; receipts are evidence, not a
gate, and Subscription leaves only a purpose-scoped HMAC replay tombstone. A
still-paid Space remains keyed by `space_id` with
`purchaser_deleted=true`, so its snapshots and period-end expiry need no raw payer
ID. Live merchant activation also requires a provider
auto-renew-cancel path. If its receipt is still pending at purge, only an
unlinked opaque cancellation handle survives in a restricted escrow; privacy
erasure proceeds, and the handle is crypto-shredded after terminal receipt.

## Federated ноды

Владелец federated ноды устанавливает **собственные лимиты** на своих спейсах (размер файлов, участников в войсе и т.д.) — независимо от ограничений глобального сервера.
