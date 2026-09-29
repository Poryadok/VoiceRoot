# Game integrations — единый спринт, подзадачи и acceptance

**Proposed target; не новый активный milestone.** [PLAN](../PLAN.md) определяет
очередь. Этот документ определяет доказательства перед будущим включением
[игрового продукта](../features/game-integrations.md), а не объявляет его готовым.
Documentation-only work не требует запуска всех сервисов/движков.

## 1. Один спринт с единым результатом

Решение владельца: возможности Voice для игрового продукта реализуются за один спринт,
от входа игрока до общения и игровых команд через мессенджер и game node.
GI-коды ниже обозначают подзадачи одного спринта, а не последовательные релизы,
MVP или перенос части функций в будущие спринты. Длительность и состав команды
ещё не определены; это требование к плану поставки, а не подтверждённая оценка.

В scope Voice входят `sdk-account` и обе конвертации, linked account, внешние
API/контракты авторства, hosted party, MMO communities, боты с командами и
карточками, поддержка этих сценариев мессенджером, opt-in уведомления,
game federation, Voice Node bundle и эксплуатационная/end-to-end проверка.

**Вне спринта:** Unity/Unreal/другие assets, распространяемые SDK wrappers,
engine-specific UI/media adapters, developer CLI/portal, образцы интеграции
для публикации и сертификация движков. Это отдельная задача инструментов,
а не незавершённая подзадача Voice. Backend app registry, выдача credentials,
scopes и provisioning API остаются в Voice. Install/update/backup средства
самого Voice Node остаются частью поставки сервера, не developer SDK tooling.
Online migration и ранее исключённые платформенные расширения не добавляются.

| Подзадача | Вход / зависимости | Наблюдаемый результат | Критерий готовности |
|---|---|---|---|
| GI0: contracts | G01–G13 и design audit, владельцы доменов | Согласованные schemas/scopes/identity/retention, versions и storage ownership | Решения закрыты перед зависимым кодом; документация и контракты согласованы |
| GI7: sdk-account и Auth | GI0 identity contracts, Auth/User | Вход без permanent account, linking, конвертация в новый и существующий аккаунт | ID01–ID13, revoke, conflict и crash recovery |
| GI1: party и Game API | GI0, GI7, Chat/Voice core | 2–4 игрока в одном hosted party chat/voice, keep-group | Реальное media, roster/revoke/reconnect/consent для обоих типов аккаунта |
| GI2: bot commands | Bot hardening, binding, game adapter | Event → slash → один реальный игровой эффект → result | Crash/retry/stale/revoke matrix, durable command semantics |
| GI3: rich companion | GI2 + Messaging/Flutter components + opt-in | Cards/buttons, event history, player notification settings, opt-in доставка в поддержанном messenger | Multi-device, forwarding, expiry, unsubscribe; mobile не заявляется без support proof |
| GI4: внешний протокол Voice | GI0 contracts; GI1/GI7 для интеграции | Versioned API, signed envelopes, errors/capabilities, conformance fixtures и внутренний test client | Независимый test client выполняет user/device flows через public API; no engine dependencies |
| GI5: MMO communities | GI7 identity + managed grants, G02/G03/G09 | Corporation → Space; rank changes affect in/out-game access | Полная roster/role lifecycle vertical |
| GI6: game node | GI5 + federation wire/lifecycle/ops, G08/G10 | Один game node, два Space, test client + messenger | Partition/revoke/restore/security/operability proof |
| GI8: эксплуатация Voice | GI0 contracts; GI1–GI7 для интеграции | App/env backend API, keys/rotation, quotas, diagnostics, единый Voice Node bundle, backup/restore, support docs | Установка на чистом хосте, общие config/version/upgrade; developer portal не требуется |
| GI9: общая приёмка | GI0–GI8 | HerdTrip-, MMO- и Dejavu-like reference flows через test clients, controlled game adapter и messenger | Вся Voice acceptance matrix, node artifacts и единый evidence package; engine SDK не gate |

GI0 закрывает контракты по мере потребности, не отдельным спринтом. После нужных
контрактов Auth, Bot, внешний протокол и federation transport можно выполнять
параллельно; итоговая интеграция учитывает зависимости таблицы. Несколько PR
допустимы, но готовность отдельной подзадачи не означает завершение спринта.
Feature flags служат безопасной интеграции и откату, а не исключению незавершённых
функций из общего результата. Спринт завершён только после GI9; `sdk-account`,
обе конвертации и федерация не переносятся молча за его границы. Запуск реализации
и место в общей очереди фиксируются в PLAN; текущая задача меняет документацию.

## 2. Работы по доменам

| Домен | Будущие изменения | Проверка до enabling PR |
|---|---|---|
| Auth/User | Delegation, provider proof, selected profile, revocation, sdk-account | Proof replay, PKCE, wrong issuer/audience, unlink/delete, privacy |
| Integration | App/env, sessions, external mappings, operations, managed grants | Unique keys, crash stages, roster CAS/snapshot, cleanup ownership |
| Gateway/proto | Versioned surface, principals/scopes, limits, error enums | OpenAPI/proto compatibility, forged IDs, direct-call negative tests |
| Chat/Space/Role | Managed membership, history scope, template provisioning | Removal/ban/Owner boundaries, race/freeze/deletion |
| Voice/Realtime | Scoped admission, native media, recovery, capture arbitration | Real media, active revoke, no global WS history assumption |
| Messaging/File | Card schema, persistence/edit/history/fallback, bounded files | Dedupe, malicious content, forwarded actions, reference ACL |
| Bot | Durable commands/results, token binding, signatures, DM consent | Failures from source audit closed with regression tests |
| Notification | Categories, consent revision, quiet hours/grouping, device policy | Opt-out queued events, DND, no push-triggered mutation |
| Flutter | Connected games, cards, result states, revoke/settings | Widget + live actions, accessibility, multi-profile isolation |
| Внешний протокол | API/signed envelopes, capability negotiation, conformance fixtures | Внутренний test client и messenger; Unity/Unreal вынесены в отдельную задачу |
| Federation | Registry, projection leases, S2S snapshots, remote lifecycle | Two-node isolation, partition, certificates, restore/purge |
| Registry/Operations | Backend environment onboarding, keys, quotas, status, node distribution | Rotation/revoke, production admission, support diagnostics; без developer portal |

Store changes идут в migrations владельцев и DATA_STORES/CONTRACT_MATRIX в
соответствующем implementation PR. Документирование не создаёт пустые сервисы,
таблицы или новые deploy profiles заранее.

## 3. Acceptance matrix

Каждый сценарий проверяется через публичный вход и фактический effect/read model,
а не только happy-path mock. Где применимы tokens, проверить также прямой обход
Gateway/SDK и старые credentials. Все game IDs и secrets — synthetic fixtures.
Упоминание SDK ниже в Voice-owned сценарии означает вызов внешнего протокола
внутренним test client; наличие распространяемого SDK не требуется. SDK01,
SDK02 и SDK04 — только отдельная задача инструментов, не gate этого спринта.
SDK03/SDK05/SDK06 проверяют серверную и messenger часть здесь; engine-specific
варианты тех же проверок выполняются при поставке инструментов. OS-specific
SDK cache/cleanup/UI не принимаются по результату server-only теста.

### Q11 bootstrap evidence (API-only clean-start passed; real-Google gate open)

The Auth-only T13a contract is independently testable with deterministic fake
Google JWKS and synthetic game-ticket keys. Its Auth module suite must prove
valid paired proofs, issuer/audience/nonce/expiry rejection, nonce and challenge
replay rejection, app/env isolation, identity uniqueness, configured admission
cap (1,000 identities per app/env and 10 active devices per identity), atomic
challenge consumption, and bootstrap credential/session/revoke semantics. It
must also prove these Auth paths remain unpublished from Gateway
and that no `guest` conversion path is called. This suite is a T13a implementation
gate only; it does not pass the clean-start or live-Google gates below.

On feature base `df0e084383ce1f9915d0bbc59651ae0227b1c75d`, the Auth module
contract suites passed: Google verifier/JWKS, REST controller and configuration
tests reported 56 passed; the PostgreSQL `SdkIdentityJdbcIntegrationTest`
reported 32 passed; full `rtk mvn -B test` reported 816 passed, zero failures,
errors, or skips. This is local Auth implementation evidence only. It does not
pass the disposable clean-start gate or real-Google provider gate below.

The identity lifecycle suites were rechecked on feature base
`7dd013ce551867074ab148dabaedfd30c1f83478` with the focused command selecting
`GoogleOidcProofVerifierTest`, `GoogleJwksTest`, `SdkIdentityRestControllerTest`,
`SdkIdentityConfigurationTest`, `SdkIdentityJdbcIntegrationTest`,
`SdkConversionProofsTest`, `SdkConversionRestControllerTest`,
`SdkConversionConfigurationTest`, and `SdkConversionJdbcIntegrationTest`.
Maven reported 169 tests, zero failures/errors/skips. Full `rtk mvn -B test`
on the same exact SHA then reported 890 tests, zero failures/errors/skips, and
BUILD SUCCESS; Testcontainers/PostgreSQL and all 23 migrations ran. Surefire
printed a post-test fork-JVM cleanup warning after `System.exit(0)` but the
Maven process exited 0. This verifies deterministic provider fixtures and
Auth-owned database/controller behavior for this exact feature SHA; it is not
a real Google login, clean-start proof, owner-receipt conversion, or
production-admission result. The real-Google gate below remains OPEN / NOT RUN.

Run both enrollment paths from disposable state. The developer path starts with
an empty `game_integration_db`, runs the service-owned migration, creates a
regular Voice account through the supported Auth test/bootstrap surface, creates
a draft application as that account, and obtains sandbox approval from a
different allowlisted Voice operator. Then create the sandbox environment,
configure its Voice-owned Google client and redirect/origin policy, and issue an
app/env-scoped service credential through the API. No direct SQL, developer
portal, production provider account, or production credential is part of this
proof. A fresh operator identity must not inherit the applicant's authority.

The node path starts from a clean disposable host and empty `federation_db`.
Provision only the documented service database login, TLS trust, signing seed,
and operator certificate pin through the test secret manager. An operator
creates a pending node through `/v1/nodes`; ownership of its configured endpoint
is checked out of band, and approval binds the exact node certificate pin. The
operator then creates a synthetic Space placement with
`POST /v1/nodes/{node}/spaces/{space}` and `{}`. The operator-publisher fixture
publishes its initial complete allowlist with
`POST /v1/nodes/{node}/spaces/{space}/snapshot`, revision `1`, one page, and a
`valid_until` no more than five seconds ahead. Use only synthetic Space, account,
profile and resource IDs; the permission may grant `read` for this fixture.
Federation has no production owning-service snapshot publisher yet, so this
operator-published snapshot proves only the authority contract. The node then
gets its signed snapshot at
`GET /v1/nodes/{node}/spaces/{space}/snapshot` and acknowledges the exact
revision/hash with a fresh nonce at
`POST /v1/nodes/{node}/spaces/{space}/lease`, using its node certificate and
node-scoped bearer. The test must not seed or repair registry rows with SQL.
For every enumerated Federation HTTP denial below, send a canonical
`X-Request-ID` and verify the response header and append-only Federation audit
row agree on that ID, the actor certificate fingerprint, target IDs, action,
result/status, and safe reason. The service replaces missing, malformed, or
repeated request IDs with a returned server UUID. A TLS handshake rejected
before HTTP dispatch has no request ID and is outside this audit gate. Verify
that audit `UPDATE`, `DELETE`, and `TRUNCATE` are rejected without SQL-seeding
or repairing registry rows.

Cryptographic fixture proof and real provider proof are separate gates. Fake
Google JWKS and synthetic game-ticket keys may prove signature, issuer, audience,
nonce, expiry, replay, and binding behavior in deterministic CI. They do not
prove a real Google integration. Before production admission, a disposable
Google OIDC client registered to Voice must complete a real login and JWKS key
refresh on the exact release SHA; record client configuration revision, test
subject hash, request IDs, verifier result, and secret-free logs. Do not retain
provider tokens or raw subjects in evidence. A missing Google credential or
unavailable provider means this gate is pending, never passed by the fake suite.

The repository-root clean-start gate creates fresh disposable stores and runs
the API-only GIS bootstrap selector plus the Federation API-only mTLS path and
enumerated denials. GIS's SQL-seeded production-credential denial remains in
the ordinary GIS suite under
`TestGameIntegrationProductionEnvironmentCredentialDeniedWithSeededFixture`;
it is not selected by the Q11 target. Federation audit immutability/failure
fault injection touches only API-created audit rows and never seeds or repairs
registry state. This is a fake-provider clean-start proof only; it does not
close the separate live-Google gate or production admission.

```powershell
rtk make game-integrations-q11-acceptance
```

The API-only clean-start gate passed on feature commit
`165a11ecbdd45e0f39ca44698d56071d584fadc8`. The target ran the GIS bootstrap
selector and Federation clean-start mTLS/audit path. On the same feature state,
GIS `rtk go test ./...` passed 92 tests in 4 packages, Federation passed 38 tests
in 1 package, and `rtk go vet ./...` passed in both modules. Before the master
sync, hosted PR #509 checks `changes`, `markdown-link-check`, and `ci-gate` were
green; the Q11 target was rerun successfully after the sync. This is API-only
clean-start evidence and does not prove real Google login or production
admission.

The T11 registry lifecycle was rerun after the T15 contract merge on feature
commit `7f55fce72d88f0274a2d60dbaabdf9585f56574c`.
`rtk make game-integration-bootstrap-acceptance` passed from an empty GIS
database, and `rtk make game-integrations-q11-acceptance` passed both the GIS
bootstrap selector and Federation clean-start mTLS/audit selector.
The GIS API test verifies authenticated owner-derived application creation,
separate operator approval, sandbox provider/redirect/origin policy, denial of
cross-owner/application/environment access, and SQL-backed credential
issue/retry/rotation/revoke behavior. An identical issue retry returns the
same secret and expiry; rotating creates generation 2, keeps the prior
credential valid during overlap, and bounds its expiry to ten minutes from the
injected rotation instant. Revoke immediately denies verification and an
identical revoke retry adds no second audit row. The ordinary GIS suite also
retains the SQL-fixture test that denies a production credential; it is not a
production admission flow. These checks use a fake Voice JWKS and do not claim
Google provider proof, production admission, or an unmeasured latency or
restore result.

T11 installation binding is asserted separately as BOT11: public registration
accepts an owner-selected Bot ID but no owner override; GIS derives the owner
from the application row. The protected GIS→Bot proof must be signed by the
`gameintegration` workload for the Bot audience and exact route/body, be fresh
and nonce-unique, and return a response MAC matching the request timestamp and
nonce. Bot proves its stored owner and `live` status. Wrong principal, replay,
tampered request/response, foreign owner, missing/deleted/disabled Bot, missing
proof configuration, or Bot/Redis outage must leave no installation or
idempotency success record. A valid proof persists the Bot ID, and an exact
owner/idempotency retry returns the same binding. Synthetic Bot rows/proofs
are fixtures only and do not establish provider or production admission. The
registry rechecks and locks application owner/status and environment status
after the network proof; concurrent owner transfer, app suspension, or
environment suspension aborts without a binding or successful operation. The
clean-bootstrap test applies `000004_t11_installation_bot_binding` and asserts
the Bot ID persists, an exact successful retry does not repeat proof, an owner
override is rejected, and proof denial creates no installation or successful
idempotency result. Proof attempts are charged once before Bot I/O in the
application quota; the failed idempotency result is saved and replayed without
another proof/audit/quota charge, a changed request hash conflicts, and an
in-flight duplicate cannot make another proof call. A stale pending claim is
reclaimed atomically after 30 seconds, repeated simulated crashes can reclaim
again without another quota charge, and concurrent recovery still permits only
one proof call. The 121st app attempt is rejected before Bot. These are GIS
database/fake-verifier checks, not proof of the live internal endpoint.

The T11 staged production-admission slice is development-only and does not
close T11: operator admission creates one app-scoped `production/pending`
environment without changing app `sandbox` status; wrong principal, owner
self-approval, wrong app state, foreign application IDs, malformed body, and
idempotency hash conflict are denied. Exact operator/key retry returns the
same pending environment, and the unique `(application_id,kind)` cap prevents
a second environment. The owner can CAS-stage only that environment's separate
Google policy with production HTTPS redirect validation; policy revision
conflicts and wrong-owner updates fail. Auth's signed resolver and the GIS
credential issuer both remain fail-closed for pending/production as applicable.
Suspension/restore never promotes pending to active, and no route accepts or
returns production secrets. Tests use PostgreSQL and fake/no provider; real
Google/user-proof, activation, and out-of-band secret provisioning remain
OPEN / NOT RUN.
GIS requires `BOT_INTERNAL_URL` together with the dedicated
`GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64`; omitting both keeps unrelated local
APIs available, while installation fails closed. Bot requires the same key and
`BOT_REDIS_ADDR` for live proof verification.

The separate live-Google provider gate is OPEN / NOT RUN. It is not invoked by
the API-only clean-start target; the opt-in harness still needs to be added and
must perform a real login and JWKS refresh with a disposable Voice-owned client.
No provider call was made for this clean-start proof.

```powershell
rtk make game-integrations-q11-google-live
```

Supporting module checks are `rtk go test ./...` from both
`src/backend/gameintegration` and `src/backend/federation`, and
`rtk mvn -f src/backend/auth/pom.xml test` for Auth. These do not replace the
clean-start or live-provider gates. The exact-SHA release evidence must attach
the clean-start logs and external Google provider run separately. No gate is
passed merely because its command exists.

Negative cases are mandatory: applicant self-approval (including when also in
the operator allowlist); caller-supplied foreign owner/account ID; operator
service token used as a player or vice versa; sandbox credential on production;
cross-app/env credential, provider subject, node, Space, audience or certificate
pin; operator certificate on node routes; node certificate on operator routes;
wrong node certificate, stale/revoked node bearer, duplicate approval, and
attempted direct SQL recovery. Each must fail without minting authority and
leave an audit event that records actor, target IDs, action, result and request
ID, with no token, provider claim or private-key bytes.

#### T14-DEV source and test map

This map records existing development behavior only. It does not mark the
parent T14 or the complete ID01–ID06 acceptance rows as closed. The exact-head
Auth, GIS, and Flutter hosted checks for `94dde2986d0024a48c65e77f3e6c98bb175418d6`
are pending; keep T14-DEV unchecked until all three finish green.
Production paths are Auth `src/backend/auth/src/main/java/voice/backend/auth/sdkidentity/SdkAuthorizationService.java`,
`SdkAuthorizationRestController.java`, and `AuthUserProfileEligibilityClient.java`;
GIS `src/backend/gameintegration/internal/httpapi/binding_challenge.go`,
`binding_exchange.go`, and `binding_authority.go`; and Flutter
`src/frontend/lib/backend/sdk_authorization_client.dart` and
`src/frontend/lib/ui/sdk/sdk_authorization_screens.dart`. Test references below
are under `src/backend/auth/src/test/java/voice/backend/auth/sdkidentity/`,
`src/backend/gameintegration/internal/httpapi/`, and `src/frontend/test/`,
respectively.

| ID | T14-DEV behavior and existing evidence | Boundary still open |
|---|---|---|
| ID01 | Auth consent binds explicit `profileId`, scopes, and policy revision; User eligibility verifies the selected owner/current state. `SdkAuthorizationJdbcIntegrationTest.browserConsentPreservesExplicitSecondaryProfileAndIssuesOnlyLinkedBootstrap`, `SdkAuthorizationRestControllerTest.approvalForwardsVoiceAuthorizationAndExplicitSelectedProfile`, `AuthUserProfileEligibilityClientContractTest`, and Flutter `sdk_authorization_widget_test.dart` (`requires explicit profile choice before approval`) cover the selected-profile path. | Full identity availability and operational rollout remain under parent T14/T13. |
| ID02 | Independent proof checks include `GoogleOidcProofVerifierTest.rejectsMissingOrEmptyProofAndExpectedBindings` and `SdkIdentityJdbcIntegrationTest.invalidGameTicketCannotSubstituteForValidatedIndependentProof` / `registryAppOrEnvironmentMismatchDeniesChallengeExchangeAndSession`; T14 request/device/redirect/PKCE checks include `SdkAuthorizationJdbcIntegrationTest.authorizeProofBindsSourceTokenDeviceAndCompleteRequestBody`, `redirectMustMatchExactlyAndUnknownScopesCannotBeRequested`, `wrongPkceDoesNotConsumeCodeAndCorrectVerifierWorksOnce`, and GIS `TestInternalBindingChallengeRequiresAuthProofAndSignsExactPersistedFacts`. | Real Google proof remains OPEN / NOT RUN; no production claim. |
| ID03 | `SdkAuthorizationJdbcIntegrationTest.concurrentCodeExchangeAcrossServiceInstancesHasExactlyOneWinner` and `bindingExchangeConsumesTheT14CodeOnceAndReplaysOnlyWithTheRegisteredDeviceProof`, plus GIS `TestBindingExchangeHandlerRunsClaimCommitAndCompletionInOrder`, cover one-use exchange and exact replay. Flutter callback/resume retry tests avoid exchanging the callback code again. | The broader conversion crash/recovery acceptance remains ID12/T17. |
| ID04 | The T14 approval API requires an explicit profile ID and checks exact ownership/current eligibility (`selectedProfileMustHaveExactOwnerAndCurrentEligibility`, `approvalRequiresExactPolicyRevisionAndCannotReissueLostCode`; controller `approvalRequiresExplicitProfileAndPositivePolicyRevision`). T14 accepts no email/nickname as an identity-link key. | No dedicated email/nickname-only merge test was found; this mapping is structural API/ownership evidence, not closure of the full ID04 row. |
| ID05 | Auth rechecks authority at exchange and linked-session use (`exchangeRechecksCurrentAuthoritiesAfterBrowserApproval`, `issuedLinkedCredentialImmediatelyObservesAuthorityRevocation`); GIS binding authority tests require authenticated Auth workload. | Full unlink/delete/suspension propagation and governed-resource drain remain downstream T16/T20 acceptance. |
| ID06 | Auth persists the explicit selected profile/revision; Flutter requires an explicit choice before approval and serializes the selected profile in the callback/session path (`browserConsentPreservesExplicitSecondaryProfileAndIssuesOnlyLinkedBootstrap`, `sdk_authorization_widget_test.dart`, `sdk_authorization_client_test.dart`). | The no-second-voice and hidden-profile roster/card/search/presence assertions remain broader T16 acceptance. |

| ID | Given / When | Then / свидетельство |
|---|---|---|
| ID01 | Игровой пользователь подключает существующий Voice | Явный профиль/scopes; только выбранная identity доступна игре |
| ID02 | Подставлены чужой subject, code, redirect, env или nonce | Reject; нет binding/session и утечки identity |
| ID03 | Code/ticket повторён одновременно двумя клиентами | Один допустимый exchange, controlled replay error |
| ID04 | Link на existing account совпадает только email/ником | Нет автоматического merge |
| ID05 | Unlink/delete/suspension во время SDK connection | Новые requests и active governed доступ закрываются; cache scoped |
| ID06 | Смена профиля мессенджера при игре | Игра не переключает actor молча; второй voice не возникает |
| ID07 | Первый вход без binding, аккаунт Voice существует или отсутствует | Одинаковые возможности подключения; рекомендованный UI сообщает о Voice сразу, без account enumeration |
| ID08 | Повторный вход с проверенным game subject | Тот же sdk-account в app/env; отдельный тип от guest, нет прав за пределами scopes/roster |
| ID09 | Конвертация sdk-account в новый постоянный аккаунт | Явная регистрация; continuity и допустимый контекст сохранены; старые credentials отозваны |
| ID10 | Конвертация в существующий аккаунт | Proof обеих сторон, выбор профиля, preview/consent; связи перенесены, исходная identity retired; нет implicit role/history union |
| ID11 | Занятый binding, бан, удалённый target или конкурентная конвертация | Контролируемый конфликт; нет перезаписи, обхода санкций или двух активных владельцев |
| ID12 | Crash/timeout/retry на каждой стадии конвертации | Та же durable operation, доступный status, recovery; нет дублей, потери audit или оживления старых tokens/actions |
| ID13 | Unlink после конвертации, старый provider ticket/token | Retired identity не восстанавливается автоматически; последующий вход следует утверждённой G01 policy |
| ID14 | Developer service/node/bot credential пытается выпустить player token, enroll/replace/recover device key или отправить за игрока; положительная enrollment/recovery | Отказать первой группе; enroll/replace/recover проходят только с независимым Auth proof и possession нового/current ключа по lifecycle policy; recovery отзывает только указанное lost device, не прочие устройства |
| ID15 | Подмена actor/chat/body/env/content bytes, duplicate/replay/lost response, stale node authority, edits/deletes/attachments, key lifecycle boundaries | Отказать до эффекта; точный receipt retry после expiry/revoke возвращает прежний результат read-only, иные bytes конфликтуют; проверяются user JWS и полный Messaging tombstone; Auth/GIS execution permit fail closed, same-operation retry не продлевает expiry, а unknown completion не считается успехом; revocation race tests prove `active→revoking→revoked`, no new permit in revoking, and successful revoke waits through commit/abort/expiry; permit `exp = min(issued+3750ms, assertion.exp)`, max clock margin 250ms and transaction 250ms; measured request-to-last-commit ≤4.25s; every current key expires at `not_before + 90 days`; rotation only before expiry, replacement gets its own 90-day validity, old key has 600s overlap; overlap equality and 90-day expiry equality reject new operations while retaining historical verification; missed renewal has no grace and requires fresh independent recovery proof; Auth cannot issue an assertion or admit a new message without T16 binding authority; new message writes also remain fail-closed without T30/T31 exact app/env/binding/chat mapping |

The internal ingress contract is protobuf `MessagingService.ApplyGameMessage`:
it carries the exact compact JWS body and separate Auth assertion, without
caller-supplied profile or sender authority. Generated Go and Dart descriptors
must stay in sync; public REST wiring remains a later T20 consumer.
| ID16 | SDK-клиент подписывает сообщение по вызову кода игры; нода скрывает предыдущую версию/сообщение | Гарантия ограничена авторизованным клиентом; подпись доказывает происхождение наблюдаемого payload, но не физическое намерение человека и не полноту истории; rollback/equivocation обнаруживаются только при наличии prior evidence/comparison |
| FED-AUTH | Нода A вызывает чужой Space/RPC, выбирает внутренний NATS subject или использует истёкший/отозванный grant | Reject; нет доступа к master NATS, нет подделки user authorship; active stream revoke проверен |
| FED-BUNDLE | Чистый хост, единый node config, установка/регистрация bundle | По одному экземпляру нужных компонентов; два Space, SDK/messenger content и media работают; внутренние порты/credentials изолированы |
| FED-UPGRADE | Restart, upgrade, component failure и backup/restore единого bundle | Проверенные migrations/recovery/readiness, сохранённые service boundaries и lifecycle fences; maintenance явно виден, нет обещания HA/zero downtime |
| SE01 | Повтор CreateSession, потеря ответа, restart orchestrator | Одна session/группа/комната и прежний operation result |
| SE02 | Crash после Chat create до Voice ready | Reconcile достраивает либо безопасно убирает owned resources |
| SE03 | Три матча одной party, повтор CreateSession и закрытие одного матча | Все три match IDs ссылаются на одну стабильную party-группу; одинаковый повтор возвращает прежний результат; failed/closed match не закрывает и не удаляет общий party-чат. Match-scoped история доступна до `closed_at + 30 days - 1 tick`, но недоступна в точке `closed_at + 30 days`. Терминальный operation receipt точно повторяется до границы `terminal_at + 30 days`; после неё оставшийся не содержащий контент tombstone не позволяет заново создать ресурс по прежнему ключу. |
| SE04 | Поздний участник, выход и повторное вступление | Для каждого контента проверяется `joined_at <= created_at < revoked_at`; на границах `joined_at` включён, `revoked_at` исключён. Rejoin открывает новый интервал и не возвращает gap/старую историю. Поверхности: history, search, quote/thread preview, attachment metadata и download fetch. Скачивание, разрешённое при выдаче URL, отзывается при fetch после `revoked_at`. |
| SE05 | User удалён/забанен во время разговора | Нет REST/history/WS/file/media доступа после установленного bound |
| SE06 | Сохранение группы после матча | Только согласившиеся; история не переслана новым людям |
| SE07 | Host меняется/уходит и backend перестаёт обновлять roster | Только полный аутентифицированный roster следующей revision с successor, уже состоящим в roster, атомарно передаёт host control; старый host теряет control сразу, client claim и Voice Owner/role mutation отклоняются. Удаление host без successor отзывает его control сразу; сессия закрывается при `lease_expires_at`. Lease валиден непосредственно до 60s от GIS commit и истекает ровно в этой точке: после неё admission/reconnect/governed reads+writes запрещены, active Voice/media fenced не позднее +5s. Exact duplicate той же revision/body не продлевает lease; lower revision stale no-op; equal revision with changed body конфликтует; partial/failed fetch не пустой roster. |
| SDK01 | Unity/Unreal packaged build, два устройства | Двусторонний реальный звук + persisted text |
| SDK02 | Permission denied, hotplug, suspend, Bluetooth change | Честные states; нет unmute, crash или зависшего capture |
| SDK03 | Packet loss/WS restart/истёк token | Bounded backoff; scoped snapshots + per-chat history, без дублей |
| SDK04 | Scene reload/Dispose/Editor stop | Tasks/callbacks/media освобождены, после Dispose нет UI calls |
| SDK05 | SDK/messenger одновременно делают voice и listen-only join на master/node | Одна account-wide session; атомарный admission, явный transfer либо conflict |
| SDK06 | Unsupported SDK/schema/capability | Typed error + fallback, отсутствие частичной небезопасной работы |
| BOT01 | Game outbox повторяет событие | Один deduplicated message; уведомления подчиняются обычной политике Messaging, отдельный intent count не обещан |
| BOT02 | Игрок slash/button вызывает действие | Backend проверяет binding/ownership; подтверждённый result в Voice |
| BOT03 | Падение до/после game commit и потеря ACK | Reconcile по прежнему command ID, ровно один эффект в game DB |
| BOT04 | Двойной клик/два устройства/новый invocation ID | Game event consumption/CAS исключает повторное расходование |
| BOT05 | Устаревшая, пересланная, изменённая или чужая карточка | Reject до side effect; безопасный fallback/result |
| BOT06 | HMAC неверен, timestamp/replay, bot другого env | Reject; нет accept/Hub completion или игрового эффекта |
| BOT07 | Revoke racing с execution admission; ранее admitted command | Revoke-first reject; admission-first bounded in-flight completion, не ложный cancel |
| BOT08 | Game backend недоступен, 429/5xx, DLQ | Bounded retries, visible status; не потеря и не ложный success |
| BOT09 | Consent revoked при queued push/action | Push подавлен; command повторно проверяет current authority |
| BOT10 | Poll response loss / worker restart | Для нового reliable mode lease повторяет ID; v1 не заявлен production |
| BOT11 | Missing membership/read scope; invalid interaction token | Whitelist не обходит user permissions; token binding enforced |
| BOT12 | Revocation между authority read и admission; retry/expired permit | Online admission сериализован с revoke; повтор не продлевает окно, expired permit не исполняется |
| BOT13 | Contradictory/reordered result receipts, same ID different payload | 409/reconciliation; success не перезаписан поздним failure |
| MMO01 | Roster revisions reordered/duplicated/truncated | Stale no-op; conflicting revision reject; partial snapshot не active |
| MMO02 | Один из двух персонажей теряет rank/membership | Пересчёт отдельных grants, без залипшего офицерского права |
| MMO03 | Смена главы корпорации | Owner не назначается обычной managed role mutation |
| MMO04 | Local-zone chat открыт из messenger | Отказ без отдельного разрешения игры на out-of-game access |
| MMO05 | Game roster источник устарел | По freshness contract доступ закрывается; нет вечного stale grant |
| FED01 | Node A grant отправлен Node B / другой Space | Reject issuer/audience/home/routing/binding mismatch |
| FED02 | S2S partition или snapshot gap | Нет stale lease renewal; новые grants закрыты, active media fenced |
| FED03 | Node controller упал при живом SFU | Watchdog/lease enforcement eject в 5s max budget |
| FED04 | Send committed, ответ потерян, master доступен | Нет shadow chat/дубликата; retry прежнего client ID |
| FED05 | Node подделывает DM/MM event, actor или push target | Master/game отвергает вне hosted scope и user proof |
| FED06 | Freeze/purge при disconnected node | Pending receipt, не ложное complete; после reconnect fences применены |
| FED07 | Restore старого backup с purged Space/roles | Ничего не оживает; reconciliation до serving |
| FED08 | Expired certificate/rotation/defederation | Trust revoked, без insecure fallback или ложного erasure claim |
| FED09 | Поиск/file URL после membership revoke | Snippet/download не обходят current authority |
| FED10 | Unexpired старый LiveKit bearer повторён после authority expiry при упавшем control plane | Media verifier не допускает track admission/delivery; одного повторного eject недостаточно |
| OPS01 | Rollback SDK/server одной поддержанной версии | Compatibility и данные сохраняются; disable новой capability |
| OPS02 | Перегрузка/tenant quota/неудачный webhook URL | Изоляция, 429/SSRF defense; чужие app не деградируют бесконтрольно |

### T51 Game Event v1 contract cases

The exact public request, verified GIS intent, S2S caller/RPC identities,
durability ownership, and retry/revoke behavior are frozen in the
[canonical Game Event v1 contract](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
These assertions belong to the T51 implementation suites; this docs freeze is
not runtime, staging, or provider evidence.

| BOT ID | Required assertion |
|---|---|
| BOT01 | Post the same `(app_id, environment_id, event_id, payload_hash)` repeatedly, restart GIS before and after outbox commit, and lose the Bot/Messaging reply after the message commit. One operation and one message result; every retry keeps the same UUIDv5 `client_message_id`. Any notification follows ordinary Messaging policy; no separate notification-intent count is guaranteed. Mention-like text remains literal and creates no mention target/notification. A changed body under the same event ID returns `409 EVENT_ID_CONFLICT` and creates no second send. |
| BOT05 | Unknown/foreign recipient, inactive binding, character binding outside the recipient, unlinked chat, DM, or unsupported event schema is rejected before Bot/Messaging publication. No caller-supplied account/sender identity can redirect the event. |
| BOT06 | Reject non-JCS bytes, body over 16 KiB, duplicate/unknown JSON keys, invalid HMAC, key-ID mismatch, timestamp outside ±300 seconds, duplicate auth/signature headers, cross-app/environment credential, and scope without `game.events.write`. While fresh, replay of the same `(credential_id, nonce, event_id, payload_hash)` is inert; reuse of that credential/nonce with another event/hash is `409 EVENT_NONCE_REUSE`. After freshness expiry, re-signing the same event/hash with a new nonce returns the saved operation; the stale original signature is rejected. |
| BOT08 | Crash at GIS inbox/outbox commit boundaries and during both S2S hops. Retryable gRPC statuses use the canonical saved-intent backoff; one valid `voice-retry-delay-ms` trailer extends it, while duplicate/malformed/out-of-range hints are ignored. An already-expired request returns a durable `expired` operation without an outbox row; expiry while queued is a terminal fact with no publication. `PERMISSION_DENIED`/`NOT_FOUND` cancel, `UNAUTHENTICATED`/`UNIMPLEMENTED` block for operator repair with the outbox retained, and conflicting Messaging idempotency becomes reconciliation-required; none is success. |
| BOT09, BOT12 | Credential revoke denies new ingress. App/environment/installation/binding revoke before dispatch cancels queued publication. A dispatch admitted before revoke may finish as one in-flight attempt; an ambiguous response after revoke becomes reconciliation-required and is never re-sent as a new publication. Already committed messages keep ordinary history ACL; later actions recheck live authority. |
| BOT11 | T11 registration proof rejects owner overrides and fails closed on missing, foreign-owner, inactive/deleted Bot, bad/replayed GIS workload proof, bad response MAC, or Bot/Redis outage; no binding row or successful idempotency result is written. A valid proof binds the Bot ID under the GIS application owner's authority, and an exact retry returns the same binding. For a new authenticated-owner registration request, GIS atomically claims the idempotency key/hash and consumes one app-wide quota attempt before Bot I/O; failed proof is charged once and its sanitized denial is replayed for the same key/hash, an in-flight duplicate cannot issue another proof, a different hash conflicts, and request 121 is rejected before Bot. A pending claim expires after 30 seconds; a stale claim is row-locked and refreshed before retrying the read-only Bot proof without charging quota twice. The database crash/reclaim test also repeats a simulated crash and ensures concurrent recovery cannot issue another proof. T51 GIS→Bot accepts only the verified `gameintegration` S2S principal and exact RPC/request hash; Bot→Messaging accepts only `bot`. GIS sources `app_owner_account_id` from the authenticated-owner application row and obtains `bot_id` from active T11 installation authority; Bot requires that value to equal its stored `bots.owner_account_id`. The hash matches `sha256:` plus lowercase hex of the complete deterministic protobuf request bytes; the JWT `request_id` equals the intent `operation_id`. Wrong issuer, audience, RPC, request hash, stale/expired token, replayed `jti`, invalid TLS/JWKS, or raw forwarded client identity is denied before message persistence. |

For two distinct event IDs, force outbox completion in reverse order and assert
that the contract makes no FIFO promise. For same-event retries, assert saved
resolved recipient/authority revision and message key are reused rather than
re-resolving the event to a different chat. T11 installation-to-Bot binding,
T16 binding authority and T30/T31 resource mapping are required test fixtures
from their owner services; synthetic fixtures do not prove those prerequisites
are implemented or prove any external game-provider admission.

Для T12 OPS02 registry suite фиксирует конкретные enforcement values и
границы: 120 регистраций installation callback на приложение за UTC-минуту,
общих между его sandbox/production environments; второе приложение получает
собственный лимит. 121-я попытка отвечает `429 RATE_LIMITED` с целым
`Retry-After` до границы следующей минуты. Denial audit объединяется в одну
строку на app/minute со счётчиком. После минутного rollover квота снова
доступна. Этот лимит относится только к новой регистрации installation; он не
обещает пропускную способность всех GIS routes или T15 delivery.

Регистрация разрешена только владельцу приложения и активной environment.
Квота владельца фиксируется до Bot authority proof: отказ proof расходует
попытку, а 121-я попытка не вызывает Bot. После admission лимитера
cross-environment и небезопасный callback отказ создают ровно одну sanitized
denial запись без installation/credential writes; foreign-owner отказ не
расходует квоту. Неверный/отсутствующий bearer не создаёт quota/store/audit
обращений. Путь callback обязан быть HTTPS:443 с
literal ASCII unreserved сегментами, без userinfo/query/fragment. DNS проверен
при регистрации и повторно в dialer; все адреса проверяются, TCP фиксируется
на разрешённом IP, TLS сверяет исходный hostname, redirect не исполняется.
Проверки используют controlled DNS/TLS servers и подтверждают отсутствие
запроса к redirect target.

Тесты suspension проверяют operator allowlist, атомарное audit/state изменение,
повтор idempotency key после последующего перехода возвращает исходные
status/revision, а owner credential/Auth policy fail closed с
`503 APP_SUSPENDED`. Заблокированный переход получает `409
APPLICATION_STATE_CONFLICT`; restore не возрождает revoke или отдельно
suspended environment.

Diagnostics проверяется на owner-only доступ, максимум 20 newest-first audit
событий, отсутствие callback URL/credential/proof/subject/payload secrets и
раздельные `developer_asserted`, `operator_approved`, `not_verified`. Migration
acceptance начинает с 000001 schema, сохраняет старое operator
`approve_sandbox` событие как `operator_approved` и оставляет прочие legacy
события с provenance `system` и result `success`. Тестовая operator approval
не является Google или иным live provider proof; отправка callback команд
остаётся T15 scope.

### T15 runtime prerequisite: app-scoped chat mapping

Messaging must resolve an active Auth binding/profile and separately obtain an
authoritative T30/T31 mapping for the exact `(application_id, environment_id,
binding_id, chat_id)` before accepting a new signed message. Auth's T16 binding
authority proves the current grant/profile and the operation-specific
`game.chat.send` authorization; it has no chat identifier. `ChatGuard.EnsureMember`
proves profile membership only and is not evidence that the chat is linked to
that app/environment/binding. Until the T30/T31 producer is available, the
resource-mapping dependency is absent and new game-authored writes fail closed;
tests with a synthetic mapping fixture prove only the consumer seam, not that
the prerequisite has shipped. Receipt-first exact retries remain readable.

## 4. Как запускать проверки при реализации

- Contracts: `rtk buf lint`, `rtk buf format -d --exit-code`, breaking/regeneration
  по [TESTING](../TESTING.md) после реальных изменений proto.
- Go: `rtk go test ./...` в затронутом сервисе; интеграции используют отдельные
  testcontainers и isolated Compose по канону, не production/game worlds.
- Auth: Maven/JUnit/Testcontainers согласно TESTING; не заменять Java mock'ом
  при проверке делегированных grants и отзыва.
- Flutter: `rtk make flutter-ci`, targeted widget и live test-client↔messenger тесты.
- Voice-owned test harness: public API, independent device keys, real media
  clients, controlled game adapter с durable effect store; не готовый engine SDK.
- Engines (отдельная задача инструментов): CI batch build + packaged sample,
  capture/playback, OS permissions и device acceptance с записью версий.
- Federation: isolated master+node+SFU harness с fault injection для S2S, game
  roster и media control. Проверяется actual access/track delivery, не только
  лог «revoked» или handler status.

Exact commands для engine version/platform и новых suites добавляются вместе с
реализацией, не придумываются как уже существующие Make targets.

## 5. Продуктовые пилоты и критерии остановки

HerdTrip pilot проверяет успешный разговор, продолжение группы после игры и
добровольный возврат в messenger. Dejavu pilot проверяет понимание цены команды,
полезность события и отсутствие обязательного notification treadmill. Студия
проверяет срок интеграции и наличие конкретного преимущества относительно своего
нынешнего решения. Нода — спрос на контроль данных плюс эксплуатационную цену.

До запуска фиксируются cohort, окно наблюдения, baseline, success/error rates,
retention definition и стоимость на активную команду. Linked identity не равна
messenger DAU; чужие SDK case-study uplift не используется как прогноз Voice.
Не масштабировать SDK только по количеству автоматически созданных identities.
Нет реального consumer/преимущества/возврата — остановить расширение платформы,
сохранив полезную интеграцию собственной игры.

## Решения перед активацией

Принятые направления отделены от открытых деталей. Владелец согласовал правила
из [feature canon](../features/game-integrations.md#принятые-владельцем-продуктовые-правила);
повторное согласование этих направлений не требуется. Остаточные сценарии и
acceptance Q01–Q12 перечислены в [design audit](game-integrations-design-audit.md).
Каждый оставшийся выбор фиксируется в owning canon до зависимого кода.

| ID | Решение | Рекомендуемый старт / gate |
|---|---|---|
| G01 | Принят узкий Auth identity contract GAME-AUTH-01 для T13a: отдельный `sdk-account`, независимые Google OIDC + app/env game-ticket proof, Auth challenge/exchange/session/revoke, cap 1,000 identities per app/env и 10 active devices per identity. G01 в целом открыт: остаются межсервисная trust matrix, conversion, conflicts/history, transfer/recovery, Gateway/registry activation и более широкий wire contract. | T13a детерминированно проверяется на fake provider fixtures; полный GI7 требует также отдельных real-Google и clean-start gates выше плюс conversion ID07–ID13 |
| G02 | Приняты раздельные reasons/grants и отсутствие implicit privilege union; открыта concurrent alt policy | Policy per game и ownership generation Q03; скрытые персонажи не раскрываются |
| G03 | Принят named human Owner по защищённому flow, не автоматический game leader | Остались loss-of-owner/dissolution recovery; roster не обходит Voice ban |
| G04 | Match-scoped history access expires exclusively at `closed_at + 30 days`; an explicit keep-group includes only consenting participants and never copies the match transcript. Terminal operation receipts remain retryable for 30 days after terminal state; after receipt purge, retain a non-content tombstone for the external resource key with no expiry, so retry cannot silently recreate it. A shared party chat is not retired by closing one match and remains governed by Chat/party lifecycle. | SE03/SE04 boundary checks; exact retry just before receipt expiry, tombstone denial at/after expiry; no content returned at match expiry |
| G05 | Current contracts are Voice Game API `/api/v1` and Federation authority `/v1`; proposed support is current v1 until a successor major is generally available, then 12 months | GI4 old/new client-server conformance and exact-version manifest; support proposal is not runtime proof; engine versions/assets remain separate |
| G06 | Отдельный Go Game Integration Service и собственное `game_integration_db` приняты; app owner, operator approval, API bootstrap and credential lifecycle are frozen in the service contract | Clean empty-DB bootstrap and scoped credential lifecycle via API; no direct SQL/portal; run Q11 acceptance before enabling |
| G07 | Managed/self-hosted pricing, quotas, admission и SLA | Sandbox limits + measured costs; никаких обещаний unlimited/free production заранее |
| G08 | Authority lease и revocation budget | Сумма propagation/expiry/skew/eject ≤5s; если не доказано, federated voice выключен |
| G09 | A complete accepted roster has a 60-second lease from its GIS DB commit. Exact same-revision/same-body retry is inert and does not renew; lower revision is stale no-op; same revision with different body conflicts. Incomplete or failed fetch is never empty. At exact lease expiry deny new admission/reconnect and governed reads/writes; fence active Voice/media within the existing ≤5-second revoke bound. Old-snapshot retries never renew. | SE07 expiry boundary; assert no access at `lease_expires_at`, active media fenced at/before `lease_expires_at + 5s`; only a complete higher revision renews |
| G10 | Node data loss/export/migration/backup и erasure terms | Один immutable home в GI6; отдельный migration protocol и operator responsibility |
| G11 | Приняты отдельный consent по категориям, quiet hours и отсутствие дублей; открыты routing/coalescing details | Mobile delivery только после mobile gate; смена получателя consent Q02 |
| G12 | Приняты выбранное игровое имя/app attribution и приватность скрытых профилей | Определить app-visible alias/profile serialization и лимиты Q07, rank/history Q01/G02 |
| G13 | Admission/revoke и completion после unlink | Online admission сериализован с revoke; зафиксировать start/completion bounds, in-flight UX и reconciliation до GI2 |

T32 contract propagation is complete after PR #536 merged: the frozen session
resource, roster, idempotency and retention rules are recorded in
`docs/architecture/game-integration-api.md` and
`docs/microservices/game-integration-service.md`; T32 and the G04/G09/Q01
decision checklist are updated in `docs/testing/game-integrations-exec-plan.md`.
Runtime behavior and exact-SHA acceptance remain open under T32–T34.

## 6. Release evidence и rollout

Дополнительная приёмка design audit: Q01 history rejoin matrix; Q02 consent
change; Q03 ownership transfer; Q04 signed revisions/files; Q05 confirmation
bypass; Q06 flood+revoke; Q07 profile/alias isolation; Q08 conversion+voice race;
Q09 moderation path; Q10 deletion+restore; Q11 clean enrollment/provider proof
as specified above; Q12 measured capacity/compatibility. Q11 ownership, approver,
provider, and bootstrap decisions are frozen. The API-only clean-start proof
passed at `165a11e`; the real-Google gate is OPEN / NOT RUN, and production
admission remains open.
Q12 has provisional targets and a measurement procedure; its results remain
unmeasured. An open runtime gate is never replaced by a skipped test when the
sprint is declared done.

Каждая capability включается отдельно: linked identity, sessions, native voice,
managed communities, cards, proactive DM, node hosting. Default off до собственного
gate. Legacy clients получают fallback; feature flag не заменяет authorization.

Evidence package: exact commit SHA, API/schema/node/messenger/test-client/dependency
versions, enabled capabilities, fixtures, non-skipped test results, build IDs
поставляемых Voice-артефактов (не engine SDK), real-media
acceptance, failure/revoke timings, data recovery proof, limits и support owner.
SDK versions и packaged engine builds относятся к evidence отдельной задачи tools.
Откат прекращает новые операции и сохраняет reconciliation уже принятых commands;
отключение флага не должно терять committed результаты или заново исполнять игру.

Текущее изменение добавляет только документацию. Bot audit source-only;
LiveKit/engine/game/federated E2E здесь не запускались. Готовность этих систем
проверяется перечисленными gates, а не фактом наличия этой спецификации.
