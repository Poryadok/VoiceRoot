# Voice — статус реализации

> Каталог продуктовых фич — [FEATURES.md](FEATURES.md) и `docs/features/`. Здесь — **что уже есть в коде**, локальный стенд и связь с E2E-тестами. Пробелы вне фич — [TODO.md](TODO.md).

---

## Продуктовый scope

**В scope v1:** все фичи из [FEATURES.md](FEATURES.md), **кроме** [federation.md](features/federation.md).

**Федерация** — отложена: реализация только при явном запросе рынка. Спека в `federation.md` сохранена как целевая архитектура; сервис `federation` в репозитории — scaffold.

---

## Локальный стенд

| Команда | Назначение |
|---------|------------|
| `make compose-up` | Infra: Postgres, Redis, NATS JetStream |
| `make compose-app-up` | Полный app stack: Auth, Gateway, User, Social, Chat, Messaging, Realtime, Space, Role, Voice, File, Search, Matchmaking, Notification, Bot, Story, Subscription, Moderation + Flutter web ([README.md](../README.md)) |
| `make compose-migrate-all` | golang-migrate для Go-owned БД |
| `make compose-migrate-e2e` | DDL для E2E encryption (messaging + chat) |
| `make compose-e2e-smoke` | Smoke E2E по фичам ([e2e-features.yml](../.github/ci/e2e-features.yml)) |

**Staging:** k8s-стек — [`deploy/staging/`](../deploy/staging/), `scripts/staging/render-and-apply.sh` ([DEPLOYMENT.md](DEPLOYMENT.md)).

---

## Статус по фичам

| Фича | Статус | Сервисы | Gateway / Flutter live-тесты (примеры) |
|------|--------|---------|------------------------------------------|
| [auth-and-contacts](features/auth-and-contacts.md) | shipped | Auth, Gateway | `TestComposeAuthLifecycle_live`, `auth_logout_e2e_live_test` |
| [friends](features/friends.md) | shipped | Social, User | `TestComposeFriends_live`, `friends_e2e_live_test` |
| [text-chat](features/text-chat.md) (DM) | shipped | Chat, Messaging, Realtime | `TestComposeDMRealtime_live`, `dm_two_users_e2e_live_test` |
| [presence](features/presence.md) | shipped | User, Realtime | `presence_e2e_live_test`, `ws_resume_e2e_live_test` |
| [user-profile](features/user-profile.md) | shipped | User, File (R2) | `avatar_e2e_live_test` |
| [voice-chat](features/voice-chat.md) (1:1, group, space) | shipped | Voice | `TestComposeVoiceCall_live`, `voice_call_signaling_e2e_live_test` |
| [file-storage](features/file-storage.md) | shipped | File | `file_attachment_e2e_live_test`, `TestComposeFileAttachment_live` |
| [forward-messages](features/forward-messages.md) | shipped | Messaging | `forward_messages_e2e_live_test` |
| Групповые чаты | shipped | Chat, Role | `TestComposeGroups_live`, `groups_e2e_live_test` |
| [spaces](features/spaces.md) | shipped | Space, Role | `TestComposeSpaces_live`, `spaces_creation_e2e_live_test` |
| [roles](features/roles.md) | shipped | Role | `custom_roles_e2e_live_test`, `TestComposeSpaceRoles_live` |
| Markdown, @mentions, пины | shipped | Messaging, Chat | `markdown_e2e_live_test`, `pins_e2e_live_test` |
| [notifications](features/notifications.md) | partial | Notification | `fcm_delivery_e2e_live_test`, `apns_e2e_live_test` (device creds — staging) |
| [matchmaking](features/matchmaking.md) | shipped | Matchmaking | `TestComposeMatchmakingSearch_live`, `matchmaking_e2e_live_test` |
| [game-catalog](features/game-catalog.md) | shipped | Matchmaking | `game_catalog_e2e_live_test` |
| [search](features/search.md) | shipped | Search | `TestComposeSearch_live`, `search_e2e_live_test` |
| [screen-share](features/screen-share.md) | shipped | Voice | `screen_share_e2e_live_test` |
| Треды, shared media | shipped | Chat, Messaging | `threads_e2e_live_test`, `shared_media_e2e_live_test` |
| [reports](features/reports.md), trust, 2FA | shipped | Moderation, Auth | `TestComposeTrust_live`, `trust_e2e_live_test` |
| [privacy](features/privacy.md) | shipped | User, Social | `privacy_actions_e2e_live_test` |
| [subscription](features/subscription.md) | partial | Subscription | `TestComposeBilling_live`, `billing_e2e_live_test` |
| [multi-profile](features/multi-profile.md), [verification](features/verification.md) | partial | User, Auth | `profiles_verification_e2e_live_test`, `TestComposeProfileFriendIsolation_live` |
| [encryption](features/encryption.md) (E2E DM) | shipped (opt-in) | Messaging, Chat, Auth | `TestComposeE2EDM_live`, `e2e_dm_live_test` |
| [bots](features/bots.md) | partial | Bot, Gateway | `TestComposeBotsSlash_live`, `bots_slash_e2e_live_test` |
| [stories](features/stories.md) | partial | Story | `TestComposeStories_live`, `stories_e2e_live_test` |
| [deep-links](features/deep-links.md) | shipped | Gateway | `TestComposeDeepLinks_live`, `deeplink_invite_e2e_live_test` |
| [onboarding](features/onboarding.md) | shipped | User (client) | `onboarding_e2e_live_test` |
| [accessibility](features/accessibility.md) | baseline | Flutter | semantics tests, `deeplink_web_chrome_test` |
| [platforms](features/platforms.md) | partial | Flutter | `mobile_layout_e2e_live_test`, `windows_version_e2e_live_test` |
| [i18n](features/i18n.md) | shipped | Flutter | `i18n_baseline_test` |
| [federation](features/federation.md) | **deferred** | Federation (scaffold) | — |

**Scaffold** (health + CI matrix, без продуктовой логики): Analytics, Federation (до запроса рынка).

Детальные критерии приёмки — в `docs/features/*.md`. Post-MVP gaps (Stories AR, prod universal links, observability staging) — [TODO.md](TODO.md).

---

## Размещение кода

| Сервис | Каталог | БД |
|--------|---------|-----|
| Auth | `src/backend/auth/` | `auth_db` (Flyway) |
| API Gateway | `src/backend/gateway/` | — |
| Realtime | `src/backend/realtime/` | Redis |
| Messaging | `src/backend/messaging/` | `messaging_db` |
| Chat | `src/backend/chat/` | `chat_db` |
| User | `src/backend/user/` | `user_db` |
| Social | `src/backend/social/` | `social_db` |
| Space, Role, Voice, File, Search, Matchmaking, Notification, Bot, Story, Subscription, Moderation | `src/backend/<service>/` | см. [DATA_STORES.md](DATA_STORES.md) |
| Flutter client | `src/frontend/` | — |
| Admin | `src/admin/` | зарезервировано |

Целевая карта — [MICROSERVICES.md](MICROSERVICES.md).

---

## Верификация

| Проверка | Команда |
|----------|---------|
| Backend | `make build-all` |
| Flutter | `make flutter-ci` |
| Compose smoke E2E | `make compose-e2e-smoke` (Docker + `VOICE_RUN_LIVE_COMPOSE=true`) |
| Полный compose E2E | `make compose-e2e-live` |
| Контракты | `buf lint`, `buf format -d --exit-code` |

Каталог E2E по фичам — [TESTING.md](TESTING.md#e2e-по-фичам) и [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml).

---

## Исторические миграции

Имена файлов в `src/backend/migrations/` с суффиксами вроде `phase13` — **не переименовываются** (уже применены на существующих БД). Новые миграции именуются по домену фичи, без номеров «фаз».
