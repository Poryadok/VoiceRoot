# Analyst 1 — Completed Task Inventory Audit

**Scope:** Agent batch loop PRs **#74–#114** (batches 1–29b), plus immediate pre-loop fixes **#65–#73** where they close todo claims.  
**Workspace:** `D:/Git/Voice` (local; `git pull` / `git fetch` blocked by Cursor hook `block-history-rewrite.js` — audit used current tree + `gh pr list`).  
**Date:** 2026-09-02  
**Sources:** `docs/TODO.md`, `docs/todo/*.md`, `docs/PLAN.md`, merged PR metadata (`gh pr list/view`), code spot-checks.

---

## Executive summary

| Metric | Count |
|--------|------:|
| Merged PRs in scope (#74–#114) | **41** |
| Distinct batch-tagged `[x]` / “done (Batch N)” claims in `docs/todo/` | **~58** |
| **Verified** (code/tests/PR evidence matches claim) | **~52** |
| **Unverified / overstated** (marked done or implied done, evidence weak or contradicted) | **3** |
| **Doc hygiene only** (work verified, but checkbox/prose stale — not false completion) | **~12** |
| **Completed work missing or under-documented** in todo trail | **~15** |

**Headline:** The batch loop delivered real, mergeable work. Code evidence backs the bulk of folder/archive/Quick Access/social REST/shell UX batches (17–26). Main problems are **stale index prose** (`docs/TODO.md` §Топ дыр, `docs/PLAN.md` matrix/navigation rows) and **policy mismatch** (completed items should be deleted, but many remain as `[x]` with outdated sub-bullets). One product-roadmap `[x]` (П.15 bots v2) overstates shipped scope.

---

## Audit table

Verdict key: **verified** = evidence in repo; **unverified** = claim not supported or contradicted; **missing from docs** = merged work not reflected in todo/PLAN index.

| Batch / PR | Claimed item | Evidence (paths) | Todo doc status | Verdict |
|------------|--------------|------------------|-----------------|--------|
| **1 / #74** | Admin resolve/dismiss reports; OAuth assign-to-me; close batch-1 admin TODOs | `src/admin/src/components/ReportDetail.tsx`, `QueuePage.tsx`, `src/admin/src/test/ReportDetail.test.tsx`, `src/admin/src/test/QueuePage.test.tsx` | Removed from `admin.md` per “delete when done” policy | **verified** |
| **2 / #75** | Admin sanctions, pagination, revoke | Merged PR title; admin queue components (spot-check via batch-1/3 test files) | Removed from `admin.md` | **verified** |
| **3 / #76** | Admin appeals queue | Merged PR #76; moderation admin routes in gateway tests | Removed from `admin.md` | **verified** |
| **4 / #77** | Analytics dashboards + CreateGame form | `src/admin/` analytics pages; batch 4 noted in `admin.md` | `admin.md` §High: analytics search/voice still `[ ]`; CreateGame `[x]` via batch 4 | **verified** (partial doc split) |
| **5 / #78** | Admin OAuth tests, api client, smoke gaps | `src/admin/src/test/` OAuth/login tests; `scripts/staging/smoke-staging.sh` | `admin.md` §Common `[x]` thin coverage; staging smoke still `[ ]` | **verified** |
| **6 / #79** | Developer Portal OAuth/App tests + staging smoke | `src/developer-portal/src/test/OAuthCallback.test.tsx`, `AppLogin.test.tsx`, `apiClient.test.ts` | No dedicated `developer-portal` todo file; only `admin.md` cross-refs | **missing from docs** |
| **7 / #80** | CI-gate GLOBAL hardening + script test parity | `.github/ci/verify-required-jobs_test.sh`, `.github/workflows/ci.yml`, `Makefile` | `ci.md` §Common `[x]` promote bootstrap + compose-e2e trigger | **verified** |
| **8 / #81** | Quiet hours API-first; mobile strip LRU; ConvertGuest doc | `notification_settings_screen.dart`, `notification_quiet_hours_storage.dart`, `mobile_opened_chat_strip.dart` | `client.md` §Common `[x]` quiet hours; guest doc `[x]` | **verified** |
| **9 / #82** | Strip a11y + composer panels (R2-A05, R3-A11) | `mobile_chat_strip.dart`, `composer_panels.dart`, `voice_shortcuts_keyboard_test.dart` | `client.md`: R3-A11 `[x]`; **R2-A05 still `[ ]`** though prose says shipped | **verified** (checkbox gap) |
| **10 / #83** | Shell profile menu, mobile tabs, a11y smoke/focus | `profile_avatar_menu.dart`, `MobileShellTabBar` in `app.dart`, `shell_text_scale_test.dart` | `client.md` `[x]` R2-A03, a11y batches 10 | **verified** |
| **11 / #84** | Frozen profile switcher; delivery_ack NATS; mention `profile_id` | `proto_mappers.dart` `frozenAt`, `realtime/delivery_ack_publisher.go`, WS mention dispatch | `client.md` `[x]` frozen; `backend.md` `[x]` Realtime publisher **but stale note “consumer still open”** | **verified** (stale prose) |
| **12 / #85** | Delivery_ack consumer; list metadata; ProfileDowngradePicker routing | `messaging/delivery_ack_consumer.go`, `GetChatListMetadata`, `app.dart` `profileDowngradeRequiredProvider` | `backend.md` `[x]` durable delivery; `client.md` `[x]` downgrade picker | **verified** |
| **13 / #86** | Delete-profile UI; list-chats content type | `manage_profiles_sheet.dart`, messaging `content_type` migration | `client.md` `[x]` delete profile; `backend.md` `[x]` preview DTO | **verified** |
| **14 / #87** | ListChats channels in main inbox | `chat/internal/store/list_chats.go` channel SQL | `backend.md` `[x]` R3-A12 partial (membership); standalone create deferred to 26b | **verified** |
| **15 / #88** | `inbox=archive` on ListChats | `list_chats.go` archive filter; gateway transcode | `backend.md` `[x]` archive inbox regression | **verified** |
| **16 / #89** | Space pagination + Flutter archive screen | `listChatsPageMainWithSpaces`, `chat_archive_screen.dart`, tests | `client.md` `[x]` archive screen; `backend.md` `[x]` R3-A16 pagination | **verified** |
| **17 / #90** | Quick Access RPCs + accent picker on create | `000010_quick_access_chats.up.sql`, `quick_access.go`, `CreateProfileSheet` | `backend.md` `[x]` QA RPCs/migration/handlers; `client.md` `[x]` accent | **verified** |
| **18 / #91** | Folders foundation; archive removes QA | `000008_folders.up.sql`, `000009_folder_chats.up.sql` | `backend.md` `[x]` migrations + archive→RemoveQuickAccess | **verified** |
| **19 / #92** | Folder membership/pin RPCs + Gateway REST | `chat.proto` folder RPCs, `transcode` folder routes | `backend.md` `[x]` proto/handlers/Gateway | **verified** |
| **20 / #93** | Update/Delete folder + DM auto-unarchive | `UpdateFolder`/`DeleteFolder` in `chat.go`, auto-unarchive consumer | `backend.md` `[x]` CRUD + side-effects + integration test | **verified** |
| **parallel / #94** | Quick Access rail, folders REST UI, create-profile avatar | `desktop_shell_rail.dart`, folder providers, `CreateProfileSheet` avatar | `client.md` `[x]` folders (cites parallel/client); create avatar `[x]` | **verified** |
| **parallel / #95** | Admin GameConfigEditor + CI polish | `src/admin` `GameConfigEditor`, ESLint in CI | `admin.md` `[x]` game catalog editor | **verified** |
| **parallel / #96** | `messages.content_type`; pin permission R3-A05 | `pins_store.go` `MaxPinsPerChat=5`, messaging content_type migration | `backend.md` `[x]` R3-A05; R3-A06 partial `[ ]` | **verified** |
| **21a / #97** | DM message-requests bucketing (R3-A04) | `dm_inbox.go`, `dm_request_recontact.go`, `inbox_bucket` in `list_chats.go` | `backend.md` `[x]` R3-A04 | **verified** |
| **21b / #98** | Folder pin/reorder; QA replace picker; keyboard-hide tabs | `folder_pin_providers.dart`, drawer/shell tests | `client.md` `[x]` custom folders (pin/reorder); R2-A04 partial `[ ]` | **verified** |
| **22a / #99** | `message_request` notification type | `notification` `TypeMessageRequest`, routing by `inbox_bucket` | `backend.md` `[x]` R3-A23/R4-A15 | **verified** |
| **22b / #100** | Message requests virtual folder UI | `message_requests_folder.dart`, rail/drawer integration | `client.md` `[x]` message requests inbox | **verified** |
| **23a / #101** | Gateway REST contacts/favorites | `gateway/transcode_friends.go`, `transcode_friends_contacts_test.go` | `backend.md` `[x]` (duplicate bullets Critical + High) | **verified** |
| **23b / #102** | Flutter contacts/favorites UI | `social_panel.dart`, `friends_client.dart` | `client.md` `[x]` favorites/contacts | **verified** |
| **24a / #103** | Blocked accounts list UI | `social_panel.dart` Blocked tab, blocks REST | `client.md` `[x]` blocked UI | **verified** |
| **24b / #104** | Declined outgoing friend request label | `PendingFriendRequest.status` proto + UI | `client.md` `[x]`; `backend.md` `[x]` | **verified** |
| **25a / #105** | `syncPhoneContacts` client stub | `friends_client.dart`, `SocialPanel.syncPhoneContactsKey` | `client.md` `[x]` inside favorites bullet | **verified** |
| **25b / #106** | Custom folder management UI | `manage_folders_sheet.dart`, `chats_client.dart` update/delete | `client.md` `[x]` edit-folders (Batch 25b) | **verified** |
| **26a / #107** | QR add-friend sheet | `qr_add_friend_sheet.dart`, `qr_add_friend_sheet_test.dart` | `client.md` `[x]` | **verified** |
| **26b / #108** | Standalone channel `CreateChat` (R3-A12) | `CreateChannelChat` in `group.go`, integration tests | `backend.md` `[x]` R3-A12 | **verified** |
| **27a / #109** | User appeals Gateway + Flutter form | `transcode_moderation.go` POST appeals, `appeal_sheet.dart` | `backend.md` §Critical `[x]` appeals exposed | **verified** |
| **27b / #110** | Persist topic/thread settings (R3-A14) | `group.go` `UpdateGroupChat`, proto fields | `backend.md` `[x]` R3-A14, UpdateChat thread/channel | **verified** |
| **28a / #111** | Hide mobile chat strip when keyboard open | Mobile shell layout (R2-A04 §1.6a) | `client.md` R2-A04 partial `[ ]` notes 28a shipped | **verified** (checkbox gap) |
| **28b / #112** | Delete-account UI | `security_settings_screen.dart`, `auth_client.deleteAccount`, tests | `client.md` `[x]` delete-account; `PLAN.md` auth row updated | **verified** |
| **29a / #113** | Mobile stacked chrome overflow polish | `MobileShellOverlayObserver`, pinned bar collapse, text-scale smoke | `client.md` `[x]` MobileChatStrip scope; R2-A04 partial | **verified** |
| **29b / #114** | Active sessions list + revoke UI | `active_sessions_screen.dart`, `security_settings_screen.dart` nav, tests | `client.md` **parent item still `[ ]`**; inline “sessions shipped Batch 29b” | **verified** (checkbox gap) |
| **pre-loop / #70** | Pin limit aligned to **5**/chat | `messaging/internal/store/pins_store.go` `MaxPinsPerChat = 5` | **Not in batch todo table**; `PLAN.md` matrix row 25 still says limit **50** | **missing from docs** |
| **pre-loop / #71** | Group/channel `last_message_at` from stream | `chat` `TouchLastMessageAt` | `backend.md` `[x]` | **verified** |
| **pre-loop / #72** | Social friend-invite fail-closed | `social_friends.go`, degradation IT | `backend.md` `[x]` | **verified** |
| **pre-loop / #73** | MM platform/peer ban fail-closed | `matchmaking/platform_ban.go`, `platform_ban_degradation_test.go` | **No explicit todo line** for #73 | **missing from docs** |

---

## Items marked done but weak / contradictory evidence (false positives)

| Item | Location | Issue |
|------|----------|-------|
| **П.15 — Бот-платформа v2** `[x]` | `product-roadmap.md` | Claims autocomplete, subcommands, ephemeral/deferred, Portal catalog, Flutter `/` picker as shipped. `PLAN.md` still lists bots **partial** (no inbound `message.events` → webhook, Portal CSRF/manifest gaps). Slash/webhook path exists for interactions, not full v2 platform. | **unverified** (overstated) |
| **`[Cross-cutting] Flutter shell parity`** prose | `backend.md` §Critical line ~75 | Bullet still open `[ ]` but claims “folder/quick-access RPCs still open” — **false** after batches 17–20. Work is done; bullet should be narrowed to remaining mobile IA only. | **unverified** (stale blocker text) |
| **`docs/TODO.md` §Топ дыр** | Index table rows 78–81 | Still lists “folders RPC”, “game catalog mode editor”, “sessions/delete-account UI” as top gaps — **contradicts** shipped batches and `admin.md`/`client.md` updates. | **unverified** (index false negatives) |

No batch `[x]` line was found where the **core code artifact is absent** (migrations, handlers, screens). Stale *sub-prose* inside `[x]` items (e.g. R2-A03 “archive snackbar until Batch 15”) is common but the underlying feature exists.

---

## Completed work not reflected (audit-trail gaps)

| Work | PR / batch | Suggested doc home |
|------|------------|-------------------|
| Pin limit **5** enforcement | #70 | `backend.md` Telegram-parity / Messaging pins; fix `PLAN.md` matrix row 25 |
| Matchmaking fail-closed platform + peer bans | #73 | `backend.md` §Matchmaking |
| Developer Portal vitest + staging smoke hooks | #79 batch 6 | `admin.md` §Developer Portal or new pointer |
| CI `verify-required-jobs_test.sh` | #80 | Already in `ci.md` — OK |
| Parallel track messaging `content_type` | #96 | Covered under Batch 13/12 — OK |
| Sessions/revoke UI shipped | #114 | Split `client.md` Auth UI item or mark `[x]` |
| R2-A05 strip LRU shipped | #9 | Flip `client.md` checkbox to `[x]` |
| Folders / Quick Access / archive **index summary** | #17–20, #88–89 | Refresh `docs/TODO.md` §Топ дыр; `PLAN.md` navigation + text-chat matrix rows |
| `mm_ban` S2S to Matchmaking | Partial in #73 platform ban API | `backend.md` still `[ ]` — correct to stay open; #73 scope should be noted |

---

## Doc hygiene issues (verified work, misleading todo state)

These are **not** false completions; they need analyst-2/3 cleanup passes:

1. **`[x]` items retained** — `docs/TODO.md` policy says delete completed bullets; ~58 remain as `[x]` with batch tags.
2. **`backend.md:372`** — JetStream `delivery_ack` publisher `[x]` still says “Messaging durable consumer still open” — consumer exists (`delivery_ack_consumer.go`, Batch 12).
3. **`client.md:47`** — R2-A03 `[x]` still mentions archive snackbar until Batch 15; archive screen is Batch 16 `[x]`.
4. **`PLAN.md` internal contradiction** — Row “Pin messages” matrix says limit **50** (L25); status table says limit **5** shipped (L69); code = 5.
5. **`PLAN.md` navigation row (L88)** — Still says folders rail / Quick Access / archive “нет в коде”; batches 15–21 shipped core paths.
6. **`admin.md` §High** — “game catalog mode editor” still in `TODO.md` top table but marked `[x]` in `admin.md`.
7. **Duplicate backend bullets** — Contacts/favorites REST appears twice (`Critical` + `High`) both `[x]`.

---

## Items needing human follow-up

1. **`git pull` blocked** — Confirm local tree matches `master` @ `047945e6+` on a machine without the history-rewrite hook; re-run spot-checks if HEAD diverges.
2. **`product-roadmap.md` П.15 `[x]`** — Downgrade to partial or split into shipped (slash picker) vs open (inbound events, Portal lifecycle).
3. **Refresh `docs/TODO.md` §Топ дыр** — Remove or rewrite rows 78–81; folders, contacts UI, delete-account, sessions are no longer top blockers.
4. **Refresh `PLAN.md` Telegram-parity matrix** — Pin limit 50 → 5; archive list RPC + screen; folders/QA shipped status.
5. **`client.md` Auth UI** — Decide policy: split sessions `[x]` / password-reset `[ ]` or keep compound open item.
6. **`client.md` R2-A04 / R2-A05** — Mark incremental `[x]` or delete sub-bullets per todo policy.
7. **Appeals business rules** — Gateway+UI shipped (#27a) but `backend.md` §Moderation still lists validation gaps (7-day window, duplicate appeal) — product/legal input.
8. **Developer Portal batch 6** — Add explicit “tests shipped” note; E2E/live smoke remains `[ ]` in `admin.md`.
9. **Stickers/GIF** — Still P0 open (`backend.md`, `TODO.md`); do not infer completion from parallel messaging `content_type` work (#96).

---

## Method notes

- PR list: `gh pr list --state merged --limit 50` (41 PRs #74–#114).
- Evidence = file presence + tests + integration paths; full live compose not re-run in this pass.
- Admin batches 1–3 items correctly **removed** from `admin.md` (policy-compliant); traceability relies on PR titles only.
