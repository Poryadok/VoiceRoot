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

Инвариант обеспечивается тем, что выдача сессии — **одна точка** в Auth: сначала завершается шаг «ensure primary profile» (идемпотентная запись/чтение в `user_db`), затем выпускается access JWT и сохраняется refresh-привязка. Пока шаг ensure не вернул стабильный `profiles.id`, access token клиенту не выдаётся.

**Скоуп:** речь о выдаче токенов Auth v1 с включённым провижинингом в `user_db` (или эквивалентном вызове **EnsurePrimaryProfile** у User Service). Произвольные старые токены или аварийные обходы не задают контракт продукта.

## Поведение v1 ([auth-and-contacts.md](../features/auth-and-contacts.md))

1. После успешного создания `accounts` (register) или при выдаче сессии (login/refresh) Auth запрашивает **идемпотентное** «обеспечить первичный профиль» для `account_id`.
2. Если строка с `is_primary = true` уже есть — возвращается её `id`.
3. Если нет — вставляется новая строка `profiles` с уникальной парой `(username, discriminator)` и человекочитаемым `display_name` (по умолчанию из email/телефона), создаётся строка `onboarding_state` для этого `profile_id`.
4. В JWT кладутся `user_id` (= `accounts.id`) и `profile_id` (= `profiles.id`).

## Мульти-профиль (будущее)

Переключение активного профиля (`SwitchProfile`) меняет только **выдачу следующего** access token (claim `profile_id`); первичный профиль остаётся в `profiles` с `is_primary = true`. Детали продукта — [multi-profile.md](../features/multi-profile.md).

## Техническая отметка (bootstrap в коде)

До выделения User gRPC в проде допускается второй JDBC datasource в Auth только на `user_db` для шага «ensure primary profile»; целевое состояние — **синхронный gRPC `EnsurePrimaryProfile`** в User Service с тем же порядком: RPC успешно завершён → затем подпись access JWT. Инвариант «нет окна без `profile_id`» сохраняется: при ошибке User Auth не возвращает сессию с токеном. Воспроизводимые критерии стыка Auth ↔ `user_db` — [EXEC_PLAN.md](../EXEC_PLAN.md).
