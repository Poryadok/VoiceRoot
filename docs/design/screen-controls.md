# Screen Controls — кнопки и интерактивные элементы по экранам

> **Source of truth для Penpot-отрисовки и Flutter-реализации.**
> Собрано из `docs/features/*.md`. Не дублирует продуктовую логику — ссылки на спеки.
> При расхождении `features/*.md` побеждает.
>
> **Numbering:** top-level `## N` are screens/panels; nested `### N.M` / `### N.Ma` are subsurfaces of the same screen (strict hierarchy, not a flat 1…N list). UI copy uses i18n EN+RU keys; Russian strings in this file are the product baseline where feature docs specify them. Product term for spaces is **Спейс** ([GLOSSARY.md](../GLOSSARY.md)).

## Условные обозначения

| Сокр.   | Значение                                       |
|---------|-------------------------------------------------|
| **H**   | Горизонтальный layout (desktop / web / tablet) |
| **V**   | Вертикальный layout (mobile)                    |
| **H+V** | Оба                                            |
| ★       | Только по подписке (Plus / Space Pro)           |
| ctx     | Контекстное меню (ПКМ desktop / long-press mobile) |
| DM      | Личный чат 1:1                                  |
| GRP     | Группа (`type = group`)                         |
| CH      | Канал (`type = channel`)                        |
| SP      | Внутри спейса                                   |

---

## 1. Shell — основная оболочка

**Penpot:** `Screen / Shell / Desktop`, `Screen / Shell / Mobile`, `Screen / Shell / MobileChatOpen`, `Screen / Chat / List`; chrome panels `Panel / Shell / Navigation`, `Panel / Shell / SideHost`
**Feature docs:** [navigation.md](../features/navigation.md), [multi-profile.md](../features/multi-profile.md), [presence.md](../features/presence.md), [stories.md](../features/stories.md), [search.md](../features/search.md)

### 1.0 Screen / Chat / List

**Penpot:** `Screen / Chat / List`  
Dedicated frame for the chat-list column (desktop middle column / mobile full list). Same controls as §1.2–1.4; not a separate product surface from Shell.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Header: search + compose | H+V | Always | See §1.2 |
| 2 | Chat list rows | H+V | Always | See §1.4 |
| 3 | Empty list state | H+V | No chats in current folder filter | Show empty copy + compose CTA (→ §54 `State / Chat / Empty` pattern) |

### 1.1 Rail / Tab bar

**Desktop rail (top → bottom):** Nav (Chats / Social / Match) → Folders zone (§1.1b) → Quick Access (§1.1c) → spacer → ProfileStack → ☰ Settings (bottom). **Settings tab removed** from top nav — only ☰ under ProfileStack.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Chats tab | H+V | Always | Show chat list |
| 2 | Social / Friends tab | H+V | Always | Show social panel |
| 3 | Matchmaking tab | H+V | Always | Open MM catalog |
| 4 | Folders zone | H — rail; V — drawer / compact rail | Always | See §1.1b |
| 5 | Quick Access zone | H — rail; V — drawer / compact rail | Always | See §1.1c |
| 6 | Profile avatar | H — rail (ProfileStack); V — tab bar or header | Always | Open profile switcher menu (§1.1a); V: also swipe on avatar to switch profiles per multi-profile.md |
| 7 | Stories ring | H+V | User has active story (overlay on profile avatar) | Open story viewer |
| 8 | ☰ Settings | H — rail bottom; V — drawer or tab bar | Always | Open `Panel / Settings / Sheet` (§5.0) |

### 1.1a Profile avatar context menu

Opens from §1.1 #6 (ПКМ desktop / long-press mobile):

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Profile row (per profile) | H+V | Always | Switch active profile; shows verification badge per profile if verified |
| 2 | Create profile | H+V | < profile limit (2 free / 5★) | Navigate to Panel / Profile / Create |
| 3 | Presence picker (Онлайн / Не активен / Не беспокоить / Невидимый) | H+V | Always | Set presence status |
| 4 | Custom status ★ | H+V | Plus subscriber | Open custom status editor (arbitrary text + emoji as one surface per presence.md) |
| 5 | Create story | H+V | Always | Navigate to Screen / Stories / Create |
| 6 | **Archive** | H+V | Always | Open `Screen / Chat / Archive` (§1.10) |

### 1.1b Folder rail item

Each folder in rail scroll zone (system + custom):

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Folder row (Все / ЛС / Группы / Каналы / Спейсы / custom) | H — rail; V — drawer | Always | Filter chat list by folder; no folder tabs in middle column |
| 2 | Unread badge on folder | H+V | Folder has unread chats | — (visual: aggregate unread count) |
| 3 | Folder tooltip | H | Hover on folder | — (display folder name + unread summary) |
| 4 | Edit folders | H — icon at bottom of folders zone; V — ctx / menu | Always | Manage / reorder / create custom folders |
| 5 | Message requests | H+V | Feature enabled; pending requests exist | Open requests folder |

### 1.1c Quick Access slot

Rail slots for pinned-profile chats (≠ folder pin, ≠ Friends favourites):

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Quick Access chat row | H — rail; V — drawer / strip | Chat in quick access (≤15 per profile) | Open chat |
| 2 | Unread badge | H+V | Chat has unread | — (visual) |
| 3 | Add to Quick Access (ctx) | H+V | ctx on chat list row | Add chat to rail quick access |
| 4 | Remove from Quick Access (ctx) | H+V | ctx on quick access row | Remove from rail |
| 5 | Drag reorder | H | Desktop | Reorder quick access slots |
| 6 | Quick Access limit reached | H+V | Already 15 chats in quick access | — (display feedback) |

### 1.2 Chat list header

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Search field | H+V | Always | Focus → global search (Panel / Search / Global) |
| 2 | New chat / compose | H+V | Always | Show menu: New DM · Create group · Create/join space |
| 3 | New DM (submenu) | H+V | From #2 | Open contact picker → create DM |
| 4 | Create group (submenu) | H+V | From #2 | Open Panel / Chat / CreateGroup |
| 5 | Create space (submenu) | H+V | From #2 | Open Panel / Space / Create |
| 6 | Join space (submenu) | H+V | From #2 | Open Panel / Space / JoinInvite or catalog |

### 1.3 Folder tabs — **Removed (deprecated tombstone)**

> **Do not draw.** Folder filter lives only in rail §1.1b. This subsection is a cross-ref tombstone only — controls moved to §1.1b #1 and #4.

### 1.3a Message request row (inside requests folder)

**Penpot:** `Screen / Shell / MessageRequests` (or row variant on `Screen / Shell / Desktop` / `Mobile`)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Sender avatar + name + message preview | H+V | Always | Preview message |
| 2 | Accept | H+V | Always | Accept request → move to main chat list |
| 3 | Decline | H+V | Always | Decline request → hide conversation |

### 1.4 Chat list row (per chat)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Row tap | H+V | Always | Open chat room |
| 2 | Stories ring on avatar | H+V | Contact has active story | Open story viewer |
| 2a | Presence / status dot on avatar | H+V | DM / friend chat; presence visible per privacy | — (visual: online / idle / DND / offline) |
| 3 | Pin (ctx) | H+V | ctx | Toggle pin to top **within current folder** (≠ Quick Access — §1.1c) |
| 4 | Mute / Unmute (ctx) | H+V | ctx | Toggle mute |
| 5 | Archive (ctx) | H+V | ctx (DM) | Move to archive — **secondary**; primary entry §1.1a #6 |
| 6 | Mark read / unread (ctx) | H+V | ctx | Toggle read state |
| 7 | Delete chat (ctx) | H+V | ctx (DM) | Delete chat for self |
| 8 | Draft indicator | H+V | Chat has unsent draft on this device | — (visual: green "Черновик:" prefix before message preview, like Telegram) |
| 9 | Premium badge ★ | H+V | Chat counterpart / sender profile has personal subscription | — (visual indicator near display name) |
| 10 | Add to folder (ctx) | H+V | ctx, custom folders exist | Pick folder to add chat to |
| 11 | Add to Quick Access (ctx) | H+V | ctx | Add chat to rail quick access (§1.1c) |
| 12 | Unread count / bold unread state | H+V | Chat has unread messages | — (visual: bold title + unread badge/count on list row) |
| 13 | Muted indicator | H+V | Chat is muted | — (visual: mute icon on row) |
| 14 | Pinned indicator | H+V | Chat is pinned in folder | — (visual: pin icon on row) |
| 15 | Delivery ticks in preview | H+V | DM, last message outgoing | — (visual: ✓ delivered, ✓✓ read in subtitle) |
| 16 | Media type label in preview | H+V | Last message is media without text preview | — (visual: Photo / Video / Voice / File / Sticker / GIF / **Article** / **Location** / **Music** / **Video message**) |
| 17 | Call preview label | H+V | Last message is call system event | — (visual: Missed call / Call / Video call) |

### 1.5 Active / pinned chats column — **Removed (H)**

> **Desktop active column removed.** Replaced by rail Quick Access (§1.1c) + folder pin (§1.4 #3) + mobile active strip (§1.6). Do not draw a separate left column of active chats.

### 1.6 Active chats strip (V only)

**Normative:** chat enters strip when user **opens** it on mobile while other chats remain open; max **100** entries (LRU evict oldest when opening 101st). Strip shows **visible cap ~8** avatars; horizontal scroll for overflow. Closing chat (back to list) **removes** from strip unless unread (keep with badge until read — **target-state**; until target-state ships, may remove on back). Keyboard open: strip **collapses** under app bar; pinned bar + composer take priority (§1.6a).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Mini avatar icon | V | Chat open, other active chats exist | Switch to that chat |
| 2 | Unread badge on icon | V | Chat has unread | — (visual indicator) |
| 3 | Scroll left/right | V | > visible width | Scroll strip |
| 4 | Active-chats limit reached | V | User already has 100 active chats | — (display feedback: cannot add more to strip; limit 100) |
| 5 | Close chat (× on strip icon) | V | Long-press strip icon | Remove from strip without opening |

### 1.6a Mobile shell chrome stacking

Vertical order when chat open (`Screen / Shell / MobileChatOpen`):

1. System status bar  
2. Active strip (§1.6) — hidden when keyboard open  
3. Chat header (§3.1)  
4. PinnedMessageBar (§3.1a) — collapses to single line if >1 pin  
5. Timeline  
6. Composer (§3.6) — resizes with keyboard  
7. Bottom tab bar (§1.1) — **hidden when keyboard open** on Android/iOS

**Mobile IA:** bottom tab bar (Chats / Social / MM) + **hamburger drawer** for folders, Quick Access, and Settings; no second bottom bar. See § H vs V summary.

### 1.7 Screen / Shell / MobileChatOpen

**Penpot:** `Screen / Shell / MobileChatOpen`  
**Alias / relation:** mobile variant of Shell with a chat room pushed open (navigation.md: list collapses to horizontal strip). Not a separate Screen ID family from `Screen / Shell / Mobile`.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Active chats strip | V | Chat room open | See §1.6 |
| 2 | Chat room body | V | Chat selected | Same as §3 Chat Room (header / timeline / composer) |
| 3 | Back to list | V | Always | Pop room → `Screen / Shell / Mobile` list |
| 4 | Bottom tab bar | V | Always | Same as §1.1 (may hide while keyboard open — platform chrome) |

### 1.8 Panel / Shell / Navigation

**Penpot:** `Panel / Shell / Navigation`  
Rail / nav chrome extracted as a panel frame for Penpot (desktop). Controls mirror §1.1; space-tree entry when inside a space.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Chats toggle | H | Always | Show chat list column |
| 2 | Social / Friends toggle | H | Always | Show social panel |
| 3 | Matchmaking entry | H | Always | Open MM catalog |
| 4 | Folders zone | H | Always | See §1.1b |
| 5 | Quick Access zone | H | Always | See §1.1c |
| 6 | Profile avatar | H | Always | Open profile switcher (§1.1a) |
| 7 | ☰ Settings | H | Always | Open `Panel / Settings / Sheet` (§5.0) |
| 8 | Home / exit space | H | Inside space tree context | Exit space → chat list |
| 9 | Space tree (embedded) | H | Space active | Navigate channels/rooms (see §10.3) |

### 1.9 Panel / Shell / SideHost

**Penpot:** `Panel / Shell / SideHost`  
Chrome host for right-side content (desktop). Not a product feature by itself — hosts one **normative mode** at a time (table below) or a transient reaction picker.

**Normative modes** (mutually exclusive; only one body mode open):

| Mode | Body embeds | Entry (H) | Entry (V) | Visible when |
|------|-------------|-----------|-----------|--------------|
| **info** | `Panel / Chat / Info` | §3.1 #6 toggle (default first open); kebab → Info | Avatar/name tap or kebab → **bottom sheet** | Always |
| **members** | `Panel / Chat / GroupMembers` (GRP) or `Panel / Space / Members` (space) | §3.1 #6; Chat Info members row | Chat Info / header → **bottom sheet** or **full-screen push** | Group/space with member list |
| **thread** | `Panel / Chat / Thread` | §3.1 #8; §3.3 #6; §3.4 #9; or §3.1 #6 | Same entries → **full-screen push** (preferred on V) | Threads enabled (GRP/CH) |
| **search** | In-chat search results (matches from §3.2; prev/next in header bar) | §3.1 #6 after §3.2 active with results | §3.2 search bar + results → **bottom sheet** | In-chat search active with ≥1 match |

**Layout:** H — right column ~300 px via §3.1 #6. V — no persistent side column; modes use bottom sheet (info / members / search) or full-screen push (thread) per H/V summary § «Panels».

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Host title | H | Side panel open | — (display: mode title — Info / Members / Thread / Search) |
| 2 | Close | H+V | Always | Close side panel / sheet → return focus to room (Escape — §3.6e) |
| 3 | Body: Group / Space members | H | Members mode | Embed `Panel / Chat / GroupMembers` or `Panel / Space / Members` |
| 4 | Body: Chat info | H | Info mode | Embed `Panel / Chat / Info` |
| 5 | Body: Thread | H | Thread mode | Embed `Panel / Chat / Thread` |
| 6 | Body: Search results | H | Search mode | Scrollable match list; row tap → jump to message (§3.2) |
| 7 | Body: Emoji / reaction picker | H | Reaction mode (transient) | Pick emoji → apply reaction / insert; **not** a persistent SideHost mode — closes after pick |

### 1.10 Screen / Chat / Archive

**Penpot:** `Screen / Chat / Archive`  
**Feature docs:** [text-chat.md](../features/text-chat.md) § «Архивирование»  
**Entry:** §1.1a #6 (profile avatar ctx → Archive). Not a row in main chat list.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return to chat list |
| 2 | Title «Архив» | H+V | Always | — (display) |
| 3 | Archived chat row | H+V | Has archived chats | Open chat room |
| 4 | Unarchive (ctx) | H+V | ctx on row | Restore to main list |
| 5 | Unarchive (swipe) | V | Swipe on row | Restore to main list |
| 6 | Empty archive state | H+V | No archived chats | — (display empty copy) |

---

## 2. Auth screens

**Penpot:** `Screen / Auth / Login`, `Screen / Auth / GuestNickname`
**Feature docs:** [auth-and-contacts.md](../features/auth-and-contacts.md)

### 2.1 Login

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Email field | H+V | Always | Input email |
| 2 | Phone field | H+V | Always (tab or toggle) | Input phone number |
| 3 | Password field | H+V | Email mode | Input password |
| 4 | OTP field | H+V | Phone mode, after send | Input OTP code |
| 5 | Send OTP | H+V | Phone mode | Send SMS / email OTP |
| 6 | Forgot password | H+V | Email mode | Open password reset flow |
| 7 | Log in | H+V | Email + password filled | Submit login |
| 8 | Register | H+V | Always | Switch to registration form |
| 9 | Continue as guest | H+V | Always | Create guest account → GuestNickname |
| 10 | Rate limit error | H+V | 5 failed attempts / 15 min from same IP | — (display: "Слишком много попыток. Повторите через N минут") |
| 11 | OTP rate limit feedback | H+V | Phone mode, OTP requested too frequently | — (display: countdown timer until next OTP allowed; 3 attempts / 10 min, new code ≥ 1 min apart) |

### 2.2 GuestNickname

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Nickname field | H+V | Always | Input nickname |
| 2 | Join as guest | H+V | Nickname filled | Create guest → main shell |

### 2.3 Guest convert reminder (banner, not screen)

**Penpot:** banner variant on `Screen / Shell / Desktop` / `Mobile` (not a standalone page)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Register CTA | H+V | Guest, ≥ 2nd session, ≤ 1/day | Open Panel / Auth / GuestConvert |
| 2 | Dismiss | H+V | Same | Hide banner for today |

### 2.4 Guest Convert (Panel / Auth / GuestConvert)

**Penpot:** `Panel / Auth / GuestConvert`
**Feature docs:** [auth-and-contacts.md](../features/auth-and-contacts.md)

Entry: from guest convert reminder CTA (#1 in §2.3).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Email field | H+V | Always | Input email |
| 2 | Password field | H+V | Always | Input new password |
| 3 | Convert account | H+V | Both fields filled | POST convert-guest → account becomes regular |
| 4 | Cancel | H+V | Always | Close modal |

---

## 3. Chat Room

**Penpot:** `Screen / Chat / Room`
**Feature docs:** [text-chat.md](../features/text-chat.md), [voice-chat.md](../features/voice-chat.md), [search.md](../features/search.md), [forward-messages.md](../features/forward-messages.md), [encryption.md](../features/encryption.md)

### 3.1 Header

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back arrow | V | Always | Return to chat list |
| 2 | Avatar + Name + Status | H+V | Always | Open Panel / Chat / Info (H) or push screen (V). Status: online → in call → custom status → **last seen** → offline per [presence.md](../features/presence.md); respect privacy |
| 3 | Voice call | H+V | DM or GRP (temporary voice, linked to group chat_id) | Start voice call (DM) / create temp voice room (GRP) → Call overlay |
| 4 | Video call | H+V | DM only | Start video call → Call overlay |
| 5 | Search in chat | H+V | Always | Toggle in-chat search bar |
| 6 | Side panel toggle | H | Always | Toggle `Panel / Shell / SideHost` — modes: info / members / thread / search (§1.9). Icon: **`icon.sidePanel`** (layout-sidebar); a11y: «Информация о чате» / `chat.sidePanel`. **V:** no header toggle — same modes via bottom sheet / full-screen push (§1.9 table) |
| 7 | Pinned messages | H+V | Has pinned messages | Open pinned list; persistent bar below header when pins exist — see §3.1a |
| 8 | Thread (sidebar icon) | H | GRP/CH with threads enabled | Toggle Panel / Chat / Thread sidebar |
| 9 | More / kebab | H+V | Always | ctx → Mute, Report, Info, E2E toggle (DM), etc. |
| 10 | E2E lock icon | H+V | DM with E2E enabled | Visual indicator; tap → E2E info / verification code |
| 11 | Typing indicator | H+V | Someone is typing | — (display: localized typing string via i18n; e.g. RU "{Name} печатает…") |

### 3.1a PinnedMessageBar

Persistent strip directly under chat header (above timeline):

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Pin thumbnail + label | H+V | ≥1 pinned message | Jump to **currently shown** pin (cycle on bar tap — **target-state**; until cycle ships, jump to latest pin) |
| 2 | Media type hint | H+V | Pinned message is media | — (visual: Photo / Video / Voice / File / text snippet) |
| 3 | Open all pins | H+V | Always | Open full pinned list popover / panel (§3.1 #7) |
| 4 | Hide bar (×) | H | Always | Collapse bar for session; pins remain — restore via header #7 |
| 5 | Pinned list popover | H+V | From #3 or §3.1 #7 | List all pins (up to **5**); tap row → jump; order = pin time desc |

### 3.2 In-chat search bar (toggled by 3.1 #5)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Search text field | H+V | Search active | Input search query |
| 2 | Previous match (↑) | H+V | Results found | Scroll to previous match |
| 3 | Next match (↓) | H+V | Results found | Scroll to next match |
| 4 | Close search | H+V | Search active | Close search bar |
| 5 | Match highlight in bubble | H+V | Results found | — (visual: highlighted query occurrence in matched message text) |
| 6 | Jump highlight flash | H+V | After deep-link / search jump to message | — (visual: brief highlight flash on target bubble, then fade) |

### 3.3 Message bubbles

**Penpot:** bubbles on `Screen / Chat / Room`; system rows (missed call, user deleted) as `Screen / Chat / Room` system-bubble variants

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Delivery ticks | H+V | DM, outgoing | — (visual: ✓ delivered, ✓✓ read) |
| 2 | Timestamp | H+V | Always (on bubble or on tap/hover) | — (display) |
| 3 | View count | H+V | GRP / CH | — (visual) |
| 4 | Reaction chips | H+V | Message has reactions | Tap chip to toggle own reaction; long-press → who reacted. **Always visible when present** (no hover-only on mobile). Multiple reactions per user: ★ only (free users see prompt to upgrade) |
| 5 | Inline link styling | H+V | Message contains URL in text | — (visual: `color.link` blue, underline on hover; distinct from spoiler/code) |
| 6 | Thread reply count | H+V | Message has thread replies, threads enabled | Open thread panel with this message |
| 6a | Link preview | H+V | Message contains URL | Tap → open URL in browser |
| 7 | Spoiler overlay | H+V | Message has `||spoiler||` | Tap to reveal |
| 8 | Image / video thumbnail | H+V | Message has media | Tap → fullscreen viewer |
| 9 | File attachment chip | H+V | Message has file | Tap → download / preview |
| 10 | Voice message player | H+V | Message is voice note | Play / pause / scrub |
| 11 | Video note player | H+V | Message is video note (round video) | Play / pause / scrub. **Penpot variant:** `Screen/Chat/Room/VideoNoteBubble` (round bubble on Room timeline — see [screens.md](screens.md) GAP) |
| 12 | Sticker (full) | H+V | Message is sticker | Tap → open sticker pack preview (tap-to-add-to-collection is cosmetic / not in feature docs — do not invent collection CTA without spec) |
| 13 | "Edited" label | H+V | Message was edited | — (display; tap → show edit timestamp) |
| 14 | Expired file placeholder | H+V | File attachment expired (retention) | — (display: "bone pile" icon + "Файл удалён. Подписка сохраняет файлы навсегда" tooltip on hover/tap) |
| 15 | Forwarded-from attribution | H+V | Message was forwarded with attribution enabled | — (display banner: "Forwarded from [Name]"; if source account deleted -> "Deleted account") |
| 16 | Sender premium badge ★ | H+V | Sender profile has personal subscription | — (visual indicator near sender name in contexts where sender name is shown) |
| 17 | Missed-call system bubble | H+V | DM, unanswered incoming call timed out | — (display system row: "Пропущенный звонок") |
| 18 | User-deleted system bubble | H+V | Counterpart account deleted | — (display system row: "Пользователь удалён") |
| 19 | Bot deferred placeholder | H+V | Bot acknowledged slash/command with deferred response | — (display: "обрабатываю…" placeholder until async reply arrives) |
| 20 | Ephemeral bot reply | H+V | Bot reply marked ephemeral (visible only to invoking user) | — (visual: ephemeral-only styling; not shown to other members) |
| 21 | Posted-as-chat author label | H+V | Message has `posted_as_chat` | — (display: authored as chat/channel name, not user profile) |

### 3.4 Message actions (hover toolbar H / ctx H+V / selection bar V)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | React | H+V | Always | Open emoji reaction picker |
| 2 | Reply | H+V | Always | Set reply-to in composer |
| 3 | Edit | H+V | Own message | Enter edit mode in composer |
| 4 | Delete | H+V | Own message OR has MANAGE_MESSAGES right | Delete message (confirm) |
| 5 | Forward | H+V | Always (unless sender disabled forwarding) | Open Panel / Chat / ForwardMessage |
| 6 | Copy as new | H+V | Always | Copy content to composer without attribution |
| 7 | Copy text | H+V | Text message | Copy to clipboard |
| 8 | Pin / Unpin | H+V | Has `TEXT_CHAT_PIN_MESSAGES` right | Toggle pin |
| 9 | Open thread | H+V | Threads enabled (GRP/CH) | Open / create thread → Panel / Chat / Thread |
| 10 | Select (multi) | H+V | Always | Enter multi-select mode → shows selection bar |
| 11 | Report message | H+V | Not own message | Open Panel / Report / Sheet |
| 12 | Share / deep link | H+V | Always | Copy `voice.gg/…/m/{id}` to clipboard |
| 13 | Bot context menu commands | H+V | Bot installed in space, message/user ctx | Show bot-registered context menu commands |

### 3.5 Multi-select bar (shown when 3.4 #10 active)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Selected count | H+V | Multi-select active | — (display) |
| 2 | Forward selected | H+V | Multi-select active | Open Panel / Chat / ForwardMessage with selected |
| 3 | Delete selected | H+V | Own messages or MANAGE_MESSAGES | Delete all selected |
| 4 | Cancel selection | H+V | Multi-select active | Exit multi-select |

### 3.6 Composer (Telegram parity)

**Layout (desktop):** `[📎] [input] [😊] [🎤|📹|➤]` — см. [text-chat.md](../features/text-chat.md) § Composer.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Attach 📎 | H+V | Always | **Activate on click/tap or Enter/Space** (not hover-only). H: opens §3.6a popup anchored to button; V: bottom sheet §3.6a |
| 2 | Text input | H+V | Always (disabled when channel forbids user posts in main timeline and user lacks post-as-chat / override) | Type message (supports markdown); when disabled show reason (channel mode) |
| 3 | Reply preview | H+V | Reply-to set | Shows quoted message; X to cancel |
| 4 | Edit preview | H+V | Editing message | Shows original; X to cancel edit |
| 5 | Emoji 😊 | H+V | Always | **Activate on click/tap or Enter/Space** (not hover-only). Opens §3.6b panel (Emoji \| Stickers \| GIFs) |
| 6 | Voice / Video note / Send (right button) | H+V | Empty input → mic (voice note); long-press/toggle → video note (≤**60 s**, [file-storage.md](../features/file-storage.md)); non-empty → Send | Record voice / video note / send text |
| 7 | Long-press Send | H+V | Non-empty input, long-press on Send | §3.6c send menu |
| 8 | Send as chat toggle | H+V | CH/GRP with permission `posted_as_chat` | Toggle: send as user profile vs from chat/channel name |
| 9 | Slash command `/` | H+V | Typing `/` at start in SP context | Show Panel / Chat / SlashCommandMenu |
| 10 | Format toolbar (optional) | H — optional visible; V — toggle | Text selected or toolbar toggled | Duplicates §3.6d formatting actions |
| 11 | @mention autocomplete | H+V | Typing `@` | Show member list popup; includes `@here` / `@everyone` per permissions |
| 12 | Slow mode timer | H+V | SP chat with slow mode, user recently sent | Shows remaining time; send disabled |
| 13 | Scheduled messages strip | H+V | Chat has scheduled messages pending | Open scheduled list above composer. **Penpot:** `Panel/Chat/ScheduledStrip` (GAP — [screens.md](screens.md)) |
| 14 | Scheduled message row | H+V | Inside scheduled strip/list | Preview scheduled message |
| 15 | Edit scheduled | H+V | Scheduled message selected | Reopen in composer + date-time picker |
| 16 | Cancel scheduled | H+V | Scheduled message selected | Delete before send |
| 17 | Send now (scheduled) | H+V | Scheduled message selected | Send immediately |
| 18 | File size error | H+V | Attach exceeds limit | Inline/banner near composer (toast optional); copy: «Файл слишком большой. Лимит: 50 MB / 200 MB ★» |
| 19 | File blocked by recipient privacy | H+V | Group chat, member privacy blocks files/voice | — (display error toast) |
| 20 | Malware detected error | H+V | ClamAV blocks file | — (display error toast) |
| 21 | Space/group limit reached error | H+V | User at space+group limit | — (display error toast) |
| 22 | Rate-limit error toast | H+V | Send blocked (5 msg / 5 sec) | — (display feedback) |
| 23 | Char-limit error | H+V | Input exceeds 4000 characters | — (display near composer) |

### 3.6f Composer error state matrix

Normative recovery UX for composer and attach/send paths. Prefer **inline / banner near composer** ([brand.md](brand.md)); toast only when non-blocking. Failed optimistic sends: bubble **failed** state + **Retry** on bubble and/or composer; remove phantom bubble on permanent failure.

| Trigger | Surface | Copy / action | §3.6 ref |
|---------|---------|---------------|----------|
| File exceeds size limit | Inline / banner | «Файл слишком большой…»; dismiss | #18 |
| Upload network / R2 PUT failure | Inline / banner + retry on attachment chip | «Не удалось загрузить. Повторить» | *(new)* |
| `ConfirmUpload` / async processing failed | Inline on pending attachment; chip error state | «Обработка не удалась» + Retry / Remove | *(new)* |
| ClamAV infected / blocked file | Toast or inline (non-blocking after pick) | «Файл заблокирован» | #20 |
| Recipient privacy blocks files/voice | Toast | Privacy reason | #19 |
| Quota / `CheckQuota` exceeded | Inline / banner | «Недостаточно места» + link to subscription if applicable | *(new)* |
| Space/group membership limit | Toast | Limit reached | #21 |
| Rate limit (5 msg / 5 s) | Toast / subtle composer shake | Wait + retry | #22 |
| Char limit > 4000 | Inline near input | Truncate hint | #23 |
| Schedule: `scheduled_at` beyond horizon (>365 d) | Inline in schedule picker / strip | Validation message; keep composer draft | #15 context |
| Schedule: edit/cancel/send-now RPC failure | Inline on scheduled row | Retry / dismiss | #15–17 |
| `send_when_online` invalid context (non-DM) | Inline if ever exposed | «Только для личных чатов» | §3.6c |
| Sticker / GIF send failure (provider, pack, wire) | Inline near emoji panel or toast | «Не удалось отправить» + Retry | §3.6b |
| Video note too short / record cancelled | Inline near mic button | Min duration hint; re-record | #6 |
| Article invalid URL / OG fetch fail | Inline in article attach flow | «Ссылка недоступна» + edit URL | §3.6a #3 |
| Generic `SendMessage` / optimistic rollback | Failed bubble + composer banner | «Не отправлено» + Retry; rollback optimistic row | §54.2 pattern |

### 3.6a Attach popup

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Photo or video | H+V | Attach open | OS picker / camera → upload |
| 2 | Document | H+V | Attach open | File picker → upload |
| 3 | Article | H+V | Attach open | Article composer / URL → instant view payload |
| 4 | Location | H+V | Attach open | Map picker → lat/lon + label |
| 5 | Music | H+V | Attach open | Audio file picker → upload; metadata extracted by **File Service**, stored on message by **Messaging** ([messaging-service.md](../microservices/messaging-service.md)) |

**Wallet** — not in product; do not draw.

### 3.6b Emoji / Stickers / GIF panel

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Tab: Emoji | H+V | Panel open | Emoji grid + recents |
| 2 | Tab: Stickers | H+V | Panel open | Sticker packs rail + grid |
| 3 | Tab: GIFs | H+V | Panel open | GIF search + recents |
| 4 | Search field | H+V | Stickers or GIFs tab | Filter packs / GIF provider |
| 5 | Pack rail | H+V | Stickers tab | Switch sticker pack |
| 6 | Insert selection | H+V | Item tapped | Insert into composer or send immediately per type |

### 3.6c Send long-press menu

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Send without sound | H+V | Long-press Send | Send with silent push flag |
| 2 | Schedule message | H+V | Long-press Send | Open date/time picker → queue for future send |
| 3 | Send when online | H+V | Long-press Send, DM | Queue until recipient presence = online |

### 3.6d Composer text ctx / formatting

ПКМ (H) / long-press selection (V) in composer input:

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Cut / Copy / Paste | H+V | Selection / ctx | Standard clipboard |
| 2 | Formatting submenu | H+V | ctx | Bold / Italic / Underline / Strikethrough / Quote / Monospace / Spoiler / Create link / Clear |
| 3 | Bold | H+V | Formatting submenu | Wrap selection `**…**` |
| 4 | Italic | H+V | Formatting submenu | Wrap `*…*` |
| 5 | Underline | H+V | Formatting submenu | Wrap `__…__` |
| 6 | Strikethrough | H+V | Formatting submenu | Wrap `~~…~~` |
| 7 | Quote | H+V | Formatting submenu | Prefix `>` |
| 8 | Monospace | H+V | Formatting submenu | Wrap `` `…` `` or code block |
| 9 | Spoiler | H+V | Formatting submenu | Wrap `\|\|…\|\|` |
| 10 | Create link | H+V | Formatting submenu | Insert markdown link |
| 11 | Clear formatting | H+V | Formatting submenu | Strip markdown wrappers |

### 3.6e Popup / sheet accessibility

Attach popup (§3.6a), emoji panel (§3.6b), send menu (§3.6c), archive screen, pinned list:

| Requirement | Rule |
|-------------|------|
| **Activation** | Open on **click/tap** or **Enter/Space** on trigger; hover may preview on H but must not be sole affordance |
| **Focus trap** | While open, Tab cycles inside popup; Shift+Tab wraps |
| **Escape** | Closes popup and **returns focus** to trigger |
| **Initial focus** | First actionable item or search field (emoji/GIF tabs) |
| **Flutter** | `Semantics` label on triggers; `FocusScope` + `Shortcuts` for Escape; live region for send errors — prefer inline/banner over toast-only ([brand.md](brand.md)) |

Profile avatar menu (§1.1a) and folder drawer follow same focus/Escape contract.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Scroll to bottom (FAB) | H+V | Scrolled up from bottom | Scroll to latest message |
| 2 | Unread separator | H+V | New messages since last visit | — (visual divider "N new messages") |

### 3.8 E2E key change banner

**Penpot:** banner on `Screen / Chat / Room` (shared Chat Room page)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Banner text ("У @ник сменился ключ шифрования") | H+V | DM with E2E, contact's identity key changed | — (display; help copy per encryption.md) |
| 2 | Continue | H+V | Banner shown | Accept new key, resume E2E |
| 3 | Don't trust | H+V | Banner shown | Reject new key; new messages not decrypted until resolved |

### 3.9 E2E unavailable messages banner

**Penpot:** banner on `Screen / Chat / Room` (shared Chat Room page)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Placeholder banner | H+V | DM with E2E, new device without key backup | — (display: "Сообщения до [дата] зашифрованы и недоступны на этом устройстве. Настройте резервную копию ключей, чтобы не потерять историю.") |
| 2 | Restore keys CTA | H+V | Banner shown | Navigate to Panel / Settings / E2EKeyBackup |

---

## 4. Social / Friends Panel

**Penpot:** `Screen / Social / Panel`
**Feature docs:** [friends.md](../features/friends.md), [user-profile.md](../features/user-profile.md), [presence.md](../features/presence.md)

### 4.1 Header / tabs

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Title "Друзья" (Friends) | H+V | Always | — (UI copy via i18n EN+RU; RU baseline matches folder tabs) |
| 2 | Add friend | H+V | Always | Open search / @username input |
| 2a | Add friend by phone | H+V | Always (tab/mode toggle with #2) | Input phone number → find / request Voice user |
| 3 | Tab: Online | H+V | Always | Filter to online friends |
| 4 | Tab: All | H+V | Always | Show all friends |
| 5 | Tab: Pending | H+V | Always | Show incoming/outgoing requests |
| 6 | Tab: Blocked | H+V | Always | Show blocked users |
| 7 | Tab: Favourites | H+V | Always | Show favourites list |
| 8 | Sync contacts / phone book | H+V | Always (or first-time prompt) | Request phone book access; sync contacts to find Voice users |
| 9 | Add friend by QR | H+V | Always | Open QR code scanner / display own QR |

### 4.2 Friend row

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Avatar + Name + Status | H+V | Always | Open Panel / Social / ProfileDetail |
| 1a | Presence / status dot on avatar | H+V | Presence visible per privacy | — (visual: online / idle / DND / offline) |
| 2 | Message | H+V | Always | Open / create DM |
| 3 | Voice call | H+V | Always | Start DM voice call |
| 4 | More (ctx) | H+V | ctx | Remove friend / Block / Report |
| 5 | Stories ring | H+V | Friend has active story | Open story viewer |
| 6 | Add to favourites (ctx) | H+V | ctx, not in favourites | Add to favourites |
| 7 | Remove from favourites (ctx) | H+V | ctx, in favourites | Remove from favourites |
| 8 | In-game activity | H+V | Friend has game running (desktop), visible per privacy | — (display: "Играет в {Game}" below status) |

### 4.3 Pending request row

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Accept | H+V | Incoming request | Accept friend request |
| 2 | Decline | H+V | Incoming request | Decline request |
| 3 | Cancel | H+V | Outgoing request | Cancel own request |

---

## 5. Settings

**Penpot:** `Panel / Settings / Sheet` (nav hub); detail screens `Screen / Settings / Privacy`, `Security`, `Notifications`, `Subscription` (abbreviated Penpot labels `Privacy` / `Security` / `Notifications` / `Subscription` are the same Screen IDs)
**Feature docs:** [privacy.md](../features/privacy.md), [auth-and-contacts.md](../features/auth-and-contacts.md), [notifications.md](../features/notifications.md), [subscription.md](../features/subscription.md), [encryption.md](../features/encryption.md), [verification.md](../features/verification.md)

### 5.0 Panel / Settings / Sheet (settings nav hub)

**Penpot:** `Panel / Settings / Sheet`  
Entry: Shell Settings tab (§1.1 #4 / §1.8 #4). H: sidebar + detail; V: list → push detail screens.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Nav: Security | H — sidebar; V — list row | Always | Open `Screen / Settings / Security` |
| 2 | Nav: Privacy | H — sidebar; V — list row | Always | Open `Screen / Settings / Privacy` |
| 3 | Nav: Notifications | H — sidebar; V — list row | Always | Open `Screen / Settings / Notifications` |
| 4 | Nav: Subscription | H — sidebar; V — list row | Always | Open `Screen / Settings / Subscription` |
| 5 | Nav: Linked accounts & verification | H — sidebar; V — list row | Always | Open linked accounts and verification status (`Panel / Settings / LinkedAccounts` / `Verification`) |
| 6 | Nav: Help | H — sidebar; V — list row | Always | Open `Panel / Settings / Help` |
| 7 | Nav: Appearance | H — sidebar; V — list row | Always | Open `Panel / Settings / Appearance` (theme, language, Chat Themes ★, App Icon ★) |
| 8 | Nav: Accessibility | H — sidebar; V — list row | Always | Open `Panel / Settings / Accessibility` (reduced motion, font scale, PTT keybind) |
| 9 | Nav: Appeal | H — sidebar; V — list row | Has active sanction (ban/warning) | Open `Panel / Settings / Appeal` |
| 10 | Nav: Sticker management | H — sidebar; V — list row | Always | Open `Panel / Settings / Stickers` |
| 11 | Log out | H+V | Always | Log out (confirm) |
| 12 | Email-only account badge | H+V | Own account has email but no phone | — (visual: "без телефона" badge on account/settings header) |
| 13 | Close / Back | H — close sheet; V — back | Always | Dismiss settings hub |

### 5.1 Privacy

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Privacy preset (Personal/Gaming/Work) | H+V | Always | Apply preset defaults |
| 2 | Who can start DM | H+V | Always | Audience multiselect (Все†/Никто† — shortcuts at top; Друзья/Друзья друзей/Участники спейса‡/Гостевые аккаунты) |
| 3 | Who can call | H+V | Always | Audience multiselect |
| 4 | Who can invite to group chats/spaces | H+V | Always | Audience multiselect |
| 5 | Who can send files | H+V | Always | Audience multiselect |
| 6 | Who can send voice messages | H+V | Always | Audience multiselect |
| 7 | Who can add as friend | H+V | Always | Audience multiselect |
| 8 | Online status visibility | H+V | Always | Audience multiselect |
| 9 | In-game status visibility | H+V | Always | Audience multiselect |
| 10 | MM rating visibility | H+V | Always | Audience multiselect |
| 11 | Phone visibility | H+V | Always | Audience multiselect |
| 12 | Story visibility | H+V | Always | Audience multiselect |
| 13 | Search by phone | H+V | Always | Audience multiselect |
| 14 | Read receipts (DM only) | H+V | Always | Toggle on/off (controls DM ✓✓ ticks; group/channel view counts are always on). **Note:** delivery ticks are in text-chat.md; this privacy toggle is not yet listed in privacy.md field table — keep UI until privacy.md is updated, or drop if product drops the setting. |
| 15 | Last seen | H+V | Always | Audience multiselect (hideable per presence.md; keep control — privacy.md should list this field when docs are aligned) |
| 16 | Disallow forwarding my messages | H+V | Always | Toggle on/off |
| 17 | Blocked users | H+V | Always | Open blocked list → unblock action per row |
| 18 | Show game activity (auto-detect) | H+V | Always (desktop relevant) | Toggle on/off (presence.md: auto-detect running games) |
| 19 | Allow guest DM (`allow_guest_dm`) | H+V | Always | Toggle on/off — separate from audience multiselect; allows guests to initiate DM |

†"Все" and "Никто" are exclusive shortcuts at the top of the multiselect — not regular items.
‡When "Space members" is selected, a **nested space multiselect** appears to pick which spaces.

### 5.2 Security

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Two-factor auth | H+V | Always | Toggle / open 2FA setup sub-panel |
| 1a | 2FA: QR / secret | H+V | Enabling TOTP | Display QR + manual secret for authenticator app |
| 1b | 2FA: Confirm code | H+V | After scanning | Input TOTP to confirm setup |
| 1c | 2FA: Backup codes reveal / download | H+V | After 2FA enabled | Show one-time backup codes; copy / download |
| 2 | Change password | H+V | Always | Open change password form |
| 3 | Active sessions | H+V | Always | Open sessions list → terminate session per row; shows voice session indicator per device |
| 4 | Linked devices | H+V | Always | Open devices list |
| 5 | Delete account | H+V | Always | Open delete account flow (confirm + password) |

### 5.3 Notifications

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Push notifications | H+V | Always | Toggle on/off |
| 2 | Mentions only | H+V | Always | Toggle on/off |
| 3 | Sounds | H+V | Always | Toggle on/off |
| 4 | Quiet hours / DND schedule | H+V | Always | Set time range |
| 5 | Per-chat / per-space overrides | H+V | Always | Open override list |
| 6 | @mention breaks mute | H+V | Always | Toggle on/off |
| 7 | Friend request notifications | H+V | Always | Toggle on/off |
| 8 | MM match notifications | H+V | Always | Toggle on/off |
| 9 | Reaction notifications | H+V | Always | Toggle on/off |
| 10 | Reply notifications | H+V | Always | Toggle on/off |
| 11 | DM message notifications | H+V | Always | Toggle on/off (new message in DM) |
| 12 | System / security notifications | H+V | Always | Toggle on/off (security alerts, app updates) |

### 5.4 Subscription

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Current plan card | H+V | Always | — (display: Free / Plus / annual) |
| 2 | Upgrade / Subscribe | H+V | Not subscribed | Open payment flow |
| 3 | Cancel plan | H+V | Subscribed | Cancel subscription (confirm + downgrade flow) |
| 4 | Manage plan | H+V | Subscribed | Change plan (monthly ↔ annual) |
| 5 | Billing history | H+V | Subscribed | Open transaction list |
| 6 | Manage profiles | H+V | Always | Navigate to profile list / downgrade picker |
| 7 | Restore purchases | H+V | Always (iOS mainly) | Trigger purchase restore |
| 8 | Payment failure banner | H+V | Grace period (days 1, 3, 7) | — (display: "Оплата не прошла"); tap → update payment method |
| 9 | Purchase clean @username for extra profile | H+V | Plus; additional profile without clean username | Buy clean @username perk separately (subscription.md) |
| 10 | Space Pro manage | H+V | User owns spaces | Manage Space Pro per space (limits, cancel); see §10.3 Space Pro banner |

---

## 6. Matchmaking

**Penpot:** `Screen / Matchmaking / GameCatalog`, `GameDetail`, `QueueSearch`, `MatchSquad`, `PartyLobby`, `MatchHistory`; overlays `Overlay / Matchmaking / MatchFound`, `MatchRating` (**Penpot frame alias:** `Overlay / Matchmaking / Rating` — same surface), `StorySuggestion`
**Feature docs:** [matchmaking.md](../features/matchmaking.md), [game-catalog.md](../features/game-catalog.md)

### 6.1 GameCatalog

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | V | Always | Return |
| 2 | Search games field | H+V | Always | Filter catalog |
| 3 | Popular section | H+V | Always | — (display game cards) |
| 4 | Recent section | H+V | User has MM history | — (display recent games) |
| 5 | Game card tap | H+V | Always | Open GameDetail |
| 6 | Browse all | H+V | Always | Expand full catalog list |
| 7 | Add game | H+V | User catalog submission enabled or admin | Navigate to Panel / Matchmaking / AddGame |

### 6.2 GameDetail

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Mode card (per mode) | H+V | Always | Select mode |
| 3 | Region picker | H+V | Always | Select region (Russia / Europe / Asia) |
| 4 | Rank self picker | H+V | Mode selected | Set own rank |
| 5 | Rank target range | H+V | Mode selected | Set desired teammate rank range |
| 6 | Role picker | H+V | Game has roles | Select role |
| 7 | Find match / Start queue | H+V | Mode + region selected | Start matchmaking |
| 8 | Game title | H+V | Always | — (display) |
| 9 | Game logo / cover | H+V | Always | — (display image) |
| 10 | Genre | H+V | Always | — (display metadata) |
| 11 | Platforms | H+V | Always | — (display metadata) |
| 12 | Regions info | H+V | Always | — (display supported regions) |
| 13 | Modes summary | H+V | Always | — (display available matchmaking modes) |
| 14 | Roles summary | H+V | Game has roles | — (display available roles for this game) |
| 15 | Rank system summary | H+V | Mode uses rank | — (display available ranks / scale) |

### 6.3 QueueSearch

**Penpot:** `Screen / Matchmaking / QueueSearch`

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Status spinner / text | H+V | Always | — (display: searching, ETA, found N/M) |
| 2 | Cancel search | H+V | Always | Cancel MM queue |
| 3 | Long search notification (15 min) | H+V | 15 min without result | — (display: "Долго ищем. Попробуйте изменить параметры"); tap → GameDetail |
| 4 | Timeout notification (30 min) | H+V | 30 min without result | — (display: "Найти не удалось"); search stops automatically |
| 5 | Party changed prompt | H+V | Party member joined/left during search | — (display: "Состав пати изменился. Поиск остановлен"); CTA: "Начать заново" |

### 6.4 MatchFound (Overlay / Matchmaking / MatchFound)

Entry: popup when match is found during queue search.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | "Матч найден" title | H+V | Match found | — (display; Penpot frame title matches copy) |
| 2 | Accept | H+V | Always | Accept match |
| 3 | Decline | H+V | Always | Decline match → own party: search resets; other party declined: own party continues search |
| 4 | Timer | H+V | Always | — (display: countdown to auto-decline) |

### 6.4a Party Lobby (pre-match)

**Penpot:** `Screen / Matchmaking / PartyLobby`

Pre-match party = voice-room participants before search (matchmaking.md). Distinct from post-accept MatchSquad.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Party member row | H+V | Always (per member) | Open Panel / Matchmaking / PlayerProfile |
| 3 | Join / leave party voice | H+V | Party exists | Join or leave pre-match voice |
| 4 | Start queue | H+V | Party leader, not yet queuing | Start MM as party → QueueSearch |
| 5 | Leave party | H+V | Always | Leave party |
| 6 | Invite to party | H+V | Party leader / invite right | Invite user into pre-match party |

### 6.5 MatchSquad

**Penpot:** `Screen / Matchmaking / MatchSquad`

Post-accept **матч-отряд**: voice + text after all parties accepted (not the pre-match queue surface).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Player row | H+V | Always (per player) | Open Panel / Matchmaking / PlayerProfile |
| 3 | Join voice | H+V | Squad formed | Join squad voice room |
| 4 | Open match text chat | H+V | Squad formed | Open матч-отряд text chat |
| 5 | Leave squad | H+V | Always | Leave матч-отряд → may open MatchRating |

### 6.6 MatchRating (Overlay / Matchmaking / MatchRating)

**Penpot (canon Screen ID in screens.md):** `Overlay / Matchmaking / Rating`  
**screen-controls alias:** `Overlay / Matchmaking / MatchRating` — same surface; prefer `Rating` when naming Penpot frames / Screen IDs.

Entry: shown when user leaves match squad.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Player row (per teammate) | H+V | Always | — (display: avatar + name) |
| 2 | Star rating (1–5) per player | H+V | Always | Rate teammate |
| 3 | Skip (per player) | H+V | Always | Skip rating for this player |
| 4 | Skip all | H+V | Always | Skip all ratings |
| 5 | Ban suggestion | H+V | Rating 1–2 given | "Забанить в ММ?" — Yes / No |
| 6 | Submit | H+V | ≥1 rating given or all skipped | Submit ratings |

### 6.7 MatchHistory

**Penpot:** `Screen / Matchmaking / MatchHistory`

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Match row | H+V | Always (per match) | Expand match details |
| 3 | Find players CTA | H+V | Always | Navigate to GameCatalog |
| 4 | Rate / report (per player) | H+V | Match completed | Open rating overlay / report |
| 5 | Add friend (per player) | H+V | Match completed, not friends | Send friend request |
| 6 | Ban in MM (per player, ctx) | H+V | ctx | Ban player from future MM matches (only MM, not messenger) |

### 6.8 AddGame (Panel / Matchmaking / AddGame)

**Penpot:** `Panel / Matchmaking / AddGame`
**Feature docs:** [matchmaking.md](../features/matchmaking.md), [game-catalog.md](../features/game-catalog.md)

Entry: from GameCatalog "Add game" button or Admin Panel per game-catalog.md.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Game name field | H+V | Always | Input game name |
| 3 | Game icon upload | H+V | Always | Pick image |
| 4 | Add mode | H+V | Always | Add game mode (name + slot count) |
| 5 | Add numeric criterion | H+V | Always | Add criterion (name + integer, applies to self/target) |
| 6 | Add enum criterion | H+V | Always | Add criterion (name + list of values, applies to self/target) |
| 7 | Per-criterion: applies to "self" toggle | H+V | Criterion added | Toggle |
| 8 | Per-criterion: applies to "target" toggle | H+V | Criterion added | Toggle |
| 9 | Submit for moderation | H+V | Name + ≥1 mode filled | Submit (goes to moderation queue) |
| 10 | Similar games hint | H+V | Search/name matches similar catalog entries | — (display candidate duplicates while adding) |

### 6.9 PostMatchStorySuggestion (Overlay / Matchmaking / StorySuggestion)

**Penpot:** `Overlay / Matchmaking / StorySuggestion`

Entry: shown after match completion.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Pre-filled story preview (game, result, teammate mentions) | H+V | After match ends | — (display) |
| 2 | Edit & publish | H+V | Always | Navigate to Screen / Stories / Create with pre-filled data |
| 3 | Dismiss | H+V | Always | Close suggestion |

---

## 7. Stories

**Penpot:** `Screen / Stories / *`
**Feature docs:** [stories.md](../features/stories.md)

### 7.1 Create

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Content type tabs: Photo / Video / Clip / Text | H+V | Always | Switch content type |
| 3 | Gallery picker | H+V | Photo / Video / Clip | Pick from gallery or camera |
| 4 | Caption input | H+V | Always | Input optional caption |
| 5 | Audience picker | H+V | Always | Standard privacy audience selector |
| 6 | Game tag | H+V | Always | Select game from catalog |
| 7 | LFP toggle ("Ищу пати") | H+V | Always | Enable LFP story mode |
| 8 | Mention users | H+V | Always | @mention picker |
| 9 | Text overlay tool | H+V | Photo/Video/Clip mode | Add text overlay (fonts, color, size, position) |
| 10 | Stickers tool | H+V | Photo/Video/Clip mode | Add stickers |
| 11 | Doodle tool | H+V | Photo/Video/Clip mode | Draw over content |
| 12 | Filters tool | H+V | Photo/Video/Clip mode | Apply filter (basic free / extended ★) |
| 13 | Trim tool | H+V | Video/Clip mode | Trim video |
| 14 | Post story | H+V | Content selected | Publish story |
| 15 | Duration limit / ★ upsell | H+V | Video/clip longer than free cap (≤60s free; longer ★) | — (display error or subscription upsell when clip too long) |

### 7.2 Viewer

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close viewer |
| 2 | Progress bar (segments) | H+V | Always | — (visual, tap to skip segment) |
| 3 | Tap right | H+V | Always | Next story |
| 4 | Tap left | H+V | Always | Previous story |
| 5 | Swipe down | V | Always | Close viewer |
| 6 | Author name + time | H+V | Always | Open author profile |
| 7 | React | H+V | Always | Emoji reaction picker |
| 8 | Reply | H+V | Always | Open composer → sends private DM to author |
| 9 | Share | H+V | Always | Copy deep link |
| 10 | More / kebab | H+V | Always | ctx: Report, Share, Mute author |
| 11 | Game tag chip | H+V | Story has game tag | Open game page in catalog |
| 12 | LFP highlight style | H+V | Story is LFP type | — (visual: special card/viewer treatment for LFP stories) |
| 13 | Join (LFP — "Присоединиться") | H+V | Story is LFP type | Opens prefilled matchmaking / party-join request (stories.md + matchmaking.md); author gets notification with viewer rank |
| 13a | Invite (LFP — "Пригласить") | H+V | Story is LFP type | Viewer invites author to their own party; author gets notification with party details (matchmaking.md) |
| 13b | LFP inactive state | H+V | Author already found party / left MM / full party | Both Join and Invite buttons disabled; tap → "Объявление больше не актуально" |
| 13c | LFP "Написать" (DM to author) | H+V | Story is LFP type | Open / create DM with story author |
| 14 | Delete | H+V | Own story | Delete story |
| 15 | Viewers count (author only) | H+V | Own story | Open Panel / Stories / StoryViewers |
| 16 | Pause (hold) | H+V | Always | Hold to pause progress |
| 17 | Anonymous viewing toggle ★ | H+V | Plus subscriber | Toggle anonymous story viewing (hides viewer from author's viewers list) |
| 18 | Repost / reshare | H+V | Viewer is mentioned in story | Repost original story with own caption → navigate to Screen / Stories / Create |

### 7.3 Archive

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Story row / grid item | H+V | Always | Open viewer for archived story |
| 3 | Create story | H+V | Always | Navigate to Create |
| 4 | Add to highlight (per story) | H+V | Always | Add to existing / new highlight → Panel / Stories / HighlightEdit |

### 7.4 Highlights

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Highlight card tap | H+V | Always | Open highlight viewer (sequential) |
| 3 | Add highlight | H+V | Always | Open Panel / Stories / HighlightEdit |
| 4 | Edit highlight (ctx) | H+V | ctx on own highlight | Open Panel / Stories / HighlightEdit |
| 5 | Delete highlight (ctx) | H+V | ctx on own highlight | Delete (confirm) |
| 6 | Privacy per highlight | H+V | Inside Panel / Stories / HighlightEdit | Audience picker |

---

## 8. Bots

**Penpot:** `Screen / Bots / Install`, `Screen / Bots / Catalog`
**Feature docs:** [bots.md](../features/bots.md)

### 8.1 Install

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Bot icon + name + description | H+V | Always | — (display) |
| 3 | Permissions detail | H+V | Always | Expand to show requested scopes (human-readable) |
| 3a | Commands list | H+V | Always | — (display bot slash commands available after install) |
| 3b | Privileged-scope warning dialog | H+V | Install requests SPACE_MANAGE_ROLES and/or TEXT_CHAT_READ_HISTORY | Confirm warning before install |
| 4 | Space picker | H+V | Always | Select target space |
| 5 | Channel whitelist | H+V | Space selected | Select text chats where bot can operate; per-chat toggles remain available later in chat settings |
| 6 | Add to space / Install | H+V | Space + channels selected; privileged warning accepted if shown | Install bot → confirm |

---

## 9. Profile / Downgrade Picker

**Penpot:** `Screen / Profile / DowngradePicker`
**Feature docs:** [multi-profile.md](../features/multi-profile.md), [subscription.md](../features/subscription.md)

### 9.1 DowngradePicker

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Profile row with checkbox | H+V | Always (per profile) | Toggle selection |
| 2 | Keep selected | H+V | Exactly 2 selected | Freeze unselected profiles, proceed |

---

## 10. Chat Info / Panels (side panels or sheets)

**Penpot:** `Panel / Chat / Info`, `Panel / Chat / GroupMembers`, `Panel / Space / *`, `Overlay / Call / *`, `Panel / Call / ScreenShare`
**Entry points from screens above already listed; this section covers internal controls.**

### 10.1 Panel / Chat / Info

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Avatar + Chat name | H+V | Always | — (display; editable for groups if admin) |
| 3 | Mute / Unmute | H+V | Always | Toggle mute |
| 4 | Search in chat | H+V | Always | Open in-chat search |
| 5 | Notification override | H+V | Always | Per-chat notification setting |
| 6 | E2E encryption toggle | H+V | DM only | Enable/disable E2E |
| 7 | Encryption code | H+V | DM with E2E enabled | Display 6-char verification code |
| 7a | Copy encryption code | H+V | DM with E2E enabled | Copy code to clipboard |
| 7b | Compare help text | H+V | DM with E2E enabled | — (display: compare with contact in voice/person; codes must match — encryption.md) |
| 8 | Shared media tabs (Media/Files/Links/Voice) | H+V | Always | Show shared content |
| 9 | Members list | H+V | GRP / CH | Show members → `Panel / Chat / GroupMembers` (tap member → profile) |
| 10 | Add members | H+V | GRP (permission); disabled at 500/500 with tooltip "Группа заполнена (500/500). Для больших сообществ создайте спейс." | Open contact picker |
| 11 | Pinned messages | H+V | Has pins | Open pinned messages list |
| 12 | Leave group / channel | H+V | GRP / CH | Leave (confirm) |
| 13 | Report chat | H+V | Always | Open Panel / Report / Sheet |
| 14 | Auto-delete timer | H+V | DM | **Orphan / future:** no backing in text-chat, encryption, or privacy feature docs — hide from Penpot/Flutter until product spec exists |
| 15 | Create group from DM | H+V | DM only | Open contact picker → create group with both participants |
| 16 | E2E enable warning | H+V | DM, toggling E2E on | Overlay: warns about search/moderation limitations; [Включить] / [Отмена] |
| 17 | E2E file retention info | H+V | DM with E2E enabled | — (display: "Файлы в E2E-чатах хранятся 90 дней") |
| 18 | Bots (text chat settings) | H+V | SP text chat, has bot-management right | Open Panel / Chat / Bots |
| 19 | Allow guests | H+V | GRP / CH (admin/owner) | Toggle whether guests may enter this chat |
| 20 | Channel settings | H+V | CH, has manage rights | Open Panel / Chat / ChannelSettings |
| 21 | Group settings | H+V | GRP, owner/admin | Open Panel / Chat / GroupSettings |
| 22 | Chat theme ★ | H+V | Plus subscriber | Open Panel / Settings / ChatThemes for this chat |

### 10.1a Panel / Chat / GroupMembers

**Penpot:** `Panel / Chat / GroupMembers`  
**Feature docs:** [text-chat.md](../features/text-chat.md), [spaces.md](../features/spaces.md) (space members use `Panel / Space / Members`)

Entry: from Chat Info #9 (GRP) or SideHost members mode for a group chat. Distinct from CreateGroup (§20) and Space Members (§24).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Member row (avatar + name + role/owner badge) | H+V | Always (per member) | Open `Panel / Social / ProfileDetail` |
| 3 | Kick member | H+V | Admin/owner; not self | Kick (confirm) |
| 4 | Transfer ownership | H+V | Owner; on another member | Transfer group ownership (confirm) |
| 5 | Leave group | H+V | Always (member); owner needs transfer first | Leave group (confirm); owner sees hint if transfer required |
| 6 | Add members | H+V | Has add-members right; under 500 cap | Open contact picker (same as Chat Info #10) |

### 10.1b Panel / Chat / ChannelSettings

**Penpot:** `Panel / Chat / ChannelSettings`
**Feature docs:** [text-chat.md](../features/text-chat.md)

Entry: from Chat Info #20.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Threads on/off | H+V | Has manage rights | Toggle threads for this channel |
| 3 | Allow user posts in main timeline | H+V | Has manage rights | Toggle whether members may post as themselves in main feed (channel default: off) |
| 4 | Allow post-as-chat | H+V | Has manage rights | Configure who may post official messages as the channel |
| 5 | Save | H+V | Changes made | Save channel settings |

### 10.1c Panel / Chat / GroupSettings

**Penpot:** `Panel / Chat / GroupSettings`
**Feature docs:** [text-chat.md](../features/text-chat.md)

Entry: from Chat Info #21.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Edit group name / avatar | H+V | Owner / admin | Edit group identity |
| 3 | Assign admins | H+V | Owner | Promote / demote admins |
| 4 | Who can add members | H+V | Owner / admin | Configure whether all members or only admins may add |
| 5 | Threads on/off | H+V | Owner / admin | Toggle threads (group default configurable) |
| 6 | Save | H+V | Changes made | Save group settings |

### 10.2 Call controls (Overlay / Call / *)

**Penpot:** `Overlay / Call / Active`, `Incoming`, `Outgoing`, `MiniBar` (`MiniBar` — required in controls; mockup may still be missing — see audit)
Entry: voice/video call from chat header or friend row.
Note: iOS CallKit / PushKit incoming UI is platform chrome (native), not duplicated as Flutter/Penpot controls.

#### 10.2a Outgoing call (Overlay / Call / Outgoing)

**Penpot:** `Overlay / Call / Outgoing`  
Entry: after initiator starts DM voice/video (or group temp voice) and before callee accepts. Distinct from `Active` (connected) and `Incoming` (callee side).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Callee avatar + display name | H+V | Always | — (display) |
| 2 | Call type label (voice / video) | H+V | Always | — (display) |
| 3 | Ringing / connecting status | H+V | Always | — (display; timeout per voice-chat.md: 30s → missed call for callee) |
| 4 | Cancel / Hang up | H+V | Always | Abort outgoing call |
| 5 | Mute mic | H+V | Always | Toggle mic **before** connect — **required** (Discord DM outgoing/ringing UI exposes mute while waiting for answer; see [aurascience Discord mobile call guide](https://aurascience.blog/how-to-ring-someone-discord-mobile-new-update)) |
| 6 | Camera on / off | H+V | Video outgoing | Toggle camera before connect — **required on video** (same Discord reference; omit on pure voice outgoing) |

#### 10.2b Active / Incoming / MiniBar (shared in-call controls)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Mute / Unmute mic | H+V | In call | Toggle microphone |
| 2 | Deafen / Undeafen | H+V | In call | Toggle all audio |
| 3 | Camera on / off | H+V | In call (video call or voice room) | Toggle camera |
| 4 | Screen share | H only | In call (desktop) | Open screen/window picker → share (see §33 Overlay picker; in-call viewer panel §10.2c) |
| 5 | Screen share pause | H only | Sharing screen | Pause stream |
| 6 | Disconnect / Hang up | H+V | In call | Leave call |
| 7 | PTT (Push-to-Talk) | H+V | In call, PTT mode enabled (settings §30 + in-call #8) | Hold configured key / button to talk |
| 8 | VAD / PTT mode switch | H+V | In call (settings) | Toggle between voice activity and PTT (mode also configurable in Accessibility / voice settings) |
| 9 | Noise cancellation | H+V | In call (settings) | Toggle noise cancellation |
| 10 | Per-user volume slider | H+V | In call | Adjust individual participant volume |
| 11 | Mute participant (mod) | H+V | Has VOICE_MUTE_OTHERS | Server-mute participant |
| 12 | Raise hand | H+V | In call, raise hand enabled | Toggle hand-raise |
| 13 | Start broadcast (commander) | H+V | In commander room, has right | Hold → broadcast to target rooms (ducking) |
| 14 | Record | H+V | In call | Toggle local recording (MP3 128kbps) |
| 15 | Record indicator ⏺ | H+V | Recording active (self only) | — (visual, only visible to recorder) |
| 16 | Participants list | H+V | In call | Show who is in the call |
| 16a | Speaking indicator (per participant) | H+V | In call, participant speaking | — (visual: avatar highlight / glow animation) |
| 17 | Accept call (incoming) | H+V | Incoming call overlay | Accept |
| 18 | Decline call (incoming) | H+V | Incoming call overlay | Decline → "missed call" in DM |
| 19 | Mini call bar (persistent) | H+V | In call, navigated away from call screen | Floating bar: avatar(s) + duration + mute + hang up; tap → return to call (`Overlay / Call / MiniBar`) |
| 20 | Hand-raise request list (organizer) | H+V | Organizer in raise-hand-enabled room | List of raised hands; per-row: Grant word / Deny |
| 21 | Voice session conflict dialog | H+V | Switching profiles while in voice call | — (display: "Ты сейчас в войс-чате (Профиль A). Выйти из него и войти сюда?"); [Выйти и переключить] / [Отмена] |
| 22 | In-call notification overlay | H+V | Notification arrives during voice chat | Inline overlay banner; tap opens related chat/thread without dropping voice |
| 23 | Voice room full | H+V | Room at capacity (32 free / 128 Space Pro) | Join disabled + tooltip / error "Комната заполнена" |
| 24 | Multi-device voice handoff | H+V | Same account joins voice from another device while already in voice | Explicit handoff UI: move session to this device or cancel (one active voice session) |

#### 10.2c Panel / Call / ScreenShare (in-call share viewer)

**Penpot:** `Panel / Call / ScreenShare`  
**Not the same as** `Overlay / Call / ScreenSharePicker` (§33).  
- **Picker (Overlay):** choose what to share (screen / window / tab + quality) before start — desktop start flow.  
- **Panel:** in-call UI while sharing or watching: local preview, remote stream switcher, pause/stop, limit feedback.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Local preview | H | Self is sharing | — (display own share preview) |
| 2 | Stop share | H | Self is sharing | Stop screen share |
| 3 | Pause / Resume share | H | Self is sharing | Pause or resume stream (frozen last frame for viewers) |
| 4 | Stream picker (thumbnails) | H+V | >1 active share in call | Switch which remote stream to watch (also §34) |
| 5 | Waiting for video | H+V | Share started, track not yet received | — (display placeholder) |
| 6 | Stream limit reached | H+V | Already 3 active streams | — (display limit; cannot start another) |
| 7 | Quality dialog (FPS / resolution) | H | Starting share (desktop) | Prefer opening via Overlay picker (§33); Panel may still surface quality when picker is not yet framed |

**Product decision:** keep **both** frames — `Panel / Call / ScreenShare` (viewer/manage) **and** `Overlay / Call / ScreenSharePicker` (start picker). Picker remains in the design system even if implementation lands later; do not collapse picker into Panel-only.

### 10.3 Space tree & admin (Panel / Space / *)

Entry: from space context in shell or chat list.
Folder label uses GLOSSARY term **Спейс** (EN key: Space).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Space tree (text channels + voice rooms) | H+V | In space | Navigate to channel/room |
| 1a | Pin / Unpin tree node | H+V | Has space tree-manage permission (roles; default admin/owner) | Toggle pin on text chat or voice room node; pinned nodes sort above unpinned in same category/root — [spaces.md](../features/spaces.md) § «Pin элемента дерева»; backend: `PinTreeNode` / `UnpinTreeNode` |
| 2 | Create text chat | H+V | Has TEXT_CHAT_CREATE_IN_SPACE | Create group/channel in space |
| 3 | Create voice room | H+V | Has right | Create voice room |
| 4 | Create category | H+V | Has right | Create category folder |
| 5 | Reorder tree | H+V | Has right | Drag-reorder nodes |
| 6 | Space settings | H+V | Has SPACE_MANAGE_SETTINGS | Open space settings |
| 7 | Invites | H+V | Has right | Open Panel / Space / Invites |
| 8 | Members | H+V | Always | Open Panel / Space / Members |
| 9 | Roles | H+V | Has SPACE_MANAGE_ROLES | Open Panel / Space / Roles |
| 10 | Bots | H+V | Has SPACE_MANAGE_BOTS | Open Panel / Space / Bots |
| 11 | Audit log | H+V | Has audit log right | Open audit log (filter by event type, filter by moderator; paginated event rows with timestamp + actor + action + target) |
| 12 | Leave space | H+V | Not owner | Leave (confirm) |
| 13 | Transfer ownership | H+V | Owner | Transfer to member (confirm + 2FA) |
| 14 | Slow mode per chat | H+V | Has TEXT_CHAT_SET_SLOW_MODE | Open Panel / Space / ChatSlowMode |
| 15 | Chat/voice overrides per channel | H+V | Has right | Open Panel / Space / ChatOverride / VoiceRoomOverride |
| 16 | Share invite link | H+V | Has invite right | Generate / copy link |
| 17 | Space verification settings | H+V | Has right | Open Panel / Space / VerificationSettings |
| 18 | QR code for invite | H+V | Has invite right | Generate and display QR code |
| 19 | Voice room members preview | H+V | Voice room has participants | — (display: avatars of connected users below room name, before joining) |
| 20 | Space banner upload (Space Pro) | H+V | Space owner, Space Pro subscription | Upload / change space banner image |
| 21 | Custom emoji management (Space Pro) | H+V | Has SPACE_MANAGE_SETTINGS, Space Pro | Upload / delete custom emoji for space |
| 22 | Space MM settings | H+V | Has right | Configure space-specific matchmaking parameters |
| 23 | Space Pro subscription | H+V | Space owner | Manage Space Pro ($5/мес) — upgrade, cancel, billing; see limits (members, voice, channels) |
| 24 | Allow guests | H+V | Has SPACE_MANAGE_SETTINGS | Toggle whether guests may enter this space |
| 25 | Report space | H+V | Always (member/visitor) | Open Panel / Report / Sheet with object type Space |
| 26 | Share / copy space deep link | H+V | Always (when link allowed) | Copy `voice.gg/…` space deep link |
| 27 | Space at-limit / Pro-cancelled banner | H+V | Owner; Space Pro cancelled and member count ≥ free cap | — (display: new joins blocked until under free limit) |
| 28 | Voice room full (tree join) | H+V | Voice room at 32/128 capacity | Join disabled + tooltip |
| 29 | Federated space unavailable (deferred) | H+V | Federation deferred; remote space unreachable | — (display: "Спейс недоступен") |

### 10.3a Panel / Space / VoiceRoomSettings

**Penpot:** `Panel / Space / VoiceRoomSettings`

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Room name field | H+V | Editing voice room | Rename room |
| 2 | Broadcast / commander mode toggle | H+V | Has SPACE_MANAGE_SETTINGS or voice-room admin rights | Enable commander mode for this room |
| 3 | Target voice rooms picker | H+V | Broadcast enabled | Select all or specific target voice rooms |
| 4 | Microphone access mode | H+V | Editing voice room | Choose Normal / Raid-School mode |
| 5 | Raise hand enabled | H+V | Editing voice room | Toggle raise-hand workflow |
| 6 | Organizer word-grant policy | H+V | Raise hand enabled or Raid-School mode | Configure grant/deny speaking requests via organizer flow |
| 7 | Who can join commander room | H+V | Broadcast enabled | Restrict room entry by roles/permissions |
| 8 | Save | H+V | Changes made | Save voice room settings |

---

## 11. Onboarding (overlays)

**Penpot:** `Overlay / Onboarding / CoachMarks`
**Feature docs:** [onboarding.md](../features/onboarding.md)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Skip / "Пропустить" | H+V | Any step | Skip all remaining steps |
| 2 | Dismiss / "Понятно" | H+V | Info step | Advance to next step |
| 3 | Step 1: Nickname field | H+V | Step 1 (after regular registration) | Input nickname (required) |
| 4 | Step 1: Email field | H+V | Step 1 | Input email (optional, recommended) |
| 5 | Step 1: Avatar upload | H+V | Step 1 | Upload avatar (optional) |
| 6 | Step 1: "Сохранить" / "Пропустить" | H+V | Step 1 | Save or skip account setup |
| 7 | Step 3: "Найти спейс" | H+V | Step 3 (discover spaces) | Open Screen / Space / Catalog |
| 8 | Step 3: "Позже" | H+V | Step 3 | Skip space discovery step |
| 9 | Step 4: "Попробовать" | H+V | Step 4 (matchmaking intro) | Open MM / try matchmaking |
| 10 | Step 4: "Позже" | H+V | Step 4 | Skip MM intro step |
| 11 | Step 5: "Начать" | H+V | Step 5 (finish) | Complete onboarding → main shell |

---

## 12. Version / Force Update (overlay)

**Penpot:** `Overlay / Version / ForceUpdate`
**Feature docs:** [updates.md](../features/updates.md)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Release notes text | H+V | Soft or force update prompt | — (display `release_notes` from version API) |
| 2 | "Обновить сейчас" | H+V | `force_update: true` | Open store / CDN update (blocking) |
| 3 | "Обновить" (soft, mobile/store) | H+V | `update_available: true`, non-desktop patch | Open store / CDN |
| 4 | "Перезапустить и обновить" (desktop soft) | H | Desktop soft update with local patch | Restart app and apply update (updates.md) |
| 5 | "Позже" | H+V | Soft update only | Dismiss prompt |

---

## 13. Report sheet

**Penpot:** `Panel / Report / Sheet`
**Feature docs:** [reports.md](../features/reports.md)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Category picker (Спам / Домогательство / Оскорбительный контент / Фейк / Читерство·Токсик в ММ / Другое) | H+V | Always | Select category |
| 2 | Comment field | H+V | Always (required for "Other") | Input description (≤ 500 chars) |
| 3 | Submit | H+V | Category selected | Send report |
| 4 | Cancel / Close | H+V | Always | Close sheet |

---

## 14. User Profile Detail (Panel / Social / ProfileDetail)

**Penpot:** `Panel / Social / ProfileDetail`
**Feature docs:** [user-profile.md](../features/user-profile.md), [friends.md](../features/friends.md), [verification.md](../features/verification.md), [stories.md](../features/stories.md), [multi-profile.md](../features/multi-profile.md)

Entry: from friend row, chat member tap, search result, @mention tap.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Banner image | H+V | User has banner (★) | — (display) |
| 3 | Avatar (animated if ★) | H+V | Always | Fullscreen avatar |
| 4 | Display name + @username | H+V | Always | — (display) |
| 5 | Verification badge (system icon) | H+V | User is verified | — (visual: personal / organization badge as **system icon** per verification.md — not Unicode ✅/®/🏢 text) |
| 6 | Presence status (online/idle/DND) | H+V | Visible per privacy settings | — (display) |
| 7 | Custom status ★ | H+V | User has custom status | — (display) |
| 8 | Bio / description | H+V | User has bio | — (display) |
| 9 | MM average rating | H+V | Visible per privacy settings (audience multiselect, not subscription-gated) | — (display) |
| 10 | Highlights strip | H+V | User has highlights, visible per privacy | Open highlight viewer |
| 11 | Stories ring on avatar | H+V | User has active story | Open story viewer |
| 12 | Common spaces | H+V | Has common spaces | — (display; tap → open space) |
| 12a | Premium badge ★ | H+V | User has personal subscription | — (visual indicator near display name) |
| 13 | Send message | H+V | Always (respects privacy) | Open / create DM |
| 14 | Voice call | H+V | Always (respects privacy) | Start DM voice call |
| 15 | Add friend | H+V | Not friends | Send friend request |
| 16 | Remove friend | H+V | Is friend | Remove (confirm) |
| 17 | Add to favourites | H+V | Is friend, not in favourites | Add to favourites list |
| 18 | Remove from favourites | H+V | Is friend, in favourites | Remove from favourites |
| 19 | Block | H+V | Always | Block user (confirm) |
| 20 | Report | H+V | Always | Open Panel / Report / Sheet |
| 21 | Ban in MM (ctx) | H+V | ctx, has MM history with user | Ban player from future MM matches (only MM, not messenger) |
| 22 | Blocked state | H+V | User is blocked by viewer | — (display: "Пользователь недоступен"; all action buttons hidden) |
| 23 | "In a call" status | H+V | User is in voice call | — (display: "В звонке" badge near presence) |
| 24 | Last seen text | H+V | User offline, visible per privacy | — (display: "был(а) N минут назад") |
| 25 | In-game activity | H+V | User has game running (desktop), visible per privacy | — (display: "Играет в {Game}") |
| 26 | Share / copy profile deep link | H+V | Always | Copy profile `voice.gg/…` deep link |
| 27 | Deleted-account state | H+V | Profile target account deleted | — (display: "Аккаунт удалён"; actions hidden) |

---

## 15. Own Profile Edit (Panel / Profile / Edit)

**Penpot:** `Panel / Profile / Edit`
**Feature docs:** [user-profile.md](../features/user-profile.md), [multi-profile.md](../features/multi-profile.md)

Entry: from settings or profile avatar long-press.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Avatar upload | H+V | Always | Pick image (static free / GIF ★) |
| 3 | Banner upload ★ | H+V | Plus subscriber | Pick banner image |
| 4 | Display name field | H+V | Always | Edit display name |
| 5 | @username field | H+V | Always | Edit username |
| 6 | Bio field | H+V | Always | Edit bio text |
| 7 | Username style upgrade ★ | H+V | Plus subscriber, has #discriminator | Upgrade to clean @username (without #1234 suffix) |
| 7a | Purchase clean @username for extra profile ★ | H+V | Additional profile; clean username sold separately | Open purchase for clean @username perk |
| 8 | Per-profile color theme picker | H+V | Always | Select color theme for visual profile indication |
| 9 | Additional phone field | H+V | Profile supports extra phone binding | Bind optional non-primary phone number |
| 10 | Set as primary profile | H+V | More than one profile exists | Make this profile the one shown in phone search |
| 11 | Privacy preset picker (Personal / Gaming / Work) | H+V | Always | Apply preset defaults for this profile |
| 12 | Email-only account badge | H+V | Account has email, no phone | — (visual: "без телефона") |
| 13 | Delete profile | H+V | More than one profile exists | Delete this profile (confirm; anti-spam rate limits apply) |
| 14 | Save | H+V | Changes made | Save profile |

---

## 15.1 Profile Create (Panel / Profile / Create)

**Penpot:** `Panel / Profile / Create`
**Feature docs:** [multi-profile.md](../features/multi-profile.md), [privacy.md](../features/privacy.md)

Entry: from profile switcher menu #2.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Display name / nickname field | H+V | Always | Input profile nickname |
| 3 | Avatar upload | H+V | Always | Pick image |
| 4 | Additional phone field | H+V | Optional | Bind additional non-primary phone number |
| 5 | Set as primary profile | H+V | Account has other profiles | Make this profile primary for phone search |
| 6 | Privacy preset picker (Personal / Gaming / Work) | H+V | Always | Apply preset defaults for the new profile |
| 7 | Create / Save | H+V | Required fields filled | Create profile |
| 8 | Create rate-limit error | H+V | Anti-spam create/delete limit hit | — (display: cannot create profile yet; try later) |

---

## 16. Registration (Screen / Auth / Register)

**Penpot:** `Screen / Auth / Register`
**Feature docs:** [auth-and-contacts.md](../features/auth-and-contacts.md)

Entry: from Login #8 "Register".

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Display name field | H+V | Always | Input name |
| 2 | Email field | H+V | Always | Input email |
| 3 | Phone field | H+V | Always (tab or toggle) | Input phone number |
| 4 | Password field | H+V | Always | Input password |
| 5 | Confirm password field | H+V | Always | Confirm password |
| 6 | Register | H+V | All required fields filled | Submit registration |
| 7 | Back to login | H+V | Always | Switch to login form |
| 8 | Account type: Personal / Organization | H+V | Always | Select personal vs organization account (org → verification DNS / manual flow) |

---

## 16.1. Password Reset (Screen / Auth / PasswordReset)

**Penpot:** `Screen / Auth / PasswordReset`
**Feature docs:** [auth-and-contacts.md](../features/auth-and-contacts.md)

Entry: from Login #6 "Forgot password".

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Email field | H+V | Always | Input email for reset link |
| 2 | Send reset link | H+V | Email filled | Send password reset email |
| 3 | Back to login | H+V | Always | Return to login |
| 4 | New password field | H+V | After following email link | Input new password |
| 5 | Confirm new password | H+V | After following email link | Confirm password |
| 6 | Reset password | H+V | Both fields filled | Submit new password |

---

## 17. Space Creation (Panel / Space / Create)

**Penpot:** `Panel / Space / Create`
**Feature docs:** [spaces.md](../features/spaces.md)

Entry: from chat list header #5 submenu.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Space name field | H+V | Always | Input name |
| 3 | Icon / avatar upload | H+V | Always | Pick image |
| 4 | Visibility picker (Public / Invite-only / Private) | H+V | Always | Select visibility |
| 5 | Template picker (Игровое / Рабочее / Общение) | H+V | Always | Select template (affects default channels) |
| 6 | Create space | H+V | Name filled | Create space |

---

## 18. Join Space by Invite (Panel / Space / JoinInvite)

**Penpot:** `Panel / Space / JoinInvite`
**Feature docs:** [spaces.md](../features/spaces.md)

Entry: from chat list header #6 submenu or deep link.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Invite code / link field | H+V | Always | Input invite code |
| 3 | Join | H+V | Code filled | Join space |

---

## 19. Space Catalog / Discovery (Screen / Space / Catalog)

**Penpot:** `Screen / Space / Catalog`
**Feature docs:** [spaces.md](../features/spaces.md)

Entry: from chat list header #6 or matchmaking or onboarding.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | V | Always | Return |
| 2 | Search field | H+V | Always | Filter spaces by name / game |
| 3 | Category / game filter | H+V | Always | Filter by category |
| 4 | Space card (icon, name, member count, verified badge) | H+V | Always | Open space preview |
| 5 | Join (per card) | H+V | Not a member | Join space |

---

## 20. Group Creation (Panel / Chat / CreateGroup)

**Penpot:** `Panel / Chat / CreateGroup`
**Feature docs:** [text-chat.md](../features/text-chat.md)

Entry: from chat list header #4 submenu or DM "Add members".

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Group name field | H+V | Always | Input group name |
| 3 | Contact / friend picker (multi-select) | H+V | Always | Select members |
| 4 | Search contacts | H+V | Always | Filter contacts list |
| 5 | Create group | H+V | Name + ≥2 members selected | Create group |

---

## 21. Forward Message (Panel / Chat / ForwardMessage)

**Penpot:** `Panel / Chat / ForwardMessage`
**Feature docs:** [forward-messages.md](../features/forward-messages.md)

Entry: from message action #5 or multi-select bar #2.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Search chats / contacts | H+V | Always | Filter targets |
| 3 | Recent chats list | H+V | Always | — (display) |
| 4 | Chat / contact row (single-select) | H+V | Always | Select one target chat (multi-select per forward-messages.md when enabled) |
| 5 | Comment field (optional) | H+V | ≥1 target selected | Add own comment before forwarded messages |
| 6 | Forward | H+V | ≥1 target selected | Send forwarded message(s) |
| 7 | Cancel | H+V | Always | Close panel |

---

## 22. Global Search Results (Panel / Search / Global)

**Penpot:** `Panel / Search / Global`
**Feature docs:** [search.md](../features/search.md)

Entry: from chat list header #1 search field.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Search text field | H+V | Always | Input query |
| 2 | Close / Clear | H+V | Always | Clear search / close |
| 3 | Section: Contacts & Chats (1st) | H+V | Results found | — (display: avatar, name, last message) |
| 4 | Section: Spaces (2nd) | H+V | Results found | — (display: icon, name, member count) |
| 5 | Section: Messages (3rd) | H+V | Results found | — (display: highlighted match, sender, date, chat name) |
| 6 | Result row tap (contact/chat) | H+V | Always | Open chat |
| 7 | Result row tap (space) | H+V | Always | Open space |
| 8 | Result row tap (message) | H+V | Always | Navigate to message in chat |
| 9 | Load more | H+V | More results available | Paginate (20 per page) |

---

## 23. Role Management (Panel / Space / Roles)

**Penpot:** `Panel / Space / Roles` (list); detail editor `Panel / Space / RoleEditor` (§23.1)
**Feature docs:** [roles.md](../features/roles.md)

Entry: from space admin #9.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Role list (drag-reorderable) | H+V | Always | Reorder priority |
| 3 | Role row tap | H+V | Always | Open `Panel / Space / RoleEditor` |
| 4 | Create role | H+V | Has SPACE_MANAGE_ROLES | Open RoleEditor for new role |
| 5 | Configure default member role | H+V | Owner; on built-in «Участник» row | Open RoleEditor for default member role |

### 23.1 Role Editor (Panel / Space / RoleEditor)

**Penpot:** `Panel / Space / RoleEditor`  
Entry: from Roles list #3–5. Nested surface of Roles — separate Penpot frame.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close editor → Roles list |
| 2 | Role name field | H+V | Always | Edit name |
| 3 | Permissions checkboxes | H+V | Always | Toggle individual permissions (roles.md / role-service) |
| 4 | Chat overrides section | H+V | Always | Set allow/deny per text chat → may open `Panel / Space / ChatOverride` |
| 5 | Voice room overrides section | H+V | Always | Set allow/deny per voice room → may open `Panel / Space / VoiceRoomOverride` |
| 6 | Delete role | H+V | Existing custom role (not built-in) | Delete (confirm) |
| 7 | Save | H+V | Changes made / create | Save role |
| 8 | Denied / no-permission state | H+V | Lacks SPACE_MANAGE_ROLES | — (display; editor read-only or blocked) |

---

## 24. Member Management (Panel / Space / Members)

**Penpot:** `Panel / Space / Members`
**Feature docs:** [spaces.md](../features/spaces.md), [roles.md](../features/roles.md)

Entry: from space admin #8.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Search members field | H+V | Always | Filter member list |
| 3 | Member row (avatar + name + role badges) | H+V | Always | Open ProfileDetail |
| 4 | Assign role (ctx) | H+V | Has MEMBER_ASSIGN_ROLES | Open role picker |
| 5 | Kick (ctx) | H+V | Has MEMBER_KICK | Kick member (confirm) |
| 6 | Ban (ctx) | H+V | Has MEMBER_BAN | Ban member (confirm + optional reason) |
| 7 | Tab: Members | H+V | Always | Show active members |
| 8 | Tab: Bans | H+V | Has MEMBER_BAN | Show ban list |
| 9 | Unban (per ban row) | H+V | Has MEMBER_BAN | Unban user |

---

## 25. Invite Management (Panel / Space / Invites)

**Penpot:** `Panel / Space / Invites`
**Feature docs:** [spaces.md](../features/spaces.md)

Entry: from space admin #7.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Active invites list | H+V | Always | — (display: code, uses, expiry) |
| 3 | Create invite | H+V | Has invite right | Open create form |
| 4 | Expiry picker | H+V | Creating invite | Set expiry (30m / 1h / 6h / 12h / 1d / 7d / never) |
| 5 | Max uses picker | H+V | Creating invite | Set max uses (1 / 5 / 10 / 25 / 50 / 100 / unlimited) |
| 6 | Generate link | H+V | Creating invite | Generate and copy invite link |
| 7 | QR code | H+V | Invite created | Display QR code for sharing |
| 8 | Copy link (per invite) | H+V | Always | Copy invite URL to clipboard |
| 9 | Revoke (per invite) | H+V | Has invite right | Revoke invite (confirm) |

---

## 25.1. Space Join Verification (Panel / Space / JoinVerification)

**Penpot:** `Panel / Space / JoinVerification`
**Feature docs:** [spaces.md](../features/spaces.md)

Entry: when joining a space that requires verification.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Phone verification prompt | H+V | Space requires phone | Verify phone number |
| 2 | Captcha widget | H+V | Space requires captcha | Solve captcha |
| 3 | Screening questions | H+V | Space has screening | Answer questions (text fields) |
| 4 | Submit answers | H+V | Screening, all answered | Submit for moderator review |
| 5 | Pending approval status | H+V | Manual approval required | — (display: "Ожидание одобрения модератором") |
| 6 | Cancel | H+V | Always | Cancel join attempt |

---

## 26. Bot Management in Space (Panel / Space / Bots)

**Penpot:** `Panel / Space / Bots`
**Feature docs:** [bots.md](../features/bots.md)

Entry: from space admin #10.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Installed bots list | H+V | Always | — (display: icon, name, status online/offline) |
| 3 | Bot row tap | H+V | Always | Open bot detail (per-chat toggles) |
| 4 | Per-chat toggle (per bot) | H+V | Inside bot detail | Enable/disable bot in specific chat |
| 5 | Remove bot | H+V | Has SPACE_MANAGE_BOTS | Remove bot from space (confirm) |
| 6 | Add bot | H+V | Has SPACE_MANAGE_BOTS | Navigate to bot catalog / URL input |
| 6a | Add bot by URL / deep link | H+V | Has SPACE_MANAGE_BOTS | Input `voice.app/bots/…` or open deep-link install → §8.1 |

---

## 27. Verification Settings (Panel / Settings / Verification)

**Penpot:** `Panel / Settings / Verification`
**Feature docs:** [verification.md](../features/verification.md)

Entry: from linked accounts screen when user opens detailed verification status / organization verification.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Section: Personal verification | H+V | Always | — (display current status) |
| 3 | Manage linked platforms | H+V | Personal verification needed | Open Panel / Settings / LinkedAccounts |
| 4 | Verification badge status | H+V | Always | — (display: verified / not verified / pending; badge rendered as system icon) |
| 5 | Section: Organization verification | H+V | Always | — |
| 6 | Domain field | H+V | Organization account | Input official domain |
| 7 | DNS TXT instruction | H+V | Domain submitted | — (display: TXT record to add) |
| 8 | Verify DNS | H+V | Domain submitted | Trigger DNS check |
| 9 | Create / switch to organization account | H+V | Personal account starting org verification | Start organization account type flow |
| 10 | Manual org verification request | H+V | Org without domain or priority partner path | Submit email application for manual review |

---

## 27.1. E2E Key Backup (Panel / Settings / E2EKeyBackup)

**Penpot:** `Panel / Settings / E2EKeyBackup`
**Feature docs:** [encryption.md](../features/encryption.md)

Entry: from security settings or E2E chat info prompt.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Backup status | H+V | Always | — (display: "Настроено" / "Не настроено") |
| 3 | Set backup password / PIN | H+V | Not yet configured | Input password → encrypt and upload keys |
| 4 | Change backup password | H+V | Already configured | Re-encrypt with new password |
| 5 | Restore keys | H+V | On new device, backup exists | Input password → decrypt and import keys |
| 6 | Delete backup | H+V | Backup exists | Delete server-side encrypted blob (confirm) |

---

## 28. Linked Accounts (Panel / Settings / LinkedAccounts)

**Penpot:** `Panel / Settings / LinkedAccounts`
**Feature docs:** [verification.md](../features/verification.md)

Entry: from settings nav #5.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Connected platforms list | H+V | Always | — (display: platform icon + name + status) |
| 3 | Connect Twitch | H+V | Not connected | Start OAuth flow |
| 4 | Connect YouTube | H+V | Not connected | Start OAuth flow |
| 5 | Disconnect (per platform) | H+V | Connected | Disconnect (confirm) |
| 6 | Verification status | H+V | Always | Open Panel / Settings / Verification |
| 7 | Organization verification | H+V | Organization account or org flow available | Open Panel / Settings / Verification |

---

## 29. Appearance Settings (Panel / Settings / Appearance)

**Penpot:** `Panel / Settings / Appearance`
**Feature docs:** [accessibility.md](../features/accessibility.md), [navigation.md](../features/navigation.md), [i18n.md](../features/i18n.md)

Entry: from settings nav #7.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Theme picker (Light / Dark / System / High Contrast) | H+V | Always | Select theme |
| 3 | Language picker (EN / RU) | H+V | Always | Select language |
| 4 | Chat themes ★ | H+V | Always (upsell if not Plus) | Open Panel / Settings / ChatThemes |
| 5 | App icon ★ | H+V | Always (upsell if not Plus) | Open Panel / Settings / AppIcon |

---

## 30. Accessibility Settings (Panel / Settings / Accessibility)

**Penpot:** `Panel / Settings / Accessibility`
**Feature docs:** [accessibility.md](../features/accessibility.md)

Entry: from settings nav #8.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Reduced motion toggle | H+V | Always | Toggle reduced motion |
| 3 | Font scale slider | H+V | Always | Adjust font scale (100% — 200%) |
| 4 | PTT keybind | H+V | Always (desktop relevant) | Set custom Push-to-Talk key (accessibility.md) |
| 5 | PTT / VAD default mode | H+V | Always | Prefer PTT or voice-activity as default for new calls |

---

## 31. Appeal (Panel / Settings / Appeal)

**Penpot:** `Panel / Settings / Appeal`
**Feature docs:** [reports.md](../features/reports.md)

Entry: from settings nav #9 (visible only when user has active sanction).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Sanction info | H+V | Always | — (display: type, date, reason) |
| 3 | Appeal text field | H+V | No prior appeal for this sanction | Input appeal text |
| 4 | Submit appeal | H+V | Text filled | Submit (one appeal per sanction; up to 7 business days) |
| 5 | Appeal status | H+V | Appeal already submitted | — (display: pending / reviewed) |

---

## 32. Story Viewers (Panel / Stories / StoryViewers)

**Penpot:** `Panel / Stories / StoryViewers`
**Feature docs:** [stories.md](../features/stories.md)

Entry: from story viewer #15 (own story, viewers count tap).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Viewer row (avatar + name) | H+V | Always | Open ProfileDetail |
| 3 | Total viewers count | H+V | Always | — (display) |

---

## 33. Screen Share Picker (Overlay / Call / ScreenSharePicker)

**Penpot:** `Overlay / Call / ScreenSharePicker` (start picker — **required design inventory**; may still be missing as shipped canon in screens.md — track as mockup gap)  
**Related (not alias):** `Panel / Call / ScreenShare` = in-call viewer / manage panel (§10.2c). **Keep both** — picker (start) ≠ panel (viewer/manage). Do not rename one into the other or drop either from the design system.
**Feature docs:** [screen-share.md](../features/screen-share.md)

Entry: from call controls #4 (screen share).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Tab: Entire screen | H | Always | Select full screen |
| 2 | Tab: Window | H | Always | Select application window |
| 3 | Tab: Browser tab | H | Always | Select browser tab |
| 4 | Include system audio toggle | H | Always | Toggle audio capture; **disabled on web** with tooltip (browser cannot capture system audio — platforms.md / screen-share.md) |
| 5 | FPS picker | H | Always | Select frame rate |
| 6 | Resolution picker | H | Always | Select resolution (360p/30fps free; 720p/30fps ★) — shows tier labels |
| 7 | Share | H | Source selected | Start screen share |
| 8 | Cancel | H | Always | Close picker |

---

## 34. Stream Viewer Picker (in call, multiple screen shares)

**Penpot:** `Overlay / Call / StreamViewerPicker`
**Feature docs:** [screen-share.md](../features/screen-share.md)

When multiple participants share screen simultaneously (up to 3).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Stream thumbnail (per sharer) | H+V | >1 active screen share | Switch to that stream |
| 2 | Active stream indicator | H+V | Viewing a stream | — (visual highlight on current) |
| 3 | Paused stream badge | H+V | Sharer paused stream | — (display pause icon over frozen last frame) |
| 4 | Stream limit reached state | H+V | Already 3 active streams in call | — (display limit reached / cannot start another stream) |

---

## 35. Slash Command Menu (Panel / Chat / SlashCommandMenu)

**Penpot:** `Panel / Chat / SlashCommandMenu`; parameter form `Panel / Chat / SlashCommandOptions` (§35.1)
**Feature docs:** [bots.md](../features/bots.md)

Entry: from composer #10 (typing `/` in space context).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Command search field | H+V | Always | Filter commands |
| 2 | Bot group header | H+V | Always (per bot) | — (display bot name + icon) |
| 3 | Command row (name + description) | H+V | Bot online and enabled in chat | Select command → if has options, open §35.1; else invoke |
| 4 | Grayed-out command | H+V | Bot offline | — (display with "Bot unavailable" tooltip) |

### 35.1 Slash Command Options (Panel / Chat / SlashCommandOptions)

**Penpot:** `Panel / Chat / SlashCommandOptions`  
Entry: after selecting a command that has options (bots.md parameter types).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Cancel → return to menu or composer |
| 2 | Command title | H+V | Always | — (display selected command name) |
| 3 | String / integer field | H+V | Option type string/integer | Input value |
| 4 | Boolean toggle | H+V | Option type boolean | Toggle yes/no |
| 5 | User picker | H+V | Option type user | Pick space member |
| 6 | Channel picker | H+V | Option type channel | Pick text chat (`group` \| `channel`) |
| 7 | Role picker | H+V | Option type role | Pick role |
| 8 | Attachment picker | H+V | Option type attachment | OS file picker → upload |
| 9 | Autocomplete suggestions | H+V | String option with `autocomplete: true` | Select suggestion (bots.md, ≤25) |
| 10 | Required marker | H+V | Option required | — (visual `*` on label) |
| 11 | Submit / Run | H+V | All required options filled | Invoke slash command |
| 12 | Cancel | H+V | Always | Dismiss without invoke |

---

## 36. Help (Panel / Settings / Help)

**Penpot:** `Panel / Settings / Help`
**Feature docs:** [accessibility.md](../features/accessibility.md)

Entry: from settings nav #6.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | FAQ / support link | H+V | Always | Open external help page |
| 3 | Keyboard shortcuts overlay | H | Always (desktop) | Open keyboard shortcut reference table |
| 4 | Contact support | H+V | Always | Open support form / email |

---

## 37. Sticker Management (Panel / Settings / Stickers)

**Penpot:** `Panel / Settings / Stickers`
**Feature docs:** [text-chat.md](../features/text-chat.md)

Entry: from settings nav #10.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Installed sticker packs | H+V | Always | — (display: pack icon + name + count) |
| 3 | Pack row tap | H+V | Always | Preview stickers in pack |
| 4 | Remove pack | H+V | User-added packs | Remove sticker pack |
| 5 | Upload own pack | H+V | Always | Upload custom sticker pack (images) |
| 6 | Browse sticker store | H+V | Always | Open sticker catalog |
| 7 | Premium sticker indicator ★ | H+V | Pack requires subscription | — (display: ★ badge on premium packs) |

---

## 38. Chat Theme Picker ★ (Panel / Settings / ChatThemes)

**Penpot:** `Panel / Settings / ChatThemes`
**Feature docs:** [subscription.md](../features/subscription.md)

Entry: from settings nav #7 (Appearance → Chat themes) or Chat Info #22.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Theme preview grid | H+V | Plus subscriber | Browse exclusive chat themes |
| 3 | Theme card tap | H+V | Plus subscriber | Preview theme on current chat |
| 4 | Apply | H+V | Theme selected | Apply theme to chat |
| 5 | Reset to default | H+V | Custom theme active | Reset to default appearance |

---

## 39. App Icon Picker ★ (Panel / Settings / AppIcon)

**Penpot:** `Panel / Settings / AppIcon`
**Feature docs:** [subscription.md](../features/subscription.md)

Entry: from settings Appearance #5 (App icon).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Icon grid | H+V | Plus subscriber | Browse custom app icons |
| 3 | Icon tap | H+V | Plus subscriber | Select icon |
| 4 | Apply | H+V | Icon selected | Change app icon |

---

## 40. Deep Link Error Screens

**Penpot:** `Screen / DeepLinks / Error`
**Feature docs:** [deep-links.md](../features/deep-links.md)

Shown when navigating via `voice.gg/…` deep links.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | "Нет доступа" (403) | H+V | User lacks permission to object | — (display error); CTA: "На главную" |
| 2 | "Не найдено" (404) | H+V | Object deleted or invalid link | — (display error); CTA: "На главную" |
| 3 | "Аккаунт удалён" | H+V | Deep link to deleted user profile | — (display error / deleted profile state); CTA: "На главную" |
| 4 | Federated space unavailable (deferred) | H+V | Federation deferred; linked remote space unreachable | — (display: "Спейс недоступен") |

---

## 41. Bot Webhook Configuration

**Penpot:** `Panel / Bots / WebhookConfig`
**Feature docs:** [bots.md](../features/bots.md)

Entry: from bot management panel.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Webhook URL field | H+V | Creating incoming webhook | Input webhook endpoint |
| 2 | Target channel picker | H+V | Always | Select channel for webhook messages |
| 3 | Webhook name / avatar | H+V | Always | Customize webhook identity |
| 4 | Create webhook | H+V | URL + channel selected | Generate webhook URL |
| 5 | Copy webhook URL | H+V | Webhook created | Copy to clipboard |
| 6 | Delete webhook | H+V | Webhook exists | Delete (confirm) |

---

## 42. Bot Catalog / App Directory

**Penpot:** `Screen / Bots / Catalog`
**Feature docs:** [bots.md](../features/bots.md)

Entry: from bot management "Add bot" or discovery.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Search bots field | H+V | Always | Filter bots by name / category |
| 3 | Category filter | H+V | Always | Filter by category |
| 4 | Bot card (icon, name, description, install count) | H+V | Always | Open bot detail → §8.1 Install |
| 5 | Install (per card) | H+V | Not installed in current space | Quick install → §8.1 |

---

## 43. In-Game Overlay

**Penpot:** `Overlay / Platform / InGame`
**Feature docs:** [platforms.md](../features/platforms.md)

Desktop overlay rendered on top of running game.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Speaking indicators (per participant) | H (overlay) | In voice call, game running | — (display: who is speaking) |
| 2 | Mute / Unmute mic | H (overlay) | In voice call | Toggle microphone |
| 3 | Deafen / Undeafen | H (overlay) | In voice call | Toggle all audio |
| 4 | Toggle overlay visibility | H (overlay) | Always (hotkey) | Show / hide overlay |

---

## 44. Matchmaking Player Profile (Panel / Matchmaking / PlayerProfile)

**Penpot:** `Panel / Matchmaking / PlayerProfile`
**Feature docs:** [matchmaking.md](../features/matchmaking.md), [user-profile.md](../features/user-profile.md)

Entry: from MatchSquad player row.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Avatar + display name | H+V | Always | — (display) |
| 3 | MM average rating | H+V | Visible per privacy settings | — (display) |
| 4 | Send message | H+V | Allowed by privacy | Open / create DM |
| 5 | Add friend | H+V | Not friends | Send friend request |
| 6 | Ban in MM | H+V | Has MM history / squad context | Ban user from future MM matches |

---

## 45. Chat Thread (Panel / Chat / Thread)

**Penpot:** `Panel / Chat / Thread`
**Feature docs:** [text-chat.md](../features/text-chat.md)

Entry: from chat header #7, message bubble #5, or message action #9.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close thread panel / screen |
| 2 | Parent message preview | H+V | Always | Jump to original message in main timeline |
| 3 | Thread replies list | H+V | Thread has replies | Open/inspect thread replies |
| 4 | Reply field | H+V | Thread open | Type reply to thread |
| 5 | Send reply | H+V | Reply field non-empty | Send thread reply |
| 6 | Thread participants summary | H+V | Thread has participants | — (display avatars / count) |

---

## 46. Highlight Edit (Panel / Stories / HighlightEdit)

**Penpot:** `Panel / Stories / HighlightEdit`
**Feature docs:** [stories.md](../features/stories.md)

Entry: from archive #4, highlights #3, or highlights #4.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Highlight name field | H+V | Always | Input highlight name |
| 3 | Cover / preview picker | H+V | Stories selected | Choose cover story / preview |
| 4 | Stories checklist | H+V | Archive has eligible stories | Select stories to include |
| 5 | Privacy picker | H+V | Always | Set per-highlight audience |
| 6 | Save | H+V | Name + ≥1 story selected | Save highlight |
| 7 | Delete highlight | H+V | Editing existing highlight | Delete (confirm) |

---

## 47. Chat Slow Mode (Panel / Space / ChatSlowMode)

**Penpot:** `Panel / Space / ChatSlowMode`
**Feature docs:** [spaces.md](../features/spaces.md), [text-chat.md](../features/text-chat.md)

Entry: from space admin #14.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Slow mode interval picker | H+V | Always | Select interval from 5 sec to 6 h |
| 3 | Disable slow mode | H+V | Slow mode active | Turn off slow mode |
| 4 | Save | H+V | Changes made | Save slow mode setting |

---

## 48. Chat Override (Panel / Space / ChatOverride)

**Penpot:** `Panel / Space / ChatOverride`
**Feature docs:** [roles.md](../features/roles.md)

Entry: from role detail or space admin #15 for a text chat.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Role selector | H+V | Always | Pick role to edit |
| 3 | Allow permissions list | H+V | Role selected | Toggle allow flags for this text chat |
| 4 | Deny permissions list | H+V | Role selected | Toggle deny flags for this text chat |
| 5 | Save | H+V | Changes made | Save override |

---

## 49. Voice Room Override (Panel / Space / VoiceRoomOverride)

**Penpot:** `Panel / Space / VoiceRoomOverride`
**Feature docs:** [roles.md](../features/roles.md), [voice-chat.md](../features/voice-chat.md)

Entry: from role detail or space admin #15 for a voice room.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Role selector | H+V | Always | Pick role to edit |
| 3 | Allow permissions list | H+V | Role selected | Toggle allow voice permissions for this room |
| 4 | Deny permissions list | H+V | Role selected | Toggle deny voice permissions for this room |
| 5 | Save | H+V | Changes made | Save override |

---

## 50. Space Verification Settings (Panel / Space / VerificationSettings)

**Penpot:** `Panel / Space / VerificationSettings`
**Feature docs:** [spaces.md](../features/spaces.md)

Entry: from space admin #17.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Require confirmed phone | H+V | Always | Toggle phone verification on join |
| 3 | Require captcha | H+V | Always | Toggle captcha on join |
| 4 | Enable screening questions | H+V | Always | Toggle screening questionnaire on join (enables editor) |
| 5 | Screening questions editor | H+V | Screening enabled (#4 on) | Add/edit/remove join questions |
| 6 | Require manual approval | H+V | Always | Toggle moderator approval before entry |
| 7 | Save | H+V | Changes made | Save verification rules |

---

## 51. LFP Request Decision (Overlay / Stories / LFPRequest)

**Penpot:** `Overlay / Stories / LFPRequest`
**Feature docs:** [matchmaking.md](../features/matchmaking.md), [stories.md](../features/stories.md)

Entry: when the author receives a Join or Invite action from an LFP story.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Request type badge | H+V | Always | — (display: join request / party invite) |
| 2 | Sender avatar + name | H+V | Always | Open requester profile |
| 3 | Rank / party details | H+V | Data provided in request | — (display viewer rank or inviter party details) |
| 4 | Accept | H+V | Request active | Accept join/invite request |
| 5 | Decline | H+V | Request active | Decline request |
| 6 | Write message | H+V | Request active | Open DM with requester |

---

## 52. Report Submitted Toast

**Penpot:** `Overlay / Report / Success`
**Feature docs:** [reports.md](../features/reports.md)

Entry: after successful report submit.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Success text | H+V | Report submitted | — (display: "Жалоба принята. Мы рассмотрим её в ближайшее время") |
| 2 | Dismiss | H+V | Toast/banner shown | Close success state |

---

## 53. Chat Bot Controls (Panel / Chat / Bots)

**Penpot:** `Panel / Chat / Bots`
**Feature docs:** [bots.md](../features/bots.md)

Entry: from chat info #18 for a space text chat.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Bot row | H+V | Installed bots exist in space | Inspect per-chat bot availability |
| 3 | Enable / disable toggle | H+V | Per bot row | Toggle bot availability in this text chat |
| 4 | Offline badge | H+V | Bot offline | — (display: bot unavailable) |

---

## 54. Empty / Error / Offline states

**Penpot:** `State / Chat / Empty`, `State / Chat / Error`, `State / Network / Offline` (desktop page `12_States_Desktop`; mobile twins on `17_States_Mobile` — page reserved, canons TBD)
**Feature / UX refs:** [brand.md](brand.md) (empty / offline guidance), [text-chat.md](../features/text-chat.md) (reconnect), [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (WS resume + Messaging catch-up)

Not full product screens — reusable state chrome used inside Shell / Chat list / Chat room.

### 54.1 State / Chat / Empty

**Product decision:** messenger-first. Primary empty CTA is **chat/compose-oriented** (start DM / browse chats / invite) — match neighboring shell/list controls (§1 compose, invite patterns). Matchmaking / Space Catalog CTAs are **secondary only**, never the default empty-chat primary.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Empty illustration / icon | H+V | Folder or chat list has zero items | — (visual; keep minimal per brand.md) |
| 2 | Short reason copy | H+V | Always | — (display; e.g. no chats in folder) |
| 3 | Primary CTA (compose / start DM / invite) | H+V | Always | Open compose / start DM / invite — same affordance family as shell list empty (§1) |
| 4 | Secondary CTA (optional) | H+V | When product copy offers alternate | Space Catalog / Matchmaking — secondary only; not the default empty primary |

### 54.2 State / Chat / Error

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Error title / icon | H+V | Load/send/history failed | — (display) |
| 2 | Short error reason | H+V | Always | — (display; avoid raw stack traces) |
| 3 | Retry | H+V | Always | Retry last failed load / request |
| 4 | Secondary: go home / back | H+V | Optional | Navigate away from broken surface |

### 54.3 State / Network / Offline

**Penpot:** `State / Network / Offline` (compact banner over shell — **not** a modal or blocking full-screen)  
**Product decision:** **dismissible**, Telegram-like, non-blocking. Not sticky until online — user can hide the banner; reconnect may continue in background; banner may reappear on a later failure.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Offline / reconnecting banner | H+V | No network or Realtime disconnected | — (display: offline / reconnecting; non-blocking chrome) |
| 2 | Status detail | H+V | Always | — (display: drafts kept locally / send pending — brand.md) |
| 3 | Retry / Reconnect CTA | H+V | When manual retry is useful | Trigger reconnect + Messaging catch-up |
| 4 | Dismiss | H+V | Always (soft banner) | Hide banner without waiting for online; may re-show on next disconnect / failure |

---

## H vs V layout differences summary

| Area | Horizontal (desktop/web/tablet) | Vertical (mobile) |
|------|---|----|
| Shell | 3-column: rail (nav + folders + quick access + profiles + ☰) + list + room | **Normative:** bottom tab bar (Chats / Social / MM) + **drawer** for folders + Quick Access + Settings; list → room push; active strip when room open |
| Profile switch | Click avatar → menu (§1.1a); Archive entry | Tap avatar → menu; swipe on avatar to switch profiles |
| Chat header | Inline in room panel; side panel toggle; last seen in status | Full-width app bar with back arrow |
| Message actions | Hover toolbar on bubble + ctx | Long-press ctx + bottom selection bar |
| Settings | ☰ at rail bottom → `Panel / Settings / Sheet` | Drawer or settings entry from tab bar |
| Panels | Side panel (right drawer) via header toggle | Bottom sheet or full-screen push |
| Overlays (call, MM) | Centered modal over content | Full-screen overlay |
| Incoming call (iOS) | App overlay | CallKit / PushKit native chrome (not Flutter inventory) |
| Screen share (start) | Available in call; web: «Include system audio» disabled + tooltip | Not available (view only) |
| Composer | `[📎 click→popup] [input] [😊 click→panel] [mic\|send]`; long-press Send → menu | Attach tap→sheet; emoji tap→panel; mic/send swap same |
| Composer format toolbar | Optional above composer; ПКМ formatting primary | Hidden behind toggle (toolbar button) |
| Search | Panel or inline in header | Full-screen search overlay |
| Thread panel | Right sidebar alongside room | Push screen |
| Folders | Rail scroll zone (§1.1b); **no middle-column tabs** | Drawer / compact rail |
| Archive | Profile avatar ctx → `Screen / Chat / Archive` | Same entry via profile menu |
| Pinned messages | Persistent bar under header (§3.1a) | Same bar; full list in sheet |
| Stories viewer | Modal overlay over content | Full-screen swipe-native overlay |
| Stories create | Modal / panel | Full-screen push |
| In-game overlay (§43) | H only | N/A |
| Empty / error (§54) | Full or panel-sized state chrome | Same pattern; mobile twins TBD on `17_States_Mobile` |
| Offline banner (§54.3) | Compact banner over shell | Compact banner / toast strip |
