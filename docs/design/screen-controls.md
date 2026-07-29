# Screen Controls — кнопки и интерактивные элементы по экранам

> **Source of truth для Penpot-отрисовки и Flutter-реализации.**
> Собрано из `docs/features/*.md`. Не дублирует продуктовую логику — ссылки на спеки.
> При расхождении `features/*.md` побеждает.

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

**Penpot:** `Screen / Shell / Desktop`, `Screen / Shell / Mobile`
**Feature docs:** [navigation.md], [multi-profile.md], [presence.md], [stories.md], [search.md]

### 1.1 Rail / Tab bar

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Chats tab | H+V | Always | Show chat list |
| 2 | Social / Friends tab | H+V | Always | Show social panel |
| 3 | Matchmaking tab | H+V | Always | Open MM catalog |
| 4 | Settings tab | H+V | Always | Open settings sheet / screen |
| 5 | Profile avatar | H — rail bottom; V — tab bar or header | Always | Open profile switcher menu |
| 6 | Stories ring | H+V | User has active story (overlay on profile avatar) | Open story viewer |

**Profile switcher menu** (opens from #5):

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Profile row (per profile) | H+V | Always | Switch active profile |
| 2 | Create profile | H+V | < profile limit (2 free / 5★) | Navigate to Panel / Profile / Create |
| 3 | Presence picker (Online / Idle / DND / Invisible) | H+V | Always | Set presence status |
| 4 | Custom status ★ | H+V | Plus subscriber | Open custom status editor |
| 5 | Create story | H+V | Always | Navigate to Screen / Stories / Create |

### 1.2 Chat list header

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Search field | H+V | Always | Focus → global search (Panel / Search / Global) |
| 2 | New chat / compose | H+V | Always | Show menu: New DM · Create group · Create/join space |
| 3 | New DM (submenu) | H+V | From #2 | Open contact picker → create DM |
| 4 | Create group (submenu) | H+V | From #2 | Open Panel / Chat / CreateGroup |
| 5 | Create space (submenu) | H+V | From #2 | Open Panel / Space / Create |
| 6 | Join space (submenu) | H+V | From #2 | Open Panel / Space / JoinInvite or catalog |

### 1.3 Folder tabs

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Все (All) | H+V | Always | Filter: all chats |
| 2 | ЛС (DM) | H+V | Always | Filter: DM only |
| 3 | Группы | H+V | Always | Filter: groups |
| 4 | Каналы | H+V | Always | Filter: channels |
| 5 | Спейсы | H+V | Always | Filter: spaces |
| 6 | Custom folder N | H+V | User created custom folders | Filter by custom folder |
| 7 | Edit folders | H+V | Always (icon or long-press) | Manage / reorder / create custom folders |
| 8 | Message requests | H+V | Has pending requests from non-contacts | Open requests folder |

### 1.4 Chat list row (per chat)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Row tap | H+V | Always | Open chat room |
| 2 | Stories ring on avatar | H+V | Contact has active story | Open story viewer |
| 3 | Pin (ctx) | H+V | ctx | Toggle pin to top |
| 4 | Mute / Unmute (ctx) | H+V | ctx | Toggle mute |
| 5 | Archive (ctx) | H+V | ctx (DM) | Move to archive |
| 6 | Mark read / unread (ctx) | H+V | ctx | Toggle read state |
| 7 | Delete chat (ctx) | H+V | ctx (DM) | Delete chat for self |

### 1.5 Active chats strip (V only)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Mini avatar icon | V | Chat open, other active chats exist | Switch to that chat |
| 2 | Unread badge on icon | V | Chat has unread | — (visual indicator) |
| 3 | Scroll left/right | V | > visible width | Scroll strip |

---

## 2. Auth screens

**Penpot:** `Screen / Auth / Login`, `Screen / Auth / GuestNickname`
**Feature docs:** [auth-and-contacts.md]

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

### 2.2 GuestNickname

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Nickname field | H+V | Always | Input nickname |
| 2 | Join as guest | H+V | Nickname filled | Create guest → main shell |

### 2.3 Guest convert reminder (banner, not screen)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Register CTA | H+V | Guest, ≥ 2nd session, ≤ 1/day | Open Panel / Auth / GuestConvert |
| 2 | Dismiss | H+V | Same | Hide banner for today |

---

## 3. Chat Room

**Penpot:** `Screen / Chat / Room`
**Feature docs:** [text-chat.md], [voice-chat.md], [search.md], [forward-messages.md], [encryption.md]

### 3.1 Header

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back arrow | V | Always | Return to chat list |
| 2 | Avatar + Name + Status | H+V | Always | Open Panel / Chat / Info (H) or push screen (V) |
| 3 | Voice call | H+V | DM or GRP (temporary voice) | Start voice call (DM) / create temp voice room (GRP) → Call overlay |
| 4 | Video call | H+V | DM only | Start video call → Call overlay |
| 5 | Search in chat | H+V | Always | Toggle in-chat search bar |
| 6 | Pinned messages | H+V | Has pinned messages | Open pinned list popover / panel |
| 7 | Thread (sidebar icon) | H | GRP/CH with threads enabled | Toggle Panel / Chat / Thread sidebar |
| 8 | More / kebab | H+V | Always | ctx → Mute, Report, Info, E2E toggle (DM), etc. |
| 9 | E2E lock icon | H+V | DM with E2E enabled | Visual indicator; tap → E2E info / verification code |

### 3.2 In-chat search bar (toggled by 3.1 #5)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Search text field | H+V | Search active | Input search query |
| 2 | Previous match (↑) | H+V | Results found | Scroll to previous match |
| 3 | Next match (↓) | H+V | Results found | Scroll to next match |
| 4 | Close search | H+V | Search active | Close search bar |

### 3.3 Message bubbles

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Delivery ticks | H+V | DM, outgoing | — (visual: ✓ delivered, ✓✓ read) |
| 2 | Timestamp | H+V | Always (on bubble or on tap/hover) | — (display) |
| 3 | View count | H+V | GRP / CH | — (visual) |
| 4 | Reaction chips | H+V | Message has reactions | Tap chip to toggle own reaction; long-press → who reacted |
| 5 | Thread reply count | H+V | Message has thread replies, threads enabled | Open thread panel with this message |
| 6 | Link preview | H+V | Message contains URL | Tap → open URL in browser |
| 7 | Spoiler overlay | H+V | Message has `||spoiler||` | Tap to reveal |
| 8 | Image / video thumbnail | H+V | Message has media | Tap → fullscreen viewer |
| 9 | File attachment chip | H+V | Message has file | Tap → download / preview |
| 10 | Voice message player | H+V | Message is voice note | Play / pause / scrub |
| 11 | Sticker (full) | H+V | Message is sticker | Tap → add to collection? (cosmetic) |

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
| 8 | Pin / Unpin | H+V | Has PIN_MESSAGES right | Toggle pin |
| 9 | Open thread | H+V | Threads enabled (GRP/CH) | Open / create thread → Panel / Chat / Thread |
| 10 | Select (multi) | H+V | Always | Enter multi-select mode → shows selection bar |
| 11 | Report message | H+V | Not own message | Open Panel / Report / Sheet |
| 12 | Share / deep link | H+V | Always | Copy `voice.gg/…/m/{id}` to clipboard |

### 3.5 Multi-select bar (shown when 3.4 #10 active)

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Selected count | H+V | Multi-select active | — (display) |
| 2 | Forward selected | H+V | Multi-select active | Open Panel / Chat / ForwardMessage with selected |
| 3 | Delete selected | H+V | Own messages or MANAGE_MESSAGES | Delete all selected |
| 4 | Cancel selection | H+V | Multi-select active | Exit multi-select |

### 3.6 Composer

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Text input | H+V | Always | Type message (supports markdown) |
| 2 | Reply preview | H+V | Reply-to set | Shows quoted message; X to cancel |
| 3 | Edit preview | H+V | Editing message | Shows original; X to cancel edit |
| 4 | Emoji button | H+V | Always | Open emoji / sticker / GIF picker |
| 5 | Attach file | H+V | Always | OS file picker → upload |
| 6 | GIF button | H+V | Always (or inside emoji picker tab) | Open GIF search |
| 7 | Stickers button | H+V | Always (or inside emoji picker tab) | Open sticker picker |
| 8 | Voice message | H+V | Text input empty | Hold → record; release → send (or tap → toggle recording mode) |
| 9 | Send | H+V | Text input non-empty | Send message |
| 10 | Slash command `/` | H+V | Typing `/` at start in SP context | Show Panel / Chat / SlashCommandMenu |
| 11 | Format toolbar | H — always visible; V — toggle | Text selected or toolbar toggled | Bold / Italic / Underline / Strikethrough / Spoiler / Code / Quote / Heading / List |
| 12 | @mention autocomplete | H+V | Typing `@` | Show member list popup |
| 13 | Slow mode timer | H+V | SP chat with slow mode, user recently sent | Shows remaining time; send disabled |

### 3.7 Date separator / scroll controls

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Scroll to bottom (FAB) | H+V | Scrolled up from bottom | Scroll to latest message |
| 2 | Unread separator | H+V | New messages since last visit | — (visual divider "N new messages") |

---

## 4. Social / Friends Panel

**Penpot:** `Screen / Social / Panel`
**Feature docs:** [friends.md], [user-profile.md], [presence.md]

### 4.1 Header / tabs

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Title "Friends" | H+V | Always | — |
| 2 | Add friend | H+V | Always | Open search / @username input |
| 3 | Tab: Online | H+V | Always | Filter to online friends |
| 4 | Tab: All | H+V | Always | Show all friends |
| 5 | Tab: Pending | H+V | Always | Show incoming/outgoing requests |
| 6 | Tab: Blocked | H+V | Always | Show blocked users |
| 7 | Tab: Favourites | H+V | Always | Show favourites list |

### 4.2 Friend row

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Avatar + Name + Status | H+V | Always | Open Panel / Social / ProfileDetail |
| 2 | Message | H+V | Always | Open / create DM |
| 3 | Voice call | H+V | Always | Start DM voice call |
| 4 | More (ctx) | H+V | ctx | Remove friend / Block / Report |
| 5 | Stories ring | H+V | Friend has active story | Open story viewer |

### 4.3 Pending request row

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Accept | H+V | Incoming request | Accept friend request |
| 2 | Decline | H+V | Incoming request | Decline request |
| 3 | Cancel | H+V | Outgoing request | Cancel own request |

---

## 5. Settings

**Penpot:** `Screen / Settings / Privacy`, `Security`, `Notifications`, `Subscription`
**Feature docs:** [privacy.md], [auth-and-contacts.md], [notifications.md], [subscription.md], [encryption.md], [verification.md]

### 5.0 Settings sheet / nav

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Nav: Security | H — sidebar; V — list row | Always | Open Security screen |
| 2 | Nav: Privacy | H — sidebar; V — list row | Always | Open Privacy screen |
| 3 | Nav: Notifications | H — sidebar; V — list row | Always | Open Notifications screen |
| 4 | Nav: Subscription | H — sidebar; V — list row | Always | Open Subscription screen |
| 5 | Nav: Linked accounts | H — sidebar; V — list row | Always | Open linked accounts (Twitch/YouTube) |
| 6 | Nav: Help | H — sidebar; V — list row | Always | Open Panel / Settings / Help |
| 7 | Nav: Verification | H — sidebar; V — list row | Always | Open Panel / Settings / Verification |
| 8 | Nav: Appearance | H — sidebar; V — list row | Always | Theme (light/dark/system/high-contrast), language |
| 9 | Nav: Accessibility | H — sidebar; V — list row | Always | Reduced motion toggle, font scale |
| 10 | Log out | H+V | Always | Log out (confirm) |

### 5.1 Privacy

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Privacy preset (Personal/Gaming/Work) | H+V | Always | Apply preset defaults |
| 2 | Who can message me | H+V | Always | Audience picker (All/Friends/Friends of friends/Space members/Nobody/Guests) |
| 3 | Who can call me | H+V | Always | Audience picker |
| 4 | Who can invite to chats/spaces | H+V | Always | Audience picker |
| 5 | Who can send files | H+V | Always | Audience picker |
| 6 | Who can send voice msgs | H+V | Always | Audience picker |
| 7 | Who can add as friend | H+V | Always | Audience picker |
| 8 | Online status visibility | H+V | Always | Audience picker |
| 9 | In-game status visibility | H+V | Always | Audience picker |
| 10 | MM rating visibility | H+V | Always | Audience picker |
| 11 | Phone visibility | H+V | Always | Audience picker |
| 12 | Story visibility | H+V | Always | Audience picker |
| 13 | Search by phone | H+V | Always | Audience picker |
| 14 | Read receipts | H+V | Always | Toggle on/off |
| 15 | Last seen | H+V | Always | Audience picker |
| 16 | Disallow forwarding my messages | H+V | Always | Toggle on/off |
| 17 | Blocked users | H+V | Always | Open blocked list → unblock action per row |

### 5.2 Security

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Two-factor auth | H+V | Always | Toggle / setup 2FA |
| 2 | Change password | H+V | Always | Open change password form |
| 3 | Active sessions | H+V | Always | Open sessions list → terminate session per row |
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

---

## 6. Matchmaking

**Penpot:** `Screen / Matchmaking / *`
**Feature docs:** [matchmaking.md], [game-catalog.md]

### 6.1 GameCatalog

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | V | Always | Return |
| 2 | Search games field | H+V | Always | Filter catalog |
| 3 | Popular section | H+V | Always | — (display game cards) |
| 4 | Recent section | H+V | User has MM history | — (display recent games) |
| 5 | Game card tap | H+V | Always | Open GameDetail |
| 6 | Browse all | H+V | Always | Expand full catalog list |

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

### 6.3 QueueSearch

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Status spinner / text | H+V | Always | — (display: searching, ETA, found N/M) |
| 2 | Cancel search | H+V | Always | Cancel MM queue |

### 6.4 MatchSquad

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Player row | H+V | Always (per player) | Open Panel / Matchmaking / PlayerProfile |
| 3 | Join voice | H+V | Squad formed | Join squad voice room |
| 4 | Start queue | H+V | Party leader, not yet queuing | Start MM as party |
| 5 | Leave party | H+V | Always | Leave party |

### 6.5 MatchHistory

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Match row | H+V | Always (per match) | Expand match details |
| 3 | Find players CTA | H+V | Always | Navigate to GameCatalog |
| 4 | Rate / report (per player) | H+V | Match completed | Open rating overlay / report |

---

## 7. Stories

**Penpot:** `Screen / Stories / *`
**Feature docs:** [stories.md]

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
| 12 | Join / Message (LFP) | H+V | Story is LFP type | Open MM form or DM author |
| 13 | Delete | H+V | Own story | Delete story |
| 14 | Viewers count (author only) | H+V | Own story | Open Panel / Stories / StoryViewers |
| 15 | Pause (hold) | H+V | Always | Hold to pause progress |

### 7.3 Archive

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Story row / grid item | H+V | Always | Open viewer for archived story |
| 3 | Create story | H+V | Always | Navigate to Create |
| 4 | Add to highlight (per story) | H+V | Always | Add to existing / new highlight → HighlightEdit |

### 7.4 Highlights

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Highlight card tap | H+V | Always | Open highlight viewer (sequential) |
| 3 | Add highlight | H+V | Always | Open Panel / Stories / HighlightEdit |
| 4 | Edit highlight (ctx) | H+V | ctx on own highlight | Open HighlightEdit |
| 5 | Delete highlight (ctx) | H+V | ctx on own highlight | Delete (confirm) |
| 6 | Privacy per highlight | H+V | Inside HighlightEdit | Audience picker |

---

## 8. Bots

**Penpot:** `Screen / Bots / Install`
**Feature docs:** [bots.md]

### 8.1 Install

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Back | H+V | Always | Return |
| 2 | Bot icon + name + description | H+V | Always | — (display) |
| 3 | Permissions detail | H+V | Always | Expand to show requested scopes (human-readable) |
| 4 | Space picker | H+V | Always | Select target space |
| 5 | Channel whitelist | H+V | Space selected | Select channels where bot can operate |
| 6 | Add to space / Install | H+V | Space + channels selected | Install bot → confirm |

---

## 9. Profile / Downgrade Picker

**Penpot:** `Screen / Profile / DowngradePicker`
**Feature docs:** [multi-profile.md], [subscription.md]

### 9.1 DowngradePicker

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Profile row with checkbox | H+V | Always (per profile) | Toggle selection |
| 2 | Keep selected | H+V | Exactly 2 selected | Freeze unselected profiles, proceed |

---

## 10. Chat Info / Panels (side panels or sheets)

**Penpot pages:** 13_Panels_Desktop, 14_Panels_Mobile, 15_Overlays
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
| 7 | Encryption code | H+V | DM with E2E enabled | Display 6-char code for verification |
| 8 | Shared media tabs (Media/Files/Links/Voice) | H+V | Always | Show shared content |
| 9 | Members list | H+V | GRP / CH | Show members (tap → profile) |
| 10 | Add members | H+V | GRP (permission) | Open contact picker |
| 11 | Pinned messages | H+V | Has pins | Open pinned messages list |
| 12 | Leave group / channel | H+V | GRP / CH | Leave (confirm) |
| 13 | Report chat | H+V | Always | Open Panel / Report / Sheet |
| 14 | Auto-delete timer | H+V | DM (future) | Set message retention |

### 10.2 Call controls (overlays from 15_Overlays)

Entry: voice/video call from chat header or friend row.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Mute / Unmute mic | H+V | In call | Toggle microphone |
| 2 | Deafen / Undeafen | H+V | In call | Toggle all audio |
| 3 | Camera on / off | H+V | In call (video call or voice room) | Toggle camera |
| 4 | Screen share | H only | In call (desktop) | Open screen/window picker → share |
| 5 | Screen share pause | H only | Sharing screen | Pause stream |
| 6 | Disconnect / Hang up | H+V | In call | Leave call |
| 7 | PTT (Push-to-Talk) | H+V | In call, PTT mode enabled in settings | Hold to talk |
| 8 | VAD / PTT mode switch | H+V | In call (settings) | Toggle between voice activity and PTT |
| 9 | Noise cancellation | H+V | In call (settings) | Toggle noise cancellation |
| 10 | Per-user volume slider | H+V | In call | Adjust individual participant volume |
| 11 | Mute participant (mod) | H+V | Has VOICE_MUTE_OTHERS | Server-mute participant |
| 12 | Raise hand | H+V | In call, raise hand enabled | Toggle hand-raise |
| 13 | Start broadcast (commander) | H+V | In commander room, has right | Hold → broadcast to target rooms (ducking) |
| 14 | Record | H+V | In call | Toggle local recording (MP3 128kbps) |
| 15 | Record indicator ⏺ | H+V | Recording active (self only) | — (visual, only visible to recorder) |
| 16 | Participants list | H+V | In call | Show who is in the call |
| 17 | Accept call (incoming) | H+V | Incoming call overlay | Accept |
| 18 | Decline call (incoming) | H+V | Incoming call overlay | Decline → "missed call" in DM |

### 10.3 Space tree & admin (Panel / Space / *)

Entry: from space context in shell or chat info.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Space tree (text channels + voice rooms) | H+V | In space | Navigate to channel/room |
| 2 | Create text chat | H+V | Has TEXT_CHAT_CREATE_IN_SPACE | Create group/channel in space |
| 3 | Create voice room | H+V | Has right | Create voice room |
| 4 | Create category | H+V | Has right | Create category folder |
| 5 | Reorder tree | H+V | Has right | Drag-reorder nodes |
| 6 | Space settings | H+V | Has SPACE_MANAGE_SETTINGS | Open space settings |
| 7 | Invites | H+V | Has right | Open Panel / Space / Invites |
| 8 | Members | H+V | Always | Open Panel / Space / Members |
| 9 | Roles | H+V | Has SPACE_MANAGE_ROLES | Open Panel / Space / Roles |
| 10 | Bots | H+V | Has SPACE_MANAGE_BOTS | Open Panel / Space / Bots |
| 11 | Audit log | H+V | Has audit log right | Open audit log |
| 12 | Leave space | H+V | Not owner | Leave (confirm) |
| 13 | Transfer ownership | H+V | Owner | Transfer to member (confirm + 2FA) |
| 14 | Slow mode per chat | H+V | Has TEXT_CHAT_SET_SLOW_MODE | Open Panel / Space / ChatSlowMode |
| 15 | Chat/voice overrides per channel | H+V | Has right | Open Panel / Space / ChatOverride / VoiceRoomOverride |
| 16 | Share invite link | H+V | Has invite right | Generate / copy link |
| 17 | Space verification settings | H+V | Has right | Phone / captcha / screening / manual approval |

---

## 11. Onboarding (overlays)

**Penpot:** `Overlay / Onboarding / CoachMarks`
**Feature docs:** [onboarding.md]

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Skip | H+V | Any step | Skip all remaining steps |
| 2 | Step CTA (varies) | H+V | Current step | Advance or perform action (e.g. "Find a space") |
| 3 | Dismiss / "Got it" | H+V | Info step | Advance to next step |

---

## 12. Version / Force Update (overlay)

**Penpot:** `Overlay / Version / ForceUpdate`
**Feature docs:** [updates.md]

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Update now | H+V | `force_update: true` | Open store / CDN update |
| 2 | Update (soft) | H+V | `update_available: true` | Open store / CDN |
| 3 | Later | H+V | Soft update only | Dismiss prompt |

---

## 13. Report sheet

**Penpot:** `Panel / Report / Sheet`
**Feature docs:** [reports.md]

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Category picker (Spam/Harassment/Offensive/Fake/Cheating/Other) | H+V | Always | Select category |
| 2 | Comment field | H+V | Always (required for "Other") | Input description (≤ 500 chars) |
| 3 | Submit | H+V | Category selected | Send report |
| 4 | Cancel / Close | H+V | Always | Close sheet |

---

## H vs V layout differences summary

| Area | Horizontal (desktop/web/tablet) | Vertical (mobile) |
|------|---|----|
| Shell | 3-column: rail + list + room | Bottom tab bar; list → room push; active chats strip when room open |
| Chat header | Inline in room panel | Full-width app bar with back arrow |
| Message actions | Hover toolbar on bubble + ctx | Long-press ctx + bottom selection bar |
| Settings | Sidebar nav + detail pane | List → push detail screen |
| Panels | Side panel (right drawer) | Bottom sheet or full-screen push |
| Overlays (call, MM) | Centered modal over content | Full-screen overlay |
| Screen share (start) | Available in call | Not available (view only) |
| Composer format toolbar | Always visible above composer | Hidden behind toggle (toolbar button) |
| Search | Panel or inline in header | Full-screen search overlay |
| Thread panel | Right sidebar alongside room | Push screen |
| Folder tabs | Horizontal tabs under search | Scrollable chip bar or collapsible |

---

## 14. User Profile Detail (Panel / Social / ProfileDetail)

**Penpot:** `Panel / Social / ProfileDetail`
**Feature docs:** [user-profile.md], [friends.md], [verification.md], [stories.md], [multi-profile.md]

Entry: from friend row, chat member tap, search result, @mention tap.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Banner image | H+V | User has banner (★) | — (display) |
| 3 | Avatar (animated if ★) | H+V | Always | Fullscreen avatar |
| 4 | Display name + @username | H+V | Always | — (display) |
| 5 | Verification badge (✅ / 🏢) | H+V | User is verified | — (visual indicator) |
| 6 | Presence status (online/idle/DND) | H+V | Visible per privacy settings | — (display) |
| 7 | Custom status ★ | H+V | User has custom status | — (display) |
| 8 | Bio / description | H+V | User has bio | — (display) |
| 9 | MM average rating ★ | H+V | Visible per privacy settings | — (display) |
| 10 | Highlights strip | H+V | User has highlights, visible per privacy | Open highlight viewer |
| 11 | Stories ring on avatar | H+V | User has active story | Open story viewer |
| 12 | Common spaces | H+V | Has common spaces | — (display; tap → open space) |
| 13 | Send message | H+V | Always (respects privacy) | Open / create DM |
| 14 | Voice call | H+V | Always (respects privacy) | Start DM voice call |
| 15 | Add friend | H+V | Not friends | Send friend request |
| 16 | Remove friend | H+V | Is friend | Remove (confirm) |
| 17 | Add to favourites | H+V | Is friend, not in favourites | Add to favourites list |
| 18 | Remove from favourites | H+V | Is friend, in favourites | Remove from favourites |
| 19 | Block | H+V | Always | Block user (confirm) |
| 20 | Report | H+V | Always | Open Panel / Report / Sheet |

---

## 15. Own Profile Edit (Panel / Profile / Edit)

**Penpot:** `Panel / Profile / Edit`
**Feature docs:** [user-profile.md], [multi-profile.md]

Entry: from settings or profile avatar long-press.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Avatar upload | H+V | Always | Pick image (static free / GIF ★) |
| 3 | Banner upload ★ | H+V | Plus subscriber | Pick banner image |
| 4 | Display name field | H+V | Always | Edit display name |
| 5 | @username field | H+V | Always | Edit username |
| 6 | Bio field | H+V | Always | Edit bio text |
| 7 | Save | H+V | Changes made | Save profile |

---

## 16. Registration (Screen / Auth / Register)

**Penpot:** `Screen / Auth / Register`
**Feature docs:** [auth-and-contacts.md]

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

---

## 17. Space Creation (Panel / Space / Create)

**Penpot:** `Panel / Space / Create`
**Feature docs:** [spaces.md]

Entry: from chat list header #5 submenu.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Space name field | H+V | Always | Input name |
| 3 | Icon / avatar upload | H+V | Always | Pick image |
| 4 | Visibility picker (Public / Invite-only / Private) | H+V | Always | Select visibility |
| 5 | Template picker (Gaming / Work / Social) | H+V | Always | Select template (affects default channels) |
| 6 | Create space | H+V | Name filled | Create space |

---

## 18. Join Space by Invite (Panel / Space / JoinInvite)

**Penpot:** `Panel / Space / JoinInvite`
**Feature docs:** [spaces.md]

Entry: from chat list header #6 submenu or deep link.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Invite code / link field | H+V | Always | Input invite code |
| 3 | Join | H+V | Code filled | Join space |

---

## 19. Space Catalog / Discovery (Screen / Space / Catalog)

**Penpot:** `Screen / Space / Catalog`
**Feature docs:** [spaces.md]

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
**Feature docs:** [text-chat.md]

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
**Feature docs:** [forward-messages.md]

Entry: from message action #5 or multi-select bar #2.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Search chats / contacts | H+V | Always | Filter targets |
| 3 | Recent chats list | H+V | Always | — (display) |
| 4 | Chat / contact row (multi-select) | H+V | Always | Toggle target selection |
| 5 | Forward | H+V | ≥1 target selected | Send forwarded message(s) |
| 6 | Cancel | H+V | Always | Close panel |

---

## 22. Global Search Results (Panel / Search / Global)

**Penpot:** `Panel / Search / Global`
**Feature docs:** [search.md]

Entry: from chat list header #1 search field.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Search text field | H+V | Always | Input query |
| 2 | Close / Clear | H+V | Always | Clear search / close |
| 3 | Section: Contacts & Chats | H+V | Results found | — (display: avatar, name, last message) |
| 4 | Section: Spaces | H+V | Results found | — (display: icon, name, member count) |
| 5 | Section: Messages | H+V | Results found | — (display: highlighted match, sender, date, chat name) |
| 6 | Result row tap (contact/chat) | H+V | Always | Open chat |
| 7 | Result row tap (space) | H+V | Always | Open space |
| 8 | Result row tap (message) | H+V | Always | Navigate to message in chat |
| 9 | Load more | H+V | More results available | Paginate (20 per page) |

---

## 23. Role Management (Panel / Space / Roles)

**Penpot:** `Panel / Space / Roles`
**Feature docs:** [roles.md]

Entry: from space admin #9.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Role list (drag-reorderable) | H+V | Always | Reorder priority |
| 3 | Role row tap | H+V | Always | Open role detail / edit |
| 4 | Create role | H+V | Has SPACE_MANAGE_ROLES | Create new role |
| 5 | Role name field | H+V | Inside role detail | Edit name |
| 6 | Role color picker | H+V | Inside role detail | Pick color |
| 7 | Permissions checkboxes | H+V | Inside role detail | Toggle individual permissions |
| 8 | Chat overrides section | H+V | Inside role detail | Set allow/deny per text chat |
| 9 | Voice room overrides section | H+V | Inside role detail | Set allow/deny per voice room |
| 10 | Delete role | H+V | Inside role detail, not built-in role | Delete (confirm) |
| 11 | Save | H+V | Changes made | Save role |

---

## 24. Member Management (Panel / Space / Members)

**Penpot:** `Panel / Space / Members`
**Feature docs:** [spaces.md], [roles.md]

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
**Feature docs:** [spaces.md]

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

## 26. Bot Management in Space (Panel / Space / Bots)

**Penpot:** `Panel / Space / Bots`
**Feature docs:** [bots.md]

Entry: from space admin #10.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Installed bots list | H+V | Always | — (display: icon, name, status online/offline) |
| 3 | Bot row tap | H+V | Always | Open bot detail (per-chat toggles) |
| 4 | Per-chat toggle (per bot) | H+V | Inside bot detail | Enable/disable bot in specific chat |
| 5 | Remove bot | H+V | Has SPACE_MANAGE_BOTS | Remove bot from space (confirm) |
| 6 | Add bot | H+V | Has SPACE_MANAGE_BOTS | Navigate to bot catalog / URL input |

---

## 27. Verification Settings (Panel / Settings / Verification)

**Penpot:** `Panel / Settings / Verification`
**Feature docs:** [verification.md]

Entry: from settings nav #7.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Section: Personal verification | H+V | Always | — (display current status) |
| 3 | Connect Twitch | H+V | Twitch not connected | Start OAuth flow |
| 4 | Connect YouTube | H+V | YouTube not connected | Start OAuth flow |
| 5 | Disconnect platform (per connected) | H+V | Platform connected | Disconnect (confirm — removes badge) |
| 6 | Verification badge status | H+V | Always | — (display: verified / not verified / pending) |
| 7 | Section: Organization verification | H+V | Always | — |
| 8 | Domain field | H+V | Organization account | Input official domain |
| 9 | DNS TXT instruction | H+V | Domain submitted | — (display: TXT record to add) |
| 10 | Verify DNS | H+V | Domain submitted | Trigger DNS check |

---

## 28. Linked Accounts (Panel / Settings / LinkedAccounts)

**Penpot:** `Panel / Settings / LinkedAccounts`
**Feature docs:** [verification.md]

Entry: from settings nav #5.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Connected platforms list | H+V | Always | — (display: platform icon + name + status) |
| 3 | Connect Twitch | H+V | Not connected | Start OAuth flow |
| 4 | Connect YouTube | H+V | Not connected | Start OAuth flow |
| 5 | Disconnect (per platform) | H+V | Connected | Disconnect (confirm) |

---

## 29. Appearance Settings (Panel / Settings / Appearance)

**Penpot:** `Panel / Settings / Appearance`
**Feature docs:** [accessibility.md], [navigation.md], [i18n.md]

Entry: from settings nav #8.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Theme picker (Light / Dark / System / High Contrast) | H+V | Always | Select theme |
| 3 | Language picker (EN / RU) | H+V | Always | Select language |

---

## 30. Accessibility Settings (Panel / Settings / Accessibility)

**Penpot:** `Panel / Settings / Accessibility`
**Feature docs:** [accessibility.md]

Entry: from settings nav #9.

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Reduced motion toggle | H+V | Always | Toggle reduced motion |
| 3 | Font scale slider | H+V | Always | Adjust font scale (100% — 200%) |

---

## 31. Story Viewers (Panel / Stories / StoryViewers)

**Penpot:** `Panel / Stories / StoryViewers`
**Feature docs:** [stories.md]

Entry: from story viewer #14 (own story, viewers count tap).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Close / Back | H+V | Always | Close panel |
| 2 | Viewer row (avatar + name) | H+V | Always | Open ProfileDetail |
| 3 | Total viewers count | H+V | Always | — (display) |

---

## 32. Screen Share Picker (Overlay / Call / ScreenSharePicker)

**Penpot:** `Overlay / Call / ScreenSharePicker`
**Feature docs:** [screen-share.md]

Entry: from call controls #4 (screen share).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Tab: Entire screen | H | Always | Select full screen |
| 2 | Tab: Window | H | Always | Select application window |
| 3 | Tab: Browser tab | H | Always | Select browser tab |
| 4 | Include system audio toggle | H | Always | Toggle audio capture |
| 5 | FPS picker | H | Always | Select frame rate |
| 6 | Resolution picker | H | Always | Select resolution |
| 7 | Share | H | Source selected | Start screen share |
| 8 | Cancel | H | Always | Close picker |

---

## 33. Stream Viewer Picker (in call, multiple screen shares)

**Feature docs:** [screen-share.md]

When multiple participants share screen simultaneously (up to 3).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Stream thumbnail (per sharer) | H+V | >1 active screen share | Switch to that stream |
| 2 | Active stream indicator | H+V | Viewing a stream | — (visual highlight on current) |

---

## 34. Slash Command Menu (Panel / Chat / SlashCommandMenu)

**Penpot:** `Panel / Chat / SlashCommandMenu`
**Feature docs:** [bots.md]

Entry: from composer #10 (typing `/` in space context).

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 1 | Command search field | H+V | Always | Filter commands |
| 2 | Bot group header | H+V | Always (per bot) | — (display bot name + icon) |
| 3 | Command row (name + description) | H+V | Bot online and enabled in chat | Select command |
| 4 | Grayed-out command | H+V | Bot offline | — (display with "Bot unavailable" tooltip) |
| 5 | Parameter input fields | H+V | Command selected, has options | Fill command options (per type: string/int/bool/user/channel/role/attachment) |
| 6 | Autocomplete suggestions | H+V | Typing in autocomplete-enabled param | Select suggestion |

---

## Addendum: missing controls in existing sections

### 1.4 Chat list row — additional ctx actions

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 8 | Add to folder (ctx) | H+V | ctx, custom folders exist | Pick folder to add chat to |

### 3.1 Chat Room Header — additional elements

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 10 | Typing indicator | H+V | Someone is typing | — (display: "User is typing…") |

### 3.3 Message bubbles — additional elements

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 12 | "Edited" label | H+V | Message was edited | — (display; tap → show edit timestamp) |

### 4.2 Friend row — additional actions

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 6 | Add to favourites (ctx) | H+V | ctx, not in favourites | Add to favourites |
| 7 | Remove from favourites (ctx) | H+V | ctx, in favourites | Remove from favourites |

### 10.1 Panel / Chat / Info — additional controls

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 15 | Create group from DM | H+V | DM only | Open contact picker → create group with both participants |

### 10.3 Space tree & admin — additional controls

| # | Control | Layout | Visible when | Tap action |
|---|---------|--------|-------------|------------|
| 18 | QR code for invite | H+V | Has invite right | Generate and display QR code |
