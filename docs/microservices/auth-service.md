# Auth Service

## Обзор

Сервис аутентификации и управления сессиями. Единственный сервис на Java — обусловлено зрелостью Spring Security для сложных auth-сценариев.

**Язык**: Java 25 LTS
**Фреймворк**: Spring Boot 3.5, Spring Security 6
**БД**: PostgreSQL `auth_db`

## Ответственность

- Регистрация (email, телефон, гостевой аккаунт)
- Логин / логаут
- JWT access token (15 мин) + opaque refresh token (30 дней)
- Refresh token rotation (одноразовые)
- 2FA (TOTP — Google Authenticator и аналоги)
- JWT blacklist (Redis, для логаута и ротации)
- Гостевые аккаунты (30-дневный TTL, ограниченные права)
- Конвертация гостевого аккаунта в полноценный
- Soft delete аккаунта (30-дневный grace period)
- JWKS endpoint для публичных ключей (используется Gateway и другими сервисами)
- **Фаза 1:** перед выдачей access JWT обеспечивается первичный профиль в `user_db` (claim `profile_id` = `profiles.id`); см. [primary-profile-bootstrap.md](primary-profile-bootstrap.md), [EXEC_PLAN.md](../EXEC_PLAN.md).
- OTP генерация и валидация (email)

### PR и ревью (bootstrap JWT ↔ User)

- Перед merge — зелёный job **`backend-auth`** в CI (`mvn -B test`). Интеграция JDBC + Redis + совпадение `profile_id` с primary-строкой в `user_db.profiles` покрыта **`AuthJdbcRedisIntegrationTest`** (регистрация / login / refresh / validate).
- Maven внутри контейнера **без** доступа к Docker socket хоста может **пропускать** этот класс Testcontainers; ориентир — CI или хостовый `mvn test` с Docker ([TESTING.md](../TESTING.md), job Auth в [.github/workflows/ci.yml](../../.github/workflows/ci.yml)).
- Меняете claims JWT или схему `profiles` — синхронизируйте потребителей (Gateway, Go) с [`DATA_MODEL.md`](../DATA_MODEL.md) и при необходимости прогоните buf / контрактные проверки.

## API (gRPC)

Канон: [`protos/voice/auth/v1/auth.proto`](../../protos/voice/auth/v1/auth.proto). Кратко:

```protobuf
service AuthService {
  rpc Register(RegisterRequest) returns (RegisterResponse);   // session: AuthSession
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
  rpc Enable2FA(Enable2FARequest) returns (Enable2FAResponse);
  rpc Verify2FA(Verify2FARequest) returns (Verify2FAResponse);
  rpc VerifyOTP(VerifyOTPRequest) returns (VerifyOTPResponse);
  rpc ConvertGuest(ConvertGuestRequest) returns (ConvertGuestResponse);
  rpc DeleteAccount(DeleteAccountRequest) returns (DeleteAccountResponse);
  rpc RestoreAccount(RestoreAccountRequest) returns (RestoreAccountResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse); // internal
  rpc GetJWKS(GetJWKSRequest) returns (GetJWKSResponse); // public
  rpc PutE2EKeyBackup(PutE2EKeyBackupRequest) returns (PutE2EKeyBackupResponse); // Phase 15
  rpc GetE2EKeyBackup(GetE2EKeyBackupRequest) returns (GetE2EKeyBackupResponse);
}
```

### Phase 15 — E2E key backup (REST via Gateway)

Клиент хранит парольно-зашифрованный бэкап ключей Signal на сервере; сервер видит только opaque blob ([encryption.md](../features/encryption.md)).

| gRPC | REST (Gateway transcoding) | Назначение |
|------|----------------------------|------------|
| `PutE2EKeyBackup` | `PUT /api/v1/auth/e2e-key-backup` | Сохранить/обновить blob (`encrypted_blob`, опционально `password_hint`); `204 No Content` |
| `GetE2EKeyBackup` | `GET /api/v1/auth/e2e-key-backup` | Получить blob для восстановления на новом устройстве; `404` до первого PUT |

- **Владение данными:** пароль и ключ расшифровки — только на клиенте; Auth хранит `encrypted_blob` как есть.
- **Лимиты Gateway:** `E2EKeyBackupPut` 5/min, `E2EKeyBackupGet` 30/min (`ratelimit.go`).
- **Клиент:** `VoiceE2eClient` + UI в `e2e_chat_settings.dart`; см. также [messaging-service.md](messaging-service.md) (key backup не в Messaging).

## Модель данных

```
accounts
├── id (UUID)
├── email (nullable, unique)
├── phone (nullable, unique)
├── password_hash (bcrypt)
├── type (regular | guest)
├── status (active | suspended | deleted)
├── totp_secret (encrypted, nullable)
├── totp_enabled (bool)
├── deleted_at (nullable, soft delete)
├── created_at
└── updated_at

refresh_tokens
├── id (UUID)
├── account_id (UUID, logical ref → accounts.id)
├── token_hash (SHA-256)
├── device_info (jsonb)
├── expires_at
├── created_at
└── revoked_at (nullable)

otp_codes
├── id (UUID)
├── account_id (UUID, logical ref → accounts.id)
├── code (encrypted)
├── type (email_verify | password_reset)
├── expires_at
├── used_at (nullable)
└── created_at

e2e_key_backups (Phase 15)
├── account_id (UUID, PK, logical ref → accounts.id)
├── encrypted_blob (TEXT, client-encrypted opaque payload)
├── password_hint (nullable)
└── updated_at
```

### V1 (Фаза 0-1) — детальный профиль для DDL

```
accounts
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── email VARCHAR(320) NULL
├── phone VARCHAR(32) NULL
├── password_hash TEXT NOT NULL
├── type VARCHAR(16) NOT NULL CHECK (type IN ('regular','guest'))
├── status VARCHAR(16) NOT NULL CHECK (status IN ('active','suspended','deleted'))
├── totp_secret BYTEA NULL
├── totp_enabled BOOLEAN NOT NULL DEFAULT false
├── deleted_at TIMESTAMPTZ NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

refresh_tokens
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → accounts.id
├── token_hash CHAR(64) NOT NULL
├── device_info JSONB NOT NULL DEFAULT '{}'::jsonb
├── expires_at TIMESTAMPTZ NOT NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── revoked_at TIMESTAMPTZ NULL

otp_codes
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → accounts.id
├── code BYTEA NOT NULL
├── type VARCHAR(32) NOT NULL CHECK (type IN ('email_verify','password_reset'))
├── expires_at TIMESTAMPTZ NOT NULL
├── used_at TIMESTAMPTZ NULL
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()

e2e_key_backups (Phase 15, Flyway V4__e2e_key_backups.sql)
├── account_id UUID PRIMARY KEY -- logical ref → accounts.id
├── encrypted_blob TEXT NOT NULL
├── password_hint TEXT NULL
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Индексы v1:
- `UNIQUE INDEX accounts_email_uq ON accounts(email) WHERE email IS NOT NULL`
- `UNIQUE INDEX accounts_phone_uq ON accounts(phone) WHERE phone IS NOT NULL`
- `INDEX refresh_tokens_account_active_idx (account_id, expires_at DESC) WHERE revoked_at IS NULL`
- `INDEX refresh_tokens_token_hash_idx (token_hash)`
- `INDEX otp_codes_account_type_idx (account_id, type, expires_at DESC)`

Правило статуса удаления:
- source of truth для логического удаления — `deleted_at`.
- `status='deleted'` должен выставляться синхронно с `deleted_at IS NOT NULL` (инвариант уровня приложения/триггера).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`user.events`** (совместно с User для событий профиля; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                      |
|-------------------------|-----------------------------|
| `user.registered`       | account_id, type, method    |
| `user.logged_in`        | account_id, device_info, ip |
| `user.logged_out`       | account_id, device_info     |
| `user.2fa_enabled`      | account_id                  |
| `user.guest_converted`  | account_id                  |
| `user.account_deleted`  | account_id                  |
| `user.account_restored` | account_id                  |

## Зависимости

- **Redis** — JWT blacklist (запись при logout и отзыве access token), OTP throttling. Сквозные HTTP rate limits (в т.ч. лимит попыток входа с одного IP) — на **API Gateway**; те же лимиты вторым слоем в Auth не дублируем. Подробнее: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).
- **Resend** — отправка email (верификация, password reset)
- **NATS** — публикация событий

## Безопасность

- Пароли: bcrypt (cost 12)
- TOTP секреты: AES-256-GCM шифрование at rest
- Refresh token: только хэш в БД, оригинал — только клиенту
- Нет SMS 2FA (v1) — только TOTP
- IP logging для аудита


