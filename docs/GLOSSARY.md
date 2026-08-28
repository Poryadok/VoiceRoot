# Глоссарий Voice

Единое место для **определений терминов** в документации продукта и бэкенда. Если в тексте встречается слово из домена (чат, канал, спейс, аккаунт и т.д.) — **сначала смотри сюда**; в фичах и микросервисах достаточно краткой отсылки «см. [GLOSSARY.md](GLOSSARY.md)».

**Правило сопровождения:** новый или изменённый термин фиксируется здесь; длинные продуктовые сценарии остаются в `docs/features/*`, правила ID и таблиц — в [DATA_MODEL.md](DATA_MODEL.md).

---

## Индекс (куда прыгнуть)

| Термин                              | Раздел                                              |
|-------------------------------------|-----------------------------------------------------|
| Чат, личный чат, групповой чат      | [Чаты и спейсы](#чаты-и-спейсы)                     |
| Группа и канал (текст)               | [Группа и канал (текст)](#группа-и-канал-текст)     |
| Голосовой групповой чат             | [Голосовой групповой чат](#голосовой-групповой-чат) |
| Матч-отряд                          | [Матч-отряд (matchmaking)](#матч-отряд-matchmaking) |
| Спейс                               | [Спейс](#спейс)                                     |
| Сводка имён в коде/БД               | [Продукт → техника](#продукт--техника)              |
| Права (bitmask, scopes бота)        | [microservices/role-service.md](microservices/role-service.md) — раздел «Идентификаторы прав» |
| Аккаунт, профиль                    | [Идентичность](#идентичность)                       |
| Архив чата, Quick Access, Папки чатов, Pin чата, Pin элемента дерева, Active strip | [Организация чатов в UI](#организация-чатов-в-ui)   |
| Запросы сообщений                   | [Запросы сообщений](#запросы-сообщений)             |
| Pin сообщения                       | [Pin сообщения](#pin-сообщения)                     |
| Sticker pack, installed pack, GIF provider | [Стикеры и GIF](#стикеры-и-gif)              |
| Стори                               | [Стори](#стори)                                     |

---

## Организация чатов в UI

Три разные сущности — не путать с **Friends → Favourites** (люди, см. [friends.md](features/friends.md)):

### Архив чата

**Архив чата** — per-profile флаг `is_archived` на `chat_members`; чат скрыт из основного списка, folder filters и Quick Access; история **не** удаляется.

| Scope | Поведение |
|-------|-----------|
| **DM** | `ArchiveChat` RPC; `ListChats` с `inbox=archive` |
| **Group / channel / space chat** | тот же флаг `is_archived` на `chat_members`; UI row action и `inbox=archive` |

**Primary entry (product decision):** ПКМ на **аватар профиля** в rail (ProfileStack) → «Архив» → `Screen/Chat/Archive`. Discoverability ниже, чем у отдельной папки в rail — принято; Saved Messages **не** в продукте. Ctx «Архивировать» на строке чата — **secondary shortcut** для **всех** типов чата (DM, group, channel, space-attached) — те же правила membership, что у mute/delete; не DM-only.

**Unarchive (state machine):**

| Событие | Поведение |
|---------|-----------|
| Ctx «Разархивировать» / swipe на экране архива | `is_archived=false`; чат возвращается в folder «Все» и matching system folders; **не** восстанавливается в Quick Access автоматически |
| **Новое входящее сообщение** (DM) | Auto-unarchive: `is_archived=false`, чат появляется в main inbox с unread; push по обычным правилам |
| **Новое входящее** (group / channel / space chat) | **Не** auto-unarchive; только ctx «Разархивировать» / swipe на экране архива |
| Исходящее сообщение архиватором | **Не** unarchive (как Telegram) |
| Folder membership / folder pin | Сохраняются в БД; пока archived — строка не показывается; после unarchive pin order восстанавливается |
| Quick Access | При archive — **удаление** из `quick_access_chats`; после unarchive — **не** auto-restore |

См. [text-chat.md](features/text-chat.md) § «Архивирование», [chat-service.md](microservices/chat-service.md) § Archive.

### Quick Access (избранное чатов)

**Quick Access** — до **15** записей **`chat_id`** на `profile_id` в rail; быстрый переход без смены folder filter. Target = **только `chat_id`**, не polymorphic space/tree node. **≠ Friends → Favourites** (люди, не чаты). При лимите 15/15 — **replace picker** (список слотов → выбор замены → atomic `RemoveQuickAccess` + `AddQuickAccess`; `FAILED_PRECONDITION` на голый `AddQuickAccess` — server safety net, не UX ошибка). Добавление: ctx «В избранное» / drag. Отдельная таблица и RPC (`List/Add/Remove/ReorderQuickAccess`) — не pin в папке. Saved Messages **не** в продукте. См. [navigation.md](features/navigation.md) § Quick Access, [screen-controls.md](design/screen-controls.md) §1.1c, [chat-service.md](microservices/chat-service.md) § Quick Access.

### Папки чатов

**Папки чатов** — фильтр списка чатов в rail (desktop) / drawer (mobile). **System** (Все, ЛС, Группы, Каналы, Спейсы): immutable, predicate в `filter_config_json`, pin overlay без `folder_chats` membership. **Custom**: explicit `folder_chats` + pin/order. **Virtual «Запросы»** — `inbox=requests`, в rail/drawer, не segmented toggle в middle column. Archived чаты excluded из всех folder filters. См. [navigation.md](features/navigation.md) § «Папки по умолчанию», [screen-controls.md](design/screen-controls.md) §1.1b.

### Active strip (mobile)

**Active strip** — горизонтальная полоска **opened-chat LRU** над room на mobile; **не** inbox preview rows. Источник — client session state + unread badges. Канон контролов — [screen-controls.md](design/screen-controls.md) §1.6; stacking — §1.6a; [navigation.md](features/navigation.md) § «Active strip».

| Правило | Поведение |
|---------|-----------|
| **Membership** | Чат попадает в strip после **open** на mobile |
| **Limit** | Max **100** opened chats; при 101-м — evict oldest + feedback (§1.6 #4) |
| **Visible cap** | ~**8** avatars; horizontal scroll |
| **Unread badge** | На иконках strip |
| **Remove** | Long-press icon → × (§1.6 #5); back to list — **remove from strip** (normative, including unread). Unread retention on back — DEFERRED (AUDIT R3-03-A09) |
| **Keyboard** | Strip **скрывается** (§1.6a) |

### Pin чата

**Pin чата** — закреп **`chat_id` внутри выбранной inbox-папки** `(profile_id, folder_id)` через `folder_chats.is_pinned` / `pin_order`; иконка pin на **list row**. Отдельно от Quick Access и от **pin элемента дерева спейса**. System folders: pin overlay на отфильтрованном списке; custom folders: pin внутри explicit membership. См. [navigation.md](features/navigation.md) § «Pin чатов», [chat-service.md](microservices/chat-service.md) § Folders.

### Pin элемента дерева

**Pin элемента дерева** — закреп **узла** `space_tree_nodes` (text chat или voice room) вверху категории/корня sidebar спейса; Space Service (`is_pinned`, `PinTreeNode`). **≠ Pin чата** (inbox folder) **≠ Quick Access** (rail `chat_id`). См. [spaces.md](features/spaces.md) § «Pin элемента дерева спейса», [navigation.md](features/navigation.md) § «Pin чатов».

### Запросы сообщений

**Запросы сообщений** — DM от **незнакомца** (не friend, не contact), попадающий в `chat_members.inbox_bucket=requests` у получателя. **UI entry:** virtual folder «Запросы» в **rail/drawer** ([screen-controls.md](design/screen-controls.md) §1.1b #5; visible when pending requests exist); `ListChats` с `inbox=requests`. **Запрещён** segmented toggle main/requests в middle column (§1.3 tombstone). Не путать с **Friends → Pending** (заявки в друзья).

| Bucket | Смысл |
|--------|-------|
| `main` | Обычный inbox |
| `requests` | Ожидает Accept/Decline |
| `declined` | Скрыт после Decline; re-contact → снова `requests` при новом сообщении |

RPC: `AcceptDMRequest`, `DeclineDMRequest` (Chat Service). См. [text-chat.md](features/text-chat.md) § «Запросы сообщений», [friends.md](features/friends.md) § «Незнакомец пишет».

---

## Стикеры и GIF

Термины composer **😊 panel** (§3.6b) — не 📎 attach menu. Wire contract — [messaging-service.md](microservices/messaging-service.md) § Stickers and GIF; catalog DDL — [chat-service.md](microservices/chat-service.md) § Sticker packs.

### Sticker pack (стикер-пак)

**Sticker pack** — набор стикеров с метаданными (`sticker_packs` + `stickers` в **Chat** `chat_db`). Бинарники — **File Service** (`intent=sticker`); отправка — **Messaging** `content_type=STICKER`. **System pack** (`is_system=true`) — предустановленный пак приложения; **user pack** — создан пользователем через Settings §37.

### Installed sticker pack (установленный пак)

**Installed sticker pack** — связь `profile_installed_packs` (profile ↔ pack + `sort_order` в composer rail). **Install** — `InstallStickerPack`; **uninstall** — только для **user-created** паков; system packs **не удаляются**, только reorder в rail. Uninstall **не** удаляет уже отправленные сообщения.

### GIF provider

**GIF provider** — внешний каталог (Giphy **или** Tenor — один на deploy). Поиск — **Chat** `SearchGifs` / `GetTrendingGifs`; импорт байтов — **File** `RequestUpload(intent=gif, source_url=…)`; send — **Messaging** `content_type=GIF`. Pagination cursor (`next_cursor`) — **только** в `SearchGifsResponse` (Chat); File/Messaging не дублируют.

### Sticker store (опционально)

**Sticker store** — UI browse каталога (#6–7 в [screen-controls.md](design/screen-controls.md) §37). **Optional post-core** — MVP обходится system packs + user upload; Premium ★ install gate — [subscription.md](features/subscription.md).

### GIF / emoji recents

| Surface | Storage | Owner |
|---------|---------|-------|
| **Emoji recents** | Client-local (device) | Flutter |
| **Sticker recents** (shortcut row) | Client-local; optional `stickers.emoji` hint | Flutter |
| **GIF recents** | Client-local (recent `file_id` / `provider_id` after send) | Flutter |
| **GIF trending** | Server `GetTrendingGifs` (Chat cache) | Chat Service |

Server **не** синхронизирует per-profile GIF recents cross-device в MVP.

---

## Pin сообщения

**Pin сообщения** — закреплённое сообщение в комнате чата; до **5** pins на чат (как Telegram). Persistent bar под header + список всех pins. Право: `TEXT_CHAT_PIN_MESSAGES`. См. [text-chat.md](features/text-chat.md) § «Закреплённые сообщения».

---

## Стори

**Стори** — короткий контент (фото / видео / текст), лента 24 часа, личный архив и **highlights** на профиле; модерация и репорты — в общем контуре [reports.md](features/reports.md). Реализация: [Story Service](microservices/story-service.md), медиа — [File Service](microservices/file-service.md); продукт — [stories.md](features/stories.md).

---

## Чаты и спейсы

### Чат

**Чат** — общее обозначение места для общения (абстракция). В коде и API всегда уточняется типом (личный, группа, канал и т.д.).

### Личный чат

**Личный чат** — общение **один на один** (два участника). Не входит в состав спейса как его «содержимое»; в UI обычно отдельный список (ЛС).

### Групповой чат

**Групповой чат** — место для общения **нескольких** участников (не 1:1). Делится на:

| Вид                         | Смысл                                                                                                                                                                                                                     |
|-----------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Текстовый групповой чат** | Поток **текстовых** сообщений (группа или канал — см. ниже)                                                                                                                                                               |
| **Голосовая комната**       | Совместный голос (и опционально видео): в **спейсе** — постоянные голосовые комнаты; у **текстовой группы** — при необходимости **временная** комната, привязанная к группе — см. [voice-chat.md](features/voice-chat.md) |

---

## Группа и канал (текст)

**Группа** (`group`) и **канал** (`channel`) — один и тот же класс сущности в **Chat** / **Messaging**: строка `chat_db.chats`, сообщения в `messaging_db` с тем же `chat_id`. Поле `type` влияет на **дефолтные настройки** и пресеты ролей; **смена `channel` ↔ `group`** должна сохранять работоспособность (меняются дефолты и правила UI, не «ломается» хранилище).

**Единственное жёсткое продуктовое различие по умолчанию:** в **канале** в **основную ленту** нельзя писать **от имени пользователя** (только от имени чата / по сценарию тредов — см. настройки); в **группе** по умолчанию в ленту пишут участники от своего имени. Всё остальное (треды, «официальные» посты от имени чата, `@here` / `@everyone`, видимость) — **настройки чата и роли**.

**Членство:** у чата **без** `space_id` состав хранится в **`chat_db.chat_members`**. У чата **с** `space_id` участники спейса **наследуются** из **`space_members`**; спейс через **Role** может сужать доступ к конкретным чатам и голосовым комнатам оверрайдами по `chat_id` / `voice_room_id`.

**Дерево спейса (sidebar):** текстовые чаты (`group` \| `channel`) и **голосовые комнаты** упорядочиваются в **одном** слое — **`space_db.space_tree_nodes`** (`kind`, `category_id`, `sort_order`; для текста — флаг `is_system`).

|                    | **Группа** (`group`)                         | **Канал** (`channel`)                          |
|--------------------|----------------------------------------------|-----------------------------------------------|
| **Дефолт ленты**   | сообщения участников от своего имени       | в основную ленту — не от имени пользователя |
| **Где в БД**       | `chat_db.chats`, `messaging_db.chat_type`    | то же                                         |
| **Messaging**      | `chat_type = group`, `chat_id = chats.id`   | `chat_type = channel`, `chat_id = chats.id`   |

---

## Голосовой групповой чат

**Голос в спейсе** — **голосовые комнаты** в `space_db.voice_rooms`, сессии Voice Service / LiveKit. Все подключённые участники общаются **от своего имени**; доступ к комнатам — **роли и оверрайды** (как к текстовым чатам) — см. [voice-chat.md](features/voice-chat.md).

**Голос у текстовой группы** — по сценарию «созвониться пати»: **временная** голосовая комната, **привязанная** к `chat_id` группы (не отдельный продуктовый тип «канал»).

Текстовые сообщения в Messaging к голосовой комнате спейса по умолчанию **не** привязываются к тому же `chat_id`, что у голоса (если нет отдельной фичи side-chat).

---

## Матч-отряд (matchmaking)

**Матч-отряд** — временный контур общения, создаваемый при успешном матче в Matchmaking: голос + связанный текстовый чат на время сессии. В продуктовых сценариях используем термин «матч-отряд», чтобы не путать его с постоянной **голосовой комнатой** спейса.

В технических полях остаются существующие идентификаторы: `voice_room_id` (голос) и `chat_id` (текст).

---

## Спейс

**Спейс** — контейнер **только для групповых чатов**: текстовые **группы** и **каналы** (строки `chats` + узлы **`space_tree_nodes`**), **голосовые комнаты** (`voice_rooms` + узлы **`space_tree_nodes`**) — в любой комбинации; публичный или приватный по каталогу — см. [spaces.md](features/spaces.md). Standalone группа или канал **вне спейса** — `space_id` пустой, без узла в `space_tree_nodes`.

**Личные чаты не являются содержимым спейса.** Модели «спейс из одних только ЛС» нет — личные чаты живут снаружи этой иерархии.

---

## Продукт → техника

| Продуктовый термин                   | `messaging_db.messages.chat_type` (если есть текст) | Первичная сущность в БД                                                        |
|--------------------------------------|-----------------------------------------------------|--------------------------------------------------------------------------------|
| Личный чат                           | `dm`                                                | `chat_db.chats`                                                                |
| Текстовая группа (вне или в спейсе)  | `group`                                             | `chat_db.chats`; вне спейса — `chat_members`; в спейсе — `space_id` + `space_tree_nodes` (`kind=text_chat`), участники через `space_members` + Role |
| Текстовый канал (вне или в спейсе)   | `channel`                                           | то же                                                                         |
| Голосовая комната в спейсе           | — (не Messaging)                                    | `space_db.voice_rooms` + `space_tree_nodes` (`kind=voice_room`) + Voice        |
| Временная голосовая комната у группы | — (не Messaging)                                    | Voice-сессия / LiveKit room, связь с `chat_db.chats` (`type = group`)          |
| Матч-отряд (MM)                      | — (не Messaging)                                    | Временная связка `voice_room_id` + `chat_id` в `matchmaking_db.matches`         |

Детали ссылок между сервисами без FK: [DATA_MODEL.md](DATA_MODEL.md) §2.

---

## Идентичность

| Термин      | Кратко                                                                  | Подробности                                                                      |
|-------------|-------------------------------------------------------------------------|----------------------------------------------------------------------------------|
| **Аккаунт** | Учётная запись для входа (`account_id` / `user_id` в JWT).              | [DATA_MODEL.md](DATA_MODEL.md) §1, `auth_db.accounts`                            |
| **Профиль** | Публичная «личность» пользователя (`profile_id` в JWT), мульти-профиль. | [DATA_MODEL.md](DATA_MODEL.md) §1, [multi-profile.md](features/multi-profile.md) |
| **Блокировка (Social)** | Запись в `social_db.blocks` между **аккаунтами** (`blocker_account_id`, `blocked_account_id`); действует на все профили заблокированной стороны. В gRPC: `BlockAccount` / `UnblockAccount`, поле тела `blocked_account_id`. | [social-service.md](microservices/social-service.md), контракт [social.proto](../protos/voice/social/v1/social.proto) |

---

## Связанные документы

| Документ                                | Зачем                                                                   |
|-----------------------------------------|-------------------------------------------------------------------------|
| [DATA_MODEL.md](DATA_MODEL.md)          | UUID, `chat_id` / текстовый канал / голос, soft delete, ссылки между БД |
| [DATA_STORES.md](DATA_STORES.md)        | Какая БД у какого сервиса                                               |
| [navigation.md](features/navigation.md) | Термины в UI/коде навигации                                             |
| [screen-controls.md](design/screen-controls.md) | Shell §1 — rail, folders, Quick Access, mobile strip, Archive   |
| [chat-service.md](microservices/chat-service.md) | Folders, Quick Access, Archive RPCs                          |
| [FEATURES.md](FEATURES.md)              | Каталог фич                                                             |
