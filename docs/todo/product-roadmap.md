# TODO — Product Roadmap

[← Индекс](../TODO.md)

Сквозные продуктовые инициативы (не привязаны к одному сервису).

План *20 product improvements* (2026-07). Спеки: [docs/features/](../features/). **П.7 мультипрофиль — закрыт.** Пересечения с domain-файлами — ссылка, не дублировать.
## Critical

_Нет открытых пунктов._

## High

- [x] **П.1 — Матчмейкинг внутри спейса** — `StartSpaceQueue`, Redis `mm:space:{space_id}`, `UpdateSpaceMmConfig`, Gateway `POST /spaces/{id}/matchmaking/queue`, Flutter вкладка ММ в спейсе (#32). Lives shipped: `TestComposeSpaceMatchmakingQueue_live`, `space_mm_e2e_live_test` (P2.1). Спека: [matchmaking.md](../features/matchmaking.md) §«Внутри спейса».
- [ ] **П.2 — Постматчевый цикл** — `RateTeammates`, `BanFromMatchmaking`, `ListMatchHistory`; таблицы `match_history`, `mm_ratings`, `mm_bans`; modal при выходе из match-squad; экран истории в профиле. Спека: [matchmaking.md](../features/matchmaking.md) §Оценка/Бан/История. Тесты: `matchmaking_rating_e2e_live_test.dart`, compose.
- [x] **П.3 — Stories «Ищу пати» → матчмейкинг** — `RespondToLfpStory` (JOIN|INVITE), NATS `story.lfp_response`, Notification inline Accept/Decline, Flutter LFP card. Критерий: JOIN → accept → пати в очереди (#35). Спека: [stories.md](../features/stories.md). → также Batch 7 (`story.lfp_created` subscriber).
- [x] **П.4 — Каталог игр + заявки пользователей** — seed Dota 2/CS2/Valorant/PUBG; `SubmitGameRequest` → `pending_moderation`; Admin модерация; Flutter wizard «Добавить игру». Lives shipped: `TestComposeGameRequestModeration_live`, `game_request_e2e_live_test` (P2.3). Спека: [game-catalog.md](../features/game-catalog.md).


## Common

### Telegram-parity audit — product policy (undecided, 2026-08-28)

Источник: `tmp/telegram-ux-audit/AUDIT.md` (anti-loop rule §4; round3 DEFER). В feature docs помечено DEFERRED/target-state — **решение человека** до реализации; после решения обновить [screen-controls.md](../design/screen-controls.md) / [notifications.md](../features/notifications.md) / [search.md](../features/search.md).

- [ ] **R3-03-A09 — Active strip: unread retention on back** — normative сейчас: back **удаляет** чат из strip (даже с unread). Альтернатива: удерживать при unread — выбрать одно правило; убрать DEFERRED caveat из §1.6.
- [ ] **R4-02-D1 — Pinned bar tap cycle** — v1 = jump to latest pin; target-state = cycle pins on bar tap — [text-chat.md](../features/text-chat.md) § «Закреплённые», §3.1a.
- [ ] **R2-13-A02 — Notifications for archived chats** — push/in-app when archived chat receives message: suppress vs badge-only vs full delivery — [notifications.md](../features/notifications.md) (нет политики).
- [ ] **R2-12-U03 — Location in Shared Media tabs** — assign `LOCATION` to Media vs Files vs Links vs отдельная вкладка — [search.md](../features/search.md) mapping table incomplete.
- [ ] **R2-07-U01 — DM header subtitle priority** — idle vs DND vs in-call vs last-seen when multiple apply — [presence.md](../features/presence.md) § header (сейчас «или» без жёсткого порядка).

- [ ] **П.5 — Верификация ранга Steam/FACEIT (MVP)** — `linked_game_accounts`, OAuth/API, badge в ММ, фильтр `verified_rank_only`. Спека: [PROJECT.md](../PROJECT.md), [matchmaking.md](../features/matchmaking.md). → пересекается [User] verification gaps (Batch 14).
- [ ] **П.6 — Верификация Twitch/YouTube/DNS** — cron `VerificationStatusRefresh`, org TXT flow, Flutter Settings → Верификация. Спека: [verification.md](../features/verification.md). → Batch 14 [User] Verification V1.
- [ ] **П.8 — Синхронизация контактов телефонной книги** — Flutter hash + `POST /contacts/sync` live есть; нет list/favorites REST + onboarding «Найди друзей» UI. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md). → [backend.md](backend.md) Social REST; [client.md](client.md) phone-book.
- [x] **Стикеры в v1** — решение 2026-08-24: **делать** (системные паки + upload своих). Зафиксировано в [text-chat.md](../features/text-chat.md), [PLAN.md](../PLAN.md) фаза 2, [ADR 005](../adr/005-rich-media-live-tests-deferred.md). Код/lives — [backend.md](backend.md) Chat.
- [ ] **Спека vs код (остальные решения человека)** — папка DM requests в спеке vs PLAN «не MVP»; user-add игр vs staff `CreateGame`; space-MM уже в коде (П.1) — зафиксировать в спеке как v1.
- [x] **П.9 — Гостевой → постоянный аккаунт** — `ConvertGuest` negative tests, NATS `user.guest_converted`, server `guest_reminder_last_shown_at`. Lives shipped: `TestComposeConvertGuestNATS_live`, `TestComposeGuestReminder_live`, `guest_reminder_e2e_live_test` (P1.1–P1.2). UX/docs leftovers: [client.md](client.md) §Guest. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md).
- [ ] **П.10 — Уведомления: тихие часы, гранулярность, voice join** — Set/Get quiet hours lives shipped (`TestComposeQuietHours_live`, `quiet_hours_e2e_live_test`, P2.5). Still open: FCM grouping (1 push per chat; [notifications.md](../features/notifications.md) §Каналы). → Batch 5 leftovers + Batch 14 [Notification].
- [x] **П.11 — Командирский режим и raise hand** — `SetCommanderMode`, `RaiseHand`, `GrantFloor`, LiveKit ducking, Flutter organizer panel. Спека: [voice-chat.md](../features/voice-chat.md). → Batch 14 [Voice] unimplemented RPCs.
- [ ] **П.12 — Качество войса/видео по подписке** — `GetEntitlements` → token `video_layer`, File upload cap, Flutter upgrade banner. Спека: [subscription.md](../features/subscription.md). → Batch 14 [Subscription] JWT tier.
- [ ] **П.13 — E2E encryption UX и multi-device** — `PutE2EKeyBackup` flow, verification code в DM, SQLCipher cache, key-change banner. Спека: [encryption.md](../features/encryption.md). → **Batch 2** encryption live tests.
- [ ] **П.14 — Stories до «ежедневного» продукта** — feed co-members, reactions UI, editor v2 (trim/stickers), post-match auto-story. Спека: [stories.md](../features/stories.md). → **Batch 7**.
- [ ] **П.15 — Бот-платформа v2 (partial)** — **shipped:** Flutter `/` slash picker, subcommands UI, Developer Portal catalog basics. **Open:** inbound `message.events` → webhook consumer; `lookupInteraction` hardcodes `CHAT_TYPE_CHANNEL` (DM/group deferred broken); Portal CSRF/manifest gaps; token rotation hub purge. Спека: [bots.md](../features/bots.md). → follow-up Bot batches.
- [ ] **П.16 — Подписка Premium + Space Pro end-to-end** — webhooks lifecycle, grace notifications, paywall/cosmetics/downgrade picker. Спека: [subscription.md](../features/subscription.md). → Batch 14 [Subscription] Critical/High.
- [ ] **П.19 — Accessibility выше baseline** — High Contrast theme, desktop shortcuts (`Ctrl+K`, message list nav), reduced motion toggle, semantics audit. Спека: [accessibility.md](../features/accessibility.md). → Batch 5 a11y хвосты.
- [x] **П.20 — Онбординг на ММ и спейсы** — `OnboardingController` flags, coach-marks MM/space/invite deep link, guest vs regular flows. Lives shipped: `onboarding_coach_e2e_live_test` (P2.12 / #20). Спека: [onboarding.md](../features/onboarding.md), [deep-links.md](../features/deep-links.md).


## Low

- [ ] **П.17 — Windows desktop first-class** — system tray, global PTT, background voice, auto-update stub, `flutter-windows` CI. Спека: [platforms.md](../features/platforms.md), [updates.md](../features/updates.md). → Batch 12 Windows sign-off.
- [ ] **П.18 — Game overlay для Windows** — MVP: speaking indicators, mute/deafen, hotkey toggle; architecture ADR (overlay process vs multi-window). Спека: [platforms.md](../features/platforms.md).


**Промпт-якорь:** `Product roadmap from docs/todo/product-roadmap.md` + `TDD: <фича> per docs/features/<spec>.md`.
