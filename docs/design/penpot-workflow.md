# Penpot — правила работы с макетами (Voice)

Практическое руководство: как собирать и править макеты в файле Voice без типовых поломок layout и «пустых» экранов.

**Связанные документы:** [penpot-setup.md](penpot-setup.md) (файл, страницы, MCP, tokens), [screens.md](screens.md) (инвентарь фреймов), [brand.md](brand.md) (UX и визуальный стиль).

---

## 1. Размещение фреймов на странице (не наслаивать)

Каждый `Screen/...`, `Panel/...`, `Overlay/...`, `State/...` — **отдельный top-level frame** на своей странице. Фреймы **не должны перекрываться**: иначе в слоях виден только верхний, viewer и MCP export показывают не тот макет.

### Вертикаль — канон; горизонталь — сравнение вариантов

**По Y (вниз)** — основной порядок экранов: каждый следующий макет **под** предыдущим, с gap **≥ 100 px** между фреймами.

**По X (вправо)** — рабочая зона сравнения **внутри одной строки**:

| Позиция в строке | Что это | Кто меняет | Ссылка в `screens.md` |
|------------------|---------|------------|------------------------|
| **Первый** (слева, `x ≈ 0`) | **Shipped snapshot** — как **сейчас в приложении** | **Только скрипт** заливки токенов из приложения (см. ниже). **Дизайнеру вручную менять запрещено.** | Основной frame ID для PR и Flutter parity |
| **Второй и далее** | **Варианты в работе** — черновики для сравнения с каноном | **Дизайнер** (и агент по задаче) | Суффикс `· v2` / `· v3` / `· draft` / `· WIP`; в inventory не подменяют канон |

### Канон (первый макет) — read-only для дизайнера

> **Первый фрейм в строке (`x ≈ 0`) в рамках дизайнерских задач менять нельзя.**

Ни layout, ни цвета, ни текст, ни компоненты, ни placeholder — **никаких ручных правок** в Penpot UI и **никаких правок через MCP** «для удобства».

Этот фрейм — **зеркало текущей реализации в приложении**. Он обновляется **исключительно скриптом заливки токенов** из репозитория (канон приложения → Penpot):

```bash
make design-tokens-check    # design/tokens/voice.tokens.json ↔ Flutter asset
make penpot-tokens-export   # → JSON для Penpot Tokens panel
# далее: import в Penpot или push через Penpot MCP (см. penpot-setup.md)
```

Скрипт: [scripts/design/voice-tokens-to-penpot.py](../../scripts/design/voice-tokens-to-penpot.py) (`make penpot-tokens-export`).  
Источник токенов: [design/tokens/voice.tokens.json](../../design/tokens/voice.tokens.json).

**Дизайнерская работа** — **только** во **втором и следующих** фреймах строки (справа от канона). Туда кладут новые layout, эксперименты, placeholder для обсуждения.

**Принятие варианта в продукт** ≠ правка левого фрейма и ≠ перетаскивание draft на место канона. После merge в приложение shipped-snapshot обновит **скрипт** при следующей заливке; до этого канон остаётся как был.

```text
y=0:     [ Chat/List — as shipped ]  [ Chat/List — v2 draft ]  [ Chat/List — density experiment ]
         ↑ x=0, канон               ↑ вариант в работе         ↑ ещё вариант

y=900:   [ Chat/Room — as shipped ]  [ Chat/Room — composer v2 ]
         ↑ следующий экран ниже      ↑ сравнение в той же «строке сценария»
```

Правила:

- **Эксперимент / редизайн** → **справа** от канона, **на той же Y**; править **только** эти фреймы.
- **Следующий экран сценария** (другой `Screen/...` / `Panel/...`) → **новая строка ниже**; слева снова shipped snapshot (не трогать руками).
- Не ставить full-screen фреймы «куда попало» по X — только **0** (канон, read-only) или **правее** (вариант для дизайна). Случайный `x=1400` без связи со строкой = overlap с соседями.

### Размеры фреймов

| Тип | Размер |
|-----|--------|
| Desktop screen / panel / overlay (контекст shell) | **1280×800** |
| Mobile screen | **390×844** |
| Wireframe-колонка (узкий slice) | 320 / 960 / 300 × 800 — **та же строка**, что и related full-screen, если сравнивается одна колонка |

Между фреймами по **Y**: gap **≥ 100 px**. Между каноном и вариантом по **X**: gap **≥ 80 px**.

### Mobile chrome (высоты панелей)

На phone-фреймах **390×844** chrome фиксирован токенами — не «угадывать» и не оставлять default board 100×100:

| Зона | Высота | Токен |
|------|-------:|-------|
| AppBar / top header | **56** | `layout.headerHeight` |
| Search row (shell) | **44** | `layout.searchRowHeight` |
| BottomNav / composer chrome | **64** | `layout.bottomNavHeight` |

Content (`ChatList` / `Messages` / `Body`) — `verticalSizing = fill`, сумма chrome + content = **844**. `Inset / AppBar` — только горизонтальный gutter ≥16; высота inset = `headerHeight`, не auto/100.

### Как ставить новый фрейм

1. **Zoom out** — увидеть строки канон + варианты.
2. **Новый сценарий** (ещё нет строки) → shipped snapshot создаёт/обновляет **скрипт**; дизайнер добавляет **вариант** справа, когда нужна работа.
3. **Вариант** → **справа** от канона **этого же** сценария, та же `y`, имя с суффиксом (`· v2`, `· v3`, `· draft`, `· WIP`).
4. Перед сохранением: bounding boxes **не пересекаются** (gap ≥ 80 px по X, ≥ 100 px по Y).
5. В [screens.md](screens.md) — viewer URL **канона** (без суффикса `·`); варианты — по необходимости, помечены как draft.

### Именование

- Канон: `Screen/Chat/List` (без суффикса). Варианты: `Screen / Chat / List · v2` или `Screen / Chat / List · v3`.
- Penpot UI может показывать пробелы: `Screen / Chat / List` — тот же ID, что `Screen/Chat/List`.
- [generate-screens-md.mjs](../../scripts/design/generate-screens-md.mjs) в inventory попадают **только** каноны (имена **без** `·`).
- Не оставлять безымянные `Board` на верхнем уровне страницы.

### Telegram-parity · v3 batch

Отдельная волна макетов после Telegram-parity UX audit (2026-08-28). **Не смешивать** с общим polish `· v2` (pages 11–15).

| Rule | Detail |
|------|--------|
| **When required** | Любая поверхность из GAP inventory в [screens.md](screens.md) (Archive, AttachMenu, EmojiPicker, SendMenu, PinnedMessageBar, ScheduledStrip, VideoNoteBubble, Navigation · v3 slices) и контроли из [screen-controls.md](screen-controls.md), помеченные GAP / audit backlog в [todo/design.md](../todo/design.md) Critical |
| **Suffix** | Middle dot + space + `v3`: `Panel / Chat / AttachMenu · v3`. В prose/ID tables допустимо `Panel/Chat/AttachMenu` без пробелов |
| **Placement** | Справа от канона (`x≈0`) **или** справа от существующего `· v2` на **той же Y**; gap ≥ 80 px по X |
| **Relation to · v2** | `· v2` — общий design review / density polish. `· v3` — **normative Telegram-parity** (folders, QA, composer, pins, schedule strip, shell order). Новые audit-контроли добавлять в **v3**, не расширять v2 retroactively |
| **Registry** | Пока frame не promoted: только строка в GAP table + чеклист [todo/design.md](../todo/design.md); **не** добавлять в shipped table [screens.md](screens.md) |
| **Promotion** | После merge во Flutter — shipped snapshot обновляет **только** `make penpot-tokens-export`; draft `· v3` остаётся справа до закрытия audit item |
| **Traceability** | PR / задача ссылается на Screen ID + viewer URL draft `· v3`; при закрытии GAP — viewer URL в shipped table |

```text
y=0:   [ Chat/List — shipped ]  [ Chat/List · v2 ]  [ Chat/List · v3 — folders+QA rail slice ]
       ↑ x=0 read-only          ↑ polish batch      ↑ Telegram-parity audit batch
```

---

## 1.5. Отступы от края экрана и рамок (запрет нулевых inset)

Контент **не должен лизать** край screen/panel frame и внутренние рамки колонок. Это касается и заголовков (`Header`, section overline), и текста, и контролов — «это же title» не отменяет gutter.

### Минимум

| Что | Минимум | Токен |
|-----|--------:|-------|
| Отступ текста/контролов от левого и правого края колонки / frame | **≥ 16 px** | `space.16` |
| Отступ от верхнего/нижнего края content-зоны (не считая системный status area) | **≥ 16 px**, если нет отдельного chrome-header | `space.16` |

Предпочтительно задавать gutter через **flex `padding` контейнера** (`leftPadding` / `rightPadding` ≥ 16), а не надеяться только на `layoutChild.leftMargin` у `Text` с `horizontalSizing = fill` — в Penpot margin у fill-текста **не сдвигает** глифы (`parentX` остаётся 0 при `leftMargin = 16`).

Надёжный паттерн, если родитель колонки должен оставаться `leftPadding = 0` (ради `AccentBar` flush left, §3.5):

```text
Inset / Header          (flex, leftPadding=16, rightPadding=16, fill × auto)
└── Header              (text)
Inset / Overline / …    (flex, leftPadding=16, rightPadding=16; topMargin на wrap)
└── Overline / …
Nav / …                 (full width, AccentWrap @ 0, Label leftMargin=16)
```

Имя wrap: `Inset / …` — чтобы в Layers было видно, что это gutter, а не контент.

### Канон для колонок (nav / list / panel)

```text
Column (flex column, leftPadding=0, rightPadding=0)   ← full-bleed фон/AccentBar OK
├── Header / Overline     → leftMargin≥16 + horizontalSizing auto|fix
│                           ИЛИ обёртка с horizontalPadding≥16
├── Nav/List row (full width, leftPadding=0)
│   ├── AccentWrap @ 0    ← flush к краю строки (§3.5)
│   └── Label leftMargin=16
└── …
```

Полоска активного пункта (§3.5) по-прежнему **flush left у строки**. Gutter 16 — у **текста и иконок**, не у `AccentBar`.

### Нельзя

- `Header` / overline / title с `parentX = 0` и без padding родителя ≥ 16.
- Нулевой горизонтальный padding у контейнера, если прямые текстовые дети без рабочего inset.
- Считать full-bleed фон или `AccentBar` оправданием для текста на краю.

### Исключения (осознанный full-bleed)

- Фоновые `Board` / scrim на весь frame.
- `AccentBar` / разделители на всю ширину.
- Медиа edge-to-edge (story viewer, image lightbox), где продукт явно full-bleed — но chrome (close, caption) всё равно с inset ≥ 16.

---

## 2. Clip content и размеры контейнеров (типовые «пустые» экраны)

Симптом: фрейм выглядит **пустым серым прямоугольником**, контент виден только при **hover** (outline/selection на обрезанных дочерних слоях).

### Причина

Вложенный `Board` с **Clip content = on** уже **уже**, чем дети внутри. Penpot обрезает всё за границей; при наведении видны контуры полного bounding box.

### Правила

| Правило | Зачем |
|---------|--------|
| **Ширина контейнера списка/контента = ширина родительского screen/panel frame** | `Screen/Chat/List` 320 → контент 320, не 100. `Screen/Chat/Room` 960 → контент 960. |
| После duplicate / auto-layout / ручного resize — **проверить W×H каждого прямого ребёнка screen frame** | Именно так ломались List, Room, Social/Panel. |
| **Clip content** включать на screen frame и на строках списка — **только если** размер контейнера совпадает с содержимым | Не клипать «на всякий случай» узкий board. |
| Text-wrapper внутри строки (`List/Row`, 56 px высота): board с title/subtitle — **высота по тексту (~34 px)**, выровнен по центру строки | Не оставлять wrapper **100 px** с offset `y = -22` — текст и фон обрезаются. |
| Не менять ширину inner board отдельно от screen frame | Типичная ошибка при копировании колонки из shell. |

### Быстрая самопроверка перед commit в Penpot

1. Открыть фрейм **без hover** — весь контент читается?
2. Layers: у контейнера списка `W` = `W` экрана?
3. Экспорт / viewer preview — не обрезаны ли строки и timestamps справа?
4. (Опционально) MCP `export_shape` по frame ID из [screens.md](screens.md).

### Размеры wireframe-колонок (контент-контейнер = ширина frame)

| Тип slice | Frame W×H | Контент-контейнер |
|-----------|-----------|-------------------|
| Список чатов | 320×800 | 320 × (800 − header) |
| Room / main column | 960×800 | 960 × (800 − header − composer) |
| Side panel | 300×800 | 300 × (800 − header) |

---

## 3. Содержимое шаблонов (placeholder data)

Правила ниже — для **вариантов в работе** (фреймы справа от канона). **Shipped snapshot слева не редактируется дизайнером** (§1).

Макет-вариант — **не схема блоков**, а **понятный пример UI**. Разработчик и ревьюер должны по Penpot понять, что где лежит, без догадок по пустым прямоугольникам.

### Обязательно заполнять

| Элемент | Как |
|---------|-----|
| **Списки** | Минимум **2–3 строки** с разным содержимым |
| **Имена** | Реалистичные: `Alex`, `Maria`, `Design Team`, не `Title` / `Text` / `Name` |
| **Subtitle / preview** | Осмысленный текст: `See you in voice`, `Playing`, `New Penpot mock` |
| **Timestamps / meta** | `12:34`, `now`, `1h`, `yesterday` |
| **Статусы** | `online`, `Playing`, `AFK` |
| **Поиск / input** | Placeholder как в продукте: `Search`, `Message...` |
| **Аватары** | Круг 40×40, цвет из `profileAccent.*` ([tokens](tokens.md)), не пустой ellipse |
| **Заголовки секций** | `Friends`, `Chats`, имя чата в header room |
| **Сообщения в room** | 2–3 bubble с разным выравниванием (incoming/outgoing) |

### Хороший placeholder (принципы)

- **Список друзей / чатов:** 2–3 строки; у каждой — имя, короткий preview или статус, timestamp справа; цветной круг-avatar (`profileAccent.*`).
- **Room:** header с именем и статусом; 2–3 message bubble (incoming/outgoing); composer с `Message...`.
- **Search / inputs:** placeholder как в продукте (`Search`, `Message...`), не `Label` / `Input`.

Конкретные имена и тексты — **доменные примеры Voice**, не привязка к одному frame ID (канонические макеты меняются; смысл placeholder сохраняется).

### Плохо (не делать)

- Удалять текст и авatars при «чистке» шаблона — остаются безликие `Board` + `Rectangle`.
- Оставлять слой с именем `Text` и содержимым `Text`.
- Один пустой row «для примера» в списке.
- Generic lorem ipsum вместо доменных примеров Voice (чат, voice, matchmaking, space).

### Компоненты Foundation

См. **§3.6** — полные правила библиотеки, типографики и showcase. Кратко: все **main instances** и typographies живут **только** на `01_Foundation`; в макетах экранов — **instances**, не перерисовка.

---

## 3.5. Выделение активного пункта (AccentWrap)

Активный пункт списка / nav (Settings sidebar, tree row и т.п.) — **один** визуальный маркер: `AccentWrap` → `AccentBar` (3 px, `profileAccent.*`).

### Канон

| Правило | Значение |
|---------|----------|
| Полоска | `AccentWrap` → `AccentBar` (3 px) **у левого края** строки: `layoutChild.absolute = true`, `parentX = 0`; не в gutter контента |
| Контент | `Label` стартует на **той же X**, что невыбранные строки (`parentX = 16`): `leftMargin = 16` у `Label`, пока бар absolute |
| Невыбранные | flex `padding` L/R **16**; без `AccentWrap` |
| Chevron | `rightPadding = 16` на всех строках |

### Структура выбранной строки

```text
Nav / … (flex row, leftPadding=0, rightPadding=16, justify=start)
├── AccentWrap (absolute, 0×0, 3×44) → AccentBar
├── Label (fill, leftMargin=16)
└── Chevron (fix)
```

Полоска **не** участвует во flex-flow — иначе окажется на `parentX = 16` («висит в воздухе» между краем и текстом).

### Нельзя (двойное выделение)

- `AccentWrap` **и** горизонтальный `padding` родителя, который сдвигает полоску внутрь.
- `AccentWrap` **и** muted fill (`color.background.muted`) на той же строке.
- Inset-padding как «стягивание» выделения вместо полоски — выбрать **один** способ.

### Flex на выбранной строке

- `justifyContent = start`.
- `AccentWrap`: `absolute = true`, flush left.
- `Label`: `horizontalSizing = fill`, `leftMargin = 16`; `Chevron` — `fix`.

---

## 3.6. Библиотека компонентов и типографика (Assets)

Единый источник UI-паттернов для макетов и `lib/ui/core/*`. Нарушения (mains на страницах экранов, дубли `Member`, orphaned `Text/*` components) ломают Assets panel и массовые правки.

### Где что живёт (только `01_Foundation`)

| Board на `01_Foundation` | Содержимое | Кто правит |
|---------------------------|------------|------------|
| `Foundation / Swatches` | цветовые swatches | токены + дизайнер |
| `Foundation / Component Mains` | **все main instances** библиотеки (определения компонентов) | дизайнер / агент при новом компоненте |
| `Foundation / Components` | shipped snapshot — **instances** legacy набора (визуальный канон v1) | только скрипт заливки; не трогать mains здесь |
| `Foundation / Components v2` | showcase: typographies `type.*` + instances всех компонентов | дизайнер / агент |

> **Все компоненты библиотеки — только на `01_Foundation`.** Main instance компонента **не** создавать и **не** оставлять на `10_Screens_*`, `13_Panels_*`, `Root Frame` других страниц. На страницах экранов — **только instances** внутри `Screen/...` / `Panel/...` / `Overlay/...`.

Если main оказался не на `01_Foundation` (типично после duplicate или MCP без переноса страницы) — **перенести в UI** в `Foundation / Component Mains` на `01_Foundation`. MCP cross-page `appendChild` для mains **не работает** — перенос в Penpot UI или процедура **recreate** ниже.

### Перенос mains на `01_Foundation` (MCP recreate)

Когда main застрял на `10_Screens_Desktop` / `13_Panels_Desktop` (`Component Mains (library)` @ x≈12800):

1. На странице с main: `generateMarkup([main], { type: 'svg' })`.
2. На `01_Foundation`: `createShapeFromSvg` → `appendChild` в `Foundation / Component Mains`.
3. `library.createComponent([imported])`, выставить `path` / `name` как у старого.
4. На **всех** страницах: `swapComponent(newComp)` только для `isComponentCopyInstance() && isComponentHead()` (не вложенные части).
5. На **странице старого main**: `remove()` старого main (страница должна быть **active**).
6. Удалить пустой `Component Mains (library)` на экранных страницах.

`swapComponent` / `remove` — только при **active** странице, где лежит shape. Не гонять batch на все компоненты — Penpot cloud даёт 504; **один компонент = один `execute_code`**.

После переноса: все mains в `Foundation / Component Mains`; на `10_*` / `13_*` нет `Component Mains (library)` и нет mains на `Root Frame`.

### Именование в Assets

Конвенция **path / name** (Penpot Assets показывает как `Category/Variant`):

| Поле | Пример | Зачем |
|------|--------|-------|
| `path` | `Button`, `List`, `Row`, `ChatBubble` | группа в Assets |
| `name` | `Primary`, `Row`, `in`, `Setting` | вариант внутри группы |

- Имя main instance на canvas: `Category / Variant` (пробелы в UI: `Button / Primary`).
- **Запрещено** несколько разных компонентов с одним leaf `name` без `path` (исторические шесть `Default` — только с разными `path`: `TabBar`, `Composer`, `Divider`, …).
- **Запрещено** дублировать существующий паттерн новым компонентом (`Member` вместо `List/Row`) — расширять или reuse.

### Типографика (не Text-компоненты)

Стили текста — **Library → Typography**, зеркало [voice.tokens.json](../../design/tokens/voice.tokens.json) `type.*`:

| Typography | Токен |
|------------|-------|
| `type/display` | `type.display` |
| `type/title` | `type.title` |
| `type/body` | `type.body` |
| `type/bodyStrong` | `type.bodyStrong` |
| `type/label` | `type.label` |
| `type/labelStrong` | label 600 |
| `type/subtitle` | `type.subtitle` |
| `type/caption` | `type.caption` |
| `type/overline` | `type.overline` |

- В макетах: `applyTypography` / стиль из Assets → Typography.
- **Не** создавать `Text / Placeholder`, `Text / Title` и т.п. как **library components** — это orphaned refs, не попадают в Assets.
- Placeholder в input/composer — стиль `type/subtitle` или `type/caption` + цвет `color.text.disabled`, не отдельный компонент.

### Сборка макетов (instances, не рисование)

| Ситуация | Делать | Не делать |
|----------|--------|-----------|
| Кнопка, row, bubble, input | instance из Assets | новый `Board` + `Rectangle` |
| Строка списка / чата | `List/Row` + внутри `Avatar/40` | клон row с raw `Ellipse` |
| Nav / settings row | `Nav/Item` или `Row/Setting` | duplicate layout вручную |
| Имя, дата, статус в header | raw `Text` + typography | component instance |
| Layout-обёртка колонки | raw `Board` + flex | — |

Повторяющийся UI без instance в Layers → ошибка сборки; перед PR заменить на component.

### Канонические компоненты (ориентир)

UI widgets (library components): `Button/*`, `Avatar/40`, `List/Row`, `ChatBubble/in|out`, `Nav/Item`, `Row/Friend|Game|Setting|Bot`, `Composer/Default`, `Input/Field`, `SearchBar/Default`, `AppBar/Default`, `TabBar/Default`, `Divider/Default`, `AccentWrap/Default`, `State/Empty`, `Banner/Offline`.

Связки:

- `List/Row` → child instance `Avatar/40`, не raw ellipse.
- Списки чатов / members в v2-макетах → `List/Row`, не отдельный `Member`.

### Новый компонент

1. Создать main на `01_Foundation` → внутри `Foundation / Component Mains` (flex column, gap ≥ 16).
2. `path` + `name` по таблице выше; placeholder по §3.
3. Добавить instance в `Foundation / Components v2` (showcase) — label + instance в двухколоночной сетке showcase.
4. В draft-макетах — только **instances**; main не копировать на страницы экранов.
5. Flutter parity — `lib/ui/core/*` + токены.

### Showcase `Foundation / Components v2`

- Секция **Typography** — samples с `type.*` typographies.
- Секция **Components** — label (`Category/Variant`) + instance; все компоненты из Assets, без broken refs.
- Дети showcase — **instances**, не mains; mains только в `Component Mains`.
- После правок layout: absolute позиции в столбик (label слева, component справа), gap 16 px, padding 24 px; board height по контенту.

---

## 4. Workflow: новый или обновлённый макет

1. **Spec** — `docs/features/`, [brand.md](brand.md).
2. **Shipped snapshot** — только `make penpot-tokens-export` + заливка в Penpot (§1); дизайнер **не** трогает `x≈0`.
3. **Дизайн** — duplicate канона → вариант справа (`· v2` / `· v3` / `· draft`); правки только там. Telegram-parity audit surfaces → **`· v3`** (§ «Telegram-parity · v3 batch»).
4. **Размещение** — §1: канон ↓, варианты →, без overlap.
5. **Сборка варианта** — §1.5 inset ≥ 16; §2 clip; §3 placeholder; §3.5 AccentWrap; §3.6 instances из Assets.
6. **Инвентарь** — frame ID канона в [screens.md](screens.md) (без `·`).
7. **PR** — ссылка на **вариант** для review; после ship в app — snapshot обновит скрипт.

---

## 5. Чеклист ревью Penpot (PR / перед merge)

- [ ] **Shipped snapshot (`x≈0`) не изменён вручную** — только скрипт заливки токенов
- [ ] Дизайн-правки только в вариантах справа (`·` в имени); канон в `screens.md` без суффикса
- [ ] Telegram-parity GAP surfaces framed as `· v3`, not retrofitted into `· v2` (§ «Telegram-parity · v3 batch»)
- [ ] Канон слева; варианты справа; нет overlap; новый сценарий — строкой ниже
- [ ] Имя `Screen/...` / `Panel/...` / `Overlay/...` по конвенции
- [ ] Контент-контейнеры = ширина frame; нет clip с overflow > 5 px
- [ ] Mobile chrome: AppBar=`headerHeight` (56), BottomNav/composer=`bottomNavHeight` (64); content fill до 844
- [ ] Строки списка 56 px; text-wrapper не 100 px
- [ ] Есть примеры текста, авatars, timestamps (не пустые блоки)
- [ ] Viewer URL добавлен/актуален в `screens.md`
- [ ] Цвета из tokens, не случайный hex
- [ ] Активный nav/list row: только `AccentWrap` flush left; нет dual padding + muted fill (§3.5)
- [ ] Нет нулевых inset: текст/контролы ≥ 16 px от края frame/колонки; Header и overline не на `parentX = 0` без padding (§1.5)
- [ ] Все library mains на `01_Foundation` → `Foundation / Component Mains`; на страницах экранов нет main instances на `Root Frame` (§3.6)
- [ ] Повторяющийный UI — component instances; typographies `type.*`, не `Text/*` library components (§3.6)
- [ ] Списки — `List/Row` + `Avatar/40`; нет дублей (`Member` и т.п.) (§3.6)

---

## 6. MCP / агент

- **Не** править shipped snapshot (`x≈0`) через MCP — только скрипт заливки токенов (§1).
- **Не** создавать component mains вне `01_Foundation` (§3.6); cross-page перенос mains через MCP не поддерживается — перенос в Penpot UI.
- Варианты и массовые шаблоны — через MCP (`execute_code`):

- явно задавать `resize(width, height)` контейнерам после `appendChild`;
- не полагаться на default width 100 px у nested board;
- после правок — `export_shape` по frame ID;
- скрипт inventory: [generate-screens-md.mjs](../../scripts/design/generate-screens-md.mjs).

Подключение MCP: [penpot-setup.md](penpot-setup.md) § Cursor + Penpot MCP.
