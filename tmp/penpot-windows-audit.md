# Penpot windows audit — Voice

**Date:** 2026-08-14  
**File:** `Voice` (`20d3f736-cc1b-8043-8008-561cb65228ef`)  
**Scope:** live MCP inventory vs `docs/design/screens.md`, `docs/design/screen-controls.md`, `docs/design/penpot-workflow.md`  
**Rule:** shipped snapshot (`x≈0`) is read-only. All rework goes to `· v2` (or new rows). Do not commit this file.

Sources: `docs/features/*`, `docs/design/screen-controls.md`. Post-v1 surfaces listed separately.

---

## Snapshot

| Page | Top-level | Canon | `· v2` | Notes |
|------|----------:|------:|-------:|-------|
| `00_References` | 0 | — | — | empty |
| `01_Foundation` | 4 | 3 | 1 (showcase) | Swatches, Components, Components v2, Component Mains |
| `10_Screens_Desktop` | 46 | 21 screens + 4 orphans | 21 | orphans overlap Shell |
| `11_Screens_Mobile` | 38 | 19 | 19 | no Chat/List / Room / Social slices (OK — in shell) |
| `12_States` | 7 | 3 + 1 orphan | 3 | stray `Title` |
| `13_Panels_Desktop` | 66 | 33 | 33 | 1:1 v2 spread |
| `14_Panels_Mobile` | 40 | 20 | 20 | **13 desktop panels have no mobile pair** |
| `15_Overlays` | 29 | 14 + 1 orphan | 14 | desktop 1280×800 + mobile 390×844 mixed on one page |

Layout grid (canon left, v2 right, Y gap 900/950) is mostly correct. No frame-vs-frame overlap except orphans sitting on `x=0,y=0`.

Library: 21 components, 9 typographies, tokens Light/Dark/HighContrast. Mains are on `01_Foundation` (good).

---

## P0 — file hygiene (do first)

These block viewer/MCP and pollute Layers.

1. **Delete stray root shapes** (not boards, not named `Screen/Panel/Overlay/State`):
   - `10_Screens_Desktop`: `Title`, `Text` ×2, `AccentBar` — all `@ 0,0`, overlap `Screen / Shell / Desktop`
   - `12_States`: `Title` `@ 0,0`, overlap `State / Chat / Empty`
   - `15_Overlays`: `Text` `@ 0,0`, overlap `Overlay / Call / Incoming`
2. **`00_References` empty** — drop the page or put Discord refs back. Empty page is noise.
3. **Typography names** are `title`, `body`, … — workflow §3.6 wants `type/title`, `type/body`, … Rename in Library.
4. **`List / Row` main still ships `Title` / `Subtitle` / `12:34`**. Almost every panel v2 inherits this. Override instance text **or** fix the main placeholder to Voice names (`Alex`, `See you in voice`, `now`). Until this is fixed, panel polish is wasted.

---

## P0 — missing windows (no frame at all)

Exist in `screen-controls.md` as named Penpot surfaces, **absent** from the file. Draw as new rows (canon via snapshot script later; start with `· v2` if no Flutter yet).

### Screens

| Frame | Spec | Why |
|-------|------|-----|
| `Screen / Auth / Register` | screen-controls §16 | Login v2 has Register CTA with nowhere to go |
| `Screen / Auth / PasswordReset` | §16.1 | Forgot password missing on Login |
| `Screen / Space / Catalog` | §19 | Join/discover spaces; shell has no entry |
| `Screen / Matchmaking / PartyLobby` | §6 | Catalog/squad flow gap |
| `Screen / DeepLinks / Error` | §40 | Invalid/expired invite, unknown user |

Optional / later: `Screen / Shell / MessageRequests` (post-v1 per text-chat.md).

### Panels

| Frame | Spec |
|-------|------|
| `Panel / Chat / ChannelSettings` | §10 |
| `Panel / Chat / GroupSettings` | §10 |
| `Panel / Chat / Bots` | §53 |
| `Panel / Matchmaking / AddGame` | §6 |
| `Panel / Space / JoinVerification` | §25.1 |
| `Panel / Space / VoiceRoomSettings` | §10.3a |
| `Panel / Space / VerificationSettings` | §50 |
| `Panel / Settings / Appearance` | §29 |
| `Panel / Settings / Accessibility` | §30 |
| `Panel / Settings / LinkedAccounts` | §28 |
| `Panel / Settings / E2EKeyBackup` | §27.1 |
| `Panel / Settings / Appeal` | §31 |
| `Panel / Settings / Stickers` | §37 |
| `Panel / Settings / ChatThemes` ★ | §38 |
| `Panel / Settings / AppIcon` ★ | §39 |

### Overlays

| Frame | Spec |
|-------|------|
| `Overlay / Call / MiniBar` | persistent call chrome |
| `Overlay / Call / ScreenSharePicker` | §33 (picker ≠ existing `Panel / Call / ScreenShare`) |
| `Overlay / Call / StreamViewerPicker` | §34 |
| `Overlay / Matchmaking / StorySuggestion` | after match |
| `Overlay / Stories / LFPRequest` | §51 |
| `Overlay / Report / Success` | §52 |

### Post-v1 (do not block v1, still track)

`Screen / Bots / Catalog`, `Panel / Bots / WebhookConfig`, `Overlay / Platform / InGame`.

---

## P0 — mobile missing vs desktop

`14_Panels_Mobile` has 20 of 33 desktop panels. Add mobile sheets (`390×844`) for:

- `Panel / Chat / SlashCommandMenu`, `SlashCommandOptions`
- `Panel / Stories / HighlightEdit`
- `Panel / Settings / Verification`
- `Panel / Space / Invites`, `Roles`, `RoleEditor`, `ChatOverride`, `ChatSlowMode`, `VoiceRoomOverride`

Skip as desktop-only chrome (unless product wants sheets): `Panel / Space / Tree`, `Panel / Shell / Navigation`, `Panel / Shell / SideHost`.

`11_Screens_Mobile` already covers full-screen flows. Chat list/room/social live inside `Shell / Mobile` / `MobileChatOpen` — OK, but those shells must get the same missing **entry buttons** as desktop (see below).

---

## P1 — existing `· v2` windows: required rework

Work **only** on right-hand drafts. Criteria: `screen-controls.md` controls visible; Voice placeholders; instances from Assets; inset ≥16; no empty glyph squares.

### Foundation components that leak into every screen

Fix mains on `01_Foundation` first — then instances update.

| Component | Problem | Fix |
|-----------|---------|-----|
| `Composer / Default` | Flattened SVG (`svg-rect` / `svg-g`). File/Send are empty squares; no emoji/GIF/sticker/mic | Rebuild as real flex: Input + icon buttons (emoji, attach, GIF, sticker, voice) + `Button / Send`. Override placeholder `Message...` |
| `Nav / Item` | SVG import, instances named `Nav / 0…2` with no label | Real icon + optional label; rail instances: Chats / Social / Matchmaking (+ Settings or avatar) |
| `List / Row` | Generic `Title` / `Subtitle` / `12:34` | Domain placeholders; expose text overrides |
| `Button / Primary` | Used as Mute/Search/Share without distinct icon variants | Icon-button variant or glyph inside Primary |
| Missing comps | No icon-btn, toggle, folder-tab, stories-ring, call-control, emoji-picker | Add mains + showcase on Components v2 |

### `Screen / Shell / Desktop · v2`

**Now:** rail `Nav / 0…2` (no Settings, no avatar/stories), SearchBar, list with mixed real (`Alex`) + generic rows, composer SVG only.

Add (screen-controls §1):

- Rail: Chats, Social, Matchmaking, Settings; profile avatar + stories ring; presence picker entry
- Chat list header: **New chat** menu (DM / Create group / Create·join space)
- Folder tabs: Все / ЛС / Группы / Каналы / Спейсы + edit folders
- Global search affordance (field exists; no rail CTA)
- List row context: pin / mute / archive / mark read (one hover/ctx example)
- Composer: same as Room (attach currently missing in shell)

### `Screen / Chat / List · v2` (320 slice)

Still `Title`/`Subtitle`/`12:34` only + Search. Fill 3 Voice rows; folders; new-chat; match shipped density.

### `Screen / Chat / Room · v2` (960 slice)

**Now:** header `Alex` / `online`; 2 bubbles; date `Today`; composer SVG.

Header: voice call, video call (DM), search-in-chat, pinned, thread, more/kebab, E2E lock.

Composer: emoji, attach, GIF, stickers, voice message, send (with glyph), `/` slash hint. Format toolbar (desktop).

One message hover toolbar: React / Reply / Forward / Edit / Pin / Report / Select.

Meta: timestamp on bubble, delivery ticks (outgoing), unread separator.

Variants worth extra frames (same row, `· v2` siblings): in-chat search bar; E2E key-change banner; reply/edit preview in composer.

### `Screen / Social / Panel · v2`

**Now:** `Friends` + `ONLINE` + 3 friend rows. No actions.

Add: Add friend; tabs Friends / Pending / Blocked; Accept/Decline; Message / Call / Profile on row.

### Auth

**`Login · v2` (desktop):** CTAs Log in / Register / Continue as guest — **no email/password/OTP/phone fields**. Mobile login has Email/Password but **no Log in CTA** in text. Unify:

- Email + password fields
- Phone / OTP path
- Forgot password → PasswordReset
- Register → Register screen
- Guest → GuestNickname → GuestConvert entry

**`GuestNickname · v2`:** desktop OK-ish; mobile still generic Title/Subtitle. Add nickname field + Join CTA; link to convert account.

### Settings screens (desktop v2)

Nav only: Privacy / Security / Notifications / Subscription. **Missing nav:** Linked accounts, Help, Appearance, Accessibility, Appeal, Stickers, Log out.

**Privacy · v2** — only 4 rows: Blocked users, Last seen, Read receipts, Who can message me. Add per §5.1: preset Personal/Gaming/Work; who can call / invite / files / voice / add friend; visibility (online, in-game, MM rating, phone, stories, search by phone); disallow forwarding; game activity; allow guest DM. Blocked users must be a **button → list**, not a dead row.

**Security · v2** — Active sessions + 2FA only. Add: change password, linked devices, delete account, E2E backup entry. 2FA needs setup substate (QR / backup codes) as extra draft if not inline.

**Notifications · v2** — generic Title/Subtitle rows. Add: push, mentions only, sounds, quiet hours, per-chat overrides, @mention breaks mute, type toggles (friend request, MM, reactions, replies, DM, system).

**Subscription · v2** — Voice Plus + Upgrade + Restore. Add: billing history, manage profiles → DowngradePicker, cancel/manage plan.

**Mobile settings v2** are almost empty (Title/Subtitle + one Primary). Mirror desktop content in single-column + back.

### Matchmaking

| Frame | Now | Need |
|-------|-----|------|
| `GameCatalog · v2` | Search + POPULAR + game cards, no CTA | Add game; Browse all / filters; named games not empty `Row / Game` |
| `GameDetail · v2` | Only `Find match` | Game title, mode/region/rank/roles, Find match |
| `QueueSearch · v2` | Decent (searching, 3/5, cancel) | Keep; add leave/expand party |
| `MatchSquad · v2` | PARTY + Start queue; rows generic | Named players; Join voice; Open profile; Invite |
| `MatchHistory · v2` | Generic rows only | Match cards; Find players; Rate/report entry |
| `PartyLobby` | **missing frame** | Draw |

### Stories

| Frame | Now | Need |
|-------|-----|------|
| `Create · v2` | Only `Post story` | Audience, game tag, LFP type, Photo/Video/Text/Clip, editor tools, mention |
| `Viewer · v2` | `Alex · 2h ago` only | Reply, React, Viewers (author), Delete/Share/Close, LFP Join/Message |
| `Archive · v2` | Generic rows | Create story CTA; real archive items |
| `Highlights · v2` | **zero text** | Highlight cards; Add highlight; privacy/edit → HighlightEdit |

### Bots / Profile

**`Bots / Install · v2`:** only `Add to space`. Restore Permissions detail row (canon had it).

**`DowngradePicker · v2`:** Keep selected + generic rows. Explicit checkboxes on named profiles.

### States (`12_States`)

Empty / Error / Offline v2 copy is fine. Add empty-state **CTA** that matches product (Find friends / Try again). Offline banner OK. Optional extra states later: search no-results, space empty tree.

---

## P1 — panels `13` / `14` · v2

Most desktop panel v2s are the same skeleton: AppBar + `List / Row` (`Title`/`Subtitle`/`12:34`) + `Button / Primary`. That is not a usable mock.

**Already better (keep as reference):** `Space / Bots` (Mod Bot / Music Bot), `Space / Roles` (Member/Moderator/Admin), `RoleEditor` (Color/Permissions), `ChatOverride` (Send messages / Attach files), `VoiceRoomOverride` (Video), `SlashCommandMenu` (`/gif` `/poll` `/remind`), `ProfileDetail` (Alex + Actions/Mutual). Still need real labels on Roles/RoleEditor (empty text nodes).

**Polish each remaining panel** (Voice names, 2–3 rows, correct CTAs, inset ≥16, AccentWrap on selected, no Mute double-layer):

| Panel | Must show |
|-------|-----------|
| `Chat / Info` | Mute (single control, not stacked Primary), pins, shared media, members, E2E, report |
| `Chat / Thread` | Parent snippet + 2 replies + composer |
| `Chat / CreateGroup` | Name field + member picker + Create |
| `Chat / GroupMembers` | Named members + add/kick |
| `Chat / ForwardMessage` | Multi-select chats + Send / Copy as new |
| `Chat / SlashCommandOptions` | Command name + Option A/B as fields not nav |
| `Search / Global` | Query field + mixed results (people/chats/messages) |
| `Space / Tree` | Categories, #text, voice rooms, unread, create + |
| `Space / Create` | Name, visibility, Create |
| `Space / JoinInvite` | Invite preview + Join |
| `Space / Invites` | Link/QR, list, revoke, create |
| `Space / Members` | Roles filter + named members |
| `Space / ChatSlowMode` | Interval picker, not generic rows |
| `Call / ScreenShare` | Window/screen list + Share (not Title/Subtitle) |
| `Shell / Navigation` | Semantic rail labels |
| `Shell / SideHost` | Host chrome + sample child (Chat Info) |
| `Settings / Sheet` | Full nav list (see Settings above) + log out |
| `Settings / Help` | FAQ topics / contact |
| `Settings / Verification` | Person/org status + start flow |
| `Profile / Create` + `Edit` | Avatar, display name, bio, accent — not one Display name row |
| `Auth / GuestConvert` | Email/phone + password + Convert (Password row exists) |
| `Stories / HighlightEdit` | Name, privacy, story picker |
| `Stories / StoryViewers` | Named viewers |
| `Matchmaking / PlayerProfile` | Game, rank, Invite / Message / Report |
| `Report / Sheet` | Object type, reason list, submit |

Known visual bug (design.md): **Mute double-layer** on `Panel / Chat / Info · v2` (`Primary / Mute` on `Button / Primary`). Same pattern on Search (`Primary / Search`) and ScreenShare (`Primary / Share`). Flatten to one labeled button.

Mobile panel v2s copy the same skeleton — fix desktop first, then respread.

---

## P1 — overlays `15` · v2

| Overlay | Now | Need |
|---------|-----|------|
| `Call / Incoming` | Maria/Alex mixed copy; Decline + Accept | One caller; Accept glyph; mute-before-join |
| `Call / Outgoing` | Calling Maria + Cancel | Avatar; no extra controls required |
| `Call / Active` | Mute + Speaker + End | Camera, screenshare (H), PTT, deafen, raise hand, participants, record. MiniBar = **new frame** |
| `Matchmaking / MatchFound` | Decent | Named squad; countdown; Accept glyph |
| `Matchmaking / Rating` | Stars + Skip | Per-player rate + report |
| `Onboarding / CoachMarks` | One hotspot | 4–5 steps (rail, list, composer, MM) per onboarding.md |
| `ForceUpdate` | Copy OK | Primary Update + store/download affordance |

Desktop vs mobile overlays share names on one page (y=0 desktop, y=6300 mobile). Fine, but **Incoming · v2 desktop is 390×844** in one slot — check which copy is the real desktop 1280×800. MCP listed both sizes under the same name.

---

## P2 — coverage / process

- Re-run `scripts/design/generate-screens-md.mjs` after new canons exist (not for `· v2`).
- `screens.md` has no mobile viewer for several desktop-only panels (SlashCommand*, Space Invites/Roles/overrides, Verification, Tree, Navigation, SideHost) — expected until mobile frames exist.
- `export_shape` QA: spot-check 2–3 v2 per page after polish (clip, empty boards, overflow).
- Flutter parity **after** design approve of v2 — not part of this audit.
- Dark / HighContrast: tokens exist; no screen themed variants. Optional later: one shell in Dark + HighContrast.

---

## Suggested order of work

1. Hygiene: delete orphans; rename typographies; fix `List / Row` + `Composer` + `Nav / Item` mains.
2. Shell + Chat List + Chat Room + Social v2 (entry points — otherwise panels are unreachable).
3. Settings four screens + Settings Sheet nav.
4. Panel polish 13 (Info, Tree, Search, Forward, CreateGroup, Space admin set).
5. Draw missing v1 windows (Register, PasswordReset, Space Catalog, PartyLobby, Appearance/A11y, MiniBar, ScreenSharePicker).
6. Mobile: missing 13 panels; then respread shell/settings/stories.
7. Overlays call chrome + onboarding steps.
8. Post-v1 frames last.

---

## Already OK (do not redo)

- Page structure 10–15, canon/v2 rows, sizes 1280×800 / 390×844 (wire slices 320/960/300 documented).
- Component mains on `01_Foundation` only.
- Token sets + Mode themes.
- QueueSearch v2, MatchFound/Rating/ForceUpdate copy, Empty/Error/Offline copy, Space Bots v2 placeholders, GuestNickname desktop CTA set.
- `docs/todo/design.md` missing-buttons checklist — this audit **confirms** it against live frames and adds missing windows + hygiene + panel emptiness.
