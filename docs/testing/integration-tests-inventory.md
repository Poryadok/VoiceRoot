# Инвентарь интеграционных тестов

Дата сверки: **2026-08-15**. Источники: [FEATURES.md](../FEATURES.md), `docs/features/`, [TESTING.md](../TESTING.md), [PLAN.md](../PLAN.md), [`.github/ci/e2e-features.yml`](../../.github/ci/e2e-features.yml), `docs/todo/`.

План закрытия пробелов: [integration-tests-gap-plan.md](integration-tests-gap-plan.md).

## Легенда

| Маркер | Смысл |
|--------|--------|
| `[exists]` | Compose live (`TestCompose*_live`) и/или Flutter live (`*_e2e_live_test.dart`) **осмысленно** покрывают путь |
| `[partial]` | Есть service IT / unit / widget / Auth IT, но нет сквозного Gateway↔клиент пути |
| `[missing]` | Документированный путь без найденного интеграционного покрытия |
| `[deferred]` | Фича отложена ([PLAN.md](../PLAN.md) / federation) — тесты не требуются до решения |
| `[n/a]` | Ручной staging/ops чеклист, не автоматизируемый IT в текущей модели |

**Слои «интеграции» по [TESTING.md](../TESTING.md):**

1. **Gateway compose live** — `src/backend/gateway/compose_*_live_test.go` (`VOICE_RUN_LIVE_COMPOSE=true`)
2. **Flutter live** — `src/frontend/test/*_e2e_live_test.dart` (`VOICE_RUN_LIVE_INTEGRATION=true`)
3. **Go service IT** — `*_integration_test.go` + testcontainers (nightly / `backend-go-integration`)
4. **Auth Java IT** — `*IntegrationTest.java` + Testcontainers
5. **Flutter `integration_test/`** — device/browser driver (сейчас почти пусто; см. README)

Smoke vs full: [e2e-features.yml](../../.github/ci/e2e-features.yml). В инвентаре «существует» = тест есть в репо, **не** обязательно в smoke.

---

## Сводка

| | Count |
|--|------:|
| **Всего кейсов** | 208 |
| `[exists]` | ~116 |
| `[partial]` | ~33 |
| `[missing]` | ~52 |
| `[deferred]` | 1 |
| `[n/a]` | 4 |

> Подсчёт по строкам кейсов (колонка статуса). При добавлении тестов обновлять таблицу и gap-plan. Counts after 2026-08-15 gap-plan sprint (FR-03/PV-04, MM-05/06, FL-02/03, AN-03/04, P0 smoke).

### Замечания по манифесту CI

Ранее smoke указывал несуществующие `func` (`TestComposeFriends_live`, `TestComposeSearch_live`) — **исправлено** на `TestComposeFriendsInvitation_live` / `TestComposeSearchInChat_live`; в smoke также E2E DM + Tier-0 delivery.
---

## 1. Текстовый чат — [text-chat.md](../features/text-chat.md)

**Статус PLAN:** shipped (DM + groups + markdown/mentions/pins/threads).

### Путь A — DM: создать, отправить, realtime

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| TC-DM-01 | REST auth → create DM → send → WS `message_create` | `[exists]` | Gateway: `TestComposeDMRealtime_live`, `TestComposeChatMessagingDM_live`; Flutter: `gateway_dm_ws_live_integration_test` («REST auth + create DM + WS message_create»), `dm_two_users_e2e_live_test` |
| TC-DM-02 | Два пользователя: JWT refresh + mark read fanout | `[exists]` | Flutter: `dm_two_users_e2e_live_test` («two users: DM, WS delivery, auth refresh, mark read») |
| TC-DM-03 | WS resume / REST catch-up после gap | `[exists]` | Gateway: `TestComposeWSResume_live`; Flutter: `ws_resume_e2e_live_test` |
| TC-DM-04 | Delivery receipts (`delivery_ack` → `message_delivered`) | `[exists]` | Gateway: `TestComposeDeliveryReceipts_live`; Flutter: `message_delivery_e2e_live_test` |
| TC-DM-05 | Typing indicator | `[exists]` | Gateway: `TestComposeTyping_live`; Flutter: `message_typing_e2e_live_test` |
| TC-DM-06 | Edit / delete + WS `message_update` | `[exists]` | Gateway: `TestComposeMessageEditDelete_live`; Flutter: `message_edit_delete_e2e_live_test` |
| TC-DM-07 | Запросы от незнакомцев (requests inbox → accept) | `[exists]` | Gateway: `TestComposeDMRequests_live`; Flutter: `dm_requests_e2e_live_test` |
| TC-DM-08 | Архивирование / скрытие DM | `[partial]` | Chat IT: `TestArchiveChat_HidesFromListChats`, `TestMuteChat_*`; Gateway transcode; Flutter client. Compose/Flutter live still open |
| TC-DM-09 | Черновики локально на устройстве | `[partial]` | Только клиентская логика; нет live IT (сервер не хранит) |

### Путь B — Группы

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| TC-GRP-01 | Create group + invite + avatar + kick | `[exists]` | Gateway: `TestComposeGroups_live`; Flutter: `groups_e2e_live_test` |
| TC-GRP-02 | Roles / leave / owner cannot leave | `[exists]` | Gateway: `TestComposeGroupRoles_live`; Flutter: `group_roles_e2e_live_test` |
| TC-GRP-03 | Лимит 500 участников (кнопка disabled) | `[missing]` | — |
| TC-GRP-04 | Service IT group/members | `[partial]` | `chat/.../group_integration_test.go`, `members_integration_test.go` |

### Путь C — Markdown, mentions, pins, reactions, threads, shared media

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| TC-MSG-01 | Markdown source + preview strip | `[exists]` | Gateway: `TestComposeMarkdownPreview_live`; Flutter: `markdown_e2e_live_test` |
| TC-MSG-02 | @user mention → WS + notification | `[exists]` | Flutter: `mentions_e2e_live_test`; Messaging IT: `messaging_mentions_integration_test.go` |
| TC-MSG-03 | @here / @everyone с правами | `[exists]` | Messaging IT + Gateway: `TestComposeMentionsEveryoneDeny_live` |
| TC-MSG-04 | Pins list/unpin + WS | `[exists]` | Gateway: `TestComposePins_live`; Flutter: `pins_e2e_live_test` |
| TC-MSG-05 | Reactions aggregate/remove | `[exists]` | Flutter: `reactions_e2e_live_test`; Messaging IT: `messaging_reactions_integration_test.go` |
| TC-MSG-06 | DM reply/thread endpoints | `[exists]` | Gateway: `TestComposeThreadsDMReply_live`; Flutter: `threads_e2e_live_test` |
| TC-MSG-07 | Shared media list after attachment | `[exists]` | Gateway: `TestComposeSharedMedia_live`; Flutter: `shared_media_e2e_live_test` |
| TC-MSG-08 | Slow mode в space channel | `[exists]` | Flutter: `spaces_slow_mode_e2e_live_test` |
| TC-MSG-09 | Стикеры / GIF как продуктовый путь | `[missing]` | — (упомянуты в text-chat; отдельного live нет) |
| TC-MSG-10 | Голосовые сообщения (audio attachment + player) | `[missing]` | — |
| TC-MSG-11 | View counters в группах/каналах | `[missing]` | — |
| TC-MSG-12 | Runtime toggle `threads_enabled` / `allow_user_main_feed` | `[partial]` | Defaults: `thread_settings_integration_test.go`; runtime UpdateChat ignores (todo/backend) |

---

## 2. Пересылка — [forward-messages.md](../features/forward-messages.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| FW-01 | Forward DM → group с атрибуцией | `[exists]` | Flutter: `forward_messages_e2e_live_test` («forward DM message to group preserves attribution») |
| FW-02 | Forward в channel | `[exists]` | Messaging IT + Gateway: `TestComposeForwardChannelCommentary_live`; Flutter channel+commentary |
| FW-03 | Copy as new (без атрибуции) | `[missing]` | — |
| FW-04 | Privacy forbid forward | `[exists]` | Messaging IT + Gateway: `TestComposeForwardPrivacyDeny_live`; Flutter FW-04 |
| FW-05 | Multi-select forward | `[missing]` | — |
| FW-06 | Forward E2E ciphertext policy | `[exists]` | Messaging IT: `TestMessagingForwardMessage_e2eToPlainDenied` / `e2eToE2EDMPreservesFlag` |

---

## 3. Войс-чат — [voice-chat.md](../features/voice-chat.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| VC-01 | 1:1 start/accept + join token | `[exists]` | Gateway: `TestComposeVoiceCall1to1_live`; Flutter: `voice_call_signaling_e2e_live_test` |
| VC-02 | Decline → `call_declined` | `[exists]` | Gateway: `TestComposeVoiceCallDecline_live`; Flutter: same file (decline test) |
| VC-03 | Bidirectional audio (media) | `[exists]` | Gateway: `TestComposeVoiceCallBidirectionalAudio_live` |
| VC-04 | Group voice start/join tokens | `[exists]` | Flutter: `group_voice_e2e_live_test` |
| VC-05 | Space voice room join/leave | `[exists]` | Flutter: `spaces_voice_e2e_live_test`; Voice IT: `voice_room_integration_test.go` |
| VC-06 | Call privacy (stranger blocked) | `[partial]` | `call_privacy_integration_test.go` + privacy live (VC covered via privacy_actions) |
| VC-07 | Commander mode / raise hand / GrantFloor | `[missing]` | roadmap П.11; unimplemented RPCs |
| VC-08 | Video layers by subscription | `[missing]` | roadmap П.12 |
| VC-09 | iOS VoIP push registration | `[exists]` | Flutter: `voip_e2e_live_test` (token register; device delivery = staging) |
| VC-10 | Recording | `[missing]` | спека «Запись» — нет live |

---

## 4. Шара экрана — [screen-share.md](../features/screen-share.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| SS-01 | Start/stop screen share on active DM call | `[exists]` | Gateway: `TestComposeVoiceScreenShare_live`; Flutter: `screen_share_e2e_live_test` |
| SS-02 | System audio / annotations | `[missing]` | — (если в спеке v1; уточнять по screen-share.md) |

---

## 5. Сторис — [stories.md](../features/stories.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| ST-01 | Create / feed / reply→DM / highlight privacy | `[exists]` | Gateway: `TestComposeStories_live`; Flutter: `stories_e2e_live_test` |
| ST-02 | Expiry → archive | `[exists]` | Gateway: `TestComposeStoriesExpiryArchive_live` |
| ST-03 | Degradation when Social down | `[exists]` | Gateway: `TestComposeStoriesWhenSocialDown_live` (liveness only — todo notes thin) |
| ST-04 | LFP story → JOIN/INVITE → MM party | `[missing]` | roadmap П.3 |
| ST-05 | Moderation hide story from feed | `[missing]` | todo/backend Story |
| ST-06 | `show_stories=Nobody` privacy floor | `[missing]` | todo/backend |
| ST-07 | Anonymous view via Premium | `[missing]` | cross-cut premium→story |

---

## 6. Auth и контакты — [auth-and-contacts.md](../features/auth-and-contacts.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| AU-01 | Register / login / refresh / JWKS lifecycle via Gateway | `[exists]` | Gateway: `TestComposeAuthLifecycle_live`; Auth: `AuthJdbcRedisIntegrationTest`, `AuthRestIntegrationTest` |
| AU-02 | Logout → JWT blacklist | `[exists]` | Flutter: `auth_logout_e2e_live_test` |
| AU-03 | Guest restrictions (DM/friends/space deny) | `[exists]` | Gateway: `TestComposeGuestRestrictions_live`; Flutter: `guest_restrictions_e2e_live_test` |
| AU-04 | Guest DM reply gated by `allow_guest_dm` | `[exists]` | Flutter: `guest_restrictions_e2e_live_test` (2nd test) |
| AU-05 | Convert guest → regular | `[exists]` | Gateway: `TestComposeConvertGuest_live`; Auth: `ConvertGuestIntegrationTest`, `GuestAccountLifecycleIntegrationTest` |
| AU-06 | Phone contacts sync / resolve hashes | `[exists]` | Gateway: `TestComposePhoneSync_live`; Auth: `ResolvePhoneHashesIntegrationTest`; Social IT: `phone_search_privacy_integration_test.go` |
| AU-07 | Guest reminder (2nd login, ≤1/day) | `[exists]` | Auth IT + Gateway: `TestComposeGuestReminder_live`; Flutter: `guest_reminder_e2e_live_test` |
| AU-08 | Guest nickname first-entry screen | `[partial]` | Flutter UI; live: `guest_onboarding_e2e_live_test` (account_type only) |
| AU-09 | 2FA enroll/verify/login gate | `[exists]` | Flutter: `trust_e2e_live_test`; Gateway: `TestComposeTrust_live` |
| AU-10 | Delete account / restore GDPR | `[partial]` | Auth: `DeleteAccountRestoreIntegrationTest` (нет compose live) |
| AU-11 | OTP rate limits | `[partial]` | Auth: `OtpRestIntegrationTest` |
| AU-12 | Active sessions list / revoke device | `[exists]` | Auth: `ActiveSessionsIntegrationTest`; Gateway: `TestComposeAuthSessions_live` |
| AU-13 | Password reset via email (Resend) | `[missing]` | todo: Resend absent |
| AU-14 | NATS `user.guest_converted` on convert | `[exists]` | Auth: `GuestConvertNatsEventIntegrationTest`; Gateway: `TestComposeConvertGuestNATS_live`; compose auth `depends_on: nats` + `AUTH_NATS_URL` |

---

## 7. Верификация — [verification.md](../features/verification.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| VR-01 | Multi-profile + verified search boost (smoke path) | `[exists]` | Flutter: `profiles_verification_e2e_live_test`; Auth: `ProfilesVerificationIntegrationTest` |
| VR-02 | Twitch/YouTube OAuth verify end-to-end | `[missing]` | roadmap П.6 |
| VR-03 | Org DNS TXT verification | `[missing]` | roadmap П.6 |
| VR-04 | Anti-spoof / badge display | `[missing]` | — |

---

## 8. Профиль пользователя — [user-profile.md](../features/user-profile.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| UP-01 | Avatar presigned upload round-trip | `[exists]` | Flutter: `avatar_e2e_live_test` |
| UP-02 | Premium animated GIF avatar | `[missing]` | todo: GIF rejected in validate |
| UP-03 | Public profile fields / bio / status | `[partial]` | User IT / client; no dedicated compose profile live |
| UP-04 | Player (MM) profile upsert/list | `[exists]` | Gateway: `TestComposeMatchmakingPlayerProfile_live`; Flutter: `player_profile_e2e_live_test` |

---

## 9. Множественные профили — [multi-profile.md](../features/multi-profile.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| MP-01 | Friend isolation across profiles | `[exists]` | Gateway: `TestComposeProfileFriendIsolation_live` |
| MP-02 | Chat isolation | `[exists]` | Gateway: `TestComposeProfileChatIsolation_live` |
| MP-03 | Create with privacy preset | `[exists]` | Gateway: `TestComposeProfileCreatePreset_live` |
| MP-04 | Voice switch with profile | `[exists]` | Gateway: `TestComposeProfileVoiceSwitch_live` |
| MP-05 | Free-tier profile limit | `[exists]` | Gateway: `TestComposeProfileFreeLimit_live` |
| MP-06 | Delete profile | `[exists]` | Gateway: `TestComposeMultiProfileDelete_live` |
| MP-07 | Downgrade (premium→free) profile picker | `[exists]` | Gateway: `TestComposeMultiProfileDowngrade_live` |
| MP-08 | Flutter multi-profile switch UX live | `[partial]` | Covered inside `profiles_verification_e2e_live_test` |

---

## 10. Друзья — [friends.md](../features/friends.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| FR-01 | Invite → accept → mutual list | `[exists]` | Gateway: `TestComposeFriendsInvitation_live`; Flutter: `friends_e2e_live_test` |
| FR-02 | Decline + block | `[exists]` | Gateway: `TestComposeFriendsDeclineBlock_live` |
| FR-03 | `allow_friend_requests` privacy denial | `[exists]` | Gateway: `TestComposeFriendsPrivacyDeny_live`; Flutter: `friends_privacy_e2e_live_test`; Social IT: `TestSendFriendInvitation_AllowFriendRequestsNobody_PermissionDenied` |
| FR-04 | Phone-book contact discovery UI path | `[partial]` | Backend phone sync exists; Flutter onboarding contacts — roadmap П.8 |
| FR-05 | Contact vs friend levels | `[partial]` | Social IT / blocks IT |

---

## 11. Presence — [presence.md](../features/presence.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| PR-01 | Presence API for peer | `[exists]` | Gateway: `TestComposePresence_live`; Flutter: `presence_e2e_live_test` |
| PR-02 | DND / invisible / custom status | `[exists]` | Gateway: `TestComposePresenceDNDInvisible_live`; Flutter: `presence_e2e_live_test` (PR-02) |
| PR-03 | In-game status privacy vs guests | `[missing]` | auth-and-contacts guest audience |

---

## 12. Приватность — [privacy.md](../features/privacy.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| PV-01 | Calls/invites/attachments blocked for strangers | `[exists]` | Gateway: `TestComposePrivacyActions_live`; Flutter: `privacy_actions_e2e_live_test` |
| PV-02 | Friend-of-friend audience | `[exists]` | Gateway: `TestComposePrivacyFoF_live`; Flutter: `privacy_fof_e2e_live_test` |
| PV-03 | DM privacy block + report + 2FA (trust bundle) | `[exists]` | Gateway: `TestComposeTrust_live`; Flutter: `trust_e2e_live_test` |
| PV-04 | Friend-request privacy (отдельный путь) | `[exists]` | same as FR-03 |
| PV-05 | User privacy service IT | `[partial]` | `user/.../privacy_integration_test.go` |

---

## 13. Спейсы — [spaces.md](../features/spaces.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| SP-01 | Create / list / get / icon | `[exists]` | Gateway: `TestComposeSpaces_live`; Flutter: `spaces_creation_e2e_live_test` |
| SP-02 | Tree: category / voice / chat / reorder | `[exists]` | Gateway: `TestComposeSpacesTree_live`; Flutter: `spaces_tree_e2e_live_test` |
| SP-03 | Invites join | `[exists]` | Gateway: `TestComposeSpacesInvites_live`; Flutter: `spaces_invites_e2e_live_test` |
| SP-04 | Channel create/list | `[exists]` | Flutter: `spaces_channel_e2e_live_test` |
| SP-05 | Channel @mention | `[exists]` | Flutter: `spaces_channel_mentions_e2e_live_test` |
| SP-06 | Shell: list + open text chat | `[exists]` | Flutter: `spaces_shell_e2e_live_test` |
| SP-07 | Space moderation kick/ban/unban | `[exists]` | Gateway: `TestComposeSpaceModeration_live`; Flutter: `spaces_moderation_e2e_live_test` |
| SP-08 | Space roles hierarchy / invite perm | `[exists]` | Gateway: `TestComposeSpaceRoles_live`; Flutter: `spaces_roles_e2e_live_test` |
| SP-09 | Space catalog discovery | `[missing]` | — |
| SP-10 | Tree update/delete/category/voice RPCs deep IT | `[partial]` | Space IT exists; todo notes thin coverage |
| SP-11 | Space Pro member-cap after billing | `[exists]` | Gateway: `TestComposeSpaceProMemberCap_live` (filler seed + webhook join) |

---

## 14. Роли — [roles.md](../features/roles.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| RL-01 | Custom role create/assign + chat override check API | `[exists]` | Flutter: `custom_roles_e2e_live_test`; Role IT: `roles_custom_integration_test.go` |
| RL-02 | `TEXT_CHAT_SEND_MESSAGES` deny enforced on SendMessage | `[exists]` | Messaging IT + Gateway: `TestComposeRolesSendDeny_live`; Flutter SendMessage 403 |
| RL-03 | Voice room `VOICE_JOIN` deny E2E | `[exists]` | Voice IT + Gateway: `TestComposeVoiceJoinDeny_live`; Flutter VOICE_JOIN deny |
| RL-04 | Verification auto-roles | `[missing]` | unimplemented |
| RL-05 | `MODERATION_MANAGE_REPORTS` integration | `[missing]` | unimplemented |

---

## 15. Матчмейкинг — [matchmaking.md](../features/matchmaking.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| MM-01 | Queue start/status/cancel | `[exists]` | Gateway: `TestComposeMatchmakingSearch_live`; Flutter: `matchmaking_queue_e2e_live_test` |
| MM-02 | Match found → accept | `[exists]` | Gateway: `TestComposeMatchmakingMatch_live`; Flutter: `matchmaking_e2e_live_test` |
| MM-03 | Search timeout 30m notify/stop | `[exists]` | Gateway: `TestComposeMatchmakingSearchTimeout_live` |
| MM-04 | History list completed squad | `[exists]` | Flutter: `matchmaking_history_e2e_live_test`; store IT history |
| MM-05 | Rate teammates (1–5) | `[exists]` | Gateway: `TestComposeMatchmakingRating_live`; Flutter: `matchmaking_rating_e2e_live_test` (rate + skip) |
| MM-06 | MM ban from matchmaking | `[exists]` | Gateway: `TestComposeMatchmakingBan_live`; store `bans_integration_test.go` |
| MM-07 | Cross-party decline semantics | `[exists]` | Matchmaking IT + Gateway: `TestComposeMatchmakingCrossPartyDecline_live` |
| MM-08 | Space-scoped matchmaking | `[missing]` | roadmap П.1 |
| MM-09 | Role-diversity 10-stack match | `[missing]` | todo: criteria broken |
| MM-10 | Device register for match_found FCM | `[exists]` | Flutter: `matchmaking_fcm_e2e_live_test` |
| MM-11 | List games catalog via Gateway | `[exists]` | Gateway: `TestComposeMatchmakingListGames_live` |

---

## 16. Каталог игр — [game-catalog.md](../features/game-catalog.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| GC-01 | Seeded Dota 2 roles/ranks | `[exists]` | Flutter: `game_catalog_e2e_live_test` |
| GC-02 | User SubmitGameRequest → moderation | `[missing]` | roadmap П.4 |
| GC-03 | Admin approve game request | `[missing]` | roadmap П.4 |

---

## 17. Репорты / модерация — [reports.md](../features/reports.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| MD-01 | Report submit 202 | `[exists]` | Gateway: `TestComposeModeration_live` / Trust; Flutter: `moderation_e2e_live_test`, `trust_e2e_live_test` |
| MD-02 | Permanent ban → login blocked | `[exists]` | Gateway: `TestComposeModeration_live` |
| MD-03 | Shadow ban | `[exists]` | Gateway: `TestComposeModerationShadowBan_live` (smoke lists it; todo claims older gap — verify content) |
| MD-04 | Appeals flow | `[partial]` | `appeals_integration_test.go` |
| MD-05 | Auto-mod threshold → shadow ban enforce | `[missing]` | todo: only logs |
| MD-06 | Sanction notifications | `[missing]` | todo |
| MD-07 | Space-local moderation (covered under SP-07) | `[exists]` | see SP-07 |

---

## 18. Шифрование — [encryption.md](../features/encryption.md) (opt-in shipped)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| ENC-01 | Enable E2E DM; search excludes body | `[exists]` | Gateway: `TestComposeE2EDM_live`; Flutter: `encryption_dm_e2e_live_test` |
| ENC-02 | Group enable rejected | `[exists]` | Gateway: `TestComposeE2E_GroupEnableRejected_live` |
| ENC-03 | Enable rejected without peer prekey | `[exists]` | Gateway: `TestComposeE2E_EnableRejectedWhenPeerMissingPreKey_live` |
| ENC-04 | Opt-out → plaintext searchable | `[exists]` | Gateway: `TestComposeE2EOptOut_live`; Flutter: `encryption_optout_e2e_live_test` |
| ENC-05 | Key backup PUT/GET | `[exists]` | Gateway: `TestComposeE2EKeyBackup_live`; Flutter: `encryption_key_backup_e2e_live_test`; Auth: `E2EKeyBackup*IntegrationTest` |
| ENC-06 | Oversized backup rejected | `[exists]` | Gateway: `TestComposeE2EKeyBackup_OversizedRejected_live` |
| ENC-07 | E2E edit | `[exists]` | Gateway: `TestComposeE2EEdit_live`; Flutter: `encryption_edit_e2e_live_test` |
| ENC-08 | E2E file no server thumbnail | `[exists]` | Flutter: `encryption_file_e2e_live_test` |
| ENC-09 | Shared media e2e_key_wire | `[exists]` | Flutter: `encryption_shared_media_e2e_live_test` |
| ENC-10 | Verification code / key-change banner multi-device | `[partial]` | Unit: `e2e_verification_code_test`, `e2e_identity_*`; roadmap П.13 for full UX |
| ENC-11 | Offline SQLCipher cache | `[partial]` | Flutter: `offline_cache_e2e_live_test` (REST page cache; not full SQLCipher E2E path) |
| ENC-12 | Search reindex must skip E2E bodies | `[exists]` | Search: `TestReindexChat_SkipsE2EBodies_postgres` (+ unit `TestReindexChat_SkipsE2EMessages`) |

---

## 19. Онбординг — [onboarding.md](../features/onboarding.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| ON-01 | Onboarding steps persist on server | `[exists]` | Flutter: `onboarding_e2e_live_test` |
| ON-02 | Guest onboarding incomplete flags | `[exists]` | Flutter: `guest_onboarding_e2e_live_test` |
| ON-03 | Coach-marks MM/space/invite deep link | `[missing]` | roadmap П.20 |

---

## 20. Accessibility — [accessibility.md](../features/accessibility.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| A11Y-01 | Keyboard shortcuts widget tests | `[partial]` | `voice_shortcuts_keyboard_test.dart` (не live IT) |
| A11Y-02 | Chrome deeplink integration_test | `[exists]` | `integration_test/deeplink_web_test.dart` (CI flutter-web-integration) |
| A11Y-03 | Semantics / high contrast / reduced motion | `[partial]` | `three_column_shell_semantics_test.dart`; roadmap П.19 |
| A11Y-04 | TalkBack/VoiceOver checklist | `[n/a]` | ручной pre-release |

---

## 21. i18n — [i18n.md](../features/i18n.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| I18N-01 | EN+RU baseline ARB | `[partial]` | `i18n_baseline_test.dart`, `i18n_extended_test.dart` (unit, не compose) |
| I18N-02 | pg_trgm search RU/EN | `[partial]` | Search IT message/profile; нет dedicated i18n live |

---

## 22. Боты — [bots.md](../features/bots.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| BT-01 | Install polling bot + `/ping` | `[exists]` | Gateway: `TestComposeBotsSlash_live`; Flutter: `bots_slash_e2e_live_test` |
| BT-02 | Ephemeral response | `[exists]` | Gateway: `TestComposeBotsEphemeral_live`; Flutter: `bots_ephemeral_live_test` |
| BT-03 | BOT-C routes / presence / create chat | `[exists]` | Gateway: `TestComposeBotsBotCRoutes_live`; Flutter: `bots_botc_live_test` |
| BT-04 | Webhook delivery | `[exists]` | Gateway: `TestComposeBotsWebhook_live` |
| BT-05 | Timeout / deferred / offline greyout / per-chat / privileged / daily limit / uninstall | `[exists]` | Gateway suite in `compose_bots_slash_live_test.go`, `compose_bots_uninstall_live_test.go` |
| BT-06 | Degradation Search/Bot down | `[exists]` | `compose_bots_degradation_live_test.go` |
| BT-07 | Autocomplete / subcommands / portal catalog UX | `[missing]` | roadmap П.15 |
| BT-08 | Slash → in-app notification cross-cut | `[missing]` | todo cross-cutting |

---

## 23. Навигация — [navigation.md](../features/navigation.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| NV-01 | Mobile layout breakpoint contract | `[exists]` | Flutter: `mobile_layout_e2e_live_test` |
| NV-02 | Desktop three-column shell | `[partial]` | widget/semantics tests |
| NV-03 | Default folders / space entry UX | `[missing]` | — |

---

## 24. Поиск — [search.md](../features/search.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| SR-01 | Global search endpoint | `[exists]` | Flutter: `search_e2e_live_test`; Gateway: `TestComposeSearchNamespace_live`, `TestComposeUsersSearch_live` |
| SR-02 | In-chat search | `[exists]` | Gateway: `TestComposeSearchInChat_live` |
| SR-03 | Search unavailable → 503; messaging still works | `[exists]` | `compose_search_degradation_live_test.go` |
| SR-04 | Shared media filters in search UI | `[missing]` | — |
| SR-05 | Profile discovery privacy audience | `[missing]` | todo Search ignores viewer privacy |
| SR-06 | Timeout placeholder | `[partial]` | `search_timeout_e2e_live_test` («placeholder») |

---

## 25. Deep links — [deep-links.md](../features/deep-links.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| DL-01 | Invite HTML redirect + resolve | `[exists]` | Gateway: `TestComposeDeepLinks_live`; Flutter: `deeplink_invite_e2e_live_test` |
| DL-02 | Resolve when Search down | `[exists]` | `TestComposeDeepLinksResolveWhenSearchDown_live` |
| DL-03 | Message/profile/channel deep link kinds | `[partial]` | invite covered; full matrix unclear |
| DL-04 | Mobile universal links / app links | `[missing]` | device `integration_test` aspirational (todo) |

---

## 26. Платформы — [platforms.md](../features/platforms.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| PL-01 | Mobile narrow layout | `[exists]` | `mobile_layout_e2e_live_test` |
| PL-02 | Windows version policy 426 | `[exists]` | Gateway: `TestComposeWindowsVersionPolicy_live`; Flutter: `windows_version_e2e_live_test` |
| PL-03 | Windows tray / PTT / overlay | `[missing]` | roadmap П.17–18 |
| PL-04 | Device driver integration_test suite | `[missing]` | todo; `integration_test/` mostly empty |

---

## 27. Обновления клиентов — [updates.md](../features/updates.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| UPD-01 | `/api/v1/version` Windows policy | `[exists]` | see PL-02 |
| UPD-02 | Force-update / Shorebird OTA | `[missing]` | — |
| UPD-03 | Desktop auto-update | `[missing]` | roadmap П.17 |

---

## 28. Уведомления — [notifications.md](../features/notifications.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| NT-01 | In-app unread + WS notification | `[exists]` | Gateway: `TestComposeInAppNotifications_live`; Flutter: `in_app_notifications_e2e_live_test` |
| NT-02 | FCM register + offline DM push payload | `[exists]` | Flutter: `fcm_e2e_live_test`, `fcm_delivery_e2e_live_test`, `fcm_android_e2e_live_test`; Gateway: `TestComposeNotificationRegisterDevice_live` |
| NT-03 | APNs register | `[exists]` | Flutter: `apns_e2e_live_test` |
| NT-04 | Quiet hours / per-chat granularity / voice join push | `[missing]` | roadmap П.10 |
| NT-05 | Real device alert delivery | `[n/a]` | staging secrets (DEPLOYMENT.md) |

---

## 29. Подписка — [subscription.md](../features/subscription.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| SUB-01 | Personal premium webhook + upload boundaries | `[exists]` | Gateway: `TestComposeBilling_live`; Flutter: `billing_e2e_live_test` |
| SUB-02 | Downgrade file limits | `[exists]` | Gateway: `TestComposeSubscriptionDowngradeFileLimits_live` |
| SUB-03 | Space Pro billing compose | `[exists]` | `TestComposeSpaceProBilling_live` + `TestComposeSpaceProMemberCap_live` |
| SUB-04 | Grace-period notifications D1/D3/D7 | `[exists]` | Gateway: `TestComposeSubscriptionGraceReminder_live` (D1 sweeper) |
| SUB-05 | Real Paddle checkout | `[missing]` | stub URLs (todo Critical) |
| SUB-06 | Premium cosmetics (banner/GIF/3rd profile) cross-smoke | `[missing]` | todo cross-cutting |

---

## 30. Файлы — [file-storage.md](../features/file-storage.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| FL-01 | Upload + attachment message | `[exists]` | Gateway: `TestComposeFileAttachment_live`; Flutter: `file_attachment_e2e_live_test` |
| FL-02 | Image thumb metadata | `[exists]` | Flutter: `file_image_thumb_e2e_live_test` asserts `thumbnailR2Key`; mapper keeps `previewUrl` for HTTP URLs only |
| FL-03 | ClamAV infected reject | `[exists]` | Flutter: `file_clamav_infected_e2e_live_test` uses scannable `.exe` / `application/x-msdownload` |
| FL-04 | Retention cron free 90d | `[missing]` | retention not enforced (was open; check backend TODO status) |
| FL-05 | Attachment privacy IT | `[partial]` | `attachment_privacy_integration_test.go` |

---

## 31. Наблюдаемость — [observability.md](../features/observability.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| OBS-01 | Compose logs collect local | `[partial]` | tooling `make compose-logs-collect`; не автотест |
| OBS-02 | Staging request_id → ws_fanout E2E | `[n/a]` | ручной runbook TESTING.md §Debug by request_id |
| OBS-03 | Alertmanager P1 firing | `[n/a]` | staging secrets |

---

## 32. Аналитика — [analytics.md](../features/analytics.md) (PLAN: partial)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| AN-01 | Staff dashboard RBAC 200/403 | `[exists]` | Gateway: `TestComposeAnalytics_live` |
| AN-02 | Export HTTP 200 | `[exists]` | Gateway: `TestComposeAnalyticsExport_live` |
| AN-03 | Ingest `message.sent` → ClickHouse <60s | `[exists]` | `TestComposeAnalyticsIngest_live` |
| AN-04 | Export writes audit log | `[exists]` | `TestComposeAnalyticsExport_live` + Gateway Redis/memory audit store |
| AN-05 | ClickHouse InsertBatch IT | `[partial]` | `clickhouse_integration_test.go` |

---

## 33. Федерация — [federation.md](../features/federation.md)

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| FED-01 | Any product federation path | `[deferred]` | PLAN deferred; scaffold only |

---

## 34. Платформенные пути вне каталога фич (TESTING / PLAN)

Нужны для soft-launch Tier-0, фигурируют в smoke/manifest.

| ID | Кейс | Статус | Где |
|----|------|--------|-----|
| PLAT-01 | Realtime Gateway WS upgrade | `[exists]` | `TestComposeRealtimeGateway_live` |
| PLAT-02 | Admin API staff | `[exists]` | `TestComposeAdminAPI_live` |
| PLAT-03 | Developer Portal | `[exists]` | `TestComposeDeveloperPortal_live` |
| PLAT-04 | Gateway contract (JWT/JWKS/rate limit/CORS) | `[partial]` | Gateway unit/httptest + `rest_transcoding_integration_test.go`; не полный compose matrix из TESTING §API Gateway |

---

## Краткая матрица smoke (tier 2)

Сверка с `e2e-features.yml` smoke_*: большинство smoke-кейсов `[exists]` в коде, кроме **битых имён** Friends/Search (см. выше). Flutter smoke не включает full-матрицу encryption_dm / profiles_verification / ws_resume (они в full) — это **gap приоритета CI**, не отсутствия файлов.

---

## Амбигуитеты / пробелы в доках

1. Нет единого «testing plan» файла с нумерованными user journeys — план собран из FEATURES + TESTING §E2E + PLAN + e2e-features.yml + roadmap TODO.
2. `TestComposeFriends_live` / `TestComposeSearch_live` в манифесте ≠ имена в Go.
3. Часть спек (стикеры, запись войса, view counters) без явных acceptance tests в CI docs.
4. Federation deferred — исключена из обязательств.
5. Observability/analytics DoD частично staging-only (`[n/a]`).
6. «Integration» на Flutter = live host tests; `integration_test/` device suite не реализована (зафиксировано в todo и README).
