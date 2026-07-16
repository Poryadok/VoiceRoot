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
- [ ] **Mobile device E2E** — App Links / custom scheme Android/iOS (сейчас только parser smoke в `deeplink_web_test.dart`).

### Guest & onboarding live

- [ ] **Onboarding coach-marks E2E** — полный tour step-through + `guest_onboarding_e2e_live_test` (compose); widget-якоря покрыты `guest_onboarding_anchor_keys_test.dart`.

### Flutter delivery

- [ ] **Commit waves A–J (Flutter client)** — один PR: state/ui/l10n/tests из аудита 2026-07-15; после merge — `make flutter-ci` на CI.
### Multi-profile

- [ ] **[Multi-Profile] DeleteProfile has no Gateway REST** — gRPC + `SoftDeleteProfile` implemented (`user_verification.go`); no `DELETE /api/v1/users/profiles/{id}` in `src/backend/gateway/transcode_users.go`; Flutter `users_client.dart` has no `deleteProfile`.
- [ ] **[Multi-Profile] No delete-profile UI** — settings exposes create only (`settings_sheet.dart` → `CreateProfileSheet`); no manage-profiles screen to remove secondary profiles.
- [ ] **[Multi-Profile] Frozen profiles invisible in switcher UI** — backend blocks switch (`frozen_at`, `JdbcProfileSwitchValidator`); `VoiceProfile` / `proto_mappers.dart` omit `frozenAt`; `ProfileSwitcher` / `ProfileAvatarSwitcher` list all profiles with no disabled state or copy.
- [ ] **[Multi-Profile] `ProfileDowngradePickerScreen` unreachable** — screen + `submitDowngradeProfiles` exist; never routed from subscription expiry/cancel (см. [Subscription] Downgrade lifecycle).


## Common

### Growth & accessibility



Baseline onboarding/deep-links/a11y — [PLAN.md](../PLAN.md); остаток vs [deep-links.md](../features/deep-links.md), [onboarding.md](../features/onboarding.md), [accessibility.md](../features/accessibility.md).

**Flutter UX waves A–J (2026-07-15):** закрыто в рабочей копии; остаток — commit PR + серверные хвосты ниже.


- [ ] **Notification settings: серверная персистентность** — Flutter UI готов (`NotificationSettingsScreen`, per-chat override, quiet hours в `SharedPreferences`); backend `GetNotificationSettings` / `UpdateNotificationSettings` / `SetQuietHours` не пишут в БД → mute/DND не влияют на push. См. Batch 14 §Notification — Critical/High.

- [ ] **Quiet hours: sync client ↔ Notification service** — после backend persist: читать/писать `SetQuietHours` вместо только `notification_quiet_hours_storage.dart`; иначе DND на втором устройстве не синхронизируется.

- [ ] **A11y: message list keyboard nav** — [accessibility.md](../features/accessibility.md) §«Навигация по списку сообщений»: `↑`/`↓`, `R`, `E` в `chat_room_panel` — не реализовано.

- [ ] **A11y: text-scale ×1.5 — расширить smoke** — сейчас только chat list/room (`chat_text_scale_test.dart`); добавить settings, matchmaking, stories, notification settings.

- [ ] **A11y: focus return после modal** — `VoiceFocusTrap` ловит фокус; проверить возврат на trigger при закрытии `showVoiceBottomSheet`, coach-marks, call overlay.

- [ ] **A11y: pre-release TalkBack / VoiceOver** — чеклист в [accessibility.md](../features/accessibility.md) §Pre-release; ручная приёмка перед mobile release (**Вы**).

- [ ] **A11y: Axe / web accessibility CI** — [accessibility.md](../features/accessibility.md) §Тестирование: Axe DevTools (или аналог) для Flutter web — не в CI.

- [ ] **VoiceListSkeleton / VoiceStatePanel — остаточные loaders** — wave A не покрыла: `chat_room_panel` (pagination), space members/bots/invites, `player_profile_sheet`, `story_viewer_screen` — всё ещё `CircularProgressIndicator` вместо skeleton/state panel.

- [ ] **VoiceListSkeleton + VoiceStatePanel widget tests** — dedicated tests отсутствуют (reconnect/onboarding/focus trap покрыты).

- [ ] **api_error_messages — расширить покрытие** — helpers есть для wave A доменов; `chat_room_panel`, search, subscription/billing, settings screens могут показывать сырые API strings / hardcoded `not authenticated`.

- [ ] **VoiceDisabledAction — расширить покрытие** — wave H: space tree / roles / slow mode; остальные permission-gated действия (chat moderation, voice room create, MM guest restrictions) без reason tooltip.

- [ ] **MobileChatStrip — scope на full-screen фичах** — strip только при `narrow && selectedChatId != null` в `app.dart`; matchmaking full-screen (`queue_search_screen`, `game_catalog_screen`) и settings sub-routes без strip — сверить с [platforms.md](../features/platforms.md) / navigation.

- [ ] **chat_info_panel notification tile — narrow E2E** — compact layout; overflow поправлен в widget-тестах; live/compose E2E на узкой ширине (связано с Critical Batch 2 при наличии стека).


**Промпт-якорь:** `Growth/A11y from docs/todo/client.md Common Batch 5`.



### Guest accounts UX



Baseline (2026-06) закрыт; хвосты UX/E2E ниже. Спека: [auth-and-contacts.md](../features/auth-and-contacts.md).

- [ ] **Convert-guest: recovery для аккаунтов после бага transport-пароля** — аккаунты, сконвертированные до фикса (2026-07), остались с неизвестным паролем; нужен self-service reset password или support-runbook.
- [ ] **Convert-guest: док auth-service.md** — явно описать, что `password` в `ConvertGuest` = новый пароль regular-аккаунта (JWT гостя достаточен), не проверка transport-пароля.
- [ ] **Convert-guest live в compose-e2e** — `TestComposeConvertGuest_live` (новый пароль + login) в CI workflow; сейчас только opt-in локально.
- [ ] **Convert-guest: negative Auth integration tests** — duplicate email, password <8, non-guest token, missing email/phone; дополнить `ConvertGuestIntegrationTest`.
- [ ] **Convert-guest: NATS `user.guest_converted`** — довести `GuestConvertNatsEventIntegrationTest`: REST convert + assert publish (сейчас stub).
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

> Story backend-хвосты — [backend.md](../backend.md) § Low § Story.
> Design/Penpot — [design.md](../design.md).
