# Text Chat — текстовый чат

## Сообщения

- **Редактирование**: да, лимит 100 лет (фактически без ограничений)
- **Удаление**: да, у всех, без лимита по времени
- **Форматирование**: markdown-подмножество (см. § «Markdown и formatting menu»)

## Треды и ответы

**Группа** и **канал** — одна техническая модель чата; отличаются **дефолтными настройками** (и пресетами ролей). Треды, запрет писать в основную ленту от пользователя, «официальные» посты от имени чата — всё задаётся настройками и правами, а не жёстко типом в БД.

| Тип чата   | Режим (дефолты; всё переопределяется настройками)                       |
|------------|------------------------------------------------------------------------|
| DM (личка) | только reply на конкретное сообщение                                   |
| Канал      | по умолчанию тред-ориентированная лента; в основную ленту не от имени пользователя |
| Группа     | по умолчанию треды выкл./вкл. в настройках; в ленту — от имени участников |

## Медиа и вложения

- Файлы и изображения: да — лимиты и хранилище см. [file-storage.md](file-storage.md)
- **Inline URL link preview** — URL в теле текста; клиент запрашивает OG/metadata (HTTPS-only, sanitized HTML); `content_type=TEXT` + optional `link_preview` attachment metadata; вкладка Shared Media **Links** — см. § «Article vs inline URL»
- **Article** (attach menu) — отдельный rich payload / instant view; `content_type=ARTICLE`; server-side metadata fetch (Messaging worker, не client CORS) — см. § «Article vs inline URL»
- Голосовые сообщения: да (аудиофайл + встроенный плеер); wire: `MESSAGE_CONTENT_TYPE_VOICE` / kind `voice_message`
- **Video note** (круглое видео): да — короткая запись до N сек, inline player
- GIF: да — first-class сообщение в чате (поиск/отправка GIF; не только generic file attach). Провайдер и wire-shape — в реализации Messaging/Chat; Premium **GIF-аватар** — отдельно в [user-profile.md](user-profile.md) / [subscription.md](subscription.md)
- Стикеры: системные паки + пользовательские (загрузка своих паков); отправка/приём как first-class сообщение; picker в composer. Sticker store / premium ★ packs — опциональное расширение после ядра паков. Live TC-MSG-09 — [ADR 005](../adr/005-rich-media-live-tests-deferred.md)
- Emoji: да

### Attach menu (composer)

Telegram-parity popup из 📎:

| Пункт | Тип сообщения | Сервис |
|-------|---------------|--------|
| Photo or video | existing media | Messaging + File |
| Document | file attachment | File |
| Article | rich article / instant view payload | Messaging (новый `content_type`) |
| Location | lat/lon + optional label + static map preview | Messaging + client map picker |
| Music | audio file + metadata | File |

**Wallet** — **не в продукте**, не планируется.

### Article vs inline URL

| | **Inline URL** (в тексте) | **Article** (attach menu) |
|--|---------------------------|---------------------------|
| **Trigger** | URL regex в `content` | Явный выбор «Article» в attach popup |
| **`content_type`** | `TEXT` (+ link metadata) | `ARTICLE` |
| **Metadata fetch** | **Клиент** — OG fetch с sanitization; fallback: title из `<title>` / hostname | **Messaging worker** — server-side HTTPS fetch, HTML sanitization; payload `{ url, title, description, thumb_file_id?, instant_view_html? }` |
| **Preview UI** | Card под bubble (thumb + title + domain) | Instant-view / expanded article card |
| **Shared Media tab** | **Links** | **Links** (label «Article» в list preview) |
| **Search in-chat** | URL substring в теле | Title/description в article payload |

См. content-type → tab mapping: [search.md](search.md) § «Фильтры shared media».

## Социальные механики

- Реакции на сообщения: да — chips под bubble, toggle; long-press → who reacted; **на mobile видимы без hover** (не только по наведению)
  - **Free:** одна реакция на пользователя на сообщение (toggle — смена emoji снимает предыдущую)
  - **Premium (★):** до **3** реакций на пользователя на одно сообщение; при попытке 4-й — upsell prompt
  - Entitlement: personal Premium на `profile_id`; enforcement — Messaging `AddReaction` / `RemoveReaction`
- @упоминания: да — событие `mention { user_id, message_id, chat_id }` по WebSocket всем подключённым устройствам; если офлайн — FCM/APNs push
- **@here** — упоминание только онлайн-участников **текстового чата** (группа или канал); требуется право `TEXT_CHAT_MENTION_ALL_ONLINE` ([роли](roles.md), [role-service.md](../microservices/role-service.md))
- **@everyone** — упоминание всех участников **текстового чата**; требуется `TEXT_CHAT_MENTION_ALL_IN_CHAT`
- Закреплённые сообщения: да, как в Telegram — см. § «Закреплённые сообщения»

### Ссылки в тексте

- URL в теле сообщения рендерятся **синим** (`color.link` / design token), underline on hover
- Отличие от spoiler (`||…||`) и inline code (`` `…` ``) — см. [brand.md](../design/brand.md) § Messaging UX

## Закреплённые сообщения

- Лимит: до **5** pins на чат (как Telegram)
- Права: `TEXT_CHAT_PIN_MESSAGES` ([roles.md](roles.md))
- **Persistent pinned bar** под header комнаты: thumb + label + тип («Photo» / текст snippet); tap → jump к сообщению; swipe или × → список всех pins (popover / panel)
- Bar не блокирует composer

## История и поиск

- История: бесконечная (лимит не установлен на старте, пересмотреть по нагрузке)
- **Поиск из чата** → поиск по тексту и ссылкам внутри этого чата
- **Поиск с главного экрана** → глобальный поиск по всем чатам
- У каждого чата есть раздел shared media: вкладки **Медиа / Стикеры / Файлы / Ссылки / Голосовые** — канон в [search.md](search.md) § «Фильтры shared media»

## Preview последнего сообщения в списке

Семантика preview в `ListChats` / строке списка — **два слоя**:

| Слой | Источник | Поля |
|------|----------|------|
| **Server DTO** | Messaging S2S `GetChatListMetadata` → Chat `ChatListItem` | `last_message_preview` (text или media label), `last_message_content_type`, `last_message_is_outgoing`, `last_message_delivery_state` (DM), `unread_count`, `last_message_at` |
| **Client overlay** | Локальный draft (SQLite/Hive) | Префикс `Черновик: …` **перекрывает** server preview для этого `chat_id` на этом устройстве; не уходит на сервер |

**Precedence на клиенте:** если есть локальный draft → показать draft; иначе server DTO. Delivery ticks и media labels — **только из server DTO** (не выводить из bubble state открытого чата).

| Случай | Отображение (server label) |
|--------|----------------------------|
| Текст | обрезанный текст последнего сообщения |
| Черновик | `Черновик: …` (локально на устройстве) |
| DM исходящие | `✓` delivered / `✓✓` read **в preview** (не только в bubble) |
| Photo | `Photo` |
| Video | `Video` |
| Voice note | `Voice` |
| File / Document | `File` |
| Sticker | `Sticker` |
| GIF | `GIF` |
| Article | `Article` |
| Location | `Location` |
| Music | `Music` |
| Video note | `Video message` |
| Missed call | `Missed call` |
| Call / Video call | `Call` / `Video call` (system preview) |

Bold title + unread badge — без изменений.

## Статусы доставки

- Как в Telegram: одна галочка = доставлено, две = прочитано
- **DM only** — delivery ticks в list preview и bubble
- **Группы / каналы** — **счётчик просмотров** на сообщении (не delivery ticks); см. § «View count»

### View count (group / channel)

| Правило | Контракт |
|---------|----------|
| **Uniqueness** | Один `profile_id` = **не более одного** view на `message_id` (dedup on first qualifying view) |
| **Qualifying view** | Участник открыл чат и message bubble entered viewport **или** explicit read cursor passed message (Messaging `RecordMessageView`) |
| **Visibility** | Виден **всем участникам** чата на bubble (как Telegram); не скрывается privacy last-seen |
| **Recount after delete** | Удаление сообщения → views удаляются вместе с message row |
| **Account delete** | Soft-deleted profile excluded from recount; periodic compaction job |
| **DM** | View count **не показывается** — только ✓/✓✓ delivery |

## Markdown и formatting menu

Поддерживаемый markdown (render + send):

| Синтаксис | Formatting submenu (ПКМ / toolbar) | Примечание |
|-----------|-----------------------------------|------------|
| `**bold**` | Bold | ✓ |
| `*italic*` | Italic | ✓ |
| `__underline__` | Underline | ✓ |
| `~~strike~~` | Strikethrough | ✓ |
| `` `code` `` / ` ```block``` ` | Monospace | code block — только markdown / paste |
| `\|\|spoiler\|\|` | Spoiler | ✓ |
| `> quote` | Quote | ✓ |
| `[text](url)` | Create link | ✓ |
| `# / ## / ###` headings | — | **Markdown-only** — не в submenu |
| `- / 1.` lists | — | **Markdown-only** — не в submenu |

Desktop optional format toolbar дублирует submenu items (не headings/lists).

## Хранение

- Все сообщения и файлы хранятся на сервере
- Детали файлового хранилища — [file-storage.md](file-storage.md)

## Лимиты

- Максимальная длина сообщения: **4000 символов** (ориентир: у конкурентов 2k–4k)

## Composer (Telegram parity)

### Layout (desktop)

```
[📎] [········ input ········] [😊] [🎤|📹|➤]
```

- **📎 (attach):** hover (H) / tap (V) → popup: Photo or video, Document, Article, Location, Music — см. § Attach menu
- **😊:** hover/tap → panel **Emoji | Stickers | GIFs** (tabs, search, recents, pack rail) — синхронизировано с § «Медиа и вложения»
- **Правая кнопка:** пустой input → **mic** (voice note); long-press/toggle → **video note**; с текстом → **Send**
- **Long-press Send:** Send without sound, Schedule message, Send when online — см. § Send options

### Text selection / formatting (ПКМ в composer)

Контекстное меню как TG: Cut / Copy / Paste + **Formatting** submenu — mapping на markdown из § «Markdown и formatting menu».

Desktop: **format toolbar** над composer **опционально** дублирует submenu (как TG desktop — можно скрыть, если есть ПКМ).

| Опция | Wire / RPC | Поведение |
|-------|------------|-----------|
| **Send without sound** | `send_silent=true` на `SendMessageRequest`; propagates в `message.sent.send_silent` | Push без звука; in-app badge/unread как обычно — [notifications.md](notifications.md) § «Send without sound» |
| **Schedule message** | `scheduled_at` на `SendMessageRequest`; row в `scheduled_messages` | Strip над composer; **edit** через `UpdateScheduledMessage` (только `status=pending`); cancel / send-now — см. [messaging-service.md](../microservices/messaging-service.md) § Scheduled messages |
| **Send when online** | `send_when_online=true` | Queued до presence `online` у получателя (DM); invisible/offline у получателя — очередь держится; отмена до dispatch |

### Side panel (desktop) / sheets (mobile)

Header toggle (`icon.sidePanel`, a11y «Информация о чате») открывает **один** из normative modes в `Panel/Shell/SideHost`:

| Mode | Desktop (SideHost body) | Mobile equivalent |
|------|-------------------------|-------------------|
| **info** | `Panel/Chat/Info` — DM profile summary / group header meta | Bottom sheet или full-screen push |
| **members** | `Panel/Chat/GroupMembers` / `Panel/Space/Members` | Full-screen members list |
| **thread** | Thread message list для выбранного `thread_parent_id` | Full-screen thread view (back → room) |
| **search** | In-chat search results list (paired with §3.2 search bar) | **No side panel** — search bar в app bar + scroll/highlight in room |

Только **один** mode активен; close SideHost → focus return to room ([screen-controls.md](../design/screen-controls.md) §1.9). Reaction/emoji picker — **не** SideHost mode (overlay/popup §3.6b).

### Composer error states

Prefer **inline/banner** у composer; toast — secondary ([brand.md](../design/brand.md)).

| Failure | UI | Recovery |
|---------|-----|----------|
| **Upload network fail** | Banner «Не удалось загрузить» + Retry | Retry upload intent; composer keeps draft |
| **Async processing fail** (`file.processed` error) | Failed attachment chip + «Обработка не удалась» | Remove attachment or retry upload |
| **Quota / file size** | Inline «Файл слишком большой…» | User picks smaller file |
| **Schedule horizon > 365d** | Inline near date picker | Pick valid date |
| **Schedule edit on non-pending** | Toast «Сообщение уже отправлено» | Open sent message in history |
| **Sticker/GIF send fail** | Inline near picker | Retry; provider search remains open |
| **Optimistic send fail** | Bubble → failed state (red icon) | Tap → Retry or Delete local failed row |
| **Rate limit (5/5s)** | Inline «Слишком много сообщений» | Wait + auto-clear |
| **Char limit 4000** | Counter turns red at limit | Trim before send |

### i18n (новые строки)

Preview labels (`Photo`, `Voice`, `Article`, …), last-seen buckets, message-request actions, composer errors — **ARB keys** в `lib/l10n/app_{en,ru}.arb` с ICU plurals для last-seen и unread; не hardcode EN/RU в widgets. Baseline keys: `chat.preview.*`, `chat.lastSeen.*`, `chat.messageRequest.*`, `composer.error.*` — см. [i18n.md](i18n.md).

## Технические решения

- **Протокол**: WebSocket (Realtime), persistent connection; reconnection: exponential backoff (1s → 2s → 4s, cap 30s)
- **Reconnect и догрузка истории** (поле `s` / `resume` в WS и курсор сообщений в Messaging): [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) — раздел «Reconnect: WebSocket-поток и история сообщений»
- **Между инстансами Realtime**: Redis Pub/Sub
- **Typing indicator**: WebSocket, throttle — событие не чаще раза в 3 сек, гасить через 5 сек без обновления
- **Read receipts**: как в Telegram для всех типов чатов — DM: галочки (одна = доставлено, две = прочитано); группы и каналы: счётчик просмотров на сообщении (см. раздел «Статусы доставки» выше)
- **Rate limiting**: 5 сообщений / 5 сек на пользователя (глобально); slow mode для текстовых чатов в спейсе (`group` \| `channel`) — 5 сек – 6 ч, настраивается админом спейса
- **Реакции**: стандартные Unicode + кастомные эмодзи спейса
- **Лимит группы**: 500 участников для обычной группы (не спейса); для больших сообществ — спейс

## Черновики сообщений

- Незаконченные сообщения сохраняются **локально на устройстве** при переходе в другой чат (как в Telegram)
- Хранение: SQLite/Hive в Flutter-клиенте; максимум один черновик на чат
- Серверного хранения нет — черновики не синхронизируются между устройствами
- Черновик автоматически удаляется при отправке сообщения

## Групповые чаты

### Создание группы
- Из DM: кнопка "Добавить участников" → выбор из друзей → группа создана
- Из списка контактов: выбрать нескольких → "Создать группу"
- Минимум 3 участника (если 2 — это просто DM)

### Права
- Создатель группы = владелец; может назначать администраторов
- Администраторы: добавляют/удаляют участников, редактируют настройки группы, удаляют сообщения
- Все участники по умолчанию могут добавлять новых (настраивается владельцем/админом)

### При достижении лимита 500
- Кнопка "Добавить участников" становится неактивной
- Тултип: "Группа заполнена (500/500). Для больших сообществ создайте спейс."

## Direct Messages (DM)

DM — персональный чат 1-на-1. В коде и БД это отдельный тип чата: `dm`.

### Создание DM
- Из профиля пользователя → кнопка "Написать"
- Из поиска — найти пользователя, открыть его профиль
- Из спейса — клик на участника → профиль → "Написать"
- Из списка друзей — как в Telegram

### Доступ и приватность
- Кто может писать в DM — настраивается в [privacy.md](privacy.md) (`allow_dm` audience)
- **Запросы сообщений** — см. § «Запросы сообщений» ниже и [friends.md](friends.md)

### Запросы сообщений

Normative inbox classification для DM (`chat_members.inbox_bucket` per `profile_id`):

| Bucket | Когда | `ListChats` filter | UI |
|--------|-------|-------------------|-----|
| **`main`** | Friend **или** contact **или** после `AcceptDMRequest` **или** инициатор DM | `inbox=main` (default) | Main chat list |
| **`requests`** | Первый DM от **незнакомца** (не friend, не contact); privacy `allow_dm` passed | `inbox=requests` | Virtual folder «Запросы» в rail ([navigation.md](navigation.md)) |
| **`declined`** | Получатель вызвал `DeclineDMRequest` | **Не** в main и **не** в requests | Hidden until re-contact rule |

**На CreateDM / EnsureDM:** инициатор → `main`; получатель → `requests` если stranger, иначе `main` для обоих.

**Accept:** `AcceptDMRequest(chat_id)` → `inbox_bucket=main` для accepter; sender уже в `main`. Оба видят чат в main inbox; `is_stranger=false`.

**Decline:** `DeclineDMRequest(chat_id)` → accepter `inbox_bucket=declined`; чат исчезает из requests. История сообщений **не** удаляется.

**Re-contact после decline:** новое входящее сообщение от того же peer → recipient bucket **возвращается в `requests`** (новый запрос); push/in-app тип `message_request` — [notifications.md](notifications.md).

**Blocked / privacy deny:** `CreateDM` / `SendMessage` → `PermissionDenied` до попадания в requests.

**ListChats fields:** `ChatListItem.inbox`, `is_stranger` (= `inbox_bucket==requests` для row recipient). См. [GLOSSARY.md](../GLOSSARY.md) § «Запросы сообщений».

### Архивирование

**Storage:** `chat_members.is_archived` (Chat Service), per `profile_id`.

| Operation | Контракт |
|-----------|----------|
| **Write** | `ArchiveChat(chat_id, archived)` |
| **Read** | `ListChats` с `inbox=main` (default) — non-archived; `inbox=archive` — только archived (`folder_id` игнорируется) |
| **Scope** | DM, group, channel, space-attached — один флаг `is_archived` и один контракт |

**UX entry (product decision H16):** **Primary** — ПКМ на **аватар профиля** в ProfileStack (rail) → «Архив» → `Screen/Chat/Archive`. **Secondary** — ctx «Архивировать» на строке чата. Отдельной folder «Archive» в rail **нет** (discoverability tradeoff принят). Saved Messages **не** в продукте.

**On archive:** чат исчезает из main inbox, folder filters и Quick Access (`RemoveQuickAccess` side-effect); folder membership и folder pin **остаются** в БД.

**Unarchive:**

| Trigger | Result |
|---------|--------|
| Ctx / swipe на экране архива | `archived=false`; возврат в inbox; Quick Access **не** восстанавливается |
| **Incoming message** (peer → archiver, DM) | Auto-unarchive + unread по обычным правилам |
| Outgoing from archiver | Остаётся archived |

Удаление DM у одного пользователя не удаляет переписку у второго (как в Telegram). Контракт API: [chat-service.md](../microservices/chat-service.md) § Archive.

## Pin чатов

См. [navigation.md](navigation.md) и [GLOSSARY.md](../GLOSSARY.md). Кратко: pin в папке / спейсе ≠ Quick Access в rail.

