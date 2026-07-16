# TODO — Product Roadmap

[← Индекс](../TODO.md)

Сквозные продуктовые инициативы (не привязаны к одному сервису).

План *20 product improvements* (2026-07). Спеки: [docs/features/](../features/). **П.7 мультипрофиль — закрыт.** Пересечения с domain-файлами — ссылка, не дублировать.
## Critical

_Нет открытых пунктов._

## High

- [ ] **П.1 — Матчмейкинг внутри спейса** — `StartSpaceQueue`, Redis `mm:space:{space_id}`, `UpdateSpaceMmConfig`, Gateway `POST /spaces/{id}/matchmaking/queue`, Flutter вкладка ММ в спейсе. Критерий: два участника спейса матчатся; посторонний из глобальной очереди — нет. Спека: [matchmaking.md](../features/matchmaking.md) §«Внутри спейса». Связано: [Space] `mm_config_json` unused (Batch 14).
- [ ] **П.2 — Постматчевый цикл** — `RateTeammates`, `BanFromMatchmaking`, `ListMatchHistory`; таблицы `match_history`, `mm_ratings`, `mm_bans`; modal при выходе из match-squad; экран истории в профиле. Спека: [matchmaking.md](../features/matchmaking.md) §Оценка/Бан/История. Тесты: `matchmaking_rating_e2e_live_test.dart`, compose.
- [ ] **П.3 — Stories «Ищу пати» → матчмейкинг** — `RespondToLfpStory` (JOIN|INVITE), NATS `story.lfp_response`, Notification inline Accept/Decline, Flutter LFP card. Критерий: JOIN → accept → пати в очереди. Спека: [stories.md](../features/stories.md). → также Batch 7 (`story.lfp_created` subscriber).
- [ ] **П.4 — Каталог игр + заявки пользователей** — seed Dota 2/CS2/Valorant/PUBG; `SubmitGameRequest` → `pending_moderation`; Admin модерация; Flutter wizard «Добавить игру». Спека: [game-catalog.md](../features/game-catalog.md).


## Common

- [ ] **П.5 — Верификация ранга Steam/FACEIT (MVP)** — `linked_game_accounts`, OAuth/API, badge в ММ, фильтр `verified_rank_only`. Спека: [PROJECT.md](../PROJECT.md), [matchmaking.md](../features/matchmaking.md). → пересекается [User] verification gaps (Batch 14).
- [ ] **П.6 — Верификация Twitch/YouTube/DNS** — cron `VerificationStatusRefresh`, org TXT flow, Flutter Settings → Верификация. Спека: [verification.md](../features/verification.md). → Batch 14 [User] Verification V1.
- [ ] **П.8 — Синхронизация контактов телефонной книги** — Flutter hash + `POST /contacts/resolve`, onboarding «Найди друзей», `discoverable_by_phone`. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md). → **Batch 4** `compose_phone_sync_live_test`.
- [ ] **П.9 — Гостевой → постоянный аккаунт** — `ConvertGuest` negative tests, NATS `user.guest_converted`, server `guest_reminder_last_shown_at`, localized errors. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md). → **Batch 6**.
- [ ] **П.10 — Уведомления: тихие часы, гранулярность, voice join** — persist `notification_settings`/`quiet_hours` в БД; `VOICE_MEMBER_JOINED` push; FCM grouping. Спека: [notifications.md](../features/notifications.md). → Batch 5 (Flutter UI готов) + Batch 14 [Notification].
- [ ] **П.11 — Командирский режим и raise hand** — `SetCommanderMode`, `RaiseHand`, `GrantFloor`, LiveKit ducking, Flutter organizer panel. Спека: [voice-chat.md](../features/voice-chat.md). → Batch 14 [Voice] unimplemented RPCs.
- [ ] **П.12 — Качество войса/видео по подписке** — `GetEntitlements` → token `video_layer`, File upload cap, Flutter upgrade banner. Спека: [subscription.md](../features/subscription.md). → Batch 14 [Subscription] JWT tier.
- [ ] **П.13 — E2E encryption UX и multi-device** — `PutE2EKeyBackup` flow, verification code в DM, SQLCipher cache, key-change banner. Спека: [encryption.md](../features/encryption.md). → **Batch 2** encryption live tests.
- [ ] **П.14 — Stories до «ежедневного» продукта** — feed co-members, reactions UI, editor v2 (trim/stickers), post-match auto-story. Спека: [stories.md](../features/stories.md). → **Batch 7**.
- [ ] **П.15 — Бот-платформа v2** — autocomplete, subcommands, ephemeral/deferred, Developer Portal catalog, Flutter `/` picker. Спека: [bots.md](../features/bots.md). → Batch 14 [Bot].
- [ ] **П.16 — Подписка Premium + Space Pro end-to-end** — webhooks lifecycle, grace notifications, paywall/cosmetics/downgrade picker. Спека: [subscription.md](../features/subscription.md). → Batch 14 [Subscription] Critical/High.
- [ ] **П.19 — Accessibility выше baseline** — High Contrast theme, desktop shortcuts (`Ctrl+K`, message list nav), reduced motion toggle, semantics audit. Спека: [accessibility.md](../features/accessibility.md). → Batch 5 a11y хвосты.
- [ ] **П.20 — Онбординг на ММ и спейсы** — `OnboardingController` flags, coach-marks MM/space/invite deep link, guest vs regular flows. Спека: [onboarding.md](../features/onboarding.md), [deep-links.md](../features/deep-links.md). → **Batch 4** onboarding E2E.


## Low

- [ ] **П.17 — Windows desktop first-class** — system tray, global PTT, background voice, auto-update stub, `flutter-windows` CI. Спека: [platforms.md](../features/platforms.md), [updates.md](../features/updates.md). → Batch 12 Windows sign-off.
- [ ] **П.18 — Game overlay для Windows** — MVP: speaking indicators, mute/deafen, hotkey toggle; architecture ADR (overlay process vs multi-window). Спека: [platforms.md](../features/platforms.md).


**Промпт-якорь:** `Product roadmap from docs/todo/product-roadmap.md` + `TDD: <фича> per docs/features/<spec>.md`.
