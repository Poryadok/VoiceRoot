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

## API (gRPC)

```protobuf
service UserService {
  // Профили
  rpc GetProfile(GetProfileRequest) returns (Profile);
  rpc GetProfiles(GetProfilesRequest) returns (ProfileList); // batch
  rpc UpdateProfile(UpdateProfileRequest) returns (Profile);
  rpc CreateProfile(CreateProfileRequest) returns (Profile); // мульти-профиль
  rpc DeleteProfile(DeleteProfileRequest) returns (Empty);
  rpc SwitchProfile(SwitchProfileRequest) returns (Profile);
  rpc ListMyProfiles(Empty) returns (ProfileList);

  // Приватность
  rpc GetPrivacySettings(GetPrivacyRequest) returns (PrivacySettings);
  rpc UpdatePrivacySettings(UpdatePrivacyRequest) returns (PrivacySettings);

  // Presence
  rpc UpdatePresence(UpdatePresenceRequest) returns (Empty);
  rpc GetPresence(GetPresenceRequest) returns (PresenceStatus);
  rpc GetBulkPresence(GetBulkPresenceRequest) returns (BulkPresenceResponse);

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

## Модель данных

```
profiles
├── id (UUID)
├── account_id (FK → auth_db.accounts)
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
├── profile_id (FK)
├── preset (personal | gaming | work)
├── show_online (everyone | friends | nobody)
├── show_game_status (everyone | friends | nobody)
├── show_mm_rating (everyone | friends | nobody)
├── show_phone (friends | nobody)
├── show_stories (everyone | friends | nobody)
├── allow_dm (everyone | friends | friends_of_friends | nobody)
├── allow_friend_requests (everyone | friends_of_friends | nobody)
├── allow_guest_dm (bool)
└── updated_at

presence (Redis Hash)
├── profile_id → { status, game, custom_status, last_seen, call_info }
```

## Публикуемые события (→ NATS)

| Событие                 | Данные                                     |
|-------------------------|--------------------------------------------|
| `user.profile_created`  | profile_id, account_id                     |
| `user.profile_updated`  | profile_id, changed_fields                 |
| `user.profile_switched` | account_id, old_profile_id, new_profile_id |
| `user.presence_changed` | profile_id, old_status, new_status         |
| `user.game_detected`    | profile_id, game_name                      |
| `user.settings_changed` | profile_id, changed_keys                   |
| `user.verified`         | profile_id, verification_type              |

## Зависимости

- **Auth Service** — account_id валидация
- **Subscription Service** — проверка лимитов (мульти-профили, кастомный статус)
- **Redis** — presence кэш (TTL 5 мин, heartbeat)
- **File Service** — загрузка аватара/баннера


