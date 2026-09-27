# Game Integration API — целевой внешний контракт

**Proposed target.** Ограниченный Auth bootstrap GAME-AUTH-01 реализован в
opt-in режиме; PostgreSQL/live-provider acceptance остаётся обязательным gate.
Остальные новые API ниже ещё не реализованы. Продуктовые требования —
[интеграции игр](../features/game-integrations.md); acceptance и решения —
[матрица](../testing/game-integrations-acceptance.md). API имена фиксируют
предлагаемую семантику; до реализации нужны reviewed OpenAPI/proto и error schema.

Этот контракт и его реализация в Voice входят в спринт. Unity/Unreal assets,
SDK wrappers, developer CLI/portal — отдельная задача; слово SDK обозначает
внешнего потребителя протокола, в тестах Voice заменяемого test client.

## 1. Границы и владельцы

```mermaid
flowchart LR
  Engine[Unity / Unreal SDK] --> Edge[Voice Gateway REST / WS]
  Game[Game backend] --> Edge
  Messenger[Voice messenger] --> Edge
  Edge --> Auth[Java Auth + User]
  Edge --> Integration[Game Integration Service]
  Integration --> Domains[Chat / Space / Role / Voice]
  Edge --> Messaging[Messaging / Realtime]
  Edge --> Bot[Bot interactions]
  Bot --> Game
  Domains --> Media[LiveKit]
```

Решение владельца: выделить **Game Integration Service**, отдельный Go-микросервис
со своим хранилищем для developer applications, bindings, внешних resource mappings,
communication sessions и orchestration operations. Сервис ещё не реализован.
G06 остаётся открытым для схемы хранилища, контрактов и deployment, но не для
выбора между отдельным сервисом и встраиванием в существующий. Ни Gateway,
ни бот не становятся универсальной БД; Auth сохраняет владение аккаунтами.

| Владелец | Ответственность |
|---|---|
| Auth, Java/Spring | Identity proof, consent grants, delegated tokens, revocation/session epochs |
| User | Профиль, privacy, публичные атрибуты и допустимая presence |
| Game Integration Service | App/env, game bindings, external mappings, session orchestration, quotas |
| Chat / Space / Role | Членство, дерево, lifecycle, policy и effective permissions |
| Messaging / File | Сообщения/история/attachments и их access/retention |
| Realtime | WS delivery; per-connection `s` / `resume` |
| Voice | Voice admission, grants, media lifecycle, revocation |
| Bot | Installation, interaction delivery, credentials бота, response routing |
| Game backend | Roster facts, владение персонажами, экономика, допустимость и commit команд |

Ни один сервис не пишет в чужую БД. Межсервисные UUID — logical references без
FK. Существующие IDs сохраняют правила [DATA_MODEL](../DATA_MODEL.md): UUIDv4 для
ресурсов, UUIDv7 сообщений выдаёт Messaging. Внешние игровые IDs — opaque строки,
не доверенные UUID Voice и не основание доступа.

## 2. Ресурсы

| Ресурс | Ключ и значимые поля | Жизненный цикл |
|---|---|---|
| Application | `application_id`, owner, optional `game_id`, enabled capabilities | draft → sandbox → active → suspended/retired |
| Environment | `environment_id`, application, allowed origins/redirects/providers | sandbox и production строго раздельны |
| Installation | `installation_id`, app/env, approving actor, target scope | active → revoked; новая установка = новый ID |
| Player binding | `binding_id`, app/env/provider/external subject, selected profile, revision | pending → active → revoked |
| Character binding | game/realm/character key, binding, display policy | attached → detached/deleted |
| Communication session | `session_id`, kind party/match/fleet, external key, optional parent party | provisioning → active → closing → closed; failed |
| Resource mapping | app/env/external key → chat/voice/Space UUID | active → retired; не переиспользовать tombstoned key молча |
| Managed grant | origin external membership/rank, subject, bounded permission | desired → applied → revoked |
| Operation | `operation_id`, immutable request hash, stage/results/error | pending → succeeded/failed; reconcile after uncertainty |

Для bot invocation operation имеет неизменяемый `command_id` и domain status;
delivery ACK не переводит operation в succeeded. Terminal mapping и result CAS
определены в [bot protocol](../features/game-bot-interactions.md).

Предлагаемые unique constraints: app/env/external session key; app/env/provider/
subject для одной активной player binding; installation/event ID; actor-scoped
idempotency key. Несколько игровых аккаунтов у человека допустимы только по
явной политике приложения; ни nickname, ни email не используются для dedupe.
Принято: sdk-account уникален в app/env/provider subject, между приложениями
нет автоматического объединения людей. Независимая identity authority не
раскрывает другим приложениям эти сопоставления. Новый device key регистрируется
через независимый user proof и отзывается отдельно; provider login не даёт
восстановить permanent Voice credentials. Конкретная recovery схема и защита
от смены владельца subject требуют G01 и Q03 из design audit.
Профиль может получать grants от нескольких персонажей; причины хранятся
раздельно. Перенос персонажа другому аккаунту отзывает прежний binding/grants
до выдачи новых; старые карточки не переадресуются новому владельцу.

## 3. Три principal, три набора credentials

| Principal | Где хранится credential | Что может |
|---|---|---|
| Game service | Secret store backend игры | Создать свои sessions, передать подтверждённый roster, события |
| Delegated player | OS secure storage / память SDK | Чат/голос и разрешённые действия выбранного профиля |
| Bot actor | Secret store адаптера игры | Только installation/scopes/destinations бота |

Server secret нельзя положить в Unity asset, Blueprint, браузер, конфиг клиента
или deep link. SDK не принимает «admin token» как упрощённую авторизацию.
Bot Token не заменяет user session. Session grants не дают доступа к остальному
inbox, друзьям, другим играм и другим профилям.

### Вход и связывание

1. SDK получает краткоживущий независимо проверяемый login proof. Для авторства
   пользователя заверения backend разработчика недостаточно: нужен вход Voice
   либо внешний provider flow, который Voice проверяет независимо от игры и
   связывает с ключом устройства. Game ticket может подтверждать roster/context,
   но не выдавать developer право получить user credential. Issuer/audience,
   signature, nonce, expiry и replay проверяются сервером; поддержанные providers
   настраиваются в portal, ключи берутся из доверенного registry; developer
   не может назначить свой issuer независимым удостоверением игрока.
2. Voice создаёт binding challenge с `state` и одноразовым nonce. Пользователь
   проходит системный browser/device flow, выбирает профиль и scopes.
3. Authorization code + PKCE связывают callback с инициатором. Redirect URI
   совпадает с allowlist; custom scheme/deep link сам не доказывает владение.
4. Auth выдаёт delegated grant на app/env/binding/profile. Game server получает
   только разрешённую ему часть binding; не master refresh token пользователя.
5. Revoke/unlink, смена profile binding, удаление аккаунта и suspension приложения
   увеличивают authority revision/epoch; действующие сессии теряют разрешения.

В embedded браузер игры пароль Voice не вводится. Для устройства без браузера
нужен device code с expiry, ограниченным polling и явным подтверждением.
Alpha admission/invite/cohort cap не обходится через игровой endpoint.

`sdk-account` — отдельный Auth-supported principal с
централизованным владением. До его реализации SDK возвращает
`CAPABILITY_UNAVAILABLE`, а не создаёт скрытый regular/guest аккаунт.
Claim/upgrade требует proof обеих identity, описанного conflict flow и политики
истории. Автоматического merge друзей/профилей/истории нет.

### Конвертация sdk-account

#### Auth durable conversion preparation GAME-AUTH-03

Auth создаёт durable operation до внешних owner effects. Новый target начинается
с current SDK credential и device proof; существующий — с linked-bootstrap
credential GAME-AUTH-02 и тем же device key. Обычный Voice Bearer остаётся в Voice
browser UI. Prepare фиксирует mode (`new` / `existing`), binding ID, source
app/env/device/generation и client UUID idempotency key. Повтор key с тем же
target/binding возвращает прежнюю operation без продления intent; другая нагрузка
даёт 409. Source account допускает несколько подготовленных intents, но только
одну подтверждённую conversion; confirmation остаётся отдельным preview/CAS шагом.

Prepare device payload:
`voice-sdk-conversion-new-v1\n<SHA-256 SDK token>\n<idempotency UUID>\n<binding UUID>`
или `voice-sdk-conversion-existing-v1\n<SHA-256 linked token>\n<idempotency UUID>\n<binding UUID>`.
Auth request hash — SHA-256 UTF-8
`<mode>\n<idempotency UUID>\n<binding UUID>\n<target account UUID or ->\n<target profile UUID or ->`.

New mode выдаёт Auth-owned `registration_intent_id` с TTL 15 минут. Обычный
Voice registration endpoint принимает optional `registrationIntentId`. В одной
локальной транзакции создаётся новый account, consumes одноразовый intent и
записывается его account ID в operation. Неверный/истёкший/использованный intent
откатывает создание account. Legacy guest registration с intent запрещена.
Pending email сохраняет обычный registration/OTP lifecycle. Attach target
требует current Voice proof **ровно записанного** account после перехода в
regular; существующий раньше account допустим только в existing mode.
New attach вызывает идемпотентный User EnsurePrimaryProfile, затем read-only
eligibility именно primary. Existing mode не вызывает provisioning или switch.

Recovery status доступен с сохранённым public device key operation даже после
отзыва source credential. Это только чтение, не player authority. Payload:
`voice-sdk-conversion-status-v1\n<operation UUID>\n<issued-at Unix seconds>`;
Auth принимает timestamp не старше 60 секунд и не дальше 30 секунд в будущем.
Ни provider credential, ни ordinary Voice token для status не сохраняются.
Prepare/status/attach ещё не freeze, не transfer и не завершённая conversion.
Последующие preview, explicit confirmation и owner receipts обязаны следовать
[conversion authority](game-conversion-authority.md); unavailable adapter
оставляет durable state без продвижения и никогда не создаёт no-op receipt.

Prepare/status/attach routes отдельно opt-in через `auth.sdk-conversion.enabled`
и доступны только при JDBC persistence:

| Route | Proof / request | Result |
|---|---|---|
| `POST /api/v1/auth/sdk/conversions/new` | SDK Bearer; `idempotencyKey`, `bindingId`, `deviceProof` | prepared operation и registration intent |
| `POST /api/v1/auth/sdk/conversions/existing` | Linked Bearer; те же поля | prepared operation с ранее выбранным permanent profile |
| `POST /api/v1/auth/sdk/conversions/{operationId}/status` | `issuedAt` Unix seconds и `deviceProof`; без source Bearer | durable operation state/revision и target/intent IDs |
| `POST /api/v1/auth/sdk/conversions/{operationId}/attach-new-target` | Current Voice Bearer; без authority fields в body | только зарегистрированный через intent новый target/primary |

Intent consume проверяет current source generation/device и lifecycle, но не
требует ещё живой пятиминутный bootstrap session. Отзыв устройства запрещает
consume, сохраняя только purpose-limited status. Unknown fields и malformed
requests дают безопасный `400`, invalid proof — `401`, изменённая idempotent
нагрузка — `409`. Registration intent не попадает в parser/validation logs.

#### Auth browser consent GAME-AUTH-02

Этот следующий Auth slice реализует подготовку связи из уже независимо
подтверждённого sdk-account. Consumer — существующий Voice browser/Flutter UI
и внутренний protocol client. Auth не принимает пароль Voice в игровом UI;
обычный Voice sign-in/registration остаётся существующим потоком. Device-code
вариант не включается без конкретного browserless consumer.

Registry policy читается через `SdkAuthorizationPolicy.resolve(app, env)`:
`applicationId`, `environmentId`, положительная immutable `revision`,
`displayName`, точные `redirectUris`, разрешённые `playerScopes`. Registry
владеет allowlist. Endpoint владельца —
`GET /internal/v1/authorizations/environments/{environment_id}`; только
выделенная Auth↔Game Integration workload identity. Недоступный policy adapter,
неактивное приложение, неверные IDs, неизвестный scope, смена revision закрывают
request/approval/exchange. Локальный developer allowlist не заменяет lookup.
Production adapter выключен до реализации проверяемого workload contract.

1. SDK создаёт authorization request с UUID idempotency key, точным redirect,
   S256 challenge (43 base64url chars), случайным state (43–128 base64url chars)
   и отсортированным набором scopes. Набор непустой, максимум 16 элементов;
   player subset — `game.identity.read`, `game.chat.read`, `game.chat.send`,
   `game.voice.join`, `game.presence.write`, `game.invites.create`; service-only
   scopes запрещены. Redirect — absolute URI без fragment/userinfo/CRLF;
   допустимые schemes задаёт registry, совпадение allowlist всегда exact.
   Source credential и device possession
   проверяются в той же транзакции. Повтор того же ключа/полезной нагрузки
   возвращает прежний request ID/expiry; другая нагрузка даёт conflict.
2. Canonical request digest — SHA-256 UTF-8
   `voice-sdk-authorization-request-v1\n<idempotency UUID>\n<redirect>\n<S256 challenge>\n<state>\n<sorted scopes joined by comma>`.
   Device JWS payload —
   `voice-sdk-authorize-v1\n<SHA-256 source token>\n<request digest>`.
   Lifetime ограничен меньшим из source-session expiry и now+5 минут.
3. Voice UI показывает app/окружение, scopes, исходный game context, выбранный
   профиль и явное согласие. UI читает эти данные из Auth consent-view с current
   regular Voice Bearer; переданные игрой display/scopes не являются источником
   consent UI. View повторно проверяет policy/source и ничего не утверждает.
   Approval требует обычный current Voice Bearer,
   **актуальный** `accounts.type=regular` в Auth DB, exact policy revision и
   read-only User eligibility выбранного profile (owner/nondeleted/nonfrozen
   и revision). `SwitchProfile` не вызывается; primary не подставляется молча.
   Pending email registration не является постоянным target до verification.
4. Approval выдаёт одноразовый random code (в store только SHA-256), TTL до
   60 секунд и не дольше request. Redirect совпадает с registry byte-for-byte;
   state возвращается неизменным. Повтор approval после успешного выпуска
   отвергается; потерянный ответ требует нового request, не восстанавливает code.
5. Exchange требует code, exact redirect, PKCE verifier (43–128 unreserved
   ASCII chars) и прежний device key. Payload:
   `voice-sdk-code-v1\n<request UUID>\n<SHA-256 code>\n<SHA-256 verifier>`.
   Перед consume заново проверяются current source/device generation,
   target regular/status/epoch/approval-session blacklist, registry revision
   и exact User eligibility revision. Expiry проверяется после ожидания locks.
6. Результат — отдельный opaque linked-bootstrap credential на 5 минут с
   app/env, исходной identity/device, **выбранным** target account/profile,
   scopes и consent/policy revision. Он не обычный Voice JWT/refresh и не
   активирует player binding или media. Binding activation и conversion
   требуют receipts по [conversion authority](game-conversion-authority.md).

Auth HTTP routes этого slice:

| Route | Proof / request | Result |
|---|---|---|
| `POST /api/v1/auth/sdk/authorizations` | SDK Bearer; `idempotencyKey`, `redirectUri`, `codeChallenge`, `state`, `scopes`, `deviceProof` | request ID, policy revision, display/scopes/expiry |
| `GET /api/v1/auth/sdk/authorizations/{requestId}` | Current regular Voice Bearer | authoritative consent view, game context, no mutation |
| `POST /api/v1/auth/sdk/authorizations/{requestId}/approve` | Current regular Voice Bearer; `profileId`, `policyRevision` | one-use code, redirect with original state, expiry |
| `POST /api/v1/auth/sdk/authorizations/{requestId}/exchange` | `code`, `redirectUri`, `codeVerifier`, `deviceProof` | linked-bootstrap credential |
| `POST /api/v1/auth/sdk/authorizations/linked-session` | Linked Bearer; `deviceProof` | current scoped claims, no returned raw token |

Linked-session possession payload:
`voice-sdk-linked-v1\n<SHA-256 linked token>`. This read checks source/device
lifecycle and generation, target epoch/logout, policy and profile revisions;
source bootstrap session expiry alone does not extend or revoke a linked grant.
Linked expiry is capped by the approval Voice token expiry. Unknown authority
fields are rejected; malformed requests return coarse `400`, invalid proof or
authority `401`, changed idempotency payload `409`.

Auth хранит authorization request/code/consent/grant отдельно от guest conversion.
Владелец User предоставляет отдельный read-only eligibility contract; до его
production adapter approval/exchange закрыты. Fake policy/eligibility допустимы
только в тестах. Scoped token не считается production-ready до binding
authority consumer и публичных Gateway limits. Direct Voice-only first login
без предварительного sdk-account остаётся последующим вариантом того же
authorization contract; этот slice не объявляет весь T14 завершённым.

#### Замороженный Auth identity slice GAME-AUTH-01

Это замороженный Auth-only контракт для реализации **T13a**, а не заявление,
что целиком закрыт G01 или готовы весь T03/T04/T06. T13a реализует только
локальный `sdk-account` principal, таблицы и ограничения ниже, проверку
Google OIDC вместе с отдельным app/env game ticket, одноразовый nonce/device
challenge, replay denial, bootstrap credential/session/revoke и app/env
uniqueness/admission cap. Для детерминированных тестов verifier принимает
тестовые JWKS/clock и синтетические подписанные fixture tokens; эти fixtures
не заменяют отдельную real-Google acceptance.

T13a использует текущую operator configuration как admission source. Он не
подключает GIS registry, не добавляет Gateway маршруты и не включает capability
в deployment; документированные Auth paths остаются unpublished до отдельного
registry/Gateway activation gate. T13a не включает браузер/PKCE/consent и
выбранный User profile (T14), per-device actor signing keys (T15), conversion,
межсервисную trust matrix, transfer/recovery/history/conflict policy или
production admission. Эти более широкие вопросы остаются в G01 и родительских
T03/T04/T06 либо в последующих Auth slices. Их не следует трактовать как
неявно решённые этим Auth-only контрактом.

G01/Q11: первый независимый provider — Google OpenID Connect, issuer строго
`https://accounts.google.com`, RS256 и ключи только из Google JWKS. Для каждого
app/env оператор Voice регистрирует отдельный **Voice-owned** Google client ID;
developer не управляет issuer, JWKS или client ID. Google subject удостоверяет
пользователя, а не владение персонажем: соответствие игровому subject отдельно
подтверждает Game Integration Service. Email, имя и avatar не импортируются.
Обязательны единственный ожидаемый `aud`, совпадающий `azp` при наличии, непустой
`sub` длиной не более 255 символов,
`nonce`, `iat` не старше 5 минут и не более 30 секунд в будущем, неистёкший `exp`.
Источник protocol semantics: [Google OIDC](https://developers.google.com/identity/openid-connect/openid-connect).

Google — первый поддержанный provider, не исключительное продуктовое обещание.
Помимо него exchange **обязательно** проверяет отдельный RS256 game ticket
ключом backend из operator app/env registry: `iss=game:<application UUID>:<environment UUID>`,
`aud=voice:sdk-enroll`, `sub=<game subject>`, тот же `nonce`, `iat`/`exp`
с теми же freshness bounds, `independent_subject_hash=SHA-256(issuer + LF + sub)`
проверенного Google proof. Это связывает два доказательства и device challenge.
Скомпрометированный backend может лгать о game subject/roster, но не выпускать
Google proof, менять device key или получать permanent Voice credentials.
Game subject является контекстом session; назначение binding и его uniqueness
остаются Game Integration Service, а не доказательством со стороны Google.

Auth HTTP surface первого среза: `POST /api/v1/auth/sdk/challenges`
принимает `applicationId`, `environmentId`, публичный ES256/P-256 JWK;
возвращает `challengeId`, `nonce`, `clientId`, `expiresAt`. Challenge живёт
5 минут и неизменно связывает app/env/provider audience/device key.
`POST /api/v1/auth/sdk/exchange` принимает `challengeId`, `providerToken`, `gameTicket`,
`deviceProof`: compact ES256 JWS с payload **в точности** UTF-8
`voice-sdk-enroll-v1\n<challenge UUID>\n<nonce>` и зарегистрированным ключом.
Auth атомарно consumes challenge, создаёт/находит sdk identity, регистрирует
устройство и возвращает `accountId`, `actorId`, `deviceId`, `accessToken`,
`expiresAt`, `accountType=sdk-account`. Повтор exchange, включая потерянный
ответ, отвергается; новый независимый login восстанавливает ту же identity.
Новый device требует нового independent login; отозванный ключ нельзя оживить.

Первый credential — случайный opaque 256-bit token, в БД только SHA-256,
TTL 5 минут, purpose `sdk-identity-bootstrap`. Он не JWT обычного Voice,
не refresh token и не даёт chat/voice access. `POST /api/v1/auth/sdk/session`
проверяет Bearer token **и** ES256 proof с точным UTF-8 payload
`voice-sdk-session-v1\n<SHA-256 lowercase hex token>`; возвращает только
собственные app/env/account/actor/device IDs. Это read-only проверка владения
bootstrap credential; replay не создаёт эффект. `POST /api/v1/auth/sdk/revoke`
с token и proof `voice-sdk-revoke-v1\n<token hash>` отзывает текущее устройство
и все его sessions. Повтор после revoke отвергается. Обычные Voice endpoints
не принимают этот opaque credential. Все identity denial ошибки одинаковы.

Хранилище Auth: отдельные `sdk_identities`, `sdk_devices`, `sdk_challenges`,
`sdk_sessions`; это отдельный тип principal, без строки legacy guest, пароля,
refresh или автоматического User profile. Уникальность app/env/issuer/sub;
случайный `actorId` стабилен только внутри identity и не является `profile_id`.
В этом срезе admitted app/env задаются operator configuration, с cap 1000
identities на app/env и не более 10 активных устройств на identity; issuance
проверяет admission повторно. Registry consumer заменит конфигурационный список
доверенным контрактом; конфигурация неизменяема в процессе, исключение app/env
из списка после restart прекращает challenge/exchange/session. Online
suspension registry и propagation требуют следующего slice до production.
Не публиковать маршруты в Gateway до его rate limits и registry activation gate.

Q03: ownership generation начинается с 1. Не угадываем transfer по IP/email/
новому устройству; Google `sub` не передаётся по обычному flow. При заявленном
compromise/transfer identity блокируется; отдельная reviewed recovery operation
закроет прежние grants и создаст новое generation без private history/consent.
Автоматическое обнаружение смены человека за provider account не обещается.

Q07: до conversion один private app-scoped actor, без дополнительного User
profile. Existing-target conversion выбирает уже существующий разрешённый
profile, поэтому заполненный лимит 2/5 не создаёт обхода или extra profile.
New-target conversion создаёт один primary profile через User API. Public
payload будущих roster/cards/presence не должен сериализовать account ID,
provider subject или скрытые permanent profiles.

Q08: до доказанного linking account-wide voice применяется отдельно к каждому
sdk-account. Conversion блокирует новые source admission, требует Voice receipt
о завершении source session; занятый target возвращает `VOICE_CONFLICT` до
commit, либо получает отдельное явное handoff consent. Reconnect и lease renewal
проверяют новую generation; mute/deafen не снимаются переносом.

Q10: revoked device public key/thumbprint, retired/deleted identity UUID,
app/env/issuer/sub fence и ownership generation сохраняются без TTL как
минимальный security tombstone; это pseudonymous anti-resurrection data, не
public profile. Provider login не снимает suspension/deletion/retirement.
Raw provider token никогда не сохраняется. Истёкшие challenges/sessions можно
удалять после expiry; текущий срез не делает erasure/restore. Backup restore
перед serving обязан сверить tombstone generation с внешним durable lifecycle
journal; пока такой journal не подключён, восстановленная Auth DB не включается
для этой capability. Raw provider subject pseudonymization и key-retention
механизм должны пройти отдельный deletion slice до production admission.

План следующих executable slices (не реализовано GAME-AUTH-01):

1. GAME-AUTH-02: registry admission + Gateway limits, Voice-owned browser
   redirect/code+PKCE/consent и User actor contract; реальная Google acceptance
   с отдельными sandbox/prod client IDs, allowed redirect/origin, двумя device
   keys и nonce replay. Без credentials остаётся только crypto fixture evidence.
2. GAME-AUTH-03: durable conversion operation/status/hash/expected revision;
   source independent proof + device possession; новый permanent target через
   существующую email registration/verification и User primary-profile RPC;
   preview/consent, source fence, Voice receipt, owner receipts, target activation.
   Tests: crash/retry каждого перехода, email pending, account bans, old token.
3. GAME-AUTH-04: existing permanent proof через browser+PKCE, explicit existing
   profile, conflict preview; occupied binding → 409, no replacement/union;
   тот же журнал и owner receipts. Tests: full profile limit, concurrent targets,
   hidden profiles, target already in voice, source reconnect.
4. GAME-AUTH-05: unlink/delete/restore/recovery, HMAC subject tombstones и
   external monotonic journal, anti-resurrection на старом backup; identity
   lookup после conversion следует target binding только после permanent
   proof, а unlink никогда не воскрешает retired source. ID09–ID13/Q08/Q10
   закрываются только этими executable тестами.

Подтверждение исходного sdk-account включает proof-of-possession его device key
и независимую пользовательскую авторизацию; одного server-issued game ticket
недостаточно для конвертации или замены device key.

Требование владельца: `sdk-account` — отдельный от текущего `guest` тип с доказанным game
subject и ограниченным app/env доверием, с конвертацией как в новый, так и в
существующий постоянный аккаунт. `ConvertGuest` не является готовым контрактом
для этой операции. Наличие identity не означает подтверждённый email или
неограниченный user credential. Auth остаётся центральным владельцем;
game service и federated node не создают постоянный аккаунт от имени игрока.

Предлагаемый протокол реализации (детали G01):

1. Начать durable conversion operation с idempotency key, исходной identity,
   expected binding revision и краткоживущим доказательством game subject.
2. В Auth зарегистрировать новый аккаунт либо аутентифицировать существующий;
   при существующем выбрать целевой профиль и подтвердить оба владения.
   Browser/PKCE flow и challenge связывают это с конкретной операцией.
3. Подготовить preview: переносимые game bindings, актуальные memberships,
   политика истории, конфликты. Никакого поиска/merge по email или имени.
   Если binding уже занят, вернуть конфликт без автоматического replacement;
   отдельный flow разрешения конфликтов утверждается в G01.
4. После подтверждения закрыть новые admission исходной identity и увеличить
   authority epoch. Проверить lifecycle/баны обеих сторон и заново вычислить
   grants выбранного профиля из актуального roster, без объединения рангов.
5. Выполнить перенос через журналируемые идемпотентные стадии владельцев данных.
   После необходимых receipts активировать целевую связь и выдать новые scoped
   credentials. Старые tokens/соединения отозвать; исходную identity оставить
   retired reference для аудита и исторического авторства, без самостоятельного
   доступа. Повторный game login разрешается в целевой binding.

Две БД не считаются атомарно обновлёнными по одному HTTP-ответу. Во время
перехода возможна явная пауза общения, но не два параллельных активных владельца
одного binding. Crash/retry продолжает ту же operation; отмена до commit не
меняет связь, после начала переноса требует recovery до terminal state, а не
слепого возврата старых tokens. Status доступен инициатору через доказанный
контекст операции и не зависит от уже отозванного обычного игрового токена.

Старые сообщения не переписываются массово на нового автора; alias/tombstone
mapping сохраняет audit и не раскрывает другим людям скрытые профили. Доступ
к старой истории не расширяется самим фактом конвертации. Перенос не оживляет
старые карточки/команды и не обходит баны; уже admitted команды завершаются
по существующим правилам in-flight admission. Unlink после конвертации не
воскрешает retired identity. Правила последующего нового игрового входа,
потери provider account, retention mapping и восстановления — открытая часть G01.

The concrete owner revisions, receipts, state machine and recovery floor for
both conversion paths are fixed in
[game-conversion-authority](game-conversion-authority.md). До включения capability нужны схемы Auth/store, отдельная trust/permissions
матрица, conversion endpoints и согласованные conflict/history/recovery UX.
Этот раздел задаёт требования и предлагаемый протокол, а не доступный API.

### Авторство сообщений пользователя

Принятая владельцем гарантия: серверный credential студии не позволяет отправить
сообщение от лица игрока; запрос приходит от авторизованного игрового клиента.
Это не гарантия намерения человека против злонамеренного игрового executable,
который контролирует SDK process и может сам вызвать отправку. Для такой более
сильной гарантии потребовался бы независимый доверенный UI подтверждения.

SDK создаёт отдельный device key для app/env, хранит private key в доступном
OS secure storage и регистрирует public key после независимого user proof.
Game backend, bot и node не получают этот private key, permanent credentials
или возможность выпускать player tokens. Сообщение подписывает канонический
envelope с actor/binding/device, app/env, chat, message ID, content digest,
authority revision, audience и freshness/replay fields. Receiver проверяет
подпись, актуальное admission и dedupe; та же message ID с иным содержимым
отвергается. Подпись не заменяет membership, scope или rate limit.

Нода проверяет user signature по master-authorized device binding, а не по
ключу оператора ноды; происхождение должно оставаться проверяемым получателем
через доверенные master keys. Wire format, алгоритм, canonicalization, key
rotation/recovery и receiver verification фиксируются в GI0/G01 до реализации.
Игра без независимого способа подтвердить пользователя не получает capability
создания sdk-account на основании одного собственного service credential.
Конвертация сохраняет app-scoped полномочия и не экспортирует ключи постоянного
аккаунта. Подпись удостоверяет происхождение, но не скрывает текст сообщения.

### Предлагаемые scopes

| Scope | Ограничение |
|---|---|
| `game.identity.read` | Только app-scoped subject и выбранный публичный профиль |
| `game.sessions.manage` | Server-only; собственные sessions, не произвольные chats |
| `game.memberships.sync` | Server-only; mapping allowlist и managed grants |
| `game.chat.read` / `game.chat.send` | Только доступные app-linked chats и история разрешённого периода |
| `game.voice.join` | Admission комнаты; speak проверяется отдельно Role/Voice |
| `game.presence.write` | Только activity своей игры, с user privacy |
| `game.invites.create` | Только разрешённая session; не импорт списка друзей |
| `game.events.publish` | Game backend → включённая bot installation |
| `game.commands.receive` | Проверенные interactions своей installation |
| `game.notifications.send` | Только отдельный user opt-in и разрешённые категории |

Это новые app scopes, они не добавляются молча в существующий Bot manifest.
Bot scopes продолжают ограничивать bot actor; пересечение scopes, installation,
membership, lifecycle и privacy всегда сужает доступ. Game owner не получает
неявного admin/Owner и не может повысить лимиты заменой ключа.

## 4. Предлагаемый REST surface

Base namespace: `/api/v1/game-integrations`. Это **reserved design**, Gateway
пока не реализует эти маршруты. JSON содержит UUID strings, UTC RFC3339 timestamps,
opaque external IDs, enum strings. Все mutations требуют authorization;
доверенный service context выводит app/env из credential, не из тела запроса.

| Method + suffix | Principal | Request → response | Повторы / основные ошибки |
|---|---|---|---|
| `GET /capabilities` | Player или service | client platform/version → enabled features, versions, limits | Read; 426 unsupported client |
| `POST /bindings/challenges` | Game login proof | provider ticket, redirect, PKCE challenge → challenge/authorize URL/expiry | nonce-bound; 400/401/429 |
| `POST /bindings/exchange` | Code + PKCE | challenge, code, verifier → delegated grant, binding | Одноразовый code; 401 invalid/replayed |
| `GET /bindings/me` | Player | — → selected profile, own bindings/scopes | Только собственные; 401 |
| `GET /bindings/{binding_id}/authority` | Service своей app/env | — → active/revoked, revision, разрешённый character context | Execution-time check, no shared cache beyond authority deadline; 403/503 |
| `DELETE /bindings/{binding_id}` | Владелец player | expected revision → revocation operation | Идемпотентно; 403/409 |
| `POST /sessions` | Service | external key, kind, parent party, policy, desired members → operation | Idempotency-Key; 409 mismatch |
| `PUT /sessions/{id}/roster` | Service | complete snapshot, source revision, bound members → operation | CAS/revision; 409 stale/conflict |
| `GET /sessions/{id}` | Authorized member/service | — → current state, applied roster revision, resource refs | 404 for inaccessible resources |
| `POST /sessions/{id}/join-grants` | Player | device instance, expected binding revision → bounded chat/media capability | Current admission; 403/409/503 |
| `POST /sessions/{id}/close` | Service | reason, expected revision → operation | Stable terminal state; 409 |
| `POST /sessions/{id}/keep-group` | Player | consenting participants, session revision → group operation | Individual consent; 403/409 |
| `POST /community-bindings` | Service + admin approval | external corporation key, allowed Space template → operation | Provision once; 403/409 |
| `PUT /community-bindings/{id}/roster` | Service | complete roster and ranks, source revision → operation | Snapshot rules; 409/422 |
| `POST /events` | Service | event envelope from bot spec → event/message operation | event_id dedupe; 409/422 |
| `GET /operations/{id}` | Initiator/authorized service | — → durable status, result IDs or error | Не раскрывает чужие operations |

Конкретные chat send/history и voice calls идут через существующие доменные
Gateway endpoints с новым ограниченным admission. Не вводится вторая модель
сообщений «только для SDK». Bot card actions описаны отдельно; публичный player
не может вызвать service-only roster sync.

Пример запроса создания (поля — proposal):

```json
{
  "external_session_key": "herdtrip:party:example-42",
  "kind": "party",
  "policy": {"history": "since_join", "persist_after_match": true},
  "members": [{"binding_id": "<uuid>", "character_binding_id": null}]
}
```

Ответ mutation обычно `202 {operation_id, status:"pending"}`. `201` допускается
только после durable завершения создания; `200` — прочитанный результат или
повтор завершённой операции. Ни 202, ни webhook ACK не означают игровой успех.

## 5. Idempotency, порядок и частичные сбои

`Idempotency-Key` scoped by principal/app/env/route и привязан к нормализованному
request hash. Повтор тех же данных возвращает тот же operation/result; тот же
ключ с иным body → `409 IDEMPOTENCY_CONFLICT`. Retention ключей/операций задаётся
до пилота и не короче документированного retry window. Долговечный external key
ресурса продолжает защищать от дубликатов после истечения transport-key cache.

Orchestration сохраняет intent, resource refs и stages. При падении после
создания Chat и до Voice provisioning worker продолжает тот же operation.
Session становится active только после согласованных ресурсов и membership;
временно созданный чат не публикуется чужим участникам. Cleanup удаляет лишь
ресурсы этой операции; общий party-chat не удаляется из-за неудачи нового match.

Roster snapshot атомарно сравнивает `source_revision`: ниже применённой — stale
no-op со статусом; та же revision + иной body → conflict; выше — новый desired
state. Частичные страницы собираются под snapshot ID/checksum и активируются
только целиком. Неудачный fetch/пустая страница не равны пустому roster.
Deletes представлены явным complete empty snapshot или tombstone.

Потеря membership немедленно закрывает новые admission; propagation к текущим
подпискам/медиа проверяется по принятому revocation SLO. `applied_revision`
публикуется только после требуемых consumer acknowledgements. При неопределённой
authority привилегированные операции fail closed. После перерыва — полная сверка.

## 6. Ошибки и recovery

Общий error body: `error_code`, безопасный `message`, `request_id`, `retryable`,
optional `operation_id`, `retry_after_ms`, `current_revision`. Tokens, чужие IDs,
permission details приватного ресурса и исходные provider claims не возвращаются.

| HTTP / code | SDK / backend |
|---|---|
| 400 `INVALID_ARGUMENT` | Исправить schema; не retry циклом |
| 401 `TOKEN_EXPIRED` | Single-flight refresh; один повтор с тем же mutation key |
| 403 `GRANT_REVOKED` / `SCOPE_DENIED` | Очистить scoped state; не auto-relink |
| 404 `RESOURCE_NOT_FOUND` | Единый ответ для отсутствующего/недоступного чужого ресурса |
| 409 `STATE_CONFLICT` / `STALE_REVISION` | Read authoritative state; не blind overwrite |
| 410 `RESOURCE_RETIRED` / `ACTION_EXPIRED` | Terminal; показать пользователю |
| 422 `CAPABILITY_UNAVAILABLE` | Fallback UI, без попытки угадать старый protocol |
| 426 `SDK_UPGRADE_REQUIRED` | Отключить affected feature, сообщить требуемую версию |
| 429 `RATE_LIMITED` | Retry-After, bounded queue, per-app quota feedback |
| 503 `AUTHORITY_UNAVAILABLE` / `NODE_UNAVAILABLE` | Backoff; не расширять доступ и не создавать замену ресурса |

Timeout после отправки mutation = unknown outcome. Сначала status/read с тем же
operation/key, затем допустимый retry. Сервер игры имеет собственный command
inbox и атомарную dedupe в транзакции игрового эффекта.

## 7. Realtime и присутствие

SDK использует REST Gateway и `/ws` через Realtime. `s`/`resume` относятся к
соединению, не к истории всех сообщений. После reconnect SDK сверяет разрешённые
sessions/chats, затем догружает историю Messaging по cursor каждого `chat_id`.
Сверка ограничена grant приложения; полный пользовательский inbox игре не виден.
Pagination cursor opaque и scoped by query/app/recipient/snapshot; default/max
page sizes публикует capabilities. Ошибка страницы не очищает локальное состояние.

Presence различает «игра запущена», «в меню», «в матче» и доступность в Voice.
Сервер проверяет, что приложение пишет только свою activity. Невидимость/DND
не меняются игрой, координаты и приватные lobby secrets не публикуются.
Invite — короткоживущий одноразовый либо ограниченный use-count opaque reference
на session, с серверной проверкой при accept. Deep link не несёт bearer token.

## 8. Данные и удаление

Owner stores хранят только необходимые связи; внешние provider subjects не
публикуются другим игрокам, логи их редактируют/псевдонимизируют. Персонаж в UI
показывается по разрешённому alias, без раскрытия других профилей владельца.
Удаление binding отзывает credentials, managed grants и подписки событий;
история сообщений следует lifecycle владельца, а не каскадному SQL delete.

Account/Space deletion, freeze и purge используют существующие lifecycle fences
и receipts; Integration и Bot обязаны быть учтены как потребители до активации.
Новая storage ownership/migration inventory добавляется в DATA_STORES только
в implementation milestone. Отзыв игры закрывает новые execution admissions;
ранее допущенные in-flight команды могут завершиться по правилам ниже.

## 9. Совместимость и admission разработчика

Перед расходованием игровых ресурсов service проверяет binding authority и
запрашивает execution admission. Предлагаемый контракт:
`POST /api/v1/game-integrations/commands/{command_id}/admission` для owning app/env/installation возвращает
одноразовый permit с command/action/binding revision, `start_before` и
`complete_before`. Voice сериализует revoke и admission по одной authority epoch
в своей транзакции. Revoke первым → reject; admission первым → команда in-flight
и может завершиться после unlink в согласованном временном окне. Это явно
показывается при отключении игры; отмена уже допущенной команды не гарантируется.

Игра атомарно сохраняет command/permit dedupe и игровой эффект в своей БД,
проверяет start/completion bounds и игровую state version. Retry admission той
же команды возвращает тот же permit и не продлевает окно. Late/new execution
требует новой проверки; expired permit не оживает при повторе webhook. При
потере ответа Voice admission status восстанавливается по command ID; нельзя
выполнить эффект по догадке. Между БД Voice и игры нет distributed atomic commit.
Простой GET или offline token check эту семантику не заменяют. G13 defaults для
первого controlled backend: admission permit сохраняет отдельный
`permit_issued_at`, а `start_before = permit_issued_at + 10s`,
`complete_before = permit_issued_at + 60s`; обе границы exclusive (`now < bound`).
10 секунд ограничивают задержку между admission и началом побочного эффекта;
60 секунд — верхняя граница короткой атомарной мутации и commit, чтобы оставить
сеть/транзакции ограниченный jitter без долгоживущего permit. Это proposed
defaults, не измеренная производительность и не срок игровой задачи: команда,
создавшая длительную задачу, завершается после атомарного принятия задачи.
Одна команда получает один permit. Повтор admission возвращает тот же permit и
исходные сроки; повтор не продлевает их. Admission unavailable → новое исполнение
не начинается. Проверки на границах `bound-1`, `bound` и `bound+1` обязательны.
Receipt о ранее committed результате принимается отдельным узким completion
правом, без возможности создать новую команду. Полностью отозванный/скомпрометированный
service credential не оживляется ради receipt: status требует доверенного
reconciliation с оператором игры и остаётся unknown до доказательства.

### T03/T06 command contract freeze (proposed v1)

Эта секция задаёт будущий wire contract; перечисленные command routes ещё не
реализованы и не входят в текущие GIS credentials. Внешняя команда доставляется
POST на HTTPS callback URL конкретной installation. GIS-owned player routes:
`POST /api/v1/game-integrations/actions/{action_id}/invoke`;
`POST /api/v1/game-integrations/commands/{command_id}/admission`;
`POST /api/v1/game-integrations/commands/{command_id}/result`;
`GET /api/v1/game-integrations/commands/{command_id}`. Admission/result требуют
будущего отдельного `game.commands.execute` scope; status GET использует
`game.commands.read`; outbound command дополнительно требует installation
capability `game.commands.receive`. Это не добавляет endpoint к существующему
Bot API и не даёт command handler доступа к production GIS registry/DB.

Wire format — `application/vnd.voice.game-command+json;version=1`, UTF-8 JSON
по RFC 8785 JCS, без BOM. Receiver rejects body bytes unless they exactly equal
UTF-8 JCS serialization; duplicate JSON keys, non-integer numeric values,
alternate escapes/whitespace and trailing bytes are invalid. UUID — lowercase
hyphenated canonical form, version is integer `1`, times are integer Unix
seconds UTC. Unknown security/action enums are rejected. HMAC-SHA256 uses the
exact 32 secret bytes of the installation key. Headers: exactly one
`Content-Type` above, `X-Voice-Key-Id` (canonical UUID), `X-Voice-Timestamp`
(unsigned decimal Unix seconds without leading zeroes), and
`X-Voice-Signature: v1=<64 lowercase hex>`. The timestamp is unsigned canonical
decimal Unix seconds and must be within ±300 seconds of receiver clock. The
request MAC input bytes are exactly UTF-8 encoding (LF=`0x0a`, no terminal LF) of
`v1\n{UPPERCASE_METHOD}\n{canonical_path}\n{timestamp_seconds}\n{key_id}\n{lowercase_sha256_hex(canonical_body_bytes)}`;
`canonical_body_bytes` are the exact request bytes that passed JCS equality.
`canonical_path` is the exact raw path component of the HTTP origin-form
request-target before route parsing or decoding. It is ASCII only, contains
nonempty segments of literal RFC 3986 unreserved bytes (`A-Z`, `a-z`, `0-9`,
`-._~`) separated by single `/` bytes, and has no query or fragment. Reject all
percent signs/percent-escapes, non-ASCII and other reserved path bytes, empty
segments, and `.`/`..` segments; never normalize or decode the path before
signing. Current route paths use literal static segments and canonical UUID
segments, so require no escaping.
Duplicate auth/content-type headers and redirects are rejected. Replays and
result outbox delivery use the originally stored canonical body bytes; only
timestamp and MAC are regenerated. The result idempotency hash is computed over
canonical body bytes and is not embedded in that body. Принимается только key id, привязанный к точным app/env/installation
и явно provisioned current/overlap key; overlap максимум 10 минут, explicit
revoke действует сразу. Timestamp skew максимум ±300 секунд; подпись сравнивается
constant-time. Secret bytes, Authorization, подпись и command arguments не
попадают в логи.

Command envelope содержит version, `command_id`, `operation_id`, `invocation_id`,
`action_id`, source `message_id`/`card_revision`, exact app/env/installation,
profile/binding revision, actor proof, game state version, canonical typed
arguments, `issued_at`, `expires_at`. Result envelope содержит version, original
IDs, `result_id`, terminal status, resulting state version и safe summary.
Изменение результата с тем же ID — 409. Ниже request/result body записаны
точными JCS UTF-8 bytes: object keys отсортированы по правилу RFC 8785,
без пробелов и завершающего newline.

```json
{"action_id":"00000000-0000-4000-8000-000000000004","actor_proof":{"profile_id":"00000000-0000-4000-8000-000000000008","proof_id":"opaque"},"app_id":"00000000-0000-4000-8000-000000000005","arguments":{"encounter_id":"encounter-42"},"binding_revision":8,"card_revision":2,"command_id":"00000000-0000-4000-8000-000000000001","environment_id":"00000000-0000-4000-8000-000000000006","expires_at":1790500120,"installation_id":"00000000-0000-4000-8000-000000000007","invocation_id":"00000000-0000-4000-8000-000000000003","issued_at":1790500000,"message_id":"00000000-0000-4000-8000-00000000000b","operation_id":"00000000-0000-4000-8000-000000000002","schema_version":1,"state_version":"encounter-42:v3"}
```

```json
{"command_id":"00000000-0000-4000-8000-000000000001","operation_id":"00000000-0000-4000-8000-000000000002","result_id":"00000000-0000-4000-8000-000000000009","schema_version":1,"state_version":"encounter-42:v4","status":"succeeded","summary":"Олень приручён"}
```

Детерминированный test-only signature vector (не deployment credential): fake
HMAC key — ровно 32 bytes
`000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f`; key ID
`00000000-0000-4000-8000-00000000000a`; timestamp `1790500000`; method `POST`;
canonical path `/callbacks/game-commands/v1`. SHA-256 canonical command body:
`dcdee56ea87770fd52707199e0f2f29fa7cbfb17e806b6c248308728c607e0a1`. Exact MAC
input is UTF-8, LF-separated, with no terminal LF:

```text
v1
POST
/callbacks/game-commands/v1
1790500000
00000000-0000-4000-8000-00000000000a
dcdee56ea87770fd52707199e0f2f29fa7cbfb17e806b6c248308728c607e0a1
```

HMAC-SHA256 output is
`db75a725f07e9eedadb3a511f2ee871f640bfe395fac27168c0302ee8f6982e9`, encoded
as `X-Voice-Signature: v1=<lowercase hex>`. Body hash is lowercase SHA-256 hex;
signature is lowercase HMAC-SHA256 hex; key ID is canonical lowercase UUID;
timestamp is canonical decimal; MAC input is UTF-8. All key/IDs are synthetic.

`command.issued_at` records command creation and does not start a permit window.
On first successful admission, GIS serializes with revoke, samples its DB clock
after taking the authority-epoch lock as canonical integer Unix seconds UTC
`permit_issued_at`, and persists that timestamp, a stable `permit_id`, and
the exclusive +10s/+60s bounds in one transaction. A usable permit is returned
only after commit; rollback creates no permit. A first command delivery delayed
to retry slot t=15 or t=31 can therefore obtain a fresh permit epoch at its
successful admission, if command expiry and authority still allow it. After a
permit row exists, every admission retry or lost-response recovery returns the
same stored ID, epoch and bounds; it never mints a new epoch or extends time.
If start-before is missed, that command expires without effect; user must invoke
again and repeat authorization/confirmation to create a new command. Revoke and
first permit insert lock the same authority epoch: revoke commit first denies
admission; permit commit first allows only that admitted operation to start
before its stored +10s bound and commit before its stored +60s bound. This grace
cannot be renewed by redelivery, admission retry, or a later command. Contract
tests must cover first delivery at t=0/15/31, admission retry before/after both
bounds, lost response, rollback, and both revoke/insert lock orderings using DB
time. The receiver contract suite must assert the exact canonical body SHA-256,
MAC input bytes, and HMAC output in the synthetic vector above, plus malformed
JCS/body bytes, duplicate headers/JSON keys, timestamp skew at ±300s, key overlap
through 10 minutes and rejection immediately after explicit revoke. These are
contract proofs required of implementation, not evidence that runtime support
already exists.

Voice commits accepted command + outbox before returning 202. Delivery attempts
are scheduled at elapsed `0, 1, 3, 7, 15, 31` seconds (six total), each with a
3-second response deadline; every retry carries identical canonical body and
IDs, with a fresh timestamp/signature. `202` means durable inbox acceptance and
stops command delivery retries. 429 `Retry-After` is honored only if another
attempt's fixed slot has not passed and it can finish before terminal deadline;
it never creates a new slot or extends the deadline. There is no seventh attempt: after
attempt six fails, keep command `reconciling` and its durable record until
`min(command.expires_at, accepted_at + 120s)`, then move it to DLQ/operator
reconciliation. An earlier command expiry prevents later sends. Result delivery
uses the same six-attempt schedule, 3-second bound and 120-second terminal
deadline; the immutable receipt remains in result DLQ until reconciliation.
Restart restores attempts, original deadline and immutable body from durable
outbox; it does not reset the schedule or bootstrap over existing rows.

| Response/failure | Sender action |
|---|---|
| 202 durable acceptance | Stop delivery; poll status/reconcile by same command ID |
| 401/403 signature, key, scope or authority | Stop automatic retries; operator-visible configuration/security error |
| 409 ID/body/revision/state conflict | Stop retries; reconcile immutable inbox/result records |
| 410 expired/revoked | Terminal for new admission; retain status/receipt evidence |
| 422 unknown schema/action/security enum | Stop retries; require compatible sender/receiver |
| 429, timeout, network failure, 5xx | Retry only on fixed schedule and before deadline |

At the game receiver, a single DB transaction inserts unique command/permit
receipt, checks actor/binding/character/state and one-shot consumption key,
applies effect, and inserts immutable result outbox. Unique command ID returns
the previous receipt/result; a different command ID colliding on the same
one-shot action is rejected without a second effect. Crash before commit leaves
no effect and permits same-ID redelivery; crash after commit recovers and resends
the saved receipt only. This is at-least-once transport plus transactional
idempotency, never transport exactly-once. Revoke and permit issue serialize on
the same authority row: revoke commit first denies issue; issue commit first
allows only that operation to start before its stored +10s bound and commit
before its stored +60s bound. Delivery tests must exercise both lock orderings,
both bounds, duplicate races, crashes before and after commit, exact retry
timestamps, 3-second stalled body, and 120-second DLQ boundary with a
controllable clock. These are contract proofs required of implementation, not
evidence that runtime support already exists.

Portal показывает capabilities, app/env keys, allowed origins/redirects,
webhook status, quotas, billing status (если появится), SDK version support и
review requirements. Sandbox не имеет production bindings, tokens или данных.
Portal — будущий инструмент вне спринта; соответствующие registry/provisioning/
status API и серверная авторизация входят в Voice и проверяются test harness.
Production admission проверяет identity/revoke, mute/report, limits и корректное
поведение сбоев; конкретные коммерческие условия — G07.

### T12 registry endpoints and security defaults

The current registry exposes three owner/operator routes:

| Route | Access and behavior |
|---|---|
| `POST /api/v1/game-integrations/applications/{app_id}/environments/{env_id}/installations` | Regular app owner; exact body `{"callback_url":"..."}` plus `Idempotency-Key`. The IDs come from the path and bearer, and the target environment must be active. |
| `PUT /api/v1/game-integrations/applications/{app_id}/suspension` | Configured regular operator only; exact body `{"suspended":true|false}` plus `Idempotency-Key`. App state and audit update atomically; replays return the saved status/revision snapshot. |
| `GET /api/v1/game-integrations/applications/{app_id}/diagnostics` | Regular owner only; returns safe app/env/install state, quota state, provenance, and at most 20 newest sanitized audit events. |

The selected T12 registration quota is 120 attempts per application per UTC
minute, shared by sandbox and production environments. The 121st request gets
`429 RATE_LIMITED` and integer `Retry-After` to the next UTC-minute boundary;
quota-denial audit rows coalesce by application and minute. This bucket covers
only installation registration. Diagnostics and other current registry routes,
operator operations, and future T15 delivery do not consume it. Invalid bearer
requests do not write registry, quota, or audit state. An authenticated
cross-scope or unsafe-destination denial is audited without partial installation
or credential writes; foreign-owner denials do not consume quota.

Callback admission accepts canonical HTTPS on port 443, with no userinfo,
query, or fragment and only literal ASCII unreserved path segments. It resolves
DNS at registration, rejects any private/special-use answer, and repeats
validation while pinning the actual connection to an approved resolved IP.
TLS verifies the original hostname. Redirect responses are terminal and cause
no second request. T12 stores this installation-scoped callback and provides
the safe transport primitive; command dispatch remains T15 work and must use
that primitive.

Suspended application credentials and Auth policy lookups fail closed with
`503 APP_SUSPENDED`; a lifecycle transition blocked by the current state returns
`409 APPLICATION_STATE_CONFLICT`. A successful restore returns the stored
pre-suspension state and does not reactivate revoked credentials or separately
suspended/retired environments. Diagnostics never returns callback URLs,
credential material/digests, OAuth subjects/assertions, provider proof,
request payloads, or secrets. `developer_asserted`, `operator_approved`, and
`provider_verified`/`provider_admitted` are distinct provenance values. T12
records the first two only and reports provider admission as `not_verified`
without independent provider evidence. Audit `result` values are `success` and
`denied`; legacy operator `approve_sandbox` rows are backfilled as
`operator_approved`, while other old rows remain `system` / `success`.

API major version не меняется молча; optional additive поля игнорируются только
если не влияют на authority. Неизвестный security/permission enum запрещает
операцию. Deprecated SDK получает migration guide и срок поддержки, который
должен быть принят перед публичным release. Контракт не требует включать друзья,
presence или invites ради одной voice capability: модульность — цель Voice.
