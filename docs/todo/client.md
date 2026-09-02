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
- [x] **Auth UI: password-reset** — `PasswordResetScreen` (email OTP → new password); forgot-password on `AuthScreen`; `VoiceAuthClient.sendPasswordResetOtp` / `resetPassword` (**Batch 30a**). **Sessions/revoke UI shipped** (`ActiveSessionsScreen`, Security settings, **Batch 29b**).
- [x] **Auth UI: delete-account** — `SecuritySettingsScreen` confirm+password → `POST /api/v1/auth/delete-account`; guest blocked; logout on success (**Batch 28b**). Restore-account UI deferred.

### Telegram-parity UX audit — Flutter (2026-08-28)

Источник: `tmp/telegram-ux-audit/AUDIT.md` (R2-A03–A05, R3-A08, R3-A11). Backend RPC-хвосты — [backend.md](backend.md) § Telegram-parity audit / § Chat navigation.

- [x] **R2-A03 — Profile switcher vs rail contract** — `ProfileAvatarMenuButton` §1.1a in rail (`profile_avatar_menu.dart`); desktop session bar is label-only (no combobox); mobile tap opens menu + swipe switch (`ProfileAvatarSwitcher`). Archive via profile RC → `/chats/archive` (`ChatArchiveScreen`, Batch 16).
- [ ] **R2-A04 — Mobile shell IA** — `MobileShellTabBar` (Chats/Social/Match) on `Scaffold.bottomNavigationBar` when narrow + no open chat (Batch 10 incremental); **incremental:** `MobileShellDrawer` hamburger stub (folders, QA, settings) shipped (parallel/client); **keyboard-hide tab bar** on narrow + open keyboard shipped (Batch 21b); **keyboard-hide active strip** on open chat shipped (Batch 28a); **incremental (Batch 29a):** pinned bar single-line collapse when >1 pin, text-scale ×1.5 smoke, strip hidden on full-screen shell overlays (MM/settings routes via `MobileShellOverlayObserver`); **deferred:** remaining stacked chrome polish.
- [x] **R2-A05 — Active strip LRU semantics** — strip tracks **opened** chats (≤100 LRU); `MobileChatStrip` + `mobile_opened_chat_strip.dart` shipped; long-press remove + 100-cap LRU eviction snackbar shipped (Batch 9).
- [x] **R3-A08 — Strip widget test wrong contract** — `mobile_chat_strip_test.dart` asserts opened-chat LRU, not inbox rows (§1.6).
- [x] **R3-A11 — Composer a11y parity** — transient §3.6b emoji panel + §3.6a attach popup with §3.6e focus trap/return (`composer_panels.dart`); click/tap activation on desktop (Batch 9).
- [x] **Archive list screen** — `Screen / Chat / Archive` via profile RC → `/chats/archive`; `ChatArchiveScreen` + `inbox=archive` list/unarchive (Batch 16).
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

- [x] **VoiceListSkeleton / VoiceStatePanel — wave A residual loaders** — PR #128: `chat_room_panel` pagination, space members/bots/invites, `player_profile_sheet`, `story_viewer_screen`, settings/search/subscription list loads → skeleton/state panel; `VoiceListSkeleton` uses `shrinkWrap: true` + `primary: false` (sheet CI).

- [ ] **VoiceListSkeleton / VoiceStatePanel — остаточные loaders** — list loads still CPI: `group_members_sheet`, `chat_info_panel`, `space_roles_sheet`, `manage_profiles_sheet` / `create_profile_sheet`, `active_sessions_screen`, `manage_folders_sheet`, `bot_install_page`, `profile_downgrade_picker_screen` (button-level CPI OK).

- [ ] **VoiceListSkeleton + VoiceStatePanel widget tests** — dedicated tests отсутствуют (reconnect/onboarding/focus trap покрыты; `social_panel_test` only asserts `find.byType(VoiceListSkeleton)`).

- [ ] **api_error_messages — расширить покрытие** — PR #128 helpers for chat/search/settings/subscription/space bots|members|invites/player profile; residual surfaces may still show raw API strings.

- [ ] **VoiceDisabledAction — расширить покрытие** — wave H: space tree / roles / slow mode; остальные permission-gated действия (chat moderation, voice room create, MM guest restrictions) без reason tooltip.

- [x] **MobileChatStrip — scope на full-screen фичах** — strip скрывается при `mobileShellOverlayDepthProvider > 0` (PageRoute поверх shell: MM catalog/search, settings sub-routes); `shouldShowMobileChatStrip` + `MobileShellOverlayObserver` (**Batch 29a**).

- [ ] **chat_info_panel notification tile — narrow E2E** — compact layout; overflow поправлен в widget-тестах; live/compose E2E на узкой ширине (связано с Critical Batch 2 при наличии стека).


### Chat UX (спека text-chat / navigation)

- [ ] **Локальные черновики** — Hive/SQLite, один на chat; multi-device sync отказано в спеке.
- [ ] **OG link preview** — unfurl на клиенте; сервер не обязан.
- [ ] **Stickers / GIF / voice-note composer** — нет UI (и нет backend packs). См. [backend.md](backend.md) § High Chat; решение спеки — [product-roadmap.md](product-roadmap.md).
- [x] **Message requests inbox UI** — virtual «Запросы» folder in rail/drawer (visible when pending > 0, unread badge); removed middle-column segmented toggle (§1.3 tombstone); accept/decline on list rows; `notificationTypeMessageRequest` settings toggle — **Batch 22b** (backend bucketing Batch 21a).
- [x] **Кастомные папки чатов** — All/DMs/Groups + custom; REST folders + rail/drawer UI shipped (parallel/client); **pin/reorder UI** in list ctx + custom-folder drag reorder shipped (Batch 21b); edit-folders management UI shipped (**Batch 25b**).
- [ ] **In-chat search: next/prev highlight** — [search.md](../features/search.md).
- [x] **Favorites / contacts UI** — Social panel Contacts + Favorites tabs; `VoiceFriendsClient` list/add/set favorite; star toggle on friends/contacts (`social_panel.dart`, `friends_client.dart`) — **Batch 23b**. **QR add friend UI** — my-code + paste profile link (`qr_add_friend_sheet.dart`) — **Batch 26a**; phone-book API stub wired (**Batch 25a**).
- [ ] **Phone contacts sync — real pipeline** — UI calls `syncPhoneContacts(const [])` → stub snackbar only (`social_panel.dart`); backend + Gateway + `TestComposePhoneSync_live` exist; needs platform contact permission + hash picker (replace empty stub).
- [ ] **QR add-friend — live camera scanner** — Batch 26a ships paste field; l10n implies camera scan. Add `mobile_scanner` (or similar) or narrow copy to paste-only.
- [x] **Blocked accounts UI** — Social panel Blocked tab; `VoiceFriendsClient` `listBlocked`/`unblockAccount`; gateway `GET/DELETE /api/v1/friends/blocks` — **Batch 24a**.
- [x] **Outgoing friend request declined label** — `PendingFriendRequest.status` from API; Requests tab shows «Declined» vs «Request pending» for outgoing rows — **Batch 24b**.
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

- [ ] **Convert-guest: recovery для аккаунтов после бага transport-пароля** — аккаунты, сконвертированные до фикса (2026-07), остались с неизвестным паролем; self-service через password-reset UI (**Batch 30a**) или support-runbook.
- [x] **Convert-guest: док auth-service.md** — `ConvertGuest` §: `password` = новый пароль regular-аккаунта (JWT гостя достаточен), не transport-пароль.
- [x] **Convert-guest live в compose-e2e** — `TestComposeConvertGuest_live` in `.github/ci/e2e-features.yml` smoke_gateway (CI tier 2).
- [x] **Convert-guest: negative Auth integration tests** — duplicate email, password <8, non-guest token, missing email/phone in `ConvertGuestIntegrationTest`.
- [x] **Convert-guest: NATS `user.guest_converted`** — `TestComposeConvertGuestNATS_live` shipped (P1.1).
- [ ] **Guest save-account reminder: server last-shown** — в спеке «локальный или серверный timestamp»; сейчас только `SharedPreferences` (кросс-устройство не синхронизируется).


**Промпт-якорь:** `Guest accounts UX from docs/todo/client.md Common Batch 6`.


### Multi-profile

- [x] **[Multi-Profile] Create flow missing avatar** — `CreateProfileSheet` optional avatar pick + presigned upload after profile switch (`parallel/client-qa-rail-profiles`).
- [ ] **[Multi-Profile] Change primary profile API/UI missing** — `is_primary` set at bootstrap only; no way to reassign which profile phone search returns.
- [x] **[Multi-Profile] Accent color not choosable on create** — `CreateProfileSheet` accent swatches from token catalog; sends `accent_color` on create (**Batch 17**).
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
