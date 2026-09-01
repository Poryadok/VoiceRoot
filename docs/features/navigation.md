# Navigation — структура и навигация

**Канон контролов (Penpot / Flutter):** [screen-controls.md](../design/screen-controls.md) §1 (Shell), §1.1a–§1.1c (profile menu, folders, Quick Access), §1.6–§1.6a (mobile chrome), §1.10 (Archive), [H vs V summary](../design/screen-controls.md#h-vs-v-layout-differences-summary). Термины — [GLOSSARY.md](../GLOSSARY.md) § «Организация чатов в UI».

**Product decisions (locked):** Quick Access = **`chat_id` only**, ≤15 per profile; **folders** in rail; **Settings below** ProfileStack; **Archive** via profile RC (нет folder «Archive» в rail); **нет Saved Messages**; federation-only — [federation.md](federation.md) (deferred, не в текущей навигации).

## Терминология

| Термин в коде   | Что это                                                      |
|-----------------|--------------------------------------------------------------|
| `space`         | агрегат-контейнер: дерево (`space_tree_nodes`: текст + голос), участники, роли |
| `channel`       | пресет текстового чата: по умолчанию в основную ленту не от имени пользователя |
| `group`         | пресет текстового чата: по умолчанию в ленту пишут участники от своего имени   |
| `dm` / `direct` | персональный чат 1-на-1                                      |

Пользовательский лейбл для `space` — отдельное решение (UI/перевод), в коде и документации используем "пространство" / `space`.

См. также: [GLOSSARY.md](../GLOSSARY.md) — **Архив чата**, **Quick Access**, **Pin чата**, **Pin элемента дерева**, **Active strip**, **Запросы сообщений**.

## Модель сущностей-отправителей

- **Пользователь** → имеет аккаунт (identity)
- **Текстовый чат** (`group` \| `channel`) → у строки `chats` есть отображаемое имя/аватар как у «комнаты»; сообщение может идти от профиля или **от имени чата** — по настройкам и ролям (`posted_as_chat` / `display_chat_id` в Messaging)
- **Спейс** → не является отправителем напрямую; для объявлений от спейса — выделенный системный текстовый чат внутри

## Структура навигации — Десктоп

```
[Rail]  [Chat list]  [Open chat (+ optional side panel)]
```

Три колонки: **rail** (навигация и организация), **список чатов** (средняя колонка), **открытый чат** (основная область; опционально side panel справа — см. [text-chat.md](text-chat.md)).

### Rail (сверху вниз)

Контролы — [screen-controls.md](../design/screen-controls.md) §1.1, §1.1a–§1.1c; внутри спейса — §1.8 #8–9 (space tree).

1. **Nav:** Chats / Social (Friends) / Matchmaking
2. **Folders** — system (Все, ЛС, Группы, Каналы, Спейсы) + custom; badge unread **на папке** (см. § «Badge unread на папке»); **Edit folders** (иконка или ctx внизу зоны папок)
3. **Quick Access (Избранное)** — до **15** **`chat_id`** активного профиля (только чаты, не polymorphic space/node); добавление через ctx «В избранное» / drag (см. § «Quick Access»); **не** то же самое, что pin в папке (см. [GLOSSARY.md](../GLOSSARY.md), RPC — [chat-service.md](../microservices/chat-service.md) § Quick Access)
4. Spacer
5. **ProfileStack** (multi-profile) — ПКМ desktop / long-press mobile → меню §1.1a (switch profile, create profile, presence, custom status ★, create story, **Archive** = primary entry для архивных чатов; discoverability ниже отдельной rail-папки — принято, см. [GLOSSARY.md](../GLOSSARY.md) § «Архив чата»)
6. **☰ Settings** — единственная точка входа в settings shell (`Panel/Settings/Sheet`); **ниже** ProfileStack

Rail **всегда виден**, не скрывается при открытии чата.

### Средняя колонка (Chat list)

- Search + Compose (New DM / Create group / Create or join space)
- Строки чатов с preview последнего сообщения
- **Без folder tabs** и **без inbox segmented control** (main/requests) — фильтр задаётся выбранной папкой в rail/drawer; «Запросы» — virtual folder §1.1b #5, не toggle в middle column (§1.3 tombstone)

### Убрано: колонка активных

Роль прежней левой «колонки активных» закрывают:
- **Quick Access** в rail
- **Pin чата** внутри папки / спейса
- **Mobile active strip** (§ «Мобайл»)

### Pin чатов

| Контекст | Поведение |
|----------|-----------|
| **System folder** (Все, ЛС, Группы, …) | Pin per `(profile_id, folder_id, chat_id)` — чат вверху **отфильтрованного** списка этой папки; иконка pin на row. Membership не хранится в `folder_chats` — только pin/order overlay |
| **Custom folder** | Явное членство в `folder_chats` + pin per folder; pin внутри membership list |
| **Спейс (space tree)** | **Pin элемента дерева** — text/voice node вверху категории; Space Service — см. [GLOSSARY.md](../GLOSSARY.md) § «Pin элемента дерева», [spaces.md](spaces.md) (≠ folder pin чата) |
| **Quick Access** | Отдельный список `chat_id` в rail; чат может быть и pinned в папке, и в Quick Access одновременно |

**Pin rules:** archived чаты не pin-able и не в Quick Access; pin order scoped to folder (`pin_order`, затем `sort_order`, затем activity). System folder «Спейсы» pin applies к **space row** в списке (не к tree nodes — tree pin = Space Service).

## Структура навигации — Мобайл

**Normative IA** (Telegram-parity; канон — [screen-controls.md](../design/screen-controls.md) §1.1, §1.6, §1.6a, §1.7, [H vs V summary](../design/screen-controls.md#h-vs-v-layout-differences-summary)):

| Зона | Поведение |
|------|-----------|
| **Bottom tab bar** | Всегда виден вне fullscreen room; **скрывается при открытой клавиатуре** (§1.6a): **Chats** / **Social** (Friends) / **Matchmaking** — desktop rail Nav §1.1 #1–3 |
| **Drawer** (☰ или swipe) | Folders, Quick Access, ProfileStack + Archive RC, Settings — **не** дублировать полный desktop rail; Settings **не** вкладка tab bar |
| **Chat list** | Полноэкранный список при tab Chats без открытого room |
| **Active strip** | При открытом room — **opened-chat LRU** (§1.6), **не** inbox preview rows |

### Drawer (сверху вниз)

1. **Folders** (§1.1b) — system + custom + virtual «Запросы»
2. **Quick Access** (§1.1c)
3. Spacer
4. **ProfileStack** — tap/long-press → §1.1a (switch profile; **Archive** = primary entry)
5. **☰ Settings** — **ниже** profiles (как desktop rail §1.1 #8)

### Active strip (normative)

См. [GLOSSARY.md](../GLOSSARY.md) § «Active strip», [screen-controls.md](../design/screen-controls.md) §1.6:

| Правило | Поведение |
|---------|-----------|
| **Membership** | Чат в strip после **open** на mobile (LRU session state) |
| **Limit** | Max **100** opened chats; 101-й → evict oldest |
| **Visible cap** | ~**8** avatars; horizontal scroll для overflow |
| **Unread badge** | На иконках strip |
| **Keyboard** | Strip **скрывается** (§1.6a); pinned bar + composer приоритетнее |
| **Remove** | Long-press icon → × (§1.6 #5); **back to list — remove from strip** (в т.ч. при unread). Удержание strip при unread после back — **DEFERRED** (AUDIT R3-03-A09) |
| **Limit feedback** | 100/100 → feedback, нельзя добавить ещё один opened chat |

- Нет открытого чата → полный список; drawer для folders / QA / settings
- Открыт чат → список сворачивается; strip вверху (`Screen / Shell / MobileChatOpen`, §1.7)

## Вход в спейс

- **Из папки «Спейсы» в rail** → как в десктоп Telegram: название спейса исчезает, остаётся только иконка + открывается колонка с деревом спейса (текстовые чаты и голос)
- **Из Quick Access** → только если в избранное добавлен **конкретный `chat_id`** (текстовый чат спейса или standalone); **не** polymorphic «иконка спейса с деревом». Обзор спейса целиком — через folder «Спейсы» или mobile drawer

## Папки по умолчанию

- Все
- ЛС (личные / DM)
- Группы
- Каналы
- Спейсы
- + кастомные (создаёт пользователь)

**Partial shipment — folder «Каналы»:** Normative IA keeps the system folder with predicate `chat.type=channel`. Shipped `ListChats` membership SQL includes `channel` (Batch 14); **standalone channel create** via `CreateChat` still requires `space_id` — folder list may be empty until R3-A12 ships. Space-attached `channel` chats also surface via first-page space merge per chat-service rules — do not remove «Каналы» from IA.

### System vs custom folders

| Тип | Rename / delete | Filter predicate | Membership |
|-----|-----------------|------------------|------------|
| **System** (Все, ЛС, Группы, Каналы, Спейсы) | **Запрещено** — immutable `folder_type=system` | Fixed in `filter_config_json` (predicate по `chat.type`, `space_id`, …); пользователь не меняет | Implicit — чат попадает по predicate; **не** строка в `folder_chats`, кроме pin overlay |
| **Custom** | Rename + delete через Edit folders | User-defined rules в `filter_config_json` (тип чата, включённые/исключённые `chat_id`, …) | Explicit `folder_chats` rows + pin/order |
| **Message requests** | N/A — **virtual folder в rail/drawer** ([screen-controls.md](../design/screen-controls.md) §1.1b #5; row **visible when** pending requests exist); **не** segmented toggle в middle column (§1.3 tombstone) | `ListChats` с `inbox=requests` | DM с `chat_members.inbox_bucket=requests` — см. [text-chat.md](text-chat.md) § «Запросы сообщений» |

**Archived chats:** excluded из всех folder filters и Quick Access (`is_archived=true`); membership в `folder_chats` и pin overlay **сохраняются** в БД — после unarchive чат снова виден в matching folders. См. [GLOSSARY.md](../GLOSSARY.md) § «Архив чата».

### Badge unread на папке

Агрегат по чатам, matching folder predicate (или `folder_chats` для custom), **исключая archived**.

| Правило | Поведение |
|---------|-----------|
| **Dedup внутри папки** | Каждый `chat_id` учитывается **не более одного раза** в badge этой папки |
| **Dedup между папками** | **Нет** — один и тот же чат с unread может увеличивать badge и в «Все», и в «ЛС», если matching обеим |
| **Muted chat** | Число **не** входит в сумму; **dot** на folder row, если единственные unread — из muted чатов без mention |
| **@mention в muted** | Unread mention **входит** в числовой badge (пробивает mute для счётчика папки) |
| **Формула (numeric badge)** | `sum(unread_count)` по чатам folder ∩ `is_archived=false`, где chat **не muted** OR имеет mention-unread |
| **Requests folder** | Отдельный badge на virtual «Запросы» — count DM в `inbox=requests` с `unread_count > 0` |

Источник `unread_count` — Messaging S2S enrichment в `ListChats` ([chat-service.md](../microservices/chat-service.md)).

### Quick Access

**≠ Friends → Favourites** (люди, не чаты) — см. [friends.md](friends.md). Контролы — [screen-controls.md](../design/screen-controls.md) §1.1c.

| Аспект | Контракт |
|--------|----------|
| **Limit** | 15 `chat_id` per `profile_id`; archived reject |
| **Add at limit** | ctx «В избранное» при 15/15 → **replace picker**: список текущих QA-слотов + «Выберите, что заменить»; atomic `RemoveQuickAccess` + `AddQuickAccess` |
| **Server at-limit error** | `AddQuickAccess` без предварительного remove → `FAILED_PRECONDITION` — **server safety net**, не UX ошибка; клиент **обязан** открыть replace picker (§1.1c #6), не показывать hard-error toast |
| **Drag reorder** | Desktop: drag row в rail zone. Mobile: long-press + drag в drawer QA list (§1.1c #5) |
| **Cross-device order** | Server SoT — `ReorderQuickAccess`; клиент применяет порядок после sync |
| **Remove** | ctx на QA row; side-effect при archive — auto-remove ([GLOSSARY.md](../GLOSSARY.md)) |

### Message requests (virtual folder)

Virtual folder «Запросы» в **folders zone** rail/drawer — **не** segmented toggle в middle column ([screen-controls.md](../design/screen-controls.md) §1.1b #5, §1.3 tombstone).

| Правило | Контракт |
|---------|----------|
| **Visibility** | Row **скрыта**, когда pending requests = **0**; **появляется** в folders zone при ≥1 DM в `inbox_bucket=requests` |
| **Badge** | Numeric badge на folder row когда `unread_count > 0` в requests inbox (отдельно от main inbox badge) |
| **Tap** | Открывает `ListChats` с `inbox=requests` — список request rows §1.3a |
| **Empty state** | При последнем accept/decline folder row **исчезает** из rail/drawer без ручного dismiss |
| **≠ Friends Pending** | Заявки в друзья — [friends.md](friends.md); не путать с DM message requests |

Bucket semantics, Accept/Decline — [text-chat.md](text-chat.md) § «Запросы сообщений», [GLOSSARY.md](../GLOSSARY.md) § «Запросы сообщений».

### Chat list row actions (ctx)

Normative ctx menu — [screen-controls.md](../design/screen-controls.md) §1.4:

| Action | Notes |
|--------|-------|
| Pin / Mute | Per-folder pin ≠ Quick Access — § «Pin чатов» |
| **Archive** | **Secondary** on row (§1.4 #5); **primary** entry — ProfileStack menu §1.1a #6 → `Screen/Chat/Archive` ([text-chat.md](text-chat.md) § «Архивирование») |
| **Mark read / unread** | §1.4 #6 — [text-chat.md](text-chat.md) § «Mark read / unread» |
| Delete chat | DM only (§1.4 #7) |
| Add to folder / Quick Access | §1.4 #10–11 |

**Archive discoverability:** отдельной folder «Archive» в rail/drawer **нет** (product decision H16). Пользователь открывает архив через **profile avatar menu** (desktop ПКМ / mobile long-press на ProfileStack). Secondary ctx «Архивировать» на строке чата — для power users.

## Service Ownership

- Feature owner (UX): клиентские приложения (Flutter Desktop/Mobile/Web)
- Data owners: `Chat Service` (чаты/папки/quick access, `ListChats` sort keys), `Space Service` (tree navigation), `Messaging Service` (`posted_as_chat` / `display_chat_id`; S2S enrichment **`last_message_preview`** и **`unread_count`** для list rows — см. [chat-service.md](../microservices/chat-service.md) § ListChats)

## Enforcement path

1. Клиент формирует навигационную структуру на основе сущностей из `Chat Service` и `Space Service`.
2. Для рендера sender semantics клиент использует поля из `Messaging Service`.
3. Права и видимость узлов применяются на стороне сервисов до отдачи данных в клиент.
