# Navigation — структура и навигация

## Терминология

| Термин в коде   | Что это                                                      |
|-----------------|--------------------------------------------------------------|
| `space`         | агрегат-контейнер: дерево (`space_tree_nodes`: текст + голос), участники, роли |
| `channel`       | пресет текстового чата: по умолчанию в основную ленту не от имени пользователя |
| `group`         | пресет текстового чата: по умолчанию в ленту пишут участники от своего имени   |
| `dm` / `direct` | персональный чат 1-на-1                                      |

Пользовательский лейбл для `space` — отдельное решение (UI/перевод), в коде и документации используем "пространство" / `space`.

См. также: [GLOSSARY.md](../GLOSSARY.md) — **Архив чата**, **Quick Access**, **Pin чата**.

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

1. **Nav:** Chats / Social (Friends) / Matchmaking
2. **Folders** — system (Все, ЛС, Группы, Каналы, Спейсы) + custom; badge unread **на папке** (см. § «Badge unread на папке»); **Edit folders** (иконка или ctx внизу зоны папок)
3. **Quick Access (Избранное)** — до **15** **`chat_id`** активного профиля (только чаты, не polymorphic space/node); добавление через ctx «В избранное» / drag (см. § «Quick Access»); **не** то же самое, что pin в папке (см. [GLOSSARY.md](../GLOSSARY.md), RPC — [chat-service.md](../microservices/chat-service.md) § Quick Access)
4. Spacer
5. **ProfileStack** (multi-profile) — ПКМ на аватар → **«Архив»** (primary entry для архивных чатов; см. [GLOSSARY.md](../GLOSSARY.md))
6. **☰ Settings** — единственная точка входа в settings shell (`Panel/Settings/Sheet`); **ниже** ProfileStack

Rail **всегда виден**, не скрывается при открытии чата.

### Средняя колонка (Chat list)

- Search + Compose (New DM / Create group / Create or join space)
- Строки чатов с preview последнего сообщения
- **Без folder tabs** — фильтр задаётся выбранной папкой в rail

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
| **Спейс (space tree)** | Pin text/voice node → закреп вверху категории/дерева; право через роли спейса — см. [spaces.md](spaces.md) (отдельно от Chat folder pin) |
| **Quick Access** | Отдельный список `chat_id` в rail; чат может быть и pinned в папке, и в Quick Access одновременно |

**Pin rules:** archived чаты не pin-able и не в Quick Access; pin order scoped to folder (`pin_order`, затем `sort_order`, затем activity). System folder «Спейсы» pin applies к **space row** в списке (не к tree nodes — tree pin = Space Service).

## Структура навигации — Мобайл

- Нет открытого чата → полный список чатов (или drawer с rail-элементами)
- Открыт чат → список сворачивается; **горизонтальная полоска мини-иконок** (active strip) вверху экрана
  - Иконки со скроллом (может быть много)
  - На иконках — бейджи непрочитанных
  - Решает проблему "не знаешь, написал ли кто в другой чат пока общаешься"
- Папки и Quick Access — в **drawer** или compact rail (кратко, без полного дублирования desktop)

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

### System vs custom folders

| Тип | Rename / delete | Filter predicate | Membership |
|-----|-----------------|------------------|------------|
| **System** (Все, ЛС, Группы, Каналы, Спейсы) | **Запрещено** — immutable `folder_type=system` | Fixed in `filter_config_json` (predicate по `chat.type`, `space_id`, …); пользователь не меняет | Implicit — чат попадает по predicate; **не** строка в `folder_chats`, кроме pin overlay |
| **Custom** | Rename + delete через Edit folders | User-defined rules в `filter_config_json` (тип чата, включённые/исключённые `chat_id`, …) | Explicit `folder_chats` rows + pin/order |
| **Message requests** | N/A — virtual folder в rail (§1.1b design) | `ListChats` с `inbox=requests` | DM с `chat_members.inbox_bucket=requests` — см. [text-chat.md](text-chat.md) § «Запросы сообщений» |

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

| Аспект | Контракт |
|--------|----------|
| **Limit** | 15 `chat_id` per `profile_id`; archived reject |
| **Add at limit** | ctx «В избранное» при 15/15 → **replace picker**: список текущих QA-слотов + «Выберите, что заменить»; atomic `RemoveQuickAccess` + `AddQuickAccess` |
| **Drag reorder** | Desktop: drag row в rail zone. Mobile: long-press + drag в drawer / compact QA strip |
| **Cross-device order** | Server SoT — `ReorderQuickAccess`; клиент применяет порядок после sync |
| **Remove** | ctx на QA row; side-effect при archive — auto-remove ([GLOSSARY.md](../GLOSSARY.md)) |

## Service Ownership

- Feature owner (UX): клиентские приложения (Flutter Desktop/Mobile/Web)
- Data owners: `Chat Service` (чаты/папки/quick access), `Space Service` (tree navigation), `Messaging Service` (posted_as_chat/display_chat_id semantics)

## Enforcement path

1. Клиент формирует навигационную структуру на основе сущностей из `Chat Service` и `Space Service`.
2. Для рендера sender semantics клиент использует поля из `Messaging Service`.
3. Права и видимость узлов применяются на стороне сервисов до отдачи данных в клиент.
