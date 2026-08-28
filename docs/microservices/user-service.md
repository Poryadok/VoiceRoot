# User Service

## Обзор

Управление профилями пользователей, настройками, приватностью и онлайн-статусом.

**Язык**: Go
**БД**: PostgreSQL `user_db`, Redis (presence cache)

## Ответственность

- Профили пользователей (аватар, имя, био, баннер, статус)
- Мульти-профили (2 бесплатно / 5 Premium, независимые контакты и настройки)
- Переключение активного профиля
- Username система (`@username#1234`, Premium `@username`, верифицированные ✅, компании ®)
- Настройки приватности (3 пресета + гранулярный контроль по полям)
- Presence (online / idle / DND / invisible)
- Кастомный статус (Premium)
- Game detection статус (из десктоп-клиента)
- Last seen timestamp
- Onboarding state (шаги туториала)
- Управление настройками (язык, тема, уведомления)
- **Поиск профилей для добавления в друзья / открытия DM** ([friends.md](../features/friends.md)) — запрос к `user_db` (не Search Service и не `search_db`; см. [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md), таблица фич)

## Поиск профилей vs Search Service

| Канал | RPC | Когда |
|-------|-----|--------|
| **User Service** | `SearchProfiles` | [friends.md](../features/friends.md): подбор по нику/отображаемому имени из канонических данных профиля; учёт приватности и блокировок (совместно с Social при необходимости). |
| **Search Service** | `SearchUsers` / `SearchGlobal` | [search.md](../features/search.md): полнотекст и глобальный поиск по проекциям в `search_db` / внешнем движке — [search-service.md](search-service.md). |

Клиент для чеклиста «Поиск пользователей» в [PLAN.md](../PLAN.md) опирается на **`UserService.SearchProfiles`** (HTTP-префикс того же сервиса: `/api/v1/users/**` — [api-gateway.md](api-gateway.md), в т.ч. **presigned аватар:** `POST /api/v1/users/me/avatar/presigned-upload` → `CreateAvatarPresignedUpload`).

## API (gRPC)

```protobuf
service UserService {
  // Профили
  rpc EnsurePrimaryProfile(EnsurePrimaryProfileRequest) returns (EnsurePrimaryProfileResponse); // S2S Auth bootstrap; см. primary-profile-bootstrap.md
  rpc GetProfile(GetProfileRequest) returns (Profile);
  rpc GetProfiles(GetProfilesRequest) returns (ProfileList); // batch
  rpc UpdateProfile(UpdateProfileRequest) returns (Profile);
  rpc CreateProfile(CreateProfileRequest) returns (Profile); // мульти-профиль
  rpc DeleteProfile(DeleteProfileRequest) returns (Empty);
  rpc SwitchProfile(SwitchProfileRequest) returns (Profile);
  rpc ListMyProfiles(Empty) returns (ProfileList);
  rpc SearchProfiles(SearchProfilesRequest) returns (SearchProfilesResponse);

  // Приватность
  rpc GetPrivacySettings(GetPrivacyRequest) returns (PrivacySettings);
  rpc UpdatePrivacySettings(UpdatePrivacyRequest) returns (PrivacySettings);

  // Presence
  rpc UpdatePresence(UpdatePresenceRequest) returns (Empty);
  rpc GetPresence(GetPresenceRequest) returns (PresenceStatus);
  rpc GetBulkPresence(GetBulkPresenceRequest) returns (GetBulkPresenceResponse); // map profile_id -> PresenceStatus

  // Настройки
  rpc GetSettings(GetSettingsRequest) returns (UserSettings);
  rpc UpdateSettings(UpdateSettingsRequest) returns (UserSettings);

  // Onboarding
  rpc GetOnboardingState(Empty) returns (OnboardingState);
  rpc CompleteOnboardingStep(CompleteStepRequest) returns (OnboardingState);

  // Verification
  rpc GetVerificationStatus(GetVerificationRequest) returns (VerificationStatus);
}
```

Канон RPC и сообщений — репозиторий `protos/voice/user/v1/user.proto`.

## Модель данных

```
profiles
├── id (UUID)
├── account_id (UUID, logical ref → auth_db.accounts.id; без меж-БД REFERENCES)
├── username (string)
├── discriminator (string, 4 digits)
├── display_name
├── avatar_url
├── banner_url
├── bio (text, 500 chars)
├── custom_status (text, nullable — Premium)
├── locale (en | ru)
├── theme (light | dark | high_contrast)
├── is_primary (bool)
├── verification_type (none | personal | organization)
├── verification_badge (nullable)
├── created_at
└── updated_at

privacy_settings
├── profile_id (UUID, logical ref → profiles.id)
├── preset (personal | gaming | work)
├── show_online (everyone | friends | nobody)
├── show_last_seen (everyone | friends | nobody) — «был(а) N назад»; independent from live online status
├── show_game_status (everyone | friends | nobody)
├── show_mm_rating (everyone | friends | nobody)
├── show_phone (friends | nobody)
├── show_stories (everyone | friends | nobody)
├── allow_dm (everyone | friends | friends_of_friends | nobody)
├── allow_friend_requests (everyone | friends_of_friends | nobody)
├── allow_guest_dm (bool)
└── updated_at

presence (Redis Hash — ephemeral)
├── profile_id → { status, game, custom_status, call_info }
└── TTL ~5 min; refreshed on heartbeat / WS activity

last_seen_at (PostgreSQL)
├── profile_id (UUID PK, logical ref → profiles.id)
├── last_seen_at (TIMESTAMPTZ, UTC)
└── updated on session end, heartbeat boundary, explicit offline transition
```

**`last_seen` storage:**

| Store | Назначение | Статус |
|-------|------------|--------|
| **Redis presence** | Live `status`, game, custom status, call | **Implemented** — TTL ~5 min |
| **PostgreSQL `last_seen_at`** | Durable «был(а) N назад» для header / profile | **Not yet in proto/code** (no PG column) |

Redis-only `last_seen` в hash **недостаточен**: после TTL 5 min фактическое время последней активности теряется — header DM не может показать «был 2 часа назад». Эфемерный cache ≠ durable last seen ([presence.md](../features/presence.md)).

При реализации: запись `last_seen_at` при transition to offline / idle boundary / graceful disconnect; `GetBulkPresence` / `GetPresence` **merge** Redis live status + PG `last_seen_at` when offline.

**`show_last_seen` enforcement:** when `show_last_seen = nobody` (or viewer not in allowed audience for `friends`), `GetPresence` / `GetBulkPresence` **omit** `last_seen_at` timestamp (live online may still respect `show_online`). Header «был(а)…» in DM — [presence.md](../features/presence.md). **Code gap:** field not in proto/DDL; no read-time filter — [todo/backend.md](../todo/backend.md).

### Current code vs full spec

**Deployed migrations** используют только `profiles` и `onboarding_state`.
`privacy_settings` и расширенные Premium-поля — **not yet in proto/code**.

```
profiles
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → auth_db.accounts.id
├── username VARCHAR(32) NOT NULL
├── discriminator CHAR(4) NOT NULL CHECK (discriminator ~ '^[0-9]{4}$')
├── display_name VARCHAR(64) NOT NULL
├── avatar_url TEXT NULL
├── bio TEXT NULL CHECK (char_length(bio) <= 500)
├── locale VARCHAR(8) NOT NULL DEFAULT 'ru' CHECK (locale IN ('ru','en'))
├── theme VARCHAR(32) NOT NULL DEFAULT 'dark' CHECK (theme IN ('light','dark','high_contrast'))
├── is_primary BOOLEAN NOT NULL DEFAULT true
├── verification_type VARCHAR(32) NOT NULL DEFAULT 'none' CHECK (verification_type IN ('none','personal','organization'))
├── verification_badge VARCHAR(32) NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

onboarding_state
├── profile_id UUID PRIMARY KEY -- logical ref → profiles.id
├── completed_steps JSONB NOT NULL DEFAULT '[]'::jsonb
├── completed BOOLEAN NOT NULL DEFAULT false
├── completed_at TIMESTAMPTZ NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Индексы:
- `UNIQUE (username, discriminator)`
- `UNIQUE (account_id) WHERE is_primary = true` (один primary-профиль на account)
- `INDEX profiles_account_id_idx (account_id)`
- `INDEX profiles_created_at_idx (created_at DESC)`

**`last_seen_at`:** spec — таблица в PostgreSQL (см. схема выше). **Code gap:** только Redis hash, без durable persistence — [todo/backend.md](../todo/backend.md).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`user.events`** (совместно с Auth для событий учётной записи; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                                     |
|-------------------------|--------------------------------------------|
| `user.profile_created`  | profile_id, account_id                     |
| `user.profile_updated`  | profile_id, changed_fields                 |
| `user.profile_switched` | account_id, old_profile_id, new_profile_id |
| `user.presence_changed` | profile_id, **old_status**, **new_status** |
| `user.game_detected`    | profile_id, game_name                      |
| `user.settings_changed` | profile_id, changed_keys                   |
| `user.verified`         | profile_id, verification_type              |

**`user.presence_changed`:** событие **публикуется** при `UpdatePresence` (`user/internal/userevents/jetstream.go`). **Proto gap:** JetStream payload сегодня без `old_status` / `new_status` — **not yet in proto** (`jetstream_events.proto`; docs ранее ошибочно помечали как «not published»).

## Зависимости

- **Auth Service** — account_id валидация
- **Subscription Service** — проверка лимитов (мульти-профили, кастомный статус)
- **Redis** — presence кэш (TTL 5 мин, heartbeat)
- **File Service** — загрузка аватара/баннера; **not yet deployed** as standalone service — минимальный R2/presigned для статичного аватара может жить в User ([user-profile.md](../features/user-profile.md), [PLAN.md](../PLAN.md))


