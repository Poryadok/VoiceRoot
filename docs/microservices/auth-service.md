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
- Отзыв всех сессий через Auth-owned `session_epoch` (staged/WIP; потребители ещё не в strict)
- 2FA (TOTP — Google Authenticator и аналоги)
- JWT blacklist (Redis, для логаута и ротации)
- Гостевые аккаунты (30-дневный TTL, ограниченные права)
- Конвертация гостевого аккаунта в полноценный
- Soft delete аккаунта (30-дневный grace period)
- JWKS endpoint для публичных ключей (используется Gateway и другими сервисами)
- **[auth-and-contacts](../features/auth-and-contacts.md):** перед выдачей access JWT Auth вызывает internal User gRPC `EnsurePrimaryProfile`; claim `profile_id` равен User-owned `profiles.id`. Auth не получает credentials к `user_db`; см. [primary-profile-bootstrap.md](primary-profile-bootstrap.md), [EXEC_PLAN.md](../EXEC_PLAN.md).
- OTP генерация и валидация (email)

### PR и ревью (bootstrap JWT ↔ User)

- Перед merge — зелёный job **`backend-auth`** в CI (`mvn -B test`). Интеграция Auth JDBC + Redis и контракт Auth ↔ User gRPC покрывают регистрацию / login / refresh / validate, включая совпадение `profile_id` с ответом `EnsurePrimaryProfile` и fail-closed поведение.
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
  rpc PutE2EKeyBackup(PutE2EKeyBackupRequest) returns (PutE2EKeyBackupResponse); // encryption.md
  rpc GetE2EKeyBackup(GetE2EKeyBackupRequest) returns (GetE2EKeyBackupResponse);
}
```

### ConvertGuest (guest → regular)

REST: `POST /api/v1/auth/convert-guest` (Gateway transcoding). Спека UX: [auth-and-contacts.md](../features/auth-and-contacts.md) § «Регистрация гостевого аккаунта».

| Поле | Семантика |
|------|-----------|
| `email` / `phone` | Идентификатор постоянного аккаунта (хотя бы один) |
| `password` | **Новый пароль**, который пользователь задаёт в форме convert-guest для `regular`-аккаунта |

- Авторизация: **JWT гостя** (Bearer); transport-пароль, сгенерированный при guest bootstrap, **не проверяется** в `ConvertGuest`.
- После submit создаётся pending email identity; тот же `account_id` / primary
  `profile_id` и session сохраняются, но права остаются guest-level.
- Только успешный email verification атомарно меняет `accounts.type` → `regular` и
  публикует NATS `user.guest_converted`; resend идемпотентен и не создаёт новый аккаунт.
- Негативные кейсы (duplicate email, password &lt; 8, non-guest token): `ConvertGuestIntegrationTest` (Auth Maven).

### E2E key backup ([encryption.md](../features/encryption.md), REST via Gateway)

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
├── session_epoch (positive monotonic floor, default 1; staged/WIP)
├── email_verified_at (nullable; null = restricted pending identity)
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

e2e_key_backups ([encryption.md](../features/encryption.md))
├── account_id (UUID, PK, logical ref → accounts.id)
├── encrypted_blob (TEXT, client-encrypted opaque payload)
├── password_hint (nullable)
└── updated_at
```

### V1 (core DM scope) — детальный профиль для DDL

```
accounts
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── email VARCHAR(320) NULL
├── phone VARCHAR(32) NULL
├── password_hash TEXT NOT NULL
├── type VARCHAR(16) NOT NULL CHECK (type IN ('regular','guest'))
├── status VARCHAR(16) NOT NULL CHECK (status IN ('active','suspended','deleted'))
├── session_epoch BIGINT NOT NULL DEFAULT 1
├── email_verified_at TIMESTAMPTZ NULL
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

e2e_key_backups ([encryption.md](../features/encryption.md), Flyway V4__e2e_key_backups.sql)
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

`email_verified_at` and verification-gated promotion are target contract gaps in the
currently deployed schema/code; see [todo/backend.md](../todo/backend.md).

Правило статуса удаления:
- source of truth для логического удаления — `deleted_at`.
- `status='deleted'` должен выставляться синхронно с `deleted_at IS NOT NULL` (инвариант уровня приложения/триггера).
- `deleted_at` открывает 30-дневное recovery window; после него отдельный идемпотентный
  erasure job удаляет/pseudonymizes PII и credentials, после чего restore невозможен.
- Message history сохраняет только непривязанный к публичному профилю author tombstone;
  legal/anti-abuse retention хранится отдельно по production policy.

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

- **User Service gRPC** (`USER_GRPC_ADDR`) — provisioning/resolve/switch профилей,
  синхронизация verification и завершения guest conversion. User единолично владеет `user_db`;
  недоступность или непригодный ответ User блокирует выдачу новой сессии.
- **Redis** — JWT blacklist (запись при logout и отзыве одного access token), OTP throttling и staged T056-P1 minimum-epoch floor без TTL. Floor обновляется только вверх из Auth DB; Gateway и Realtime читают его fail-closed в strict-режиме. Сквозные HTTP rate limits (в т.ч. лимит попыток входа с одного IP) — на **API Gateway**; те же лимиты вторым слоем в Auth не дублируем. Подробнее: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).
- **Resend** — отправка email (верификация, password reset)
- **NATS** — публикация событий

## Безопасность

- Пароли: bcrypt (cost 12)
- TOTP секреты: AES-256-GCM шифрование at rest; `AUTH_TOTP_ENCRYPTION_KEY` обязателен при `auth.persistence=jdbc` без `auth.totp.test-bypass` (иначе Auth не стартует; `DEFAULT_DEV_KEY` только memory/dev bypass)
- Refresh token: только хэш в БД, оригинал — только клиенту
- Нет SMS 2FA (v1) — только TOTP
- IP logging для аудита

### T056-P1: session epoch (staged/WIP)

`accounts.session_epoch` — durable источник истины, `BIGINT NOT NULL DEFAULT
1`, положительный и монотонный. Auth атомарно увеличивает его при отзыве всех
сессий и никогда не уменьшает. Новый access JWT получает обязательный
положительный integer claim `session_epoch`; `jti` остаётся per-session claim для
узкого logout/отзыва одного токена.

До завершения зависимостей работает только rollout-контракт `expand → seed →
strict`: миграция и repository/transactional revocation в Auth, заполнение
Redis floor, затем strict-проверки Gateway и Realtime. Legacy JWT без claim и
не seeded floor допустимы только в явно включённом compatibility-режиме; в
strict missing/corrupt claim или floor, а также Redis error — fail-closed.
Redis Pub/Sub не является correctness mechanism для отзыва и лишь ускоряет
адресное закрытие сокетов в Realtime. Текущий staged claim сам по себе не
означает, что отзыв всех существующих access/refresh сессий уже реализован.
