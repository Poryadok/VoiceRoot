# Жизненный цикл первичного профиля (Auth ↔ User)

Источники: [DATA_MODEL.md](../DATA_MODEL.md) (JWT `user_id` / `profile_id`), [user-service.md](user-service.md), [auth-service.md](auth-service.md).

## Роли

| Сервис | БД | Что владеет |
|--------|-----|-------------|
| Auth | `auth_db` | `accounts`, refresh, JWT issuance |
| User | `user_db` | `profiles` (канонический `profile_id`), онбординг |

## Момент создания первичного профиля

Первичная строка в `user_db.profiles` с `is_primary = true` появляется **в том же запросе**, в котором Auth впервые выдаёт клиенту пару access + refresh после успешной аутентификации (или после успешного **refresh**). Точки входа продукта v1 ([auth-and-contacts.md](../features/auth-and-contacts.md)):

| Операция Auth | Когда создаётся `profiles` |
|---------------|----------------------------|
| **Register** | Сразу после фиксации новой строки `accounts`, до возврата сессии клиенту |
| **Login** | При первом успешном входе аккаунта, у которого ещё нет первичного профиля — до возврата сессии |
| **Refresh** | Если по какой-то причине первичного профиля ещё нет (редкий/переходный случай) — до выдачи новой пары токенов; обычно профиль уже есть, вызывается только идемпотентное чтение `id` |

Гостевая регистрация проходит тот же контур, что и обычная: после создания `accounts` выполняется обеспечение первичного профиля в `user_db`.

## Инвариант: нет окна «JWT без `profile_id`»

Для **всех** успешных ответов Auth v1 ([auth-and-contacts.md](../features/auth-and-contacts.md)), которые возвращают **`access_token`** (register / login / refresh):

1. В теле ответа (и в `AuthSession` в gRPC) всегда присутствует непустой **`profile_id`** — тот же UUID, что у первичного профиля в `user_db`.
2. В подписанном **access JWT** claim **`profile_id`** всегда задан и совпадает с этим значением **до** того, как клиент получил ответ (профиль материализован в БД **до** подписи токена).
3. Нет поддерживаемого сценария, где клиент законно получил новый access token от Auth, а в JWT отсутствует `profile_id` или он не соответствует существующей строке `profiles` для данного `account_id`.

Инвариант обеспечивается тем, что выдача сессии — **одна точка** в Auth: сначала успешно завершается internal gRPC `UserService.EnsurePrimaryProfile`, затем выпускается access JWT и сохраняется refresh-привязка. Пока User не вернул стабильный пригодный `profiles.id`, access token клиенту не выдаётся.

**Скоуп:** речь о выдаче токенов Auth v1 через обязательный вызов
`EnsurePrimaryProfile` у User Service. Произвольные старые токены или аварийные обходы не задают контракт продукта.

## Поведение v1 ([auth-and-contacts.md](../features/auth-and-contacts.md))

1. После успешного создания `accounts` (register) или при выдаче сессии (login/refresh) Auth запрашивает **идемпотентное** «обеспечить первичный профиль» для `account_id`.
2. Если строка с `is_primary = true` уже есть — возвращается её `id`.
3. Если нет — вставляется новая строка `profiles` с уникальной парой `(username, discriminator)` и человекочитаемым `display_name` (по умолчанию из email/телефона), создаётся строка `onboarding_state` для этого `profile_id`.
4. В JWT кладутся `user_id` (= `accounts.id`) и `profile_id` (= `profiles.id`).

### Guest → regular

После успешной конвертации guest в regular Auth сохраняет те же `account_id` и `profile_id`, затем internal-only вызывает `UserService.MarkAccountRegular(account_id)`. Операция снимает `is_guest_account` у всех профилей account (включая soft-deleted) и идемпотентна. Она не создаёт primary profile; его provisioning остаётся обязанностью `EnsurePrimaryProfile`.

## Мульти-профиль (будущее)

Переключение активного профиля (`SwitchProfile`) меняет только **выдачу следующего** access token (claim `profile_id`); первичный профиль остаётся в `profiles` с `is_primary = true`. Детали продукта — [multi-profile.md](../features/multi-profile.md).

## Технический контракт

`user_db` доступен только User Service. Auth настраивается адресом `USER_GRPC_ADDR` и синхронно
вызывает `EnsurePrimaryProfile`: RPC успешно завершён и ответ прошёл проверку account/profile state
→ затем подпись access JWT. При ошибке, пустом/несогласованном, удалённом или frozen-профиле Auth
не возвращает новую сессию. Связанные операции используют существующие User RPC
`ResolvePrimaryProfileIDs`, `SwitchProfile`, `SetVerification`, `ClearVerification` и
`MarkAccountRegular`; прямой SQL или второй datasource в Auth не допускаются. Воспроизводимые
критерии стыка Auth ↔ User — [EXEC_PLAN.md](../EXEC_PLAN.md).

## Owner parent proof and lock ordering (accepted target)

The canonical User association/nonreuse core commits the profile reservation,
immutable owner association, verified opaque Auth parent handle, onboarding and
Search projection in the SAME User transaction for every new primary, secondary,
guest or missing-primary repair birth. Parent authority and local selector
material are prepared BEFORE User opens that transaction. The transaction
rechecks proof expiry, exact raw identity binding, keyset and local terminal
fences. Missing proof denies a new birth; it never adopts a new row as a legacy
NOW baseline. Existing retained-row NOW capture may have an unresolved parent,
but that does not prove complete historical Role coverage.

Auth must release account SQL locks and callback-needed process monitors before
calling User. ConvertGuest validates in a short first phase, preflights the
actual guest primary outside locks, then rechecks current token/epoch/security,
guest state and command bindings in its final locked mutation/session phase.
Verified-email issuance and verify2FA use the same outside-lock preflight and
final local security recheck. Enrollment-only enable2FA is unchanged. Failed final
issuance does not delete an idempotently created User primary or its association.

OAuth first validates the peeked code and original account epoch in a short
phase WITHOUT floor preparation or code consumption. User preflight occurs
outside Auth locks and must return the exact selected profile. The final account
lock rechecks original origin/security and proof validity, then preserves the
existing floor preparation → atomic exact-code consume → full client/redirect/
PKCE/expiry revalidation → local signing order. A lost race or expiry denies;
consumed codes are never restored. Legacy refresh keeps its existing consumed
credential semantics and does not inherit OAuth's pre-consume retry behavior.

No Auth↔User distributed transaction or zero in-flight deletion guarantee is
claimed. Positive terminal fences and subsequent fresh eligibility remain
authoritative. The accepted contract and real SQL/callback/restore oracles are
tracked in the [owner-channel ExecPlan](../testing/role-identity-owner-channel-exec-plan.md);
full all-birth/transport integration and activation remain in progress.
