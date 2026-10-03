# Game integrations — повторный аудит решений

Дата: 2026-09-26. Проверены документы, не runtime. Источник принятых решений —
уточнения владельца в текущей задаче. Независимый read-only reviewer проверил
identity, federation и bot contracts; результаты сверены с основным аудитом.

## Итог и scope

Один спринт реализует Voice: Game Integration Service, Auth/User sdk-account,
обе конвертации, внешние API и проверку авторства, мессенджер, managed communities,
ботов/карточки/уведомления, federation и единый Voice Node distribution.
Unity/Unreal/прочие assets, SDK wrappers, engine media/UI adapters, developer
CLI/portal и публикуемые примеры — отдельная задача. Серверные registry/API
остаются в scope. Test clients и controlled game adapter — внутреннее средство
приёмки, не обещание готового SDK. Средства установки/обновления Voice Node
остаются частью серверной поставки.

Принятые 15 продуктовых направлений записаны в
[feature canon](../features/game-integrations.md#принятые-владельцем-продуктовые-правила).
Их не нужно заново согласовывать. T03 has now adopted the remaining technical
defaults in the owning API and Federation contracts. Entries below distinguish
closed decision gates from runtime, measurement, and live-provider acceptance
that remains assigned to consumer tasks.

## Исправленные противоречия

- Убраны Unity/Unreal packages и packaged builds из результата спринта Voice;
  GI4 теперь принимает внешний протокол, SDK01/02/04 вынесены в задачу tools.
- Готовность API не доказывает готовность engine SDK: проверки серверной стороны
  выполняются test client ↔ messenger с реальным media и durable game effects.
- Game Integration Service закреплён как отдельный сервис, а не открытый выбор G06.
- `sdk-account` — отдельный тип, не legacy guest; обе конвертации обязательны.
- Game-server ticket не считается независимым доказательством пользовательского
  авторства. Device proof и service/node authority — разные проверки.
- Дополнительное подтверждение опасной команды больше не обозначено безусловно
  optional; Q05 specifies risk classes, a five-minute single-use challenge,
  canonical argument binding, and atomic challenge/command consumption.
- Voice Node поставляется одним bundle с single-instance компонентами;
  это не обещание HA или объединения внутренних доверенных identities.

## Decision status and remaining acceptance

Rows marked `Принято` or `Решение заморожено` record adopted contract decisions;
they do not claim code or runtime acceptance. Consumer tasks remain responsible
for implementing each rule and passing its listed tests. Any item still called
a proposal is a measured qualification target, not a product contract.

### Q01. Повторное вступление и интервалы истории

Основание: [party history](../features/game-integrations.md#партия-и-общение)
задаёт since_join, а SE04 проверяет нового участника. Первоначально не было
определено, возвращается ли история первого членства при rejoin, скрыт ли
период отсутствия и доступны ли цитаты и старые download URLs. Решение T32
ниже закрывает этот пробел отдельно от общего срока retention G04.

Решение заморожено для T32: entitlement проверяется по неизменяемому
времени создания сообщения и отдельному интервалу членства `[joined_at,
revoked_at)`: начало включено, отзыв исключён. Повторное вступление создаёт
новый интервал и не возвращает доступ к промежутку отсутствия или к прошлому
интервалу. Одно правило применяется к history, search, quotes/thread previews,
file metadata и download fetch-time authorization; создание URL само по себе
не переживает отзыв. Время берётся с серверного commit, клиентские часы не
используются. Acceptance: SE04 проверяет обе временные границы и все перечисленные
поверхности при join → leave → сообщения без пользователя → rejoin → revoke.

### Q02. Новые scopes и смена владельца приложения

Основание: [API scopes](../architecture/game-integration-api.md#предлагаемые-scopes)
и node registration описывают первоначальный consent, но не передачу приложения
другой студии, изменение оператора/назначения или расширение scopes.

Принято: consent revision scoped to account/app/environment/scope set. Scope
expansion, Owner generation, operator, or destination/trust-recipient change
requires fresh consent and immediately revokes old credentials/grants. Queued,
not-yet-admitted actions are cancelled or revalidated; admitted actions follow
only the immutable G13 permit window. Notification delivery dedupes by
recipient/category/source event and rechecks consent after quiet hours. Runtime
and consumer tests remain in T14/T58–T59; acceptance Q02/BOT09 covers each
change and queued-versus-admitted behavior. The owning defaults are in the
[T03 API freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).

### Q03. Новый владелец того же game subject или персонажа

Основание: [API model](../architecture/game-integration-api.md#2-ресурсы)
сохраняет stable external key, а current owner proof не отличает передачу аккаунта
от входа прежнего владельца. Продажа персонажа, перераспределение provider subject
или recovery у провайдера не должны переносить личную историю и consent.

Принято: one active character per app/env/sdk identity by default; multiple
characters require explicit environment policy and independent bindings,
consent, and grants. Ownership changes create a new generation and revoke old
bindings; without an authoritative provider transfer signal, transfer is never
inferred. A new owner receives no private history or grants. Runtime remains in
T17–T19/T37–T39; acceptance Q03/MMO02 covers same external ID/new owner, hidden
alternate, and no inherited history/authority. See the
[T03 API freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).
нет доступа к старому permanent account, ключам, истории и командам.

### Q04. Подписи edits, deletes и вложений на чужой ноде

Основание: [авторство](../architecture/game-integration-api.md#авторство-сообщений-пользователя)
покрывает отправленный envelope, но не versioned edit/delete, attachment digest
и показ старой подписанной версии вместо актуальной. Подпись не доказывает
полноту истории и сама не предотвращает сокрытие сообщений нодой.

Решение принято в [замороженном T15 контракте](../architecture/game-integration-api.md#frozen-t15-device-signature-and-message-revision-contract):
каждый create/edit/user-delete подписан device key как JCS/ES256 JWS; для
create/edit signature покрывает base64url исходных UTF-8 bytes и их digest.
Edits образуют hash chain. Moderator/system delete — отдельный EdDSA JWS,
подписанный Messaging после owning authorization, с фиксированными issuer,
audience, chat/message/revision/action/reason fields и ключом из master-pinned
Messaging JWKS over node mTLS. Attachment provenance связывает immutable File
revision, length и SHA-256 через подписанный manifest; без верифицируемого
manifest вложение fail-closed. Auth выпускает четырёхсекундное RS256 device-
status assertion через pinned Auth JWKS over node mTLS; node refresh и monotonic expiry с combined
clock uncertainty ≤250ms закрывают новые writes в ≤4.25s после revoke commit,
а истечение/partition fail closed. Точный сохранённый receipt читается до
expiry/current-authority checks только при byte-identical JWS и не создаёт
эффект; иные bytes конфликтуют. Receiver отвергает rollback после наблюдённой
более высокой revision и помечает равную revision с другим hash как
equivocation. Clients label stale/unverified content and suppress it if no
verified revision exists. Это не обещает полноту истории или обнаружение
неувиденного rollback. Acceptance: ID14–ID16, включая attachment mutation,
rollback, forged tombstone, revoked key и две расходящиеся реплики. Decision
gate Q04 закрыт; runtime и measured evidence остаются в T15/GI4/GI6.

### Q05. Обязательное подтверждение команды на сервере

Принято in [Bot interactions](../features/game-bot-interactions.md): explicit
allowlisted read-only actions alone may skip confirmation. Irreversible,
value/currency, one-shot, privacy/export, authority, external-message, and
unknown actions require a server-issued single-use 256-bit challenge; only its
hash is stored, lifetime is five minutes, and it binds actor/session epoch,
app/env/installation, source message/card revision, action, and canonical
argument hash. Confirm is authenticated and atomically consumes the challenge
with command acceptance; concurrent confirmations use CAS, one wins. Direct
invoke cannot bypass it; unknown risk/summary or challenge outage denies the
action. This proves protocol consent, not a physical click in a developer-
controlled executable. GI2/GI3 implement it; acceptance includes direct invoke,
replay, changed price/actor, 5m boundary, and competing confirmations.

### Q06. Отзыв прав под нагрузкой и целостность control plane

Основание: [Federation authority v1](../architecture/federation-authority-v1.md)
is the active contract. Before this T03 freeze it did not allocate propagation,
clock, ejection, or reserved control-plane capacity within the revoke bound.

Принято: revoke-to-eject ≤5.0s, allocated to authority propagation ≤2.0s,
combined clock uncertainty ≤250ms, and active media enforcement ≤2.75s after
observation. Nodes subtract the clock uncertainty from wall-clock expiry before
monotonic conversion; excess uncertainty fails closed. Command drain retains its stricter 4.25s bound. Revoke/freeze/lease
expiry use a separately bounded priority lane and reserved DB/worker capacity;
stale authority fails closed. T70–T78/T93 must prove these values under 2×
qualified event/snapshot load while unrelated Spaces remain available. The
canonical timing allocation is in [Federation authority G08/Q06](../architecture/federation-authority-v1.md#g08q06-revoke-budget)
and summarized in the [T03 API freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).

### Q07. Отображение sdk-account и ограничения профилей

Основание: [identity](../features/game-integrations.md#identity-и-приватность)
отделяет персонажа от профиля, но не задаёт постоянный profile ID sdk-account,
учёт лимита профилей при conversion и поведение при заполненном лимите target.
Глобальный profile ID/аватар может раскрыть больше, чем разрешённый игровой alias.

Принято: expose only stable opaque profile references scoped to app/environment;
never expose a global Voice profile ID or infer cross-app identity. The user
selects the app-approved alias; normalize it to NFC and limit it to 64 Unicode
scalar values. Avatar/hidden-profile fields require separate consent and app
policy. Apply the existing profile cap consistently to login, conversion,
roster, cards, search, and presence. T14/T17–T19/T38–T39 implement and prove
this; Q07/ID cases cover full profile cap, hidden profiles, two apps, conversion,
alias boundaries, and public payload inspection. See the [T03 API
freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).

### Q08. Account-wide voice при конвертации и отдельных sdk-accounts

Основание: одна account-wide voice session принята, но два ещё не связанных
sdk-account могут принадлежать одному человеку. Без доказанной связи Voice
не может считать их одной identity и не должен угадывать по устройству/IP.

Принято in [T03-AUTH-LIFECYCLE](../architecture/game-integration-api.md#замороженный-auth-identity-slice-game-auth-01):
until linking, limits apply to each independently proven account; Voice does not
infer shared identity from device/IP. Conversion checks both current sessions,
requires explicit handoff or conflict, and cannot leave two active sessions or
silently unmute. T17–T19/Voice implement it; acceptance covers permanent account
in a call, sdk-account in another room, conversion racing reconnect and lease
renewal.

### Q09. Детальная семантика block/report и жалобы на оператора

Основание: report/block and sanction separation are accepted; this T03 freeze
now defines interaction behavior and operator complaint handling below. A
personal block never rewrites authoritative roster/membership.

Принято: a personal block hides/blocks direct interaction but does not mutate
authoritative membership; Voice moderation ban remains the access authority.
Owner loss/dissolution freezes privileged writes until a Voice operator verifies
a named human and records a new owner generation. Node operators/game leaders
cannot assign Voice ownership. Operator complaints use Voice-owned intake with
IDs, timestamps, and evidence digests by default, not copied chat content.
T37–T39/moderation implement this; Q09/MMO03 covers shared-space block, blocked
bot, Owner loss/dissolution, and hostile/unavailable node operator. See the
[T03 API freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).

### Q10. Политики retention, удаления и восстановления identity

Основание: [conversion](../architecture/game-integration-api.md#конвертация-sdk-account)
оставляет retired audit reference; account deletion и сохранение signatures,
dedupe, abuse history и node backups должны быть совместимы.

Принято: v1 has one immutable node home and no online cross-node migration.
Operators own encrypted backups and hardware recovery; Voice owns identity,
consent, generation, and revoke authority. Restore must merge current Voice
fences and non-expiring external-key tombstones before serving. Owner export is
limited to app-owned configuration and explicitly exportable app data; it
excludes provider secrets, Voice credentials, other users' private profiles, and
unconsented message history. After node loss, resources stay unavailable until
same-home restore or explicit retirement. FED06–FED08/Q10 prove filtering,
delete/restore, and no resurrection. See the [T03 API
freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).

### Q11. Кто и как впервые получает developer/node полномочия

Основание: [API](../architecture/game-integration-api.md#9-совместимость-и-admission-разработчика)
предполагает app/env registry, [node](../architecture/game-federation.md#4-регистрация-и-доверие)
— approval. Developer portal теперь вне спринта, поэтому требуется рабочий
bootstrap через Voice API/операторский процесс, а не недоступный UI.

Решения закреплены в [Auth identity slice](../architecture/game-integration-api.md#замороженный-auth-identity-slice-game-auth-01),
[GIS bootstrap](../microservices/game-integration-service.md#g06q11-application-bootstrap-decision)
и [Federation authority v1](../architecture/federation-authority-v1.md):
authenticated Voice account owns the app; a distinct allowlisted Voice operator
approves sandbox; production is a separate reviewed admission; node enrollment
uses pinned operator mTLS and out-of-band endpoint ownership attestation. Google
OIDC is the real independent provider contract; fake JWKS proves verifier logic
only. The remaining gate is evidence, not provider selection: follow the clean
empty-DB/host walkthrough and negative cases in
[acceptance Q11](game-integrations-acceptance.md#q11-bootstrap-evidence-contract-runtime-gate-remains-open).
Do not count fake-provider CI as real-provider acceptance or production approval.

### Q12. Совместимость, SLO и вместимость поставки

Accepted support policy: keep `/api/v1` and Federation `/v1` supported until a
successor major is generally available, then provide a 12-month migration
window. Compatibility must be proven with old-client/new-server and
new-client/previous-server fixtures before production admission; this decision
is not runtime compatibility evidence.

Provisional single-host node qualification targets for planning are RPO ≤24h,
RTO ≤4h, 250 concurrent authenticated clients, 50 concurrent media publishers,
and 25 durable messages/second for a 60-minute steady-state run. The load profile
uses two Space, real media, durable writes, a concurrent revoke, and a host
restart; record p50/p95 latency, error rate, resource use, recovery point and
recovery duration. Restore the latest scheduled backup into a fresh isolated
host, validate owner/generation/revocation fences, then measure until readiness
and permitted reads/writes return. Write a timestamped synthetic canary at least
once per minute through the measured failure; RPO is failure time minus the
latest recovered committed canary, and RTO is failure injection to readiness
plus successful scoped read/write checks. These are unverified qualification proposals,
not production capacity, SLO, or RPO/RTO claims. Revisit them against measured
costs and supported hardware before production admission. Retry/retention values
and G08/G09/G13 bounds keep their owning contracts and evidence gates; price/SLA
is not inferred from using one service instance.

#### Q12 measurement record and status

Keep three fields distinct in every report: **contractual target** (a bound
already adopted by its owning contract), **provisional qualification profile**
(the planning values above, not a product promise), and **observed result**.
For each observation record `PASS`, `FAIL`, or `NOT RUN`; never promote a
provisional value to a measured result. `NOT RUN` includes work blocked on an
unselected qualified host or missing runtime evidence.

The report must capture: exact source commit SHA; component, API/schema,
database, host OS/runtime, container engine and image versions; host model/SKU
and topology when selected; warm/cold state; run start/end and configuration;
load and fault profile; population; source locations/IDs for raw observations;
start and stop event IDs; monotonic elapsed durations per host; and clock
uncertainty for cross-host comparisons. Include each repetition and the
reported statistic when available. For restore, include backup identity,
failure-injection event, latest recovered committed one-minute canary, readiness
event, and successful scoped read/write check IDs. Keep missing fields explicitly
`UNKNOWN` and unperformed measurements `NOT RUN`; hosted CI does not establish
real-media eject, restore RPO/RTO, or provider proof.

OPS02 has one already adopted enforcement value: the installation callback
registration limiter is 120 attempts per application per UTC minute, shared
across that app's environments, with the documented 429/Retry-After and minute
rollover behavior. This is not a general GIS route-capacity or tenant-isolation
threshold. Those targets remain `OPEN` until a qualified host and workload are
chosen; any future observations use the same report fields above.

До GI8/GI9: зафиксировать измеримые значения, платформы мессенджера для проверки
и политику несовместимого upgrade. Нельзя считать media verifier реализуемым
только потому, что LiveKit выдаёт JWT: FED10 обязан доказать fencing на media
path. Длительность спринта/команда ещё не заданы; этот документ не оценивает срок.

## Границы и следующие действия

G01–G13 сохраняют решения по доменам, Q01–Q12 уточняют конкретные сценарии. Это
один backlog спринта Voice, а не дополнительные релизные этапы. Decision inputs
for Q01–Q11 are recorded in owning contracts; dependent tasks must still pass
runtime acceptance. Q12's 12-month major-version migration window is adopted,
while capacity, RPO, and RTO remain provisional until T08/T93 measure them on
qualified hosts.

Приоритет обсуждения: Q03/Q07/Q11 (identity/bootstrap), Q01/Q02/Q05 (доступ и
consent), Q04/Q06/Q08 (federation/media), Q09/Q10/Q12 (эксплуатация и lifecycle).
Свой протокол шифрования не вводится: действующий
[encryption canon](../features/encryption.md) остаётся отдельным источником;
подпись авторства не означает E2E-приватность от игры или content node.

Проверка этого изменения: Markdown local links, anchors добавленных ссылок,
diff whitespace и поиск устаревших scope-утверждений. Runtime/source audit,
тесты сервисов, провайдеров, медиатрафика и движков не выполнялись.
