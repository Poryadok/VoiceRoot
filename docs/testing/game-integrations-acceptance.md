# Game integrations — единый спринт, подзадачи и acceptance

**Proposed target; не новый активный milestone.** [PLAN](../PLAN.md) определяет
очередь. Этот документ определяет доказательства перед будущим включением
[игрового продукта](../features/game-integrations.md), а не объявляет его готовым.
Documentation-only work не требует запуска всех сервисов/движков.

## 1. Один спринт с единым результатом

Решение владельца: весь описанный игровой продукт реализуется за один спринт,
от входа игрока до общения и игровых команд через мессенджер и game node.
GI-коды ниже обозначают подзадачи одного спринта, а не последовательные релизы,
MVP или перенос части функций в будущие спринты. Длительность и состав команды
ещё не определены; это требование к плану поставки, а не подтверждённая оценка.

В общий scope входят `sdk-account` и обе конвертации, linked account, SDK для
Unity и Unreal, hosted party, MMO communities, боты с командами и карточками,
opt-in уведомления, game federation, эксплуатация и end-to-end проверка.
Windows x64 для обоих движков — согласуемая платформа этой поставки; остальные
платформы, online migration и иные явно исключённые возможности не превращаются
в скрытые этапы этого плана и не заявляются реализованными.

| Подзадача | Вход / зависимости | Наблюдаемый результат | Критерий готовности |
|---|---|---|---|
| GI0: contracts | G01–G13, владельцы доменов | Согласованные schemas/scopes/identity/retention, versions и storage ownership | Решения закрыты перед зависимым кодом; документация и контракты согласованы |
| GI7: sdk-account и Auth | GI0 identity contracts, Auth/User | Вход без permanent account, linking, конвертация в новый и существующий аккаунт | ID01–ID13, revoke, conflict и crash recovery |
| GI1: party и Game API | GI0, GI7, Chat/Voice core | 2–4 игрока в одном hosted party chat/voice, keep-group | Реальное media, roster/revoke/reconnect/consent для обоих типов аккаунта |
| GI2: bot commands | Bot hardening, binding, game adapter | Event → slash → один реальный игровой эффект → result | Crash/retry/stale/revoke matrix, durable command semantics |
| GI3: rich companion | GI2 + Messaging/Flutter components + opt-in | Cards/buttons, event history, player notification settings, opt-in доставка в поддержанном messenger | Multi-device, forwarding, expiry, unsubscribe; mobile не заявляется без support proof |
| GI4: Unity и Unreal SDK | GI0 contracts; GI1/GI7 для интеграции | Оба package/plugin, native media, identity/conversion UI, samples и docs | Оба packaged builds проходят conformance и real-media suite |
| GI5: MMO communities | GI7 identity + managed grants, G02/G03/G09 | Corporation → Space; rank changes affect in/out-game access | Полная roster/role lifecycle vertical |
| GI6: game node | GI5 + federation wire/lifecycle/ops, G08/G10 | Один game node, два Space, SDK + messenger | Partition/revoke/restore/security/operability proof |
| GI8: developer operations | GI0 contracts; GI1–GI7 для интеграции | App/env registry, keys/rotation, quotas, diagnostics, единый Voice Node bundle с одним экземпляром сервисов, backup/restore, support docs | Установка на чистом хосте, общие config/version/upgrade, оператор восстанавливает ноду без ручной сборки микросервисов |
| GI9: общая приёмка | GI0–GI8 | HerdTrip-, MMO- и Dejavu-like reference flows на общем продукте | Вся acceptance matrix, packaged artifacts и единый evidence package |

GI0 закрывает контракты по мере потребности, не отдельным спринтом. После нужных
контрактов Auth, Bot, оба engine wrappers и federation transport можно выполнять
параллельно; итоговая интеграция учитывает зависимости таблицы. Несколько PR
допустимы, но готовность отдельной подзадачи не означает завершение спринта.
Feature flags служат безопасной интеграции и откату, а не исключению незавершённых
функций из общего результата. Спринт завершён только после GI9; `sdk-account`,
второй движок и федерация не переносятся молча за его границы. Запуск реализации
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
| Unity/Unreal | Native packages, media adapter, samples/UI, diagnostics | Packaged binaries, main-thread callbacks, cleanup, hotplug |
| Federation | Registry, projection leases, S2S snapshots, remote lifecycle | Two-node isolation, partition, certificates, restore/purge |
| Portal/Operations | Environment onboarding, keys, quotas, status, docs | Rotation/revoke, production admission, support diagnostics |

Store changes идут в migrations владельцев и DATA_STORES/CONTRACT_MATRIX в
соответствующем implementation PR. Документирование не создаёт пустые сервисы,
таблицы или новые deploy profiles заранее.

## 3. Acceptance matrix

Каждый сценарий проверяется через публичный вход и фактический effect/read model,
а не только happy-path mock. Где применимы tokens, проверить также прямой обход
Gateway/SDK и старые credentials. Все game IDs и secrets — synthetic fixtures.

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
| ID14 | Developer service/node credential пытается выпустить player token, заменить device key или отправить за игрока | Reject; independent user proof и device signature обязательны, произвольный developer issuer не принимается |
| ID15 | Подмена actor/chat/body/env подписанного сообщения, replay, отозванный device key | Receiver rejects tampering/revoked key; retry одного сообщения не создаёт дубль |
| ID16 | SDK-клиент подписывает сообщение по вызову кода игры | Гарантия ограничена авторизованным клиентом; продукт не заявляет proof человеческого намерения против разработчика executable |
| FED-AUTH | Нода A вызывает чужой Space/RPC, выбирает внутренний NATS subject или использует истёкший/отозванный grant | Reject; нет доступа к master NATS, нет подделки user authorship; active stream revoke проверен |
| FED-BUNDLE | Чистый хост, единый node config, установка/регистрация bundle | По одному экземпляру нужных компонентов; два Space, SDK/messenger content и media работают; внутренние порты/credentials изолированы |
| FED-UPGRADE | Restart, upgrade, component failure и backup/restore единого bundle | Проверенные migrations/recovery/readiness, сохранённые service boundaries и lifecycle fences; maintenance явно виден, нет обещания HA/zero downtime |
| SE01 | Повтор CreateSession, потеря ответа, restart orchestrator | Одна session/группа/комната и прежний operation result |
| SE02 | Crash после Chat create до Voice ready | Reconcile достраивает либо безопасно убирает owned resources |
| SE03 | Три матча одной party | Общая разрешённая party-группа, независимые match lifecycles |
| SE04 | В session добавлен поздний участник | Не видит историю до allowed boundary при since_join policy |
| SE05 | User удалён/забанен во время разговора | Нет REST/history/WS/file/media доступа после установленного bound |
| SE06 | Сохранение группы после матча | Только согласившиеся; история не переслана новым людям |
| SE07 | Host меняется или уходит | Ни server secret, ни ложный roster не возникают; lifecycle deterministic |
| SDK01 | Unity/Unreal packaged build, два устройства | Двусторонний реальный звук + persisted text |
| SDK02 | Permission denied, hotplug, suspend, Bluetooth change | Честные states; нет unmute, crash или зависшего capture |
| SDK03 | Packet loss/WS restart/истёк token | Bounded backoff; scoped snapshots + per-chat history, без дублей |
| SDK04 | Scene reload/Dispose/Editor stop | Tasks/callbacks/media освобождены, после Dispose нет UI calls |
| SDK05 | SDK/messenger одновременно делают voice и listen-only join на master/node | Одна account-wide session; атомарный admission, явный transfer либо conflict |
| SDK06 | Unsupported SDK/schema/capability | Typed error + fallback, отсутствие частичной небезопасной работы |
| BOT01 | Game outbox повторяет событие | Одна карточка/message, один notification intent |
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

## 4. Как запускать проверки при реализации

- Contracts: `rtk buf lint`, `rtk buf format -d --exit-code`, breaking/regeneration
  по [TESTING](../TESTING.md) после реальных изменений proto.
- Go: `rtk go test ./...` в затронутом сервисе; интеграции используют отдельные
  testcontainers и isolated Compose по канону, не production/game worlds.
- Auth: Maven/JUnit/Testcontainers согласно TESTING; не заменять Java mock'ом
  при проверке делегированных grants и отзыва.
- Flutter: `rtk make flutter-ci`, targeted widget и live SDK↔messenger тесты.
- Engines: CI batch build + packaged sample на выбранной матрице; фактический
  capture/playback и OS permissions требуют device acceptance с записью версий.
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

Это открытые решения дизайна, а не обязательный запрос подтверждения текущей
документации. Рекомендуемые варианты позволяют подготовить конкретный следующий
implementation slice; каждый выбор фиксируется в feature/service canon до кода.

| ID | Решение | Рекомендуемый старт / gate |
|---|---|---|
| G01 | sdk-account отдельно от guest и оба пути конвертации приняты владельцем; открыты trust matrix, conflicts/history/recovery и wire contract | GI7 в этом же спринте; обязательны новый и существующий permanent target, ID07–ID13 |
| G02 | Несколько персонажей/аккаунтов в одной корпорации | Reasons/grants раздельно; policy per game, запрет implicit privilege union без решения |
| G03 | Владение corporation Space | Named human Owner по текущему защищённому flow; game leader не получает Owner автоматически |
| G04 | Retention, history boundary, idempotency/result retention | Match since_join, explicit keep-group; сроки и retry budgets утвердить до хранения pilot data |
| G05 | Engines/platform versions, shared core, support window | Unity и Unreal на Windows x64 в одном спринте; точные версии после media spike внутри GI4 |
| G06 | Отдельный Go Game Integration Service принят владельцем; открыты storage schema, contracts и deployment | Собственное хранилище, не Gateway DB; спроектировать миграции до GI0 schema freeze |
| G07 | Managed/self-hosted pricing, quotas, admission и SLA | Sandbox limits + measured costs; никаких обещаний unlimited/free production заранее |
| G08 | Authority lease и revocation budget | Сумма propagation/expiry/skew/eject ≤5s; если не доказано, federated voice выключен |
| G09 | Roster freshness и доступ при падении game backend | Bounded source lease, fail closed для managed доступа; конкретный срок перед GI5 |
| G10 | Node data loss/export/migration/backup и erasure terms | Один immutable home в GI6; отдельный migration protocol и operator responsibility |
| G11 | Proactive DM categories и device/push policy | Отдельный consent, digest/quiet hours; mobile delivery только после mobile gate |
| G12 | Character display и контекст rank/history | Выбранный профиль + app/character label; без раскрытия скрытых альтов |
| G13 | Admission/revoke и completion после unlink | Online admission сериализован с revoke; зафиксировать start/completion bounds, in-flight UX и reconciliation до GI2 |

## 6. Release evidence и rollout

Каждая capability включается отдельно: linked identity, sessions, native voice,
managed communities, cards, proactive DM, node hosting. Default off до собственного
gate. Legacy clients получают fallback; feature flag не заменяет authorization.

Evidence package: exact commit SHA, schema/SDK/dependency versions, enabled
capabilities, fixtures, non-skipped test results, packaged build IDs, real-media
acceptance, failure/revoke timings, data recovery proof, limits и support owner.
Откат прекращает новые операции и сохраняет reconciliation уже принятых commands;
отключение флага не должно терять committed результаты или заново исполнять игру.

Текущее изменение добавляет только документацию. Bot audit source-only;
LiveKit/engine/game/federated E2E здесь не запускались. Готовность этих систем
проверяется перечисленными gates, а не фактом наличия этой спецификации.
