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
- OTP генерация и валидация (email)

## API (gRPC)

```protobuf
service AuthService {
  rpc Register(RegisterRequest) returns (AuthResponse);
  rpc Login(LoginRequest) returns (AuthResponse);
  rpc Logout(LogoutRequest) returns (Empty);
  rpc RefreshToken(RefreshRequest) returns (AuthResponse);
  rpc Enable2FA(Enable2FARequest) returns (Enable2FAResponse);
  rpc Verify2FA(Verify2FARequest) returns (AuthResponse);
  rpc VerifyOTP(VerifyOTPRequest) returns (Empty);
  rpc ConvertGuest(ConvertGuestRequest) returns (AuthResponse);
  rpc DeleteAccount(DeleteAccountRequest) returns (Empty);
  rpc RestoreAccount(RestoreAccountRequest) returns (AuthResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (TokenClaims); // internal
  rpc GetJWKS(Empty) returns (JWKSResponse); // public
}
```

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


