# Скоуп данных v1 (перед проектированием таблиц)

Документ фиксирует **первую осмысленную итерацию** персистентной модели: что входит в объём схемы БД до детальных таблиц в [microservices/](microservices/) (секция «Модель данных») и миграций. Источник фаз продукта: [PLAN.md](PLAN.md). Общие правила ID, ссылок между БД и полей: [DATA_MODEL.md](DATA_MODEL.md). Инвентарь БД: [DATA_STORES.md](DATA_STORES.md).

---

## 1. Определение «данные v1»

**Данные v1** = всё, что нужно для **Фазы 0 (фундамент)** и **Фазы 1 (MVP: личные сообщения)** из [PLAN.md](PLAN.md): регистрация/логин, один активный профиль на аккаунт (без полноценного мульти-профиля как продукта), друзья и блокировки, DM-чаты, сообщения с **базовым** edit/delete на бэкенде (одна политика; разветвление UX «для всех/себя» и метка «(ред.)» — Фаза 3), отметка прочитанного, базовое presence, профиль с полем аватара (URL объекта в R2 — см. PLAN Фаза 1), Realtime без собственной PostgreSQL.

В этом документе `v1` = **первая волна миграций Фазы 0–1**, а не «целевое состояние сервиса». Поэтому упоминания `*_db` в [MICROSERVICES.md](MICROSERVICES.md) и `docs/microservices/*.md` трактуются как target-state и могут быть частично вне первой волны.

Цель v1 по данным: пользователь с аккаунтом и профилем может состоять в дружбе, иметь DM, отправлять и читать сообщения, видеть список диалогов с превью и непрочитанным — в соответствии с границами «Не входит в Фазу 1» в [PLAN.md](PLAN.md).

---

## 2. Что сознательно не входит в данные v1

Ниже — **не проектируем отдельные таблицы / не разворачиваем отдельные БД** в первой волне миграций (позже — по фазам PLAN и [MICROSERVICES.md](MICROSERVICES.md)):

| Область                                                            | Отложено до (ориентир)                                                                                      |
|--------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------|
| Групповые чаты, спейсы, каналы, роли в спейсах                     | Фазы 4–5                                                                                                    |
| Реакции, треды, пины, пересылка, Markdown, вложения                | Фазы 3–6+ (вложения/typing — 3; реакции/forward — 4; Markdown/пины — 6; треды — 10; см. [PLAN.md](PLAN.md)) |
| Typing indicator                                                   | Фаза 3 (эфемерно в Realtime; отдельная БД не требуется)                                                     |
| Push, Notification Service                                         | FCM Web/Android — [PLAN.md](PLAN.md) Фаза 6; APNs / VoIP / доработка — Фаза 8                               |
| Голос/видео, полноценный File Service, вложения в чате             | Фазы 2–3+ (**аватар профиля в R2** — уже Фаза 1; `file_db` и таблицы вложений — позже)                      |
| Matchmaking, Search (отдельная `search_db`), Analytics, ClickHouse | Фазы 7, 9+                                                                                                  |
| Базовые репорты, 2FA, гранулярная приватность профиля              | Фаза 11                                                                                                     |
| Subscription                                                       | Фаза 12                                                                                                     |
| Мульти-профиль (продукт), верификация значков                      | Фаза 13                                                                                                     |
| Авто-мод, панель модераторов, E2E (DM), боты, федерация, сторис    | Фазы 14–19+                                                                                                 |

Для **Messaging** в v1 достаточно ядра «сообщение в чате» + **read receipts** (или эквивалент для `mark_read` и счётчика непрочитанных). Таблицы `reactions`, `pins`, полноценные треды и вложения — вне v1.

Для **Chat** в v1 — тип **`dm`** и участники; **папки** (`folders`, `folder_chats`) можно отложить до появления UX папок в плане или ввести минимально, если список диалогов без них невозможен — решение фиксируется в [chat-service.md](microservices/chat-service.md#модель-данных).

Для **User** в v1 — **профиль** и базовые поля (в т.ч. URL аватара в R2 и «О себе» — PLAN Фаза 1); **мульти-профиль** как продукт и верификация — **Фаза 13**; расширенные Premium-поля — **Фаза 12**; технически одна строка `profiles` с `is_primary = true` на аккаунт достаточна для Фазы 1.

Для **Social** в v1 — заявки в друзья, дружба, блокировки на уровне **аккаунта** ([social-service.md](microservices/social-service.md)). Полный **контакт-лист / телефонная синхронизация** можно второй очередью в рамках Фазы 1, если продуктово обязателен; иначе — отдельная миграция после ядра.

Решения по спорным зонам из фич:
- **Onboarding**: backend-флаг завершения обязателен для повторяемого UX; минимально включаем в `user_db` (например поле/таблица состояния онбординга).
- **Privacy**: гранулярная модель из `features/privacy.md` остаётся вне первой волны, но нужен минимальный enforceable baseline для DM (например allow-by-default + блокировки из Social, без полной матрицы аудиторий).
- **Verification**: OAuth/DNS-артефакты и cron re-check — вне первой волны; в `user_db` допускается только компактный статус/тип badge как nullable-поля под будущее.
- **Version policy (`/version`)**: source of truth у Gateway (config store/control-plane таблица), но без отдельной service-owned PostgreSQL БД в v1 inventory.

---

## 3. Сервисы и БД в объёме v1

| Сервис                                        | PostgreSQL     | В объёме v1        |
|-----------------------------------------------|----------------|--------------------|
| API Gateway                                   | —              | Нет таблиц         |
| Auth Service                                  | `auth_db`      | Да                 |
| User Service                                  | `user_db`      | Да                 |
| Social Service                                | `social_db`    | Да                 |
| Chat Service                                  | `chat_db`      | Да                 |
| Messaging Service                             | `messaging_db` | Да                 |
| Realtime Service                              | —              | Redis only         |
| Остальные из [DATA_STORES.md](DATA_STORES.md) | соотв. `*_db`  | Нет в первой волне |

---

## 4. Минимальные сущности по сервисам (черновик для ER)

Ниже — **не финальная схема**, а согласование с `docs/microservices/*.md` и сужение под Фазу 1. Детальные колонки, индексы и миграции — в соответствующих карточках сервисов (секция «Модель данных») и в коде миграций.

### 4.1 Auth (`auth_db`)

Соответствие [auth-service.md](microservices/auth-service.md), именование — [DATA_MODEL.md](DATA_MODEL.md), [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md):

- `accounts` — учётная запись, `deleted_at` для soft delete.
- `refresh_tokens` — ссылка на аккаунт колонкой **`account_id`** (не `user_id`).
- `otp_codes` — верификация email / сброс пароля, привязка к `account_id`.

JWT: claim **`user_id`** = `accounts.id` (то же значение, что логическое **`account_id`**).

### 4.2 User (`user_db`)

По [user-service.md](microservices/user-service.md), v1-минимум:

- `profiles` — хотя бы один профиль на аккаунт; `account_id` как UUID без FK на другой кластер ([DATA_MODEL.md](DATA_MODEL.md)).
- **Presence в Фазе 1** по PLAN: онлайн/оффлайн и last seen — допустим **Redis** как в доке User Service; персистентный `last_seen` в PostgreSQL — по необходимости для «последний визит» после перезапуска; зафиксировать выбор в [user-service.md](microservices/user-service.md#модель-данных).
- `onboarding_completed` (или эквивалентное состояние шагов) — минимально включаем в v1, так как это backend-персистентный флаг продуктового поведения.

Таблица **`privacy_settings`** — полная гранулярность из фич вне первой волны; в v1 достаточно базовых правил доступа к DM через Social-блокировки и дефолтную политику.

### 4.3 Social (`social_db`)

По [social-service.md](microservices/social-service.md):

- `friendships` — `pending` / `accepted` / `declined`, связь по **`profile_id`** сторон.
- `blocks` — **`blocker_account_id`**, **`blocked_account_id`** (блок на уровне аккаунта).

Таблица **`contacts`** — опционально для v1 (см. п. 2).

### 4.4 Chat (`chat_db`)

По [chat-service.md](microservices/chat-service.md), сужение:

- `chats` — `type = dm` для MVP; перечисление типов можно заложить в схеме наперёд.
- `chat_members` — связь чат ↔ профиль, уникальность пары `(chat_id, profile_id)`.

### 4.5 Messaging (`messaging_db`)

По [messaging-service.md](microservices/messaging-service.md), v1:

- `messages` — идентификатор, `chat_id`, отправитель `sender_profile_id`, текст, метки времени; **Фаза 1** по PLAN — базовое редактирование и удаление: как минимум `edited_at` и `deleted_at` (soft delete) или согласованный эквивалент; без тредов/реакций/пинов/`forward` в первой миграции (nullable под будущее — решение в [messaging-service.md](microservices/messaging-service.md#модель-данных)).
- `read_receipts` — `chat_id`, `profile_id`, `last_read_message_id` (или согласованный эквивалент для списка диалогов и `mark_read`).

`chat_type` в сообщениях для v1 может быть фиксирован **`dm`** до появления групп и каналов.

### 4.6 Realtime

Без PostgreSQL; Redis Pub/Sub и реестр соединений — вне ER PostgreSQL ([realtime-service.md](microservices/realtime-service.md)).

---

## 5. Проход по фичам (трассировка на сущности v1)

Используется как чеклист «ничего не забыть» при детализации схем в `docs/microservices/*.md` и миграциях:

| Фича (файл)                                           | Затрагиваемые сервисы / БД | Заметки для v1                                                                                                                                                   |
|-------------------------------------------------------|----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| [auth-and-contacts.md](features/auth-and-contacts.md) | Auth, User, Social         | Регистрация/логин → `accounts`, OTP; профиль → `profiles`                                                                                                        |
| [friends.md](features/friends.md)                     | Social, User               | Заявки и друзья → `friendships`; блоки → `blocks`; поиск пользователя для заявки (PLAN Фаза 1) — выдача по `user_db`/API User, не отдельная `search_db` (Фаза 9) |
| [user-profile.md](features/user-profile.md)           | User                       | Отображаемое имя, аватар URL (загрузка файлов — Фаза 3)                                                                                                          |
| [presence.md](features/presence.md)                   | User, Realtime             | v1: упрощённо online/offline + last seen — см. п. 4.2                                                                                                            |
| [text-chat.md](features/text-chat.md)                 | Chat, Messaging, Realtime  | Фаза 1: без Markdown в UI, реакций, тредов, пинов; **edit/delete на API** — базовая политика (PLAN); полнотекст и глобальный поиск — не `search_db`, см. Фаза 9  |
| [navigation.md](features/navigation.md)               | —                          | Терминология UI; на схему не влияет напрямую                                                                                                                     |
| [onboarding.md](features/onboarding.md)               | User                       | Нужен backend-state (`onboarding_completed` или эквивалент), чтобы туториал не сбрасывался после переустановки                                                   |
| [privacy.md](features/privacy.md)                     | User, Social               | Полная матрица аудиторий отложена; в v1 обязательны блокировки и базовый DM-enforcement                                                                          |
| [verification.md](features/verification.md)           | User, Auth                 | Полная OAuth/DNS верификация и re-check вне первой волны; допускается только placeholder-статус на профиле                                                       |
| [deep-links.md](features/deep-links.md)               | Chat, Messaging, User      | Для DM в данных v1 фиксируем адресацию по `profile_id`/`username`, не по alias `user_id(account_id)`                                                             |
| [updates.md](features/updates.md)                     | API Gateway                | `/api/v1/version` обязателен; хранение policy — config-store/control-plane, не отдельная PostgreSQL БД Gateway                                                   |

Фичи вне v1 (спейсы, роли, поиск, MM, уведомления, подписка и т.д.) — не требуют таблиц в первой волне; при появлении событий NATS позже — не смешивать с обязательным набором v1.

---

## 6. События и ссылки между сервисами

- Публикации из Auth / User / Social / Chat / Messaging — по таблицам в соответствующих `microservices/*.md`.
- **Удаление аккаунта** (soft delete в `accounts`): обработка в остальных сервисах (анонимизация, запрет входа, политика данных) должна быть описана в продуктовых/операционных доках; на уровне схемы v1 — везде, где есть `account_id` / `profile_id`, заложить ожидаемое поведение в карточках `docs/microservices/*.md` и миграциях (без FK между БД, см. [DATA_MODEL.md](DATA_MODEL.md)).

---

## 7. Следующие шаги

1. Детализация таблиц — в секциях «Модель данных» [docs/microservices/](microservices/); волна v1 — этот документ (п. 4) + [OPERATIONS.md](OPERATIONS.md) (миграции).
2. При реализации — сверять миграции с [OPERATIONS.md](OPERATIONS.md) (expand/contract, владелец миграций).
3. Сверка с [PLAN.md](PLAN.md) Фаза 0: чеклист уже ссылается на этот скоуп и перечисленные `*_db`; при появлении устаревших имён схем в коде — не смешивать с целевыми миграциями per service.

---

## 8. Матрица v1-only (фикс для генерации DDL)

Чтобы исключить смешение target-state и первой волны, для DDL v1 используется только подмножество ниже:

| Сервис | Таблицы v1 | Что явно не входит в v1 |
|--------|------------|-------------------------|
| Auth (`auth_db`) | `accounts`, `refresh_tokens`, `otp_codes` | — |
| User (`user_db`) | `profiles`, `onboarding_state` | `privacy_settings`, расширенные Premium-настройки |
| Social (`social_db`) | `friendships`, `blocks` | `contacts` |
| Chat (`chat_db`) | `chats` (только `type=dm`), `chat_members` | `folders`, `folder_chats`, сценарии `group/channel` |
| Messaging (`messaging_db`) | `messages`, `read_receipts` | `reactions`, `pins`, `message_attachments`, полноценные треды/forward |

Все детальные типы, CHECK/PK/UNIQUE и v1-индексы фиксируются в карточках:
- [microservices/auth-service.md](microservices/auth-service.md)
- [microservices/user-service.md](microservices/user-service.md)
- [microservices/social-service.md](microservices/social-service.md)
- [microservices/chat-service.md](microservices/chat-service.md)
- [microservices/messaging-service.md](microservices/messaging-service.md)

## 9. Закрытые TBD по первой волне

- `folders` и `folder_chats` в Chat: **вне v1**, отдельная миграция после MVP DM.
- `last_seen`: в v1 хранится в Redis presence, без отдельной персистентной колонки в PostgreSQL.
- Onboarding: в v1 хранится персистентно в `user_db.onboarding_state`.


