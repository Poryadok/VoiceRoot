# TODO — Client

[← Индекс](../TODO.md)

Flutter/web/mobile, a11y, onboarding, deep links, guest UX (`src/frontend/`).

## Critical

_Пока пусто — критичные клиентские блокеры добавляйте сюда._

## High

### Mobile & push setup (**Вы**)

- [ ] **iOS Team ID** — `Runner.entitlements` associated-domains: заменить `TEAMID`; SHA-256 в assetlinks вместо `PLACEHOLDER` ([`DEPLOYMENT.md`](../DEPLOYMENT.md)).
- [ ] **Firebase / FCM prod** — `google-services.json`, web config в CI secrets; FlutterFire для staging/prod клиента.

### Deep links & mobile acceptance

- [ ] **Приёмка invite→join** — universal link открывает приложение / web fallback ([deep-links.md](../features/deep-links.md)).
- [ ] **Mobile device E2E** — App Links / custom scheme on Android/iOS still open. Host scaffold: `integration_test/device_driver_smoke_test.dart` + CI `flutter-device-driver` (#42); Chrome parser: `deeplink_web_test.dart`. Physical App Links / AASA / NT-05 remain.

### Guest & onboarding live

- [x] **Onboarding coach-marks E2E** — `onboarding_coach_e2e_live_test` (spaces/MM + invite deep link); guest: `guest_onboarding_e2e_live_test`; widget-якоря: `guest_onboarding_anchor_keys_test` / `onboarding_overlay_test`.

### Flutter delivery

- [ ] **Commit waves A–J (Flutter client)** — один PR: state/ui/l10n/tests из аудита 2026-07-15; после merge — `make flutter-ci` на CI.
### Multi-profile

- [x] **[Multi-Profile] No delete-profile UI** — `ManageProfilesSheet` in settings (`settings_manage_profiles`); `DELETE /api/v1/users/profiles/{id}` via `VoiceUsersClient.deleteProfile`; blocks primary/active delete (Batch 13).
- [x] **[Multi-Profile] Frozen profiles invisible in switcher UI** — `VoiceProfile.frozenAt` + `proto_mappers.dart`; `ProfileSwitcher` disabled items + `(Frozen)` label; `ProfileAvatarSwitcher` skips frozen on swipe (Batch 11).
- [x] **[Multi-Profile] `ProfileDowngradePickerScreen` unreachable** — **done (Batch 12):** routed when free tier + >2 profiles (`profileDowngradeRequiredProvider` in `app.dart`); screen + `submitDowngradeProfiles` existed.


### Auth UI (REST есть, экранов нет)

- [ ] **AuthScreen: телефон + OTP** — спека [auth-and-contacts.md](../features/auth-and-contacts.md): телефон default; `auth_screen.dart` только email.
- [ ] **Auth UI: sessions / revoke, delete-account, password-reset** — REST есть (`GET /api/v1/auth/sessions`, delete/restore, `POST /api/v1/auth/password/reset`); экранов нет.

### Telegram-parity UX audit — Flutter (2026-08-28)

Источник: `tmp/telegram-ux-audit/AUDIT.md` (R2-A03–A05, R3-A08, R3-A11). Backend RPC-хвосты — [backend.md](backend.md) § Telegram-parity audit / § Chat navigation.

- [x] **R2-A03 — Profile switcher vs rail contract** — `ProfileAvatarMenuButton` §1.1a in rail (`profile_avatar_menu.dart`); desktop session bar is label-only (no combobox); mobile tap opens menu + swipe switch (`ProfileAvatarSwitcher`). Archive entry shows unavailable snackbar until Chat archive RPC ships (Batch 10).
- [ ] **R2-A04 — Mobile shell IA** — `MobileShellTabBar` (Chats/Social/Match) on `Scaffold.bottomNavigationBar` when narrow + no open chat (Batch 10 incremental); **deferred:** hamburger drawer (folders, QA, settings), keyboard-hide tab bar, stacked chrome overflow rules.
- [ ] **R2-A05 — Active strip LRU semantics** — strip tracks **opened** chats (≤100 LRU); `MobileChatStrip` + `mobile_opened_chat_strip.dart` shipped; long-press remove + 100-cap LRU eviction snackbar shipped (Batch 9).
- [x] **R3-A08 — Strip widget test wrong contract** — `mobile_chat_strip_test.dart` asserts opened-chat LRU, not inbox rows (§1.6).
- [x] **R3-A11 — Composer a11y parity** — transient §3.6b emoji panel + §3.6a attach popup with §3.6e focus trap/return (`composer_panels.dart`); click/tap activation on desktop (Batch 9).
- [ ] **Archive list screen** — `Screen / Chat / Archive` (desktop + mobile); entry via profile RC only — [navigation.md](../features/navigation.md); blocked on Chat archive list RPC ([backend.md](backend.md)).
- [ ] **Mobile stacked chrome** — active strip + app bar + pinned bar + composer + bottom tabs overflow rules §1.6a — Penpot · v3 + Flutter layout.


## Common

### Growth & accessibility



Baseline onboarding/deep-links/a11y — [PLAN.md](../PLAN.md); остаток vs [deep-links.md](../features/deep-links.md), [onboarding.md](../features/onboarding.md), [accessibility.md](../features/accessibility.md).

**Flutter UX waves A–J (2026-07-15):** закрыто в рабочей копии; остаток — commit PR + серверные хвосты ниже.


- [x] **Quiet hours: client dual-write** — `notification_settings_screen` reads/writes `Get/SetQuietHours` API; `notification_quiet_hours_storage.dart` is offline cache only (API-first load, local fallback on failure). Live: `quiet_hours_e2e_live_test` (NT-04).

- [x] **A11y: message list keyboard nav** — `↑`/`↓`, `R`, `E` wired in `voice_shortcuts.dart` + `chat_room_panel`; widget tests in `voice_shortcuts_keyboard_test.dart` (Batch 9).

- [x] **A11y: text-scale ×1.5 — расширить smoke** — `shell_text_scale_test.dart`: settings sheet, notification settings, game catalog, story archive; chat list/room remain in `chat_text_scale_test.dart` (Batch 10).

- [x] **A11y: focus return после modal** — `VoiceFocusReturn` + coach-mark `VoiceFocusTrap`; call overlay restores on dispose; `showVoiceBottomSheet` escape restores focus (Batch 10).

- [ ] **A11y: pre-release TalkBack / VoiceOver** — чеклист в [accessibility.md](../features/accessibility.md) §Pre-release; ручная приёмка перед mobile release (**Вы**).

- [x] **A11y: Axe / web accessibility CI** — landmark contract `make a11y-web-axe` (Axe analog) in flutter CI; full CanvasKit Axe deferred.

- [ ] **VoiceListSkeleton / VoiceStatePanel — остаточные loaders** — wave A не покрыла: `chat_room_panel` (pagination), space members/bots/invites, `player_profile_sheet`, `story_viewer_screen` — всё ещё `CircularProgressIndicator` вместо skeleton/state panel.

- [ ] **VoiceListSkeleton + VoiceStatePanel widget tests** — dedicated tests отсутствуют (reconnect/onboarding/focus trap покрыты).

- [ ] **api_error_messages — расширить покрытие** — helpers есть для wave A доменов; `chat_room_panel`, search, subscription/billing, settings screens могут показывать сырые API strings / hardcoded `not authenticated`.

- [ ] **VoiceDisabledAction — расширить покрытие** — wave H: space tree / roles / slow mode; остальные permission-gated действия (chat moderation, voice room create, MM guest restrictions) без reason tooltip.

- [ ] **MobileChatStrip — scope на full-screen фичах** — strip только при `narrow && selectedChatId != null` в `app.dart`; matchmaking full-screen (`queue_search_screen`, `game_catalog_screen`) и settings sub-routes без strip — сверить с [platforms.md](../features/platforms.md) / navigation.

- [ ] **chat_info_panel notification tile — narrow E2E** — compact layout; overflow поправлен в widget-тестах; live/compose E2E на узкой ширине (связано с Critical Batch 2 при наличии стека).


### Chat UX (спека text-chat / navigation)

- [ ] **Локальные черновики** — Hive/SQLite, один на chat; multi-device sync отказано в спеке.
- [ ] **OG link preview** — unfurl на клиенте; сервер не обязан.
- [ ] **Stickers / GIF / voice-note composer** — нет UI (и нет backend packs). См. [backend.md](backend.md) § High Chat; решение спеки — [product-roadmap.md](product-roadmap.md).
- [ ] **Кастомные папки чатов** — All/DMs/Groups + custom; RPC Unimplemented. MobileChatStrip на MM/settings — пункт выше.
- [ ] **In-chat search: next/prev highlight** — [search.md](../features/search.md).
- [ ] **Favorites / QR add friend / phone-book sync UI** — gRPC contacts есть, REST list нет; QR в продукте отсутствует. [friends.md](../features/friends.md).
- [ ] **Idle 5 мин → `UpdatePresence idle`** — сейчас idle только если клиент сам пришлёт. [presence.md](../features/presence.md).
- [ ] **Windows: автоопределение игры** — process list → `game_title` + toggle; 0 кода. Post-MVP рядом с П.17.
- [ ] **E2E fingerprint + key-change banner** — [encryption.md](../features/encryption.md); П.13.
- [ ] **Shorebird OTA или явный defer** — [updates.md](../features/updates.md).
- [ ] **Expired file «bones» placeholder + refresh URL** — [file-storage.md](../features/file-storage.md).
- [ ] **Voice: local MP3 recording; ducking via `setVolume`; in-voice overlay без кражи фокуса** — [voice-chat.md](../features/voice-chat.md).
- [ ] **Screen share: desktop source picker + system audio (Windows); явный simulcast 720/360/180** — [screen-share.md](../features/screen-share.md).
- [ ] **Settings language sync** — profile.locale vs OS pref; API error codes локализация. [i18n.md](../features/i18n.md).
- [ ] **Help FAQ vs реальный UI** — онбординг ядро complete; FAQ не совпадает со спейсами/ММ.


**Промпт-якорь:** `Growth/A11y from docs/todo/client.md Common Batch 5`.



### Guest accounts UX



Baseline (2026-06) закрыт; хвосты UX/E2E ниже. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md).

- [ ] **Convert-guest: recovery для аккаунтов после бага transport-пароля** — аккаунты, сконвертированные до фикса (2026-07), остались с неизвестным паролем; нужен self-service reset password или support-runbook. Backend `POST /api/v1/auth/password/reset` **есть**; Flutter UI нет.
- [x] **Convert-guest: док auth-service.md** — `ConvertGuest` §: `password` = новый пароль regular-аккаунта (JWT гостя достаточен), не transport-пароль.
- [x] **Convert-guest live в compose-e2e** — `TestComposeConvertGuest_live` in `.github/ci/e2e-features.yml` smoke_gateway (CI tier 2).
- [x] **Convert-guest: negative Auth integration tests** — duplicate email, password <8, non-guest token, missing email/phone in `ConvertGuestIntegrationTest`.
- [x] **Convert-guest: NATS `user.guest_converted`** — `TestComposeConvertGuestNATS_live` shipped (P1.1).
- [ ] **Guest save-account reminder: server last-shown** — в спеке «локальный или серверный timestamp»; сейчас только `SharedPreferences` (кросс-устройство не синхронизируется).


**Промпт-якорь:** `Guest accounts UX from docs/todo/client.md Common Batch 6`.


### Multi-profile

- [ ] **[Multi-Profile] Create flow missing avatar** — spec §создание: «ник, аватар»; `CreateProfileSheet` only `display_name` + privacy preset; no presigned upload on create.
- [ ] **[Multi-Profile] Change primary profile API/UI missing** — `is_primary` set at bootstrap only; no way to reassign which profile phone search returns.
- [ ] **[Multi-Profile] Accent color not choosable on create** — backend `CreateProfileRequest.accent_color` + palette default (`profileaccent`); `CreateProfileSheet` does not expose picker (only post-create in settings `_AccentPicker`).
- [ ] **[Multi-Profile] `profile_accent_storage` legacy dual-write** — stale comment «until User Service exposes accent_color» (`profile_accent_storage.dart`); settings picker still writes local index while server has `profiles.accent_color`.
- [ ] **[Multi-Profile] Guest + multi-profile product rule undocumented** — no `CreateProfile` tier/guest guard; settings create action visible for guests; clarify in `multi-profile.md` or gate in UI/API.


## Low

### Stories (post-MVP, client)

- [ ] **Story editor v2** — stickers, doodle, filters, clip trim (§Редактор / §Клип).
- [ ] **Full per-story `PrivacyAudiencePicker`** — space multiselect on create.
- [ ] **Anonymous view (Premium)** — backend `MarkViewed.anonymous`; client UX отложен.

### Windows & desktop

- [ ] **Windows sign-off** — скилл `voice-project-full-verification`: `compose-config-ci`, `buf-ci`, `flutter-ci` — OK; `backend-test-ci-short` — после `c3598f3` fix [`jetstream_test.go`](../../src/backend/messaging/internal/messageevents/jetstream_test.go) (Flush + EnsureStream) **перепроверить на Windows/Docker**; compose smoke E2E не гонялся. См. [TESTING.md](../TESTING.md) § «Локальные грабли».
### Multi-profile

- [ ] **[Multi-Profile] `profile_context_controller` untested** — MM cancel, space exit, WS reconnect on `activeProfileId` change; widget tests cover switcher only (`profile_switcher_test.dart`, `create_profile_sheet_test.dart`).



**Промпт-якорь:** `Client from docs/todo/client.md` + приоритет/подсекцию.

> Story backend-хвосты — [backend.md](backend.md) § Low § Story.
> Design/Penpot — [design.md](design.md).
