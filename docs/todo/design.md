# TODO — Design

[← Индекс](../TODO.md)

Penpot, design tokens, screen frames, parity дизайн ↔ Flutter (`docs/design/`, `design/tokens/`).

## Critical

_Пока пусто._

## High

### Penpot v2 — review и polish

- [ ] **Design review v2 (pages 11–15)** — пройти все `· v2` в Penpot viewer, собрать замечания или approve. Канон (`x≈0`) не трогать; `screens.md` для draft с `·` не обновлять ([penpot-workflow.md](../design/penpot-workflow.md) §1).
- [x] **Fix `11_Screens_Mobile` · v2 chrome** — AppBar/BottomNav высоты и порядок flex; oversized Primary с desktop clone. Токены `layout.bottomNavHeight` / `layout.searchRowHeight` (+ doc).
- [ ] **Polish `13_Panels_Desktop` panels 1–16 · v2** — parity с panels 17–33: AccentWrap, list rows 56px, avatars `profileAccent.*`, Voice placeholders, inset ≥16.
- [ ] **Visual fixes v2 (spot-check)** — починить баги из экспорта: Mute double-layer на `Panel / Chat / Info · v2`, clip/overflow где видно.

### Missing buttons — audit `10_Screens_Desktop`

Сверка UI page 10 (канон + `· v2`) с `docs/FEATURES.md` / `docs/features/*`. Панели/оверлеи на 13–15 есть, но **нет entry-point кнопок** на экранах → фича недоступна из текущего UI. Есть на Room · v2: `File` (attach) + `Send`; emoji и остальное — нет.

#### `Screen / Chat / Room` (+ composer в `Shell / Desktop · v2`)

Composer / ввод ([text-chat.md](../features/text-chat.md)):

- [ ] **Emoji** — открыть emoji picker (и кастомные эмодзи спейса)
- [ ] **GIF**
- [ ] **Stickers** — системные + свои паки
- [ ] **Voice message** — запись голосового (hold/tap mic)
- [ ] **Markdown / format toolbar** — bold / italic / spoiler / code (или явный affordance)
- [ ] **Attach на Shell · v2** — `File` есть только на `Chat / Room · v2`, в shell-composer нет
- [ ] **Иконки на File / Send** — сейчас пустые квадраты без glyph

Header чата ([voice-chat.md](../features/voice-chat.md), [search.md](../features/search.md), [text-chat.md](../features/text-chat.md)):

- [ ] **Voice call** (DM / start temporary voice for group)
- [ ] **Video call** (DM)
- [ ] **Search in chat** — лупа → поиск по текущему чату + prev/next
- [ ] **Chat info / more** — вход в `Panel / Chat / Info` (пины, shared media, участники, mute, E2E…)
- [ ] **Pinned messages** — плашка/кнопка к закреплённым
- [ ] **Thread** — вход в тред (канал/группа) → `Panel / Chat / Thread`

Действия на сообщении (hover / context / selection bar):

- [ ] **React** — добавить реакцию
- [ ] **Reply**
- [ ] **Forward** (+ multi-select → `Panel / Chat / ForwardMessage`; [forward-messages.md](../features/forward-messages.md))
- [ ] **Copy as new** — пересылка без атрибуции
- [ ] **Edit** / **Delete**
- [ ] **Pin** / **Unpin**
- [ ] **Report message** → `Panel / Report / Sheet`
- [ ] **Open thread** (если треды вкл.)
- [ ] **Select** — режим мультивыбора

Метаданные (не «кнопки», но нет affordance в макете): timestamps, delivery/read ticks (DM), view count (group/channel).

#### `Screen / Shell / Desktop` · `Chat / List`

([navigation.md](../features/navigation.md), [search.md](../features/search.md), [text-chat.md](../features/text-chat.md), [multi-profile.md](../features/multi-profile.md), [presence.md](../features/presence.md)):

- [ ] **New chat / New DM**
- [ ] **Create group** → `Panel / Chat / CreateGroup`
- [ ] **Create / join space** → `Panel / Space / Create` / `JoinInvite`
- [ ] **Folder tabs** — ЛС / Группы / Каналы / Спейсы / custom (+ на v2 пропала даже «Все» из канона)
- [ ] **Edit folders** / manage custom folders
- [ ] **Global search** — явная лупа/фокус (поле Search есть; нет CTA в rail)
- [ ] **Settings** — вход в settings shell / `Panel / Settings / Sheet`
- [ ] **Profile switcher** — меню профилей аккаунта
- [ ] **Presence / status picker** — Online / Idle / DND / Invisible + custom status (Plus)
- [ ] **Stories ring / Create story** — кольцо на аватаре + вход в create
- [ ] **Matchmaking entry** — вход в каталог / правую панель ММ ([matchmaking.md](../features/matchmaking.md))
- [ ] **Chat list row actions** — pin, mute, archive, mark read, delete DM (context)
- [ ] **Message requests folder** — папка «Запросы» для DM от незнакомцев
- [ ] **Rail labels** — `Nav / 0…2` без семантики (chats / social / MM / settings?)

#### `Screen / Social / Panel`

([friends.md](../features/friends.md), [user-profile.md](../features/user-profile.md)):

- [ ] **Add friend** / search people
- [ ] **Tabs** — Friends / Pending / Blocked (или отдельные секции)
- [ ] **Accept / Decline** friend request
- [ ] **Message** / **Call** / **Profile** на строке друга
- [ ] **Remove friend** / **Block**

#### Settings screens

`Privacy · v2` — сейчас 4 ряда; по [privacy.md](../features/privacy.md) не хватает контролов/входов:

- [ ] **Privacy preset** — Личный / Игровой / Рабочий
- [ ] **Who can call** / **invite to chats·spaces** / **send files** / **send voice** / **add as friend**
- [ ] **Visibility**: in-game status, MM rating, phone, stories, search by phone
- [ ] **Disallow forwarding my messages**
- [ ] **Blocked users** — не только счётчик, а кнопка → список

`Security · v2`:

- [ ] **Linked devices** (было в каноне, в v2 нет)
- [ ] **Delete account** / data export (если в спеке auth — уточнить при отрисовке)
- [ ] **E2E keys / sessions** entry (если живёт в security, не только в chat info)

`Notifications · v2`:

- [ ] **Quiet hours (scheduled DND)**
- [ ] **Per-chat / per-space overrides** (канон: «Per-chat overrides»; в v2 нет)
- [ ] **@mention breaks mute** toggle
- [ ] **Notification type toggles** — friend request, MM match, reactions, replies…

`Subscription · v2`:

- [ ] **Billing history**
- [ ] **Manage profiles** → downgrade / profile list
- [ ] **Cancel / manage plan** (кроме Upgrade + Restore)

#### Matchmaking screens

- [ ] **Add game** на `GameCatalog` ([matchmaking.md](../features/matchmaking.md))
- [ ] **Browse all** / filters (канон имел Browse all)
- [ ] **Region / rank / criteria** на `GameDetail` (сейчас почти пусто vs спека)
- [ ] **Join voice** на `MatchSquad` (канон имел; v2 — только Start queue)
- [ ] **Find players** CTA на `MatchHistory · v2` (канон имел)
- [ ] **Rate / report player** entry после матча (оверлей есть на 15; кнопки с history/squad — нет)
- [ ] **Open player profile** на строке сквада → `Panel / Matchmaking / PlayerProfile`

#### Stories screens

([stories.md](../features/stories.md)):

`Create · v2` (урезано vs канон/спека):

- [ ] **Audience** picker
- [ ] **Game tag**
- [ ] **LFP / «Ищу пати»** type + Join CTA preview
- [ ] **Text / Video / Clip** modes (не только Photo)
- [ ] **Editor tools** — text overlay, stickers, doodle, filters, trim
- [ ] **Mention** users in story

`Viewer · v2`:

- [ ] **Reply** (private DM) — канон имел Reply; v2 только «Tap to react»
- [ ] **React** — явная кнопка/picker, не только hint
- [ ] **Viewers** (author) → `Panel / Stories / StoryViewers`
- [ ] **Delete** / **Share** / **Close**
- [ ] **Join / Message** на LFP-стори

`Archive · v2` / `Highlights · v2`:

- [ ] **Create story** (канон archive)
- [ ] **Add to highlight** / **Add highlight** (канон highlights)
- [ ] **Highlight privacy** / edit → `Panel / Stories / HighlightEdit`

#### Auth / Profile / Bots

- [ ] **Login: phone / OTP** рядом с email ([auth-and-contacts.md](../features/auth-and-contacts.md))
- [ ] **Forgot password**
- [ ] **Guest convert** entry после guest nickname (панель есть на 13)
- [ ] **Bots / Install**: review **Permissions** detail (канон имел row; v2 — только Add to space)
- [ ] **DowngradePicker**: явные **checkboxes** выбора профилей (сейчас только rows + Keep selected)

#### Не на page 10, но нет entry с экранов page 10

Кнопки открытия существующих панелей/оверлеев (13/15) — добавить на shell/room/social при отрисовке:

- Screen share / mute / camera / PTT / raise hand / start broadcast / record — call chrome ([voice-chat.md](../features/voice-chat.md), [screen-share.md](../features/screen-share.md); оверлеи на 15)
- Slash-command affordance `/` в composer → `Panel / Chat / SlashCommand*`
- Report user/space/story entry points
- Space tree / invites / roles / bots / slow mode / overrides — entry из space context (панели на 13 есть)

## Common

### Penpot

Penpot = active design tool ([penpot-setup.md](../design/penpot-setup.md)); правила макетов — [penpot-workflow.md](../design/penpot-workflow.md); Figma — legacy ([figma-setup.md](../design/figma-setup.md)). Runtime tokens stay in git.

- [x] **Penpot file rename** — UI name `Voice` (file ID `20d3f736-cc1b-8043-8008-561cb65228ef`).
- [x] **Penpot workflow doc** — [penpot-workflow.md](../design/penpot-workflow.md): clip/контейнеры, вертикальный канон + варианты по X, placeholder content.
- [x] **Penpot v2 spread (pages 11–15)** — эталон с `10_Screens_Desktop` (21 `· v2`) размножен на mobile screens, states, desktop/mobile panels, overlays. Итого **89** draft-фреймов `· v2` справа от канона; shipped snapshot не менялся.

- [ ] **Orphan cleanup `10_Screens_Desktop`** — удалить stray top-level boards: `Board`, `BlockerCard`, `SettingsNav` @ `x≈0, y≈0` (не канон, не `· v2`).
- [ ] **export_shape QA (pages 11–15)** — по 2–3 фрейма с каждой страницы; viewer URLs для PR (только draft `· v2`, не канон).
- [ ] **Missing buttons pass (`10_Screens_Desktop`)** — закрыть чеклист выше на draft `· v2` (иконки + entry points); канон не трогать.

## Low

### Design ↔ code parity

- [ ] **Flutter chat/auth polish** — further list/room density vs Penpot (optional follow-up).
- [ ] **Flutter v2 parity (после approve)** — перенести утверждённые `· v2` в `src/frontend/lib/ui/`; shipped snapshot в Penpot обновить **только** скриптом заливки токенов (`make penpot-tokens-export`), не ручным merge в левый фрейм.


**Промпт-якорь:** `Design from docs/todo/design.md` + приоритет.
