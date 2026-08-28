# Presence — статусы присутствия

## Системные статусы

| Статус        | Описание                                                |
|---------------|---------------------------------------------------------|
| Онлайн        | Активен                                                 |
| Не активен    | Приложение открыто, но нет активности более **5 минут** |
| Не беспокоить | DND — уведомления заглушены по настройкам               |
| Невидимый     | Отображается как офлайн для других                      |

## Кастомный статус

- Произвольный текст + эмодзи
- Только по подписке

## Автоопределение игры

- Приложение определяет запущенные игры через список процессов на ПК
- Показывает "Играет в Dota 2" (и т.п.)
- Только на десктопе — на мобайле недоступно
- Можно отключить в настройках

## В войс-чате

- Показывается статус **"В звонке"** без указания какого конкретно чата

## Хранение: presence vs last seen

| Данные | Store | TTL / durability |
|--------|-------|------------------|
| Live status (online / idle / DND / invisible) | Redis (User Service) | ~5 min heartbeat |
| Game / custom status / in-call flag | Redis | Same session |
| **Last seen timestamp** | **PostgreSQL `last_seen_at`** | **Durable** — required for «был N назад»; контракт — [user-service.md](../microservices/user-service.md) |

`send_when_online` (Messaging) consumes **live** presence from User (`GetPresence` / `GetBulkPresence`), not durable last seen alone.

## Interim storage (current code)

Until PostgreSQL `last_seen_at` ships, User Service persists **interim** last activity in Redis:

| Key | TTL | Written when |
|-----|-----|--------------|
| `voice:user:presence:{profile_id}` (hash) | **5 min** | Every heartbeat / `UpdatePresence` / WS activity |
| `voice:user:last_seen:{profile_id}` (string, unix ts) | **30 days** | Same heartbeat — updated on **every** upsert, not only offline transition |

**Read path today:** `GetPresence` / `GetBulkPresence` merge live hash (if exists) with `last_seen` string when session expired. **Gap:** no viewer-aware `show_last_seen` filter; invisible/offline may leak timestamp to unauthorized viewers — [user-service.md](../microservices/user-service.md), [todo/backend.md](../todo/backend.md).

**Target (PG):** on offline transition / graceful disconnect, flush `last_seen_at` to PostgreSQL; Redis string becomes cache only. Rounding table below applies to durable PG value.

## Heartbeat write semantics

Each `UpdatePresence` (client heartbeat ~60 s or WS ping):

1. Refresh session hash fields (`status`, `game`, `custom_status`, `call_info`, `ts_unix`) + **5 min TTL**
2. Set `voice:user:last_seen:{profile_id}` = `ts_unix` with **30 d TTL** (always — including invisible/DND)
3. Publish `user.presence_changed` when **status enum** changes (target spec: include `old_status`, `new_status`, `profile_id`)

**Fan-out filter (spec):** Realtime WS `presence_update` delivers to subscribers who pass **`show_online`** audience check; **`last_seen` timestamp omitted** on wire when viewer fails **`show_last_seen`**. **Invisible** users appear offline to others; Notification Service treats invisible as **offline for push routing** (in-app still delivered) — [notifications.md](../features/notifications.md) § Presence routing. **Code gap:** broadcast may skip privacy filter and omit delta fields — [todo/backend.md](../todo/backend.md).

## В header chat room

Приоритет subtitle (DM) — **первое подходящее** по таблице (не «только online» при одновременном in-call):

| Условие | Subtitle (RU baseline) |
|---------|------------------------|
| **online ∧ in_call** | «в сети · в звонке» — **combined**, не одно из двух |
| online only | «в сети» |
| in_call only | «В звонке» (без указания какого чата) |
| custom status (★ Plus) | текст + эмодзи |
| offline / idle + `show_last_seen` allowed | «был(а) … назад» (rounding — таблица ниже) |
| **idle** (live session, no activity >5 min) | «Не активен» **или** last-seen row above if product prefers timestamp over idle label — client maps `status=idle` |
| offline + last seen hidden | без времени |

**GRP / CH:** subtitle в §3.1 #2 — member count / channel topic, **не** эта таблица presence.

**Avatar dot:** live online indicator на avatar **независим** от subtitle wording; in-call **не** заменяет online dot. Design inventory — [screen-controls.md](../design/screen-controls.md) §3.1 #2; feature cross-ref — [text-chat.md](text-chat.md) § «Header subtitle (DM)».

### Privacy: `show_last_seen`

Поле **`show_last_seen`** (`PrivacyAudience` в `PrivacySettings`, User Service) — кто видит durable `last_seen_at` и subtitle «был(а) … назад» в header / profile.

| Значение аудитории | Поведение для зрителя |
|--------------------|----------------------|
| Разрешено (friend / all / …) | Subtitle last-seen по таблице rounding ниже |
| Запрещено | Subtitle **offline** без времени; **не** раскрывать фактический `last_seen_at` |

**Enforcement:** User Service `GetBulkPresence` / `GetPresence` **фильтрует** `last_seen` на read path по `(viewer_profile_id, target_profile_id)` + Social friends/contacts + `show_last_seen` audience. Live online status использует отдельное поле `show_online`. Header chat room **не** обходит фильтр.

См. [privacy.md](privacy.md) § «Видимость данных» (поле `show_last_seen` — normative; добавляется в `PrivacySettings` proto вместе с PG `last_seen_at`).

### Last seen rounding

| Δt since `last_seen_at` | Display (RU baseline) |
|-------------------------|------------------------|
| < 1 min | «был(а) только что» |
| 1–59 min | «был(а) N мин. назад» |
| 1–23 h | «был(а) N ч. назад» |
| 1–6 d | «был(а) N дн. назад» |
| > 7 d | дата («был(а) 12 авг.») — i18n plurals по локали |

## Последний раз онлайн

- Показывается "был(а) N минут/часов назад" из **durable** `last_seen_at`
- Видимость — **`show_last_seen`** privacy audience ([privacy.md](privacy.md)); enforcement на read в User Service (см. § «Privacy: show_last_seen» выше)

## Видимость статуса

Настройка приватности — отдельная для каждого типа данных:
- Все
- Только друзья
- Никто

## События

`user.presence_changed` публикуется при смене status; target payload includes `old_status`, `new_status` — [user-service.md](../microservices/user-service.md). Realtime live fan-out to friends — **partial** in code ([todo/backend.md](../todo/backend.md)).
