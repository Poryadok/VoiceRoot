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

## В header chat room

Приоритет отображения subtitle (DM) — **первое подходящее**:

1. **online** — «в сети»
2. **in call** — «В звонке» (без указания какого чата); **не скрывает** online dot на avatar — subtitle может комбинировать «в сети · в звонке» если оба true
3. **custom status** — текст + эмодзи (★ Plus)
4. **last seen** — «был(а) … назад» (если offline/idle и разрешено приватностью)
5. **offline** — без last seen, если скрыто настройками

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
