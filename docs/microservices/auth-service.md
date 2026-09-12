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
- Отзыв всех сессий через Auth-owned `session_epoch`; strict-потребители Gateway и Realtime проверяют floor fail-closed
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
- Maven внутри контейнера **без** доступа к Docker socket хоста может **пропускать** Testcontainers suites. `backend-auth` после единственного `mvn -B test` fail-closed проверяет Surefire report каждого класса с `@Testcontainers(disabledWithoutDocker = true)`: report должен существовать, содержать `tests > 0`, `skipped=0`, `failures=0` и `errors=0`; ориентир — CI или хостовый `mvn test` с Docker ([TESTING.md](../TESTING.md), job Auth в [.github/workflows/ci.yml](../../.github/workflows/ci.yml)).
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

### Ownership-transfer step-up proof

Auth implements `IssueOwnershipTransferProof` and `ConsumeOwnershipTransferProof`.
The issue surface requires a verified Gateway delegated-user principal; consume
requires a verified Space service principal. Gateway routing and Space transfer
integration remain separate rollout requirements. Unverified/legacy metadata never
creates proof authority.

- Auth checks password on every request. If the account has 2FA enabled, it also
  requires a valid TOTP or one unused backup code. It issues an opaque
  high-entropy proof bound to `account_id`, active `profile_id`, `space_id`,
  `new_owner_profile_id`, UUID `operation_id`, current `session_epoch`, and
  verified-factor set. Plaintext is returned once; Auth persists only its hash.
- The proof expires after five minutes and is revoked when session epoch, password,
  2FA or other account security state changes. Validation/consume never treats a
  missing, expired, revoked, mismatched or previously used proof as usable.
- Only the authenticated Space service principal may call consume. Consume is
  atomic and one-use; its durable receipt is keyed by `operation_id` and includes
  the exact bound identities. A retried identical consume returns that receipt,
  while a changed binding or another operation fails closed.
- Auth owns factors, proof hash, revocation state and receipt. Space owns the
  transfer request/idempotency record and no service other than Auth stores the
  proof plaintext. Gateway derives actor only from verified claims, relays the opaque
  proof to Space, redacts it from logs/traces/metrics, and neither creates, validates
  nor persists it.
- Public failure disclosure is deliberately coarse: no session is
  `UNAUTHENTICATED`; a factor failure or unusable/mismatched proof is
  `PERMISSION_DENIED`; malformed bindings are `INVALID_ARGUMENT`. Trusted caller
  failure, store failure or unavailable Auth never permits a transfer.

See [spaces.md](../features/spaces.md#контракт-подтверждения-передачи-владения)
for the end-to-end request, Space idempotency and error contract.


#### Durable proof implementation

`auth.persistence=jdbc` provides the proof service; memory mode has no proof store
and the adapter fails closed. Principal listener/configuration is documented in
[Auth README](../../src/backend/auth/README.md). The public issue route is
`POST /api/v1/auth/ownership-transfer-proof` through Gateway; the internal consume
RPC is never a public route.

Flyway `V12__ownership_transfer_proofs.sql` (golang-migrate mirror
`000013_ownership_transfer_proofs.up.sql`) adds `ownership_transfer_proofs` with
unique `operation_id`, SHA-256 proof hash, exact account/profile/Space/target/epoch
bindings, verified factors, five-minute expiry and durable receipt ID/time. A proof
contains 32 random bytes encoded as unpadded base64url. Re-issuing an existing
operation cannot replace its proof or return its plaintext again. An identical
consume retry returns the already committed receipt, including after later expiry
or revocation; it never creates a new grant. Changed proof/bindings fail closed.
Times are normalized to PostgreSQL microsecond precision before persistence and
response so retried receipts remain identical.

`accounts.security_revision` is monotonic. Database triggers advance it on changes
to password, TOTP secret/enabled state, session epoch, account status/deletion/type,
email/phone and pending-verification state, and on backup-code replacement/use.
Reverting credentials does not revive an older proof. Issue and consume hold the
account row lock for their complete transaction. Existing JDBC backup-code consume
and replacement use the same account-before-backup lock order; replacement and
factor consumption are atomic. An issue that consumes a backup factor reloads its
new revision before saving the proof, and a later write failure rolls both back.
Only the existing receipt replay bypasses new-grant expiry/security checks.

Focused verification: `OwnershipTransferProofServiceTest`,
`OwnershipTransferProofJdbcIntegrationTest`, and `AuthOwnershipProofAdapterTest`
cover factors, all bindings, expiry, revocation, rollback, concurrent consumers and
security writers, coarse status disclosure and hash-only persistence. Proofs and
factor material are never included in adapter errors or logs.

#### Consumed-receipt recovery

`GetOwnershipTransferReceipt` is an internal read-only recovery RPC for Space's
durable ownership-transfer journal. It recovers a previously committed receipt
after a consume response is lost, without storing or resending the opaque proof.
It creates no grant and never consumes an issued proof. There is no public route.

The request contains seven required fields: UUID `account_id`, `profile_id`
(the original owner/actor), `space_id`, `new_owner_profile_id`, `operation_id`,
positive original `session_epoch`, and `proof_digest` (exactly 64 lowercase hex
characters encoding SHA-256 of the original opaque proof's UTF-8 bytes, with no
prefix or whitespace). Every field must match the stored consumed proof.
The response contains only `receipt_id`, the five bound UUID fields,
`session_epoch`, `consumed_at`, and `verified_factors`, identical to the original
consume receipt. Neither proof plaintext nor its digest is returned.

Only a fresh verified Space service principal may call this RPC on the private
Auth TLS listener. It uses the same Phase 0 audience, exact RPC, request hash,
request ID, credential expiry and replay checks as consume. Gateway delegated
users and other service callers cannot recover receipts. The legacy listener
denies this method, including requests with signed service credentials.

Missing or invalid credentials yield `UNAUTHENTICATED`; a verified wrong caller
yields `PERMISSION_DENIED`. Malformed UUIDs, nonpositive epochs or malformed
digests yield `INVALID_ARGUMENT`. Missing, unconsumed and mismatched records all
yield the same coarse `PERMISSION_DENIED`; storage/configuration failure yields
`UNAVAILABLE`. Errors do not disclose stored bindings, digest, factors or account
state.

Recovery matches the original epoch; it does not reauthorize against the current
account, session floor, password, 2FA, security revision or proof expiry. A retained
consumed receipt remains recoverable after account deletion, including physical
removal of its account row. The store uses a separate read-only `READ_COMMITTED`
transaction, suspending any caller transaction; an uncommitted consume cannot
become recovery authority, even in the same thread. Lookup takes no account or
proof row lock and changes no rows.

An absent receipt is not evidence that an in-flight consume will never commit.
Space must persist its immutable original bindings and proof digest before
consume, and persist a confirmed receipt before advancing its transfer protocol.
Once Space decides to abort, a late receipt must not revive that operation.

Consumed receipts survive ordinary five-minute pending-proof cleanup. This slice
adds no cleanup job or retention extension. The existing 30-day account-erasure
policy and [data erasure rules](../DATA_MODEL.md) remain authoritative. Before
activating receipt erasure or pseudonymization, Auth and Space must coordinate
pending-journal settlement and durable terminal operation fences; historical
lookup is not an exemption from erasure policy.

`OwnershipTransferReceiptLookupTest` and
`OwnershipTransferReceiptLookupJdbcIntegrationTest` cover exact binding/digest
matching, historical and hard-deleted-account recovery, no state mutation, and
uncommitted/rolled-back consume isolation on separate and ambient transactions.

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
├── session_epoch (положительный монотонный durable epoch, default 1)
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

`email_verified_at` and the verification-gated promotion are shipped for email
registration and `convert-guest` (PR #180): Auth creates a restricted pending
session, accepts bearer-scoped `POST /api/v1/auth/otp/send` and `otp/verify`, and
promotes the account to `regular` only after successful verification, with durable
recovery for the User sync and conversion event. Remaining client UX and live-provider
acceptance are tracked in [todo/client.md](../todo/client.md) and
[todo/backend.md](../todo/backend.md).

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
| `user.account_deleted`  | account_id; stable envelope `event_id` |
| `user.account_restored` | account_id                  |

## Зависимости

### Linked provider verification

`linked_identities` хранит единственную account/provider identity вместе с owning
`profile_id` и монотонной `source_revision`. Активную identity нельзя молча перенести на
другой профиль: V1 требует unlink → link. `verification_source_sync_targets` — durable
outbox-like state по account/profile/provider; target считается synced только после успешного
`UserService.ApplyVerificationSourceState`. Callback, unlink и scheduled refresh повторяют
pending targets. Provider outage сохраняет последнее verified state; подтверждённая потеря
критерия создаёт новую revoked revision. Owner-only list возвращает `profile_id`, чтобы клиент
показывал sources выбранного профиля.

- **User Service gRPC** (`USER_GRPC_ADDR`) — provisioning/resolve/switch профилей,
  синхронизация verification и завершения guest conversion. User единолично владеет `user_db`;
  недоступность, `DEADLINE_EXCEEDED` или непригодный ответ User блокирует выдачу новой сессии.
  Каждый blocking RPC (`EnsurePrimaryProfile`, `ResolvePrimaryProfileIDs`, `SwitchProfile`,
  `ApplyVerificationSourceState`, legacy `SetVerification` / `ClearVerification`,
  `MarkAccountRegular`) получает новый per-call deadline:
  `auth.user-grpc.deadline` / `AUTH_USER_GRPC_DEADLINE` — положительная ISO-8601 `Duration`.
  Если переменная **отсутствует**, используется `PT15S`; явные пустое, malformed, zero или negative
  значения являются ошибкой конфигурации и останавливают startup. Deadline создаётся при создании
  каждого `ClientCall`, а не один раз при создании Spring singleton stub.
- **Redis** — JWT blacklist (запись при logout и отзыве одного access token), OTP throttling и T056-P1 minimum-epoch floor без TTL. Auth записывает floor операцией `max` без TTL, а Gateway и Realtime в strict-режиме читают его fail-closed. Сквозные HTTP rate limits (в т.ч. лимит попыток входа с одного IP) — на **API Gateway**; те же лимиты вторым слоем в Auth не дублируем. Подробнее: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).
- **Resend** — отправка email (верификация, password reset)
- **NATS** — публикация событий

## Безопасность

- Пароли: bcrypt (cost 12)
- TOTP секреты: AES-256-GCM шифрование at rest; `AUTH_TOTP_ENCRYPTION_KEY` обязателен при `auth.persistence=jdbc` без `auth.totp.test-bypass` (иначе Auth не стартует; `DEFAULT_DEV_KEY` только memory/dev bypass)
- Refresh token: только хэш в БД, оригинал — только клиенту
- Нет SMS 2FA (v1) — только TOTP
- IP logging для аудита

### T056-P1: session epoch

`accounts.session_epoch` — положительный монотонный durable source of truth
Auth. Auth атомарно увеличивает его для отзыва всех сессий и никогда не
уменьшает. Каждый access JWT, выпущенный Auth, содержит положительный integer
claim `session_epoch`; `jti` остаётся per-session механизмом logout/отзыва.

Auth — единственный writer ключа `auth:session:min_epoch:<account_id>` со
значением положительного `int64` без TTL; запись выполняется только операцией
`max`. Перед каждым прямым выпуском JWT/session — registration (regular и guest),
login, refresh, OAuth exchange, 2FA reissue, profile switch, guest conversion
и restore — Auth подготавливает durable epoch относительно floor. Redis-ahead
значение reconcile-ится только вверх; ошибка floor, невалидное значение или
неуспешный reconcile дают fail-closed. При ошибке подготовки epoch
JDBC-транзакция create+prepare откатывается до User RPC; успешная транзакция
завершается до RPC. Memory profile этот rollback не моделирует.

При `auth.persistence=jdbc` startup Auth до запуска любого `SmartLifecycle`
постранично seed-ит и сверяет Redis floor с durable `accounts.session_epoch`.
`auth.session-epoch.seed.page-size` задаёт положительный размер страницы и по
умолчанию равен `256`. Hook зависит от инициализации БД, но не от конкретного
Flyway bean: он работает и с внешними миграциями, а недоступный Redis или
отсутствующая schema завершают startup ошибкой.

В Compose включены strict-проверки Gateway и Realtime: они отклоняют
missing/corrupt claim/floor и ошибку чтения Redis; Gateway проверяет non-empty
`jti` blacklist до floor, а Realtime — на upgrade, inbound operations и
outbound fan-out.

Compose strict proof не завершает rollout для всех окружений: требуется отдельная
operational acceptance. Immediate account-targeted close через Redis Pub/Sub пока
не реализован; authority остаются strict JWT/floor проверки.

## Space deletion proof and recovery (P3 Auth slice implemented)

This is a distinct `space_delete` family and never reuses ownership-transfer
tokens, rows or receipts. Public `IssueSpaceDeletionProof` binds account, active
profile, positive session epoch, canonical Space/operation IDs and the exact
UTF-8 confirmation name. Password is always verified; enabled 2FA requires
exactly one TOTP or unused backup code. The opaque proof is returned once; Auth
stores only its SHA-256, exact-name SHA-256, remaining bindings and security
revision. TTL is exactly five minutes and equality is expired; password/session/
2FA security changes revoke it.

Only signed Space workload identity may consume, look up or acknowledge a
deletion receipt. Consume is atomic and immutable; exact replay returns the same
receipt, while changed purpose or binding fails closed. Lookup requires account,
profile, epoch, Space, operation plus both name and proof digests, creates no
grant and reveals only an already committed receipt. Missing, unconsumed,
expired-before-consume, revoked and mismatched states all return coarse
`PERMISSION_DENIED`. Space stores exact receipt bytes/hash before acknowledgement;
missing lookup cannot abort an in-flight consume.

Unconsumed proof rows retain through `expires_at`. A consumed receipt awaiting
Space acknowledgement has no time-based deletion. An acknowledged receipt
retains until `max(consumed_at + 30 days, acknowledged_at + 24 hours)`. If
account erasure precedes acknowledgement, raw bindings become an Auth-keyed
HMAC-SHA-256 lookup index over `voice-auth-space-delete-receipt-v1`, NUL and
deterministic binding bytes while the immutable receipt remains recoverable.
Only Auth workload identity has that distinct key; staff and break-glass have no
access. Rotation is `P90D`, maximum restorable-backup age is `P30D`, missing key
fails closed, and destruction waits for every dependent receipt and backup.
This pseudonymous evidence is accepted as compatible with account erasure.

The Auth implementation is enabled with `auth.persistence=jdbc`. Flyway
`V13__space_deletion_proofs.sql` and the equivalent golang-migrate
`000014_space_deletion_proofs.up.sql` create the separate hash-only proof and
receipt store. The ordinary listener exposes only issue; consume, committed-only
lookup and acknowledgement are available only on the protected listener to a
verified `service:space` principal. Issue resolves its actor only from the
`Authorization` metadata of the current request; missing metadata fails with
`UNAUTHENTICATED` and never falls back to a remembered credential from another
request. Deployments must provide the dedicated `ReceiptErasureKeyring` before
account erasure can pseudonymize or recover receipts; an absent or missing
retained key fails closed.
