# Text Chat — текстовый чат

## Сообщения

- **Редактирование**: да, лимит 100 лет (фактически без ограничений)
- **Удаление**: да — **`DeleteScope.FOR_EVERYONE`** (у всех) или **`DeleteScope.FOR_ME`** (только у себя, soft-hide для viewer); без лимита по времени. Wire: [messaging.proto](../../protos/voice/messaging/v1/messaging.proto) `DeleteMessageRequest.scope`
- **Форматирование**: markdown-подмножество (см. § «Markdown и formatting menu»)

### Удаление сообщений (UX)

Confirm sheet после **Delete** (§3.4 #4) или **Delete selected** (§3.5 #3) — [screen-controls.md](../design/screen-controls.md):

| Контекст | Доступные опции | Wire |
|----------|-----------------|------|
| **DM**, своё сообщение | «Удалить у меня» / «Удалить у всех» | `FOR_ME` / `FOR_EVERYONE` |
| **DM**, чужое сообщение | «Удалить у меня» only | `FOR_ME` |
| **Group / channel**, своё сообщение | «Удалить у меня» / «Удалить у всех» | `FOR_ME` / `FOR_EVERYONE` |
| **Group / channel**, чужое + `MANAGE_MESSAGES` | «Удалить у всех» (moderator) | `FOR_EVERYONE` |
| **Group / channel**, чужое без права | — (Delete hidden) | — |

**Multi-select (§3.5):** batch delete показывает тот же scope picker; если в выборке есть чужие сообщения без `MANAGE_MESSAGES` — удаляются только свои (остальные пропускаются или picker ограничен `FOR_ME` only — product: **skip non-owned** with toast «Удалено N из M»).

**Без лимита по времени** — «удалить у всех» доступно для любого возраста сообщения (как Telegram).

### Multi-select mode

Вход: message action **Select** ([screen-controls.md](../design/screen-controls.md) §3.4 #10) или long-press на bubble (V). Активирует **multi-select bar** §3.5.

| Действие | Control | Контракт |
|----------|---------|----------|
| Forward batch | §3.5 #2 | `ForwardMessage` для выбранных id — [forward-messages.md](forward-messages.md) |
| Delete batch | §3.5 #3 | `DeleteMessage` per id + scope picker (§ «Удаление сообщений (UX)») |
| Cancel | §3.5 #4 | Exit multi-select |

Одиночный Forward без multi-select — §3.4 #5. Copy-as-new / без атрибуции — §3.4 #6.

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
- **Video note** (круглое видео): да — короткая запись до **60 с** ([file-storage.md](file-storage.md), [screen-controls.md](../design/screen-controls.md) §3.6 #6), inline player
- GIF: да — first-class сообщение в чате (поиск/отправка через 😊 panel; не attach menu). Provider search — **Chat** `SearchGifs`; send wire — **Messaging** `content_type=GIF` — [messaging-service.md](../microservices/messaging-service.md) § Stickers and GIF. Premium **GIF-аватар** — отдельно в [user-profile.md](user-profile.md) / [subscription.md](subscription.md)
- Стикеры: системные паки + пользовательские (загрузка своих паков — **static PNG/WebP images only**, §37); отправка/приём как first-class сообщение; picker в composer **😊** → Stickers. Управление паками — Settings §37 ([screen-controls.md](../design/screen-controls.md)). Wire contract (Chat catalog + File bytes + Messaging send) — [messaging-service.md](../microservices/messaging-service.md) § Stickers and GIF, [chat-service.md](../microservices/chat-service.md) § Sticker packs, [file-service.md](../microservices/file-service.md) § Stickers and GIF assets. **Not yet in proto/code.** Sticker store / premium ★ packs — **optional** post-core; MVP = system packs + user upload. Термины — [GLOSSARY.md](../GLOSSARY.md) § «Стикеры и GIF». Live TC-MSG-09 — [ADR 005](../adr/005-rich-media-live-tests-deferred.md)
- Emoji: да
- **Expired file (retention):** объект удалён из R2 по retention cron — bubble показывает placeholder §3.3 #14 («кучка костей» + tooltip «Файл удалён. Подписка сохраняет файлы навсегда»); policy — [file-storage.md](file-storage.md); Premium forever — [subscription.md](subscription.md)

### Attach menu (composer)

Telegram-parity popup из 📎:

| Пункт | Тип сообщения | Сервис |
|-------|---------------|--------|
| Photo or video | existing media | Messaging + File |
| Document | file attachment | File |
| Article | rich article / instant view payload | Messaging (`content_type=ARTICLE`) |
| Location | lat/lon + optional label + static map preview | Messaging + client map picker |
| Music | audio file + metadata | File (metadata extract → Messaging store; [messaging-service.md](../microservices/messaging-service.md) § Content types) |

**Не в attach menu:**

| Surface | Где | Wire |
|---------|-----|------|
| **Stickers** | Composer **😊** → tab Stickers ([screen-controls.md](../design/screen-controls.md) §3.6b) | `content_type=STICKER` — [messaging-service.md](../microservices/messaging-service.md) § Stickers and GIF |
| **GIF** | Composer **😊** → tab GIFs (provider search) | `content_type=GIF` — same § |
| **Wallet** | — | **Не в продукте**, не планируется |
| **Saved Messages** | — | **Не в продукте**, не планируется |

Stickers/GIF — first-class сообщения, но отправка **только** через emoji panel, не через 📎.

### Article vs inline URL

| | **Inline URL** (в тексте) | **Article** (attach menu) |
|--|---------------------------|---------------------------|
| **Trigger** | URL regex в `content` | Явный выбор «Article» в attach popup |
| **`content_type`** | `TEXT` (+ link metadata) | `ARTICLE` |
| **Metadata fetch** | **Клиент** — OG fetch с sanitization; fallback: title из `<title>` / hostname | **Messaging Service** (background worker on attach/send) — server-side HTTPS fetch, HTML sanitization; **not** Gateway transcoding, **not** File Service, **not** client CORS; payload `{ url, title, description, thumb_file_id?, instant_view_html? }` — [messaging-service.md](../microservices/messaging-service.md) § Content types |
| **Preview UI** | Card под bubble (thumb + title + domain) | Instant-view / expanded article card |
| **Shared Media tab** | **Links** | **Links** (label «Article» в list preview) |
| **Search in-chat** | URL substring в теле | Title/description в article payload |
| **Bubble UI** | §3.3 #6a Link preview (card под TEXT bubble) | §3.3 #6b Article / instant-view bubble |

См. content-type → tab mapping: [search.md](search.md) § «Фильтры shared media».

## Социальные механики

- Реакции на сообщения: да — chips под bubble, toggle; long-press → who reacted; **на mobile видимы без hover** (не только по наведению)
  - **Free:** одна реакция на пользователя на сообщение (toggle — смена emoji снимает предыдущую)
  - **Premium (★):** до **3** реакций на пользователя на одно сообщение; при попытке 4-й — upsell prompt
  - Entitlement: personal Premium на `profile_id`; enforcement — Messaging `AddReaction` / `RemoveReaction`
- @упоминания: да — событие `mention { sender_profile_id, message_id, chat_id }` по WebSocket всем подключённым устройствам; если офлайн — FCM/APNs push
- **@here** — упоминание только онлайн-участников **текстового чата** (группа или канал); требуется право `TEXT_CHAT_MENTION_ALL_ONLINE` ([роли](roles.md), [role-service.md](../microservices/role-service.md))
- **@everyone** — упоминание всех участников **текстового чата**; требуется `TEXT_CHAT_MENTION_ALL_IN_CHAT`
- Закреплённые сообщения: да, как в Telegram — см. § «Закреплённые сообщения»
- **Share message link:** ctx / hover **Share** (§3.4 #12) копирует `https://voice.gg/…/m/{messageId}` в буфер; toast «Ссылка скопирована». URL shapes — [deep-links.md](deep-links.md) (`/ch/{chatId}/m/{id}` или `/s/{spaceId}/c/{chatId}/m/{id}`). Открытие ссылки — scroll + highlight flash §3.2 #6.

### Ссылки в тексте

- URL в теле сообщения рендерятся **синим** (`color.link` / design token), underline on hover
- Отличие от spoiler (`||…||`) и inline code (`` `…` ``) — см. [brand.md](../design/brand.md) § Messaging UX

## Закреплённые сообщения

- Лимит: до **5** pins на чат (как Telegram); `PinMessage` отклоняет 6-й; UI — «Максимум 5 закреплённых» + unpin из списка
- Права: `TEXT_CHAT_PIN_MESSAGES` ([roles.md](roles.md))
- **Persistent pinned bar** под header комнаты (до **5** pins): thumb + label по `content_type` (Photo / Video / Voice / File / Sticker / GIF / Article / Location / Music / Video message / text snippet — как §1.4 #16)
  - Tap bar → jump к **текущему** pin (v1: latest; cycle — target-state)
  - **Open all** (chevron / §3.1 #7) → список всех pins — §3.1a #3
  - **× или swipe-left (V)** → hide bar на сессию (pins остаются; restore через header #7) — §3.1a #4–5
- Bar не блокирует composer — [screen-controls.md](../design/screen-controls.md) §3.1a

## История и поиск

- История: бесконечная (лимит не установлен на старте, пересмотреть по нагрузке)
- **Поиск из чата** → поиск по тексту и ссылкам внутри этого чата
- **Поиск с главного экрана** → глобальный поиск по всем чатам
- У каждого чата есть раздел shared media: вкладки **Медиа / Стикеры / Файлы / Ссылки / Голосовые** — канон в [search.md](search.md) § «Фильтры shared media»

## Preview последнего сообщения в списке

Семантика preview в `ListChats` / строке списка — **два слоя**:

| Слой | Источник | Поля |
|------|----------|------|
| **Server DTO** | Messaging S2S `GetChatListMetadata` → Chat `ChatListItem` | `last_message_preview` (text или media label), `last_message_content_type`, `last_message_is_outgoing`, `last_message_delivery_state` (DM), `unread_count`, `last_message_at`. UI labels — [screen-controls.md](../design/screen-controls.md) §1.4 #15–16; wire DTO gap — [messaging-service.md](../microservices/messaging-service.md) § Chat list metadata |
| **Client overlay** | Локальный draft (SQLite/Hive) | Префикс `Черновик: …` **перекрывает** server preview для этого `chat_id` на этом устройстве; не уходит на сервер |

**Precedence на клиенте:** если есть локальный draft → показать draft; иначе server DTO. Delivery ticks и media labels — **только из server DTO** (не выводить из bubble state открытого чата).

**Delivery state (DM, outgoing last message):** durable enum `last_message_delivery_state` из `GetChatListMetadata` — `none` \| `sent` \| `delivered` \| `read` → UI: (none/sent — без tick), `✓`, `✓✓`. WS `delivery_ack` **не** источник для list row после reconnect — [messaging-service.md](../microservices/messaging-service.md) § Delivery state matrix.

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

### Mark read / unread (list row)

Ctx на строке чата — [screen-controls.md](../design/screen-controls.md) §1.4 #6; rail/drawer list — [navigation.md](navigation.md) § «Chat list row actions».

| Действие | Когда в ctx | API / эффект |
|----------|-------------|--------------|
| **Mark read** | `unread_count > 0` | REST `MarkRead` с `last_read_message_id` = latest в чате → badge = 0, bold off |
| **Mark unread** | Чат прочитан (`unread_count = 0`) | REST `MarkRead` с `last_read_message_id` = сообщение **непосредственно перед** текущим cursor (≥1 unread); пустой чат → no-op |

**DM:** normative сегодня (`MarkRead` shipped). **Group/channel:** те же ctx labels; list badge когда group read parity — partial shipment ([messaging-service.md](../microservices/messaging-service.md) § MarkRead).

Не меняет содержимое сообщений — только `read_receipts` и `unread_count` (Messaging enrichment). Peer ✓✓ — § «Статусы доставки»; после REST опционально WS `mark_read` для multi-tab.

## Статусы доставки

- Как в Telegram: одна галочка = доставлено, две = прочитано
- **DM only** — delivery ticks в list preview и bubble
- **Privacy opt-out** — `show_read_receipts` toggle ([privacy.md](privacy.md) § «Read receipts»; UI §5.1 #14): when off, peer не получает ✓✓ и viewer не видит ✓✓ peer'а; delivered ✓ не скрывается
- **Группы / каналы** — **счётчик просмотров** на сообщении (не delivery ticks); см. § «View count»

**WS vs REST (не смешивать):**

| Concern | Durable (list + history) | Ephemeral (live bubble) |
|---------|--------------------------|-------------------------|
| Read ✓✓ | `Messaging.MarkRead` REST/gRPC → `read_receipts` | WS `mark_read` / `message_read` (multi-tab sync) |
| Delivered ✓ | `GetChatListMetadata.last_message_delivery_state` (peer `last_delivered_message_id`) | WS `delivery_ack` → `message_delivered` |
| After reconnect | **Обязателен** `ListChats` / metadata refresh | `resume` не восстанавливает ticks |

Контракт derivation — [messaging-service.md](../microservices/messaging-service.md) § Durable delivery derivation, § MarkRead; протокол WS — [realtime-service.md](../microservices/realtime-service.md) § Read / delivery dual path; сводка — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § WS vs REST.

### View count (group / channel)

| Правило | Контракт |
|---------|----------|
| **Uniqueness** | Один `profile_id` = **не более одного** view на `message_id` (dedup on first qualifying view) |
| **Qualifying view** | Участник открыл чат и message bubble entered viewport **или** read cursor прошёл сообщение (`MarkRead` / `last_read_message_id` ≥ message). Dedicated `RecordMessageView` RPC — **not yet in proto**; spec sketch — [messaging-service.md](../microservices/messaging-service.md) § View count |
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

- **📎 (attach):** **click/tap или Enter/Space** (H/V) → popup §3.6a; hover-only не единственный affordance — [screen-controls.md](../design/screen-controls.md) §3.6e
- **😊:** **click/tap или Enter/Space** → panel **Emoji | Stickers | GIFs** (tabs, search, recents, pack rail) — синхронизировано с § «Медиа и вложения»
- **Правая кнопка:** пустой input → **mic** (voice note); long-press mic / toggle 📹 → **video note**; с текстом → **Send**
- **Voice / video note capture:** hold-to-record, swipe-up lock, swipe-left / × cancel, max **60 s**, min **1 s** — [screen-controls.md](../design/screen-controls.md) §3.6g
- **Long-press Send:** Send without sound, Schedule message, Send when online — см. § Send options

### Text selection / formatting (ПКМ в composer)

Контекстное меню как TG: Cut / Copy / Paste + **Formatting** submenu — mapping на markdown из § «Markdown и formatting menu».

Desktop: **format toolbar** над composer **опционально** дублирует submenu (как TG desktop — можно скрыть, если есть ПКМ).

### Send options (long-press Send)

Normative menu — [screen-controls.md](../design/screen-controls.md) §3.6c. **Long-press** на Send доступен для **text** и для pending **voice/video note**, **sticker/GIF**, **attachment** ready to send.

| Опция | Wire / RPC | Поведение |
|-------|------------|-----------|
| **Send without sound** | `send_silent=true` на `SendMessageRequest`; propagates в `message.sent.send_silent` | Push без звука; in-app badge/unread как обычно — [notifications.md](notifications.md) § «Send without sound» |
| **Schedule message** | `scheduled_at` на `SendMessageRequest`; row в `scheduled_messages` | Strip над composer; **edit** через `UpdateScheduledMessage` (только `status=pending`); cancel / send-now — см. [messaging-service.md](../microservices/messaging-service.md) § Scheduled messages |
| **Send when online** | `send_when_online=true` | Queued до presence `online` у получателя (**DM only**); invisible/offline у получателя — очередь держится; отмена до dispatch; GRP/CH → validation §3.6f |

> **Proto gap:** `send_silent`, `scheduled_at`, `send_when_online`, `UpdateScheduledMessage`, `scheduled_messages` table — **not yet in proto/code**; normative contract — [messaging-service.md](../microservices/messaging-service.md) § Send options, § Scheduled messages ([todo/backend.md](../todo/backend.md)).

### Side panel (desktop) / sheets (mobile)

Normative mode table — [screen-controls.md](../design/screen-controls.md) §1.9. Header toggle (`icon.sidePanel`, a11y «Информация о чате») открывает **один** из modes в `Panel/Shell/SideHost` (H) или sheet/push (V):

| Mode | Desktop (SideHost body) | Mobile equivalent | Entry (V) |
|------|-------------------------|-------------------|-----------|
| **info** | `Panel/Chat/Info` — DM profile summary / group header meta | Bottom sheet | Avatar/name tap or kebab → Info |
| **members** | `Panel/Chat/GroupMembers` / `Panel/Space/Members` | Bottom sheet **or** full-screen push | Chat Info / header → members |
| **thread** | `Panel/Chat/Thread` для `thread_parent_id` | **Full-screen push** (preferred) | §3.1 #8; thread affordance on bubble |
| **search** | SideHost scrollable match list (paired with §3.2) | **No SideHost / no bottom sheet** — §3.2 bar in app header; prev/next; jump + highlight | §3.1 #5 |

Только **один** mode активен; close SideHost / sheet → focus return to room (§1.9, §3.6e). Reaction/emoji picker — **не** SideHost mode (transient overlay §3.6b / §1.9 #7).

### Header subtitle (DM)

Subtitle под именем в §3.1 #2 — normative priority table в [presence.md](presence.md) § «В header chat room» (online + in-call combination, custom status, last seen rounding, `show_last_seen` privacy). Avatar online dot независим от subtitle wording.

### Composer error states

Prefer **inline/banner** у composer; toast — secondary ([brand.md](../design/brand.md)). Полная матрица — [screen-controls.md](../design/screen-controls.md) §3.6f.

| Failure | UI | Recovery |
|---------|-----|----------|
| **Upload network fail** | Banner «Не удалось загрузить» + Retry | Retry upload intent; composer keeps draft |
| **Async processing fail** (`file.processed` error) | Failed attachment chip + «Обработка не удалась» | Remove attachment or retry upload |
| **Quota / file size** | Inline «Файл слишком большой…» | User picks smaller file |
| **Schedule horizon > 365d** | Inline near date picker | Pick valid date |
| **Schedule edit on non-pending** | Toast «Сообщение уже отправлено» | Open sent message in history |
| **Schedule edit/cancel/send-now RPC failure** | Inline on scheduled row | Retry / dismiss — §3.6f |
| **`send_when_online` on non-DM** | Inline «Только для личных чатов» | Hide option or block submit — §3.6c / §3.6f |
| **Sticker/GIF send fail** | Inline near picker | Retry; provider search remains open |
| **Optimistic send fail** | Bubble → failed state (red icon) | Tap → Retry or Delete local failed row |
| **Rate limit (5/5s)** | Inline «Слишком много сообщений» | Wait + auto-clear |
| **Char limit 4000** | Counter turns red at limit | Trim before send |
| **ClamAV / infected file** | Toast or inline «Файл заблокирован» | Pick another file — §3.6f #20 |
| **Recipient privacy blocks attach** | Toast with privacy reason | — §3.6f #19 |
| **Space/group membership limit** | Toast «Лимит достигнут» | — §3.6f #21 |
| **Quota / CheckQuota exceeded** | Inline «Недостаточно места» | Subscription upsell if applicable |
| **Article URL / OG fetch fail** | Inline in article flow | Edit URL or remove attach |
| **Video note too short / cancelled** | Inline near mic | Min 1 s; re-record — §3.6g / §3.6f |

### i18n (новые строки)

Preview labels (`Photo`, `Voice`, `Article`, …), last-seen buckets, message-request actions, composer errors — **ARB keys** в `lib/l10n/app_{en,ru}.arb` с ICU plurals для last-seen и unread; не hardcode EN/RU в widgets. Baseline keys: `chat.preview.*`, `chat.lastSeen.*`, `chat.messageRequest.*`, `composer.error.*` — см. [i18n.md](i18n.md).

## Технические решения

- **Протокол**: WebSocket (Realtime), persistent connection; reconnection: exponential backoff (1s → 2s → 4s, cap 30s)
- **Reconnect и догрузка истории** (поле `s` / `resume` в WS и курсор сообщений в Messaging): [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) — раздел «Reconnect: WebSocket-поток и история сообщений»
- **Между инстансами Realtime**: Redis Pub/Sub
- **Typing indicator**: WebSocket, throttle — событие не чаще раза в 3 сек, гасить через 5 сек без обновления; отображение в header — [screen-controls.md](../design/screen-controls.md) §3.1 #11
- **Read receipts / просмотры**: механизм **зависит от типа чата** — DM: ✓/✓✓ delivery ticks (list preview + bubble); группы и каналы: **view count** на bubble, не delivery ticks (см. § «Статусы доставки»)
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
| **`declined`** | Получатель вызвал `DeclineDMRequest` | **No list RPC** — hidden from `inbox=main` and `inbox=requests`; re-contact via new `SendMessage` → recipient returns to `requests` |

**На CreateDM / EnsureDM:** инициатор → `main`; получатель → `requests` если stranger, иначе `main` для обоих.

**Accept:** `AcceptDMRequest(chat_id)` → `inbox_bucket=main` для accepter; sender уже в `main`. Оба видят чат в main inbox; `is_stranger=false`.

**Decline:** `DeclineDMRequest(chat_id)` → accepter `inbox_bucket=declined`; чат исчезает из requests. История сообщений **не** удаляется.

**Re-contact после decline:** новое входящее сообщение от того же peer → recipient bucket **возвращается в `requests`** (новый запрос); push/in-app тип `message_request` — [notifications.md](notifications.md).

**Blocked / privacy deny:** `CreateDM` / `SendMessage` → `PermissionDenied` до попадания в requests.

**ListChats fields:** `ChatListItem.inbox`, `is_stranger` (= `inbox_bucket==requests` для row recipient). См. [GLOSSARY.md](../GLOSSARY.md) § «Запросы сообщений».

**Row actions (§1.3a):** Accept / Decline only — **нет Block** на request row. Block stranger — profile / report path ([friends.md](friends.md), [privacy.md](privacy.md)).

### Архивирование

**Storage:** `chat_members.is_archived` (Chat Service), per `profile_id`.

| Operation | Контракт |
|-----------|----------|
| **Write** | `ArchiveChat(chat_id, archived)` |
| **Read** | `ListChats` с `inbox=main` (default) — non-archived; `inbox=archive` — только archived (`folder_id` игнорируется) |
| **Scope** | DM, group, channel, space-attached — один флаг `is_archived` и один контракт |

**UX entry (product decision H16):** **Primary** — ПКМ на **аватар профиля** в ProfileStack (rail) → «Архив» → `Screen/Chat/Archive`. **Secondary** — ctx «Архивировать» на строке чата (DM, group, channel, space-attached). Отдельной folder «Archive» в rail **нет** (discoverability tradeoff принят). **Saved Messages** и **Wallet** — **не** в продукте.

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

