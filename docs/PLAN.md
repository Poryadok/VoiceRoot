# Voice — план продукта и исполнения

> Каталог заявленных возможностей — [FEATURES.md](FEATURES.md), канон поведения — `docs/features/*`, архитектуры — [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md). Активный scope, порядок и release gates задаёт этот файл.

Срез проверен 2026-09-04 на commit `2c1a908a29e89bcdc077dc590fbb8b4db9ae14c5`. Он описывает состояние этого commit, а не обещание о будущем `master`.

## Стратегия

Voice сначала должен стать убедительным бесплатным продуктом для небольшой группы: Web → Windows, вход по email или как гость, личные и групповые чаты, invite-only Space, голос и полный цикл поиска команды. Неполные возможности скрываются флагом или не выводятся в основной сценарий.

Default admission для внешнего alpha — server-signed одноразовый invite с expiry/revoke и fail-closed configurable cohort cap; email и guest registration требуют invite. Владелец перед `G1` принимает только предложенный размер когорты, а не проектирует механизм.

Приоритет задают несколько законченных пользовательских потоков, а не сумма строк в TODO. Главный дифференциатор — матчмейкинг, связанный с голосовым ростером, временным чатом команды и post-match историей. Billing, mobile, публичный каталог, bots и визуальная полировка не должны задерживать проверку этого ядра.

Работа идёт одновременно по двум независимым веткам:

- `H` — короткие действия и решения владельца, которые нельзя честно заменить кодом агента;
- `A` — автономные продуктовые этапы. Они должны выполняться без ожидания `H` на local/compose, fake или sandbox окружении.

Зависимость от владельца ставится на release gate, а не на весь engineering milestone. Если задача смешанная, агент сначала доводит код, тесты, конфиг и инструкцию до состояния «нужен один конкретный ввод или один acceptance run».

Текущий WIP плана:

| Состояние | Milestone | Правило |
|---|---|---|
| Active | `A1` | Единственный активный продуктовый milestone. Fleet дробит его на независимые service/client/contract/verification задачи и исполняет их максимально параллельно в отдельных worktree. |
| Next | `A2` | Входит в WIP только после интеграции и полного vertical DoD `A1`. |
| Queued | `A3–A7` | Не получают code WIP до закрытия предыдущего milestone; разрешены лишь чтение канона и подготовка, непосредственно разблокирующая Active. |

`H`-задачи не занимают слот `A`: владелец и агенты могут параллельно готовить access, decisions и activation evidence, но это не открывает второй продуктовый milestone.

Scope разделён явно:

- `alpha scope` — `A1–A4` и перечисленный в `G1` функциональный slice `A5`;
- `post-alpha committed scope` — остаток `A5`, `A6–A7`, затем отдельные milestones для mobile, verification, bots и stories по данным `G1`; эти фичи не deferred, но не получают WIP до entry review после alpha;
- `deferred` — только federation и её производные, пока владелец отдельно не изменит scope.

## PLAN и TODO

| Источник | Назначение |
|---|---|
| Этот файл | Глобальный порядок, границы этапов и release gates |
| [TODO.md](TODO.md), `docs/todo/*.md` | Инвентарь локальных дефектов, рисков и доказательств |
| [FEATURES.md](FEATURES.md) | Каталог возможностей; scope/status берётся отсюда, из этого PLAN и feature docs, а не из checkbox count |
| `docs/features/*` | Каноническое продуктовое поведение и feature DoD |
| [TESTING.md](TESTING.md) | Требуемые уровни проверки |

Датированный аудит на указанном SHA нашёл 545 открытых и 152 оставленных выполненных пункта, хотя [TODO.md](TODO.md) требует выполненное удалять. Все 100 design-пунктов относятся к Penpot/design parity, причём 88 — атомарный missing-buttons audit; также найдены stale-записи и точные дубли. Это разовое доказательство проблемы, не второй живой счётчик: актуальный инвентарь остаётся только в `docs/todo/*`.

Поэтому количество открытых пунктов не является процентом готовности. `Critical` и `High` в TODO означают локальную тяжесть дефекта или риск конкретной фичи, но не место фичи в глобальной очереди.

Правила дальнейшего ведения:

- feature-sized результат живёт здесь; RPC, миграция, кнопка, тест и отдельный дефект — в TODO;
- пункт TODO должен называть наблюдаемый gap, источник контракта и проверяемый DoD;
- дубли объединяются, stale-записи удаляются, выполненные записи удаляются после переноса нужного контекста в код или канонический документ;
- мелкая задача берётся вне активного milestone только при риске security, privacy, data loss, падении обязательной проверки или если она разблокирует вертикальный поток;
- чистка backlog выполняется небольшими пакетами параллельно и никогда не подменяет создание продукта.

## Честный статус

Статусы относятся к текущему заявленному функциональному объёму, но не включают явно future/post-v1 возможности, federation и финальную Penpot-полировку:

- `shipped` — текущий основной scope реализован и проверяется;
- `core-live` — главный пользовательский поток работает, но остаётся существенное расширение или hardening;
- `partial` — один или несколько центральных потоков ещё не замкнуты;
- `stub` — центральный путь фейковый или его нельзя безопасно показать;
- `deferred` — не входит в текущую очередь.

| Фича | Статус | Состояние и крупнейший остаток |
|---|---|---|
| [Текстовый чат](features/text-chat.md) | core-live | DM, группы, каналы, треды, folders, Quick Access, archive и pins ≤5 работают; plain-text list preview есть, но delivery/media labels, group/channel read-модель, send options и rich composer неполны. |
| [Пересылка сообщений](features/forward-messages.md) | core-live | Attribution, copy-as-new и commentary работают; нужна одинаковая проверка всех rich payload и будущих типов контента. |
| [Войс-чат](features/voice-chat.md) | partial | Join/call signaling и часть organizer UI есть; actual-media E2E, command/mute/speak enforcement, движение между комнатами, roster/lifecycle и active-session UX не замкнуты. |
| [Шара экрана](features/screen-share.md) | partial | API/signaling core есть; actual-media E2E, source picker, system audio и staging RTC acceptance не закрыты. |
| [Сторис](features/stories.md) | partial | Create/feed/LFP ядро есть; нужны audience/privacy, editor и media lifecycle. |
| [Регистрация и контакты](features/auth-and-contacts.md) | partial | Email, guest, sessions и reset UI работают; delete пока soft/deactivation path, а phone/OTP, erasure/tombstone и часть hardening не закрыты. |
| [Верификация](features/verification.md) | partial | Backend sync и базовые provider paths в основном есть; UI, remaining provider/cron paths и live-provider acceptance неполны. |
| [Профиль пользователя](features/user-profile.md) | core-live | Avatar, bio, status, switch и banner field работают; нужны animated media, premium gating и полная persistence. |
| [Множественные профили](features/multi-profile.md) | core-live | Create/switch/delete/frozen/downgrade picker работают; нужны entitlement lifecycle, primary-profile flow и regression coverage. |
| [Друзья и контакты](features/friends.md) | core-live | Requests, block, DM gate, contacts/favorites и QR paste работают; нужен phone-book hash pipeline и остаточный fail-closed hardening. |
| [Статусы присутствия](features/presence.md) | core-live | REST, WS в общем чате и auto-idle работают; нужны friend fan-out, game detection и privacy edges. |
| [Приватность](features/privacy.md) | partial | Основные visibility и block gates работают, но viewer-aware presence/last-seen fan-out и `show_read_receipts` ещё не обеспечены; discovery и inactive-profile границы тоже неполны. |
| [Спейсы](features/spaces.md) | core-live | Create/join/tree/invites, backend delete/transfer и gRPC audit read работают; нужны полная Gateway/Flutter lifecycle/audit-вертикаль, catalog/templates, audit filters/writers и entry requirements. |
| [Ролевая модель](features/roles.md) | core-live | Custom roles и основные send/join deny работают; voice organizer и остальные text/voice permissions требуют единой проверки на всех входах. |
| [Матчмейкинг](features/matchmaking.md) | partial | Solo/space queue и catalog работают; нет voice-roster party, полного post-match цикла и cleanup. |
| [Каталог игр](features/game-catalog.md) | core-live | Seed, game selection и staff moderation работают; нужна полная current-вертикаль modes/roles и recent/popular UX. |
| [Репорты / модерация](features/reports.md) | partial | Report/appeal endpoints и admin resolution core есть; sanction enforcement/delivery across transports и полный appeal loop не доказаны end-to-end. |
| [Шифрование](features/encryption.md) | core-live | Opt-in E2E DM и key backup работают; нужны понятный fingerprint/key-change и recovery UX. |
| [Онбординг](features/onboarding.md) | partial | Screens/flags и часть coach marks есть; async progression может зависнуть, а обязательный переход к Space discovery не замкнут до готовности каталога. |
| [Accessibility](features/accessibility.md) | core-live | Semantics, keyboard/focus и text-scale baseline есть; нужны physical screen-reader acceptance и high-contrast polish. |
| [Локализация](features/i18n.md) | partial | EN/RU UI baseline есть; locale persistence/profile sync и локализация backend messages неполны. |
| [Боты](features/bots.md) | partial | Registry/install, slash, webhook/polling и runtime core есть; нужны chat-type/scopes/lifecycle hardening и полный live bot flow. |
| [Навигация](features/navigation.md) | core-live | Folders, Quick Access, archive, requests и desktop shell работают; нужны mobile chrome и остаточные row/state gaps. |
| [Поиск](features/search.md) | partial | Global/in-chat query, ACL и исключение encrypted payload работают; exact message jump, historical backfill, свежие projections, rich-content mapping и admin gate неполны. |
| [Deep links / Sharing](features/deep-links.md) | partial | Invite parsing/compose работают; остальные Space/chat/message/profile targets, prod well-known/signing и physical mobile acceptance не закрыты end-to-end. |
| [Платформы](features/platforms.md) | partial | Web и Windows проверяются в CI; Windows daily-driver и mobile release path ещё не завершены. |
| [Обновления клиентов](features/updates.md) | partial | Version/force-update paths есть частично; desktop delivery и реализация заданного mobile OTA path открыты. |
| [Уведомления](features/notifications.md) | partial | In-app, quiet hours и message-request core есть; sanction delivery пока push-oriented, а contracts/presentation, реальные push, типы и grouping неполны. |
| [Подписка](features/subscription.md) | stub | Внутренние entitlement/grace/events частично есть, но реальный checkout и provider lifecycle нельзя считать готовыми. |
| [Хранение файлов](features/file-storage.md) | partial | Backend upload, R2, retention и SHA verification работают; основной non-E2E download/expired URL UX, dedup, async processing, transcode и previews неполны. |
| [Наблюдаемость](features/observability.md) | partial | Код и provisioning baseline есть; live staging, полная telemetry chain, P1 routing и restore evidence не приняты. |
| [Продуктовая аналитика](features/analytics.md) | partial | ClickHouse ingest и staff dashboards существуют; pod/restart durability, dedup, consumer health и семантика неполны, но analytics не gate бесплатного alpha. |
| [Федерация](features/federation.md) | deferred | Спека и scaffold сохраняются, реализация не планируется без отдельного решения владельца. |

## Технический дизайн сейчас

Отсутствующий, устаревший или не утверждённый Penpot frame не блокирует функциональную реализацию. До отдельного visual-approval этапа агент строит технический UI по следующему порядку источников:

1. `docs/features/*` и сервисные контракты;
2. [screen-controls.md](design/screen-controls.md);
3. [brand.md](design/brand.md), tokens, a11y и i18n;
4. существующие `VoiceTheme`, `src/frontend/lib/ui/core/*` и уже shipped UI;
5. утверждённый Penpot — как дополнительная ссылка, если он не конфликтует с каноном выше.

Технический дизайн — не заглушка. Он обязан закрывать рабочий путь, loading/empty/error/offline/permission/entitlement states, responsive layout, keyboard/focus, semantics, i18n и тесты. Нельзя оставлять fake action, demo data, raw colors, обход backend-контракта или кнопку без честного состояния.

Агент сам выбирает обратимые локальные детали: token spacing, существующий component/icon, dialog против sheet по текущим паттернам, нейтральный текст. Документированные data/API, permissions, privacy/security и billing contracts агент реализует автономно. Остановка и решение владельца нужны только для неописанной или противоречивой продуктовой семантики, а также для breaking/необратимого решения о data model, destructive recovery или крупной IA.

Финальные Penpot registry/GAP/export/pixel-parity и общий визуальный pass находятся в `H5`; до него Flutter может считаться функционально готовым, но не visual-approved.

## H — задачи владельца

Эта ветка короткая и не является очередью для агентов. Владелец получает только минимальный пакет: рекомендуемый default, альтернативы, точный вопрос или нужное значение и уже подготовленную проверку.

Каждый decision handoff содержит один ID, один точный вопрос, рекомендуемый default, не более двух альтернатив, затронутый slice/gate и цену задержки. Владелец не разбирает raw TODO и не получает пакет из несвязанных решений.

### H1 — Продуктовые решения без канона

**Состояние:** по требованию; не блокирует документированные части `A`.

**Действие:** выбрать вариант только для реально конфликтующего или отсутствующего поведения. Текущие alpha-семантики email activation, composable Space entry policy и удаления аккаунта/Space зафиксированы. Следующие owner decisions появляются только при входе соответствующего scope: phone/SMS provider и anti-abuse flow, а также крупная mobile IA.

**Разблокирует:** конкретную узкую ветку реализации; остальные части milestone продолжаются.

**Готово / доказательство:** решение записано в соответствующем `docs/features/*` или ADR, после чего у агента есть проверяемый DoD; [product-roadmap.md](todo/product-roadmap.md) может только ссылаться на этот канон.

### H2 — Внешние аккаунты, legal и credentials

**Состояние:** не нужны для local/compose и fake-provider DoD; нужны только перед соответствующим release gate.

**Действие:** создать или подтвердить merchant/provider accounts и передать через штатный secret store значения для billing, Resend, Cloudflare R2, encrypted off-cluster backup destination, optional GIF-search provider, Firebase/FCM, APNs, OAuth providers и реального Alertmanager destination. Staging и production получают разные account/key/bucket/endpoint sets; внутри каждого environment отдельно задаются User/File buckets так, как требует deploy manifest. Для R2 и backup target утвердить lifecycle/retention/versioning/immutability policy из подготовленного checklist. Для Windows предоставить code-signing identity/certificate; для mobile — Apple/Google developer distribution и signing assets. Для платежей также подтвердить legal/tax/offer setup, product/price IDs и отдельные sandbox/live webhook secrets и URLs.

**Разблокирует:** production attachments, подписанный Windows build, live email, push, verification, alerts, mobile distribution и paid beta.

**Готово / доказательство:** secret names и IDs из подготовленного agent checklist существуют в каждом целевом environment; validator отклоняет shared/mismatched staging/production resources и перепутанные User/File buckets, значения не попали в Git или лог.

### H3 — Домены, доступы и wiring внешнего окружения

**Состояние:** отложено до готового release candidate.

**Действие:** предоставить доступ к staging и `production/voice-prod`, утвердить production DNS/TLS/firewall, persistent storage class/PVC policy для stateful workloads и независимый off-cluster backup target. По [DEPLOYMENT.md](DEPLOYMENT.md) staging используется только для integration/regression/demo: сначала на нём собирается evidence, затем внешний invite alpha выпускается в production. Отдельное долговечное alpha-окружение допустимо только после описания его durability/privacy/backup/lifecycle policy в DEPLOYMENT. Агент заранее готовит manifests, validators, dry-run и точные команды.

**Разблокирует:** внешний Web/Windows alpha, prod deep links и staging evidence.

**Готово / доказательство:** evidence разделено по gate. Для `G1` проверены production DNS и end-to-end TLS для API/Web/LiveKit/JWKS: Cloudflare Full (strict) либо явно утверждённый эквивалент, origin certificate/HTTPS и origin allowlist; stateful workloads не используют ephemeral storage, off-cluster encrypted backup достижим по минимальным credentials; staging и production используют раздельные проверенные R2/backup credentials и endpoints; также проверены immutable Windows artifact/update feed и минимальные доступы. AASA/assetlinks, mobile domains и signing относятся только к `G3`.

### H4 — Физическая приёмка и go/no-go

**Состояние:** только на release candidate.

**Действие:** агент или QA выполняет все доступные автоматические и физические проверки и приносит готовый evidence report. Владелец предоставляет только недоступное агенту устройство/account при необходимости, назначает on-call/rollback owner и принимает go/no-go. Проверяются только включаемые платформы и возможности.

| Gate | Минимальная versioned physical matrix |
|---|---|
| `G1` | non-empty supported Windows OS/arch и Web browser/version matrix; clean-machine trusted Windows install и old→new update из signed production feed с fallback; cold start; two-client real mic+speaker call; network loss/reconnect; PTT/tray/hotkeys; принятие cohort cap и предложенных alpha RPO/RTO |
| `G2` | live purchase/cancel scheduling и entitlement visibility; expiry/grace/downgrade остаются sandbox/test-clock проверкой |
| `G3` | TalkBack/VoiceOver, background voice, App/Universal Links и real push на целевых устройствах |

**Разблокирует:** соответствующий внешний gate; mobile и paid не блокируют бесплатный Web/Windows alpha.

**Готово / доказательство:** checklist хранит SHA, OS/device, network profile и pass/fail каждого пункта; владелец получил готовый отчёт и записал решение о выпуске или конкретном дефекте.

### H5 — Финальный визуальный дизайн в Penpot

**Состояние:** сознательно отложено; не блокирует `A1–A7`.

**Действие:** когда продуктовые потоки устраивают владельца, одним пакетом утвердить visual direction, tokens и reference screens для auth/onboarding, chat, Space/voice, matchmaking и settings/profile. Не более двух review rounds на пакет; детальная registry/GAP/export/parity-проверка остаётся агентской.

**Разблокирует:** статус `visual-approved` и финальную brand polish, но не functional alpha.

**Готово / доказательство:** владелец утвердил visual direction, tokens и ключевые Penpot screens/states; список утверждённых snapshot зафиксирован для последующего agent parity pass.

## A — автономная работа агентов

Milestone — законченный пользовательский результат. Его можно дробить на небольшие PR и параллельные сервисные треки, но нельзя объявлять готовым по числу закрытых TODO. `Dependencies` ниже никогда не требуют незавершённого `H`: live activation вынесена в release gates.

### A1 — Пользователь может ежедневно переписываться

**Результат:** два новых пользователя без ручной настройки проходят onboarding, находят друг друга и стабильно общаются в Web.

**Объём:** email/guest/session/reset, friends/request/block, DM/group/channel/thread, realtime reconnect, global inbox catch-up, history catch-up per selected chat, unread/read state, archive/folders/Quick Access, минимальный account soft-delete и базовые attachments. Для A1 soft-delete означает немедленный отзыв sessions, запрет новых DM в обе стороны, скрытие удалённого peer из новых snapshots и один локальный неперсистентный terminal marker «Пользователь удалён» в уже загруженной истории. Полная 30-дневная erasure/pseudonymization, durable cross-service tombstones и restore UX остаются будущим scope `A4`. Auth перестаёт ходить напрямую в `user_db` для profile/verification/phone flows и использует User contract. Исправляются видимые profile/chat switching regressions.

**Dependencies:** существующие Auth, Chat, Messaging, Realtime, Social, User и File contracts; внешние providers не нужны.

**DoD:** новый пользователь проходит путь без CLI и hidden token; Auth не требует credentials к `user_db`, межсервисные profile operations идут через contract и fail closed; после reconnect клиент завершает global paginated inbox snapshot и не стирает cache при неуспешной странице, DM/group/channel сохраняют per-member unread→read; attachment переживает service restart и скачивается с тем же hash; полная история приходит через cursor только per selected `chat_id`; block/privacy deny не fail-open; после account soft-delete старые sessions недействительны, send в обе стороны запрещён, fresh snapshot скрывает DM, а локально загруженная история показывает один local non-persisted terminal marker «Пользователь удалён»; empty/error/offline состояния не маскируют потерю данных.

**Verification:** unit/integration по затронутым сервисам, Flutter widget tests и live multi-account compose E2E для login → contact/request → DM/group/channel → per-member unread/read → attachment upload → restart/reconnect → global inbox snapshot (`main` / `requests` / `archive`) → history for selected chat → download/hash → block/delete.

### A2 — Invite-only Space с рабочим голосом

**Результат:** группа создаёт закрытый Space, настраивает дерево и роли, входит в голосовые комнаты и безопасно управляет жизненным циклом Space.

**Объём:** Gateway/Flutter vertical slice для join/leave/transfer, text/voice tree, invites, role checks на всех входах, roster/session events, движение между комнатами, join/speak/mute permissions и понятные RTC states. Audit log получает доступный owner/admin path. Delete flow следует зафиксированному контракту: owner confirmation, 7 дней hidden/frozen recovery, затем идемпотентный cross-service purge и attachment GC; это узкий последующий slice, не blocker остального `A2`.

**Dependencies:** `A1` identity/chat foundation; local LiveKit и test media достаточно.

**DoD:** owner не заперт в Space; invite/member/role transitions согласованы; запрещённое действие не проходит через альтернативный transport; два test-клиента реально передают и получают media track, а не только signaling; пользователь видит connecting/reconnecting/permission/device errors; session cleanup детерминирован. Audit read path сохраняет lifecycle entries и доступен только owner/admin по каноническому ACL.

**Verification:** Space/Role/Voice integration tests, Flutter widget tests и compose E2E create → invite → join → text → test-media exchange → move → role deny → audit read/persistence → transfer/owner leave.

### A3 — Полный цикл поиска команды

**Результат:** Voice демонстрирует своё отличие: solo или текущая voice-party находит команду, общается во временном text+voice контексте и завершает матч с историей.

**Объём:** snapshot party из voice roster, queue reset при изменении состава, search/match events, accept/decline/timeouts, temporary squad chat/voice, cleanup, post-match rating/history и `mm_ban`. Catalog остаётся источником games/modes/roles.

**Dependencies:** `A2` roster/session events; текущий game catalog.

**DoD:** состав party нельзя подменить client payload; одновременные join/leave/cancel идемпотентны; участники найденной команды обмениваются сообщением и test-media во временном text+voice контексте; отказ/timeout/завершение освобождают ресурсы, а после cleanup старый контекст отклоняет доступ; история и санкции видны в следующем поиске.

**Verification:** deterministic service tests, race/idempotency cases и multi-client compose E2E solo + party → match → temporary chat send/receive + test-media exchange → result → history → cleanup → authorization deny/ban.

### A4 — Репозиторий готов к безопасному внешнему alpha

**Результат:** репозиторий содержит staging-compatible harness, с которым оператор после выдачи доступов может развернуть exact SHA, доказать безопасность базовых потоков, восстановить данные и получить сигнал о P1.

**Объём:** fail-closed privacy/S2S, report → resolve/dismiss → sanction → appeal, полный account deletion lifecycle поверх A1 soft-delete (30-дневное restore window, затем необратимая PII/credential/profile-media erasure или pseudonymization и cross-service tombstones), alpha admission guard, versioned release-profile/capability/client-compatibility, operational-tier и durable-store manifests, secret/config validation, repeatable migrations, backup/restore automation, Loki/request-id chain, Prometheus/Grafana, actionable alerts, degraded-mode checks и минимальные runbooks для deploy/canary/rollback, restore, Redis/NATS triage и P1 ack/escalation. Full per-service runbooks и chaos program остаются вне alpha gate. Product Analytics не входит в launch observability gate.

**Dependencies:** `A1–A3` для полного product smoke; инфраструктурный код проверяется раньше на disposable/local environment.

**DoD:** автономный harness закрывает пять доказуемых блоков:

- Capability/admission: exact-SHA manifest и config validator запрещают dev defaults/test checkout; operator/admin mint path аудируется; email и guest проходят invite mint → atomic single-use redeem с expiry/revoke; replay, concurrent redeem и cap race отклоняются. Из каждой disabled-записи генерируется negative UI/REST/WS suite. Federation отсутствует в release images/rollout/readiness.
- Client compatibility: каждая заявленная OS/arch/browser tuple получает smoke; risk-based sampling применяется только к расширенному regression и записан в matrix. Previous/current Web+Windows проверяются с previous/current backend, version/forced-update/update-URL flow; backend rollback совместим с уже обновлённым клиентом.
- Deploy/recovery: на disposable environment повторяются deploy/migrate, expand-contract, canary с ненулевым synthetic sample и измеримый rollback trigger. Backup из source восстанавливается только в isolated validation namespace/cluster с synthetic tenant/object и никогда не пишет обратно в source. Manifest покрывает authoritative DB/object/event stores из [DATA_STORES.md](DATA_STORES.md), проверяет cross-domain invariants и rebuild исключённых cache/projections. Drill измеряет RPO/RTO относительно agent-recommended provisional thresholds; владелец принимает их только в `G1/H4`.
- Operations: Role и Matchmaking имеют Tier 1 SLO/dashboard/alerts/degraded mode; все Tier 0/1 scrape targets и datasources healthy; login/DM даёт request-id chain Gateway → Messaging → NATS → Realtime; P1 проверяется через alert sink.
- Product smoke: mail sink/fake provider доказывает register/verification/reset и expired/bounce; затем идут A1 DM/group/channel unread/read + attachment restart/hash/delete/restore, report → resolve/dismiss → sanction enforcement + WS/UI delivery → appeal без ручного `sanction_id`, account delete → immediate deny/hide → restore и test-clock expiry → irreversible erasure/tombstone, A2 Space/voice и A3 temporary chat/test-media/history/cleanup/authorization deny.

**Verification:** CI/full на exact SHA, config/render tests, migration rerun, disposable backup→restore, canary observation/rollback-trigger drill, compose/staging-compatible smoke и review [OPERATIONS.md](OPERATIONS.md) / [DEPLOYMENT.md](DEPLOYMENT.md).

### A5 — Web/Windows становится daily driver

**Результат:** в test-signed alpha-like Web/Windows окружении пользователь устанавливает или открывает Voice, восстанавливает session и выполняет ежедневные chat/Space/profile flows; публично доверенный zero-warning путь завершается только в `G1`.

**Объём:** versioned supported Windows OS/arch и Web browser/version matrix; Windows packaging/startup/update и native Realtime path с agent-generated test signature, immutable production-compatible artifact manifest, sandbox update feed и disposable trust setup; system tray, voice while hidden и global PTT/hotkeys; responsive technical shell; folders/Quick Access/archive/requests/search; profile switch/manage; presence/privacy; notification center и read state; безопасный Space catalog/templates. Включается только то, что имеет честные loading/error/permission states. Субъективная visual polish относится к `H5`.

**Dependencies:** `A1–A4`; live push не требуется для Web/Windows free alpha.

**DoD:** test-signed install/update/cold start/reconnect сохраняют session в disposable trust setup; public trusted signature и clean-machine zero-warning install относятся к `G1`. Closing window to tray не обрывает voice; global PTT/hotkeys работают и не срабатывают в запрещённом состоянии. Keyboard позволяет открыть chat, folder/archive, search, profile и Space; notification center и read state согласованы; invisible/privacy gates не обходятся; catalog не раскрывает private Space/profile; unsupported mobile-only или premium действия скрыты либо объяснены.

**Verification:** Flutter CI, widget tests стабильных technical contracts, Windows install/update/start → tray/background voice → global PTT/hotkey smoke и Web/Windows E2E session restore → navigation/search → notification/read → presence/privacy → profile switch → Space catalog/join.

### A6 — Rich communication и media pipeline

**Результат:** общение поддерживает ожидаемые современные форматы без ручной обработки и не создаёт неконтролируемое хранение.

**Объём:** silent/scheduled/when-online delivery, stickers/GIF, voice notes, rich forward validation, file dedup/references, async processing, safe previews, image/video/GIF/PDF transformations, retention/quota и screen-share source picker/system audio там, где платформа позволяет. Third-party GIF search использует provider-neutral adapter и deterministic local fixture catalog; system/user-upload GIF не зависит от live provider.

**Dependencies:** `A1` messaging/file foundation и `A2` media session foundation; premium quota тестируется seeded entitlement.

**DoD:** каждый тип контента имеет единый validation/ACL path; retry идемпотентен; processing status виден клиенту; orphan cleanup и retention доказаны; forward не обходит attachment rules; unsupported capture явно недоступен. Без выбранного provider/key live GIF search выключен через capability manifest, а adapter полностью проходит fake contract suite.

**Verification:** contract/property/integration tests, deterministic GIF-provider fake, processor failure/retry cases, storage lifecycle tests и Flutter E2E composer → send/forward → preview/download. Live provider activation — отдельный release-capability check после точечного `H1` выбора и `H2` key.

### A7 — Provider-ready monetization

**Результат:** бесплатный продукт не зависит от платежей, а engineering для paid beta полностью готов на fake/sandbox; live-включение отдельно требует `G1`, merchant/legal setup из `H2` и acceptance/go-no-go из `H4`.

**Объём:** provider-independent subscription state machine, versioned paid-offer/capability manifest, checkout adapters, webhook signature/idempotency, cancel/resume/renew/failure, grace/downgrade/freeze/unfreeze, `subscription.events` и Auth JWT tier. При входе milestone в WIP агент сам материализует и version-freeze полную canonical benefit matrix из [subscription.md](features/subscription.md), по умолчанию включая все записанные benefits: User/Flutter cosmetics, banner/status/username/themes/icons; Messaging — до трёх реакций; Chat/Space — лимиты; File — размер/priority; Voice — stream quality и room limit; multi-profile freeze/unfreeze; Space Pro member/channel limits, banner и custom emoji; Notification — grace reminders. Exclusive stickers/emoji, animated media и custom emoji опираются на `A6`. `H1` нужен только при реальном конфликте канона или предложении исключить benefit; такой вопрос не останавливает остальные slices. Исключённый benefit сначала получает явное scope-решение и обновление канона, а не просто исчезает из UI. UI использует технический дизайн и не показывает test URL как продукт.

**Dependencies:** `A1–A6` и стабильные limits/contracts из активных features; fake provider и seeded entitlements достаточны для автономного milestone.

**DoD:** entitlement меняется только из доверенного lifecycle; replay/out-of-order webhook безопасен; downgrade не теряет данные и предлагает выбор; все consumers сходятся после redelivery; каждый benefit в paid-offer manifest имеет server/client E2E и совпадает с checkout/plan management; disabled billing оставляет полностью рабочий free product.

**Verification:** provider contract suite с fake/sandbox, webhook replay/order tests, NATS redelivery E2E, benefit-matrix coverage, grace/downgrade scenarios и Flutter checkout/management tests без реальной карты.

После `A7` новый цикл планирования выбирается по данным alpha: mobile release, расширенная verification, bots ecosystem, полный stories editor и прочие current-scope расширения. Они остаются в feature docs/TODO, но не конкурируют с активными milestone по числу checkbox.

## Release gates

| Gate | Назначение | Требования |
|---|---|---|
| `G0 — product proof` | Локальная демонстрация ценности без внешних аккаунтов | `A1–A3`; зелёный multi-client compose E2E |
| `G1 — free invite alpha` | Ограниченный Web/Windows запуск без billing и mobile push | `A1–A4`; из `A5` — production signed artifact/feed, supported client matrix, old→new update/fallback, tray/background voice/PTT, stable session/reconnect, responsive shell, keyboard critical path и notification/read consistency; из `H2` — Resend, R2 и encrypted off-cluster backup credentials/policies, Windows signing и реальный alert destination; `H3` и соответствующая часть `H4`; staging qualification пройден, затем на `production/voice-prod` для того же exact SHA доказаны persistent storage, invite admission/cap и negative signup, capability/ops/client-compatibility manifests с Role+Matchmaking Tier 1, negative disabled-path и degraded-mode smoke, email register/verification/reset delivery, attachment upload→restart→download/hash→delete/restore, expand-contract deploy/canary с ненулевым synthetic sample и SLO observation window, server rollback after client update, target-generated full-profile backup восстановлен в isolated validation target без записи в production с принятыми alpha RPO/RTO, healthy Tier 0/1 scrape+dashboards, request-id chain, P1 receipt и multi-client smoke A1–A3 из `A4` |
| `G2 — paid beta` | Включение Premium/Space Pro | `A7`; на том же exact G2 RC SHA повторены G1 regressions/free path с billing on и off, target canary/rollback; ops-manifest повышает Subscription и критические entitlement consumers до Tier 1 и доказывает dashboards/alerts/P1-P2 receipt для provider/signature/payment errors, webhook lag и NATS redelivery/convergence; durable-store manifest/restore повторён с `subscription_db` и provider reconciliation source, сохраняя active/cancel-scheduled/grace entitlements и идемпотентный webhook replay без двойного доступа/списания; offer manifest согласован с [subscription.md](features/subscription.md), каждый продаваемый benefit имеет E2E; provider/market matrix разрешает checkout только для отдельно включённых провайдеров, и для каждого live path доказан `purchase → signed webhook → entitlement → cancel scheduled` с entitlement до period end; sandbox/test clock доказывает period-end и failed-payment grace → downgrade/freeze; sandbox/live используют разные webhook secrets/URLs, а negative cross-environment event test доказывает отказ; merchant/legal часть `H2` и go/no-go из `H4` |
| `G3 — mobile beta` | Android/iOS distribution, push и universal links | gate не активен до следующего planning cycle: после `G1` сначала создаётся отдельный autonomous mobile milestone с DoD; затем нужны signing/push из `H2`, prod links из `H3` и device acceptance из `H4` |
| `G4 — visual-approved` | Финальный единый визуальный уровень | `H5`; versioned matrix активных platforms/screens и loading/empty/error/permission states; agent parity pass; exact-SHA visual report, где каждый diff исправлен или имеет явный owner waiver; cross-platform QA |

`G1` не ждёт `G2–G4`. Phone auth, paid tier, mobile push, public discovery и незавершённые rich types должны быть выключены или честно ограничены, а не изображать готовность.

## Как агенты исполняют план

- Одновременно активен ровно один `A` milestone. Это один общий outcome и один integration queue, а не один последовательный агент: captain заранее дробит milestone на независимые `T-*` задачи по контрактам, сервисам, Flutter и verification.
- Все готовые независимые задачи активного milestone запускаются максимально параллельно в изолированных treehouse worktree: один crew — один worktree и непересекающиеся write scopes. Профили/контракты, backend-сервисы, Flutter и verify идут параллельно, когда их входы уже определены; зависимые slices ждут contract seam, а не создают второй milestone.
- Fleet state ведётся локально в `tmp/fleet/`: у каждой `T-*` есть outcome, scope, канон, worktree, owner profile, verification и integration status. Результаты интегрируются через git/PR, worktree возвращается после merge или отмены.
- Первый приоритет — закрыть пользовательский вертикальный путь и его failure states. Массовая чистка Low/Common не получает fleet раньше milestone DoD.
- Один PR содержит одну проверяемую смысловую задачу. Большой milestone закрывается серией небольших PR, каждый с тестами своего слоя; следующий `A` milestone не входит в code WIP до интеграции всей серии и зелёного vertical DoD текущего.
- Новый Critical/High vertical behavior получает feature/compose E2E в том же PR. Для неизбежной staged contract chain каждый промежуточный PR несёт contract test и выключенную capability; финальный enabling PR добавляет live E2E до включения флага.
- Смешанный blocker оформляется как handoff package: что уже готово, рекомендуемый default, точное действие владельца, безопасная проверка и какие независимые треки продолжаются.
- Если спецификация достаточна, агент не ждёт Penpot или дополнительного подтверждения. Если поведения нет в каноне, агент не изобретает его и записывает узкий вопрос в `H1`.
- Статус строки в матрице обновляется в том же PR, где существенно изменился пользовательский поток. TODO чистится после merge, а не хранит журнал закрытых batch/PR.

## Неподвижные архитектурные границы

- Auth остаётся Java/Spring в `src/backend/auth/`; перенос в Go требует отдельного решения.
- Realtime в `src/backend/realtime/` владеет WebSocket event flow и протоколом `s` / `resume`.
- После reconnect global inbox state сверяется через Chat REST `ListChats` и durable Messaging metadata; история сообщений догружается через Messaging REST/API с cursor отдельно для выбранного `chat_id`. Глобального WS replay/event-log catch-up нет.
- Сервис владеет своей БД; межсервисные записи идут через контракт или событие, а не прямой JDBC/SQL в чужую схему. Auth обращается к User-owned profile данным только через User gRPC и не получает credentials к `user_db`.
- Node.js для frontend/CI — 24.
- Federation и её производные остаются deferred; scaffold может компилироваться в full-repo CI, но `G0–G4` profiles исключают Federation deployment, DB/migrations, readiness и alerts.

## Верификация

Минимальная проверка определяется затронутым слоем и [TESTING.md](TESTING.md). Для milestone используются:

| Слой | Базовая проверка |
|---|---|
| Proto/contracts | `buf lint`, `buf format -d --exit-code` и breaking check по правилам CI |
| Go/backend | tests изменённых модулей; перед release candidate — `make build-all` и полный service matrix |
| Auth Java | Maven tests и Flyway validation в `src/backend/auth/`; после `mvn -B test` CI проверяет Surefire reports: каждый Auth suite с `@Testcontainers(disabledWithoutDocker = true)` существует, выполнил `tests > 0`, не был skipped и не содержит failures/errors |
| Flutter | `make flutter-ci`; Web/Windows и нужные widget/integration paths |
| Интеграция | feature E2E из [`.github/ci/e2e-features.yml`](../.github/ci/e2e-features.yml) |
| Compose smoke | `make compose-e2e-smoke` |
| Полный compose | `make compose-e2e-live` |
| Release | exact-SHA deploy/render, migrations, canary observation, authenticated smoke, backup→restore, observability и rollback-trigger evidence |

Live provider/device проверки добавляются только в соответствующий gate. Их отсутствие не превращает успешно проверенный fake/sandbox milestone в «заблокированную работу», но запрещает объявлять live gate пройденным.
