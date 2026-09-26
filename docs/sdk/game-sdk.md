# Voice Game SDK — Unity и Unreal

**Proposed target, SDK ещё не поставляется.** [Продукт](../features/game-integrations.md),
[Game API](../architecture/game-integration-api.md), [acceptance](../testing/game-integrations-acceptance.md).
Сигнатуры ниже — псевдо-API для согласования developer experience, не компилируемый
пример готового пакета. SDK не зависит от установленного приложения Voice.

## 1. Состав поставки

| Артефакт | Контракт |
|---|---|
| Unity package | C# async API, lifecycle component, main-thread events, optional prefabs |
| Unreal plugin | C++ module, Blueprint async nodes/delegates, game-instance subsystem, optional UMG widgets |
| Shared contract | REST schemas, event/error enums, fixtures, compatibility/conformance tests |
| Media adapter | Подключение к Voice-approved LiveKit room, capture/playback, устройства |
| Backend examples | Minimal trusted game adapter, roster sync, token exchange, webhook verification |
| Samples | Четыре игрока/party; game event → messenger action; reconnect/revoke demo |
| Developer docs | Quickstart, API reference, permissions, debugging, migration, release notes |

Не требуется заранее выбирать единый C++ core для обоих движков. Общими обязаны
быть wire semantics и conformance tests; допустимы native C#/C++ wrappers поверх
поддержанного media runtime. Поддержка Unity/Unreal в выбранной версии LiveKit
проверяется отдельным spike до обещания платформ; Flutter media code не переносится
в игровой процесс автоматически. Собственный codec/SFU не входит в SDK.

## 2. Платформы и release matrix

| Scope | Платформы | Доказательство |
|---|---|---|
| Единый спринт | Windows x64; Unity и Unreal | Оба packaged builds на двух компьютерах, общий conformance suite, real audio, reconnect/revoke |
| Не включено в текущую platform matrix | Linux/macOS | Для заявления поддержки нужны native build, devices, packaging/signing, network fault suite |
| Не включено в текущую platform matrix | Android/iOS | Для заявления поддержки нужны background/foreground, interruptions, permissions, push/link handoff |
| Consoles / WebGL | Не обещаны; отдельная capability/review | Platform requirements и media support proof |

В рамках подзадачи GI4 фиксируются точные engine versions, native dependency versions и
поддержанные scripting/backend modes. Unity IL2CPP/AOT, stripping/link.xml и
архитектуры native libraries проверяются в player build, не только Editor.
Для Unreal проверяются Development/Shipping, packaging, plugin version,
Blueprint-only проект и dedicated-server target без audio initialization.

## 3. Developer quickstart как проверяемый путь

1. Зарегистрировать application и sandbox environment; получить публичный app ID.
2. Включить необходимые capabilities, зарегистрировать redirect/provider и
   webhook адаптера. Server credentials поместить в secret store backend.
3. Установить подписанный/versioned package. Добавить `VoiceClient` component
   либо subsystem. Передать публичный app/env ID и token provider callback.
4. Реализовать backend login proof и `EnsureSession`/roster sync по Game API.
5. Добавить готовую party-панель либо свои UI bindings. Подключить join/leave,
   mute/PTT/device controls, consent и report.
6. Запустить sample с двумя sandbox-пользователями; провести reconnect,
   exclusion, refresh failure и application shutdown.
7. Сформировать production readiness report. Production credentials никогда
   не требуются для запуска примера, автоматических тестов и SDK build.

Время установки и первого успешного разговора измеряется на разработчике, который
не участвовал в создании SDK. Нельзя обещать «пять минут», пока это не проверено.

## 4. Минимальная поверхность SDK

| Модуль / метод | Поведение |
|---|---|
| `Initialize(config, tokenProvider)` | Проверяет версии/capabilities, не включает микрофон |
| `Identity.Connect()` | Разрешённый binding flow с выбором профиля |
| `Identity.Disconnect()` | Выход локального SDK; unlink — отдельное явное действие |
| `Sessions.Get(sessionId)` | Читает серверный state/roster без trust клиентскому списку |
| `Sessions.Join(sessionId)` | Получает только разрешённые grants и подписки |
| `Sessions.Leave(sessionId)` | Закрывает local resources; изменение game roster решает backend |
| `Chat.ListMessages(chatId, cursor)` | История через Messaging, grant-scoped |
| `Chat.Send(chatId, text, clientMessageId)` | Pending/confirmed/failed; стабильный ID при retry |
| `Voice.Join(roomId)` / `Leave()` | Admission + media, асинхронный результат |
| `Voice.SetMuted/Deafened` | Локальное состояние и соответствующий серверный state |
| `Voice.SetInputDevice/OutputDevice` | Проверка наличия; событие ошибки без crash |
| `Voice.SetPushToTalk(active)` | Capture только при разрешённом speak и активной кнопке |
| `Presence.SetActivity(activity)` | Только своя игра, privacy-aware |
| `Invites.Create/Accept` | Opaque invite, expiry и серверная проверка |
| `Safety.Block/Report` | Делегирует каноническим Voice flows, без локального-only бана |
| `Dispose/Shutdown` | Отмена задач, audio stop, unsubscribe, безопасное освобождение |

Для первого входа SDK/sample рекомендует сразу сообщать «Общение через Voice»
и предлагать подключение аккаунта независимо от того, есть ли он у игрока.
Форма экрана остаётся за игрой. SDK знает только состояние binding; регистрация
или вход происходят в Voice flow. `sdk-account` — отдельный от `guest` тип.
Его capability должна поддерживать переход в новый и существующий постоянный
аккаунт, preview конфликтов, operation status и recovery; конкретные методы
конвертации утверждаются в G01. Pending conversion приостанавливает затронутые
сессии; после завершения SDK очищает старый scoped cache и получает новые grants.
См. [пользовательский сценарий](../features/game-integrations.md#рекомендуемый-первый-вход-для-любой-игры)
и [протокол конвертации](../architecture/game-integration-api.md#конвертация-sdk-account).

Серверные `CreateSession`, `SyncRoster`, `PublishGameEvent` не экспортируются как
привилегированные методы player SDK. При игре без backend вызывается отдельный
managed broker по доказанной player identity; публичный host может запросить
действие, но не сам удостоверить произвольное членство.

Пример желаемого Unity flow:

```csharp
// Pseudocode: names define the proposed developer experience only.
await voice.InitializeAsync(config, gameAuth.GetVoiceTokenAsync, cancellation);
await voice.Sessions.JoinAsync(sessionId, cancellation);
partyPanel.Bind(voice.Sessions, voice.Chat, voice.Voice);
// Called by the player's explicit microphone/join control.
await voice.Voice.JoinAsync(roomId, cancellation);
```

В Unreal тот же flow выражается async Blueprint nodes
`Initialize → Join Session → Bind Party Widget → Join Voice`, с отдельными
success/error/cancel outputs. Blueprint graph не содержит secrets или raw JWT
debug prints. C++ API и Blueprint wrapper возвращают одинаковые error codes.

## 5. Lifecycle и threading

Client state: `Uninitialized → Initializing → SignedOut/Ready → Connecting →
Connected → Reconnecting → Connected`; terminal `Revoked`, `UpgradeRequired`,
`Disposed`. Chat availability и media availability независимы: потеря audio
device не разлогинивает текстовый чат.

- Auth/HTTP/media выполняются вне game thread. UI callbacks приходят в engine
  main thread с документированным порядком для одного ресурса.
- Refresh single-flight: несколько одновременных 401 не создают несколько
  refresh rotations. Revoked grant не refresh-ится в бесконечном цикле.
- Все длительные операции принимают cancellation; timeout возвращает stage и
  operation ID. Отмена ожидания не отменяет уже committed серверное действие.
- SDK имеет один экземпляр на локального игрока; split-screen/multi-user не
  заявлены до отдельного identity/audio дизайна.
- Scene reload и PIE/Editor stop не оставляют захват микрофона, tasks или callbacks.
  После Dispose callbacks к уничтоженным Unity objects/UObjects запрещены.
- Backoff следует Realtime contract (1s → 2s → 4s, cap 30s) с jitter;
  ограниченный retry budget и явное offline состояние вместо reconnect storm.
- Suspend/resume проверяет grant заново; старый session token не делает нового
  участника допустимым после удаления из lobby.

## 6. Голос и аудио

SDK предоставляет capture/playback, device selection, per-user volume, mute,
deafen, PTT/voice activation, speaking indicator и diagnostics. Начальные режимы
и доступность suppression/echo cancellation проверяются на выбранной media
зависимости; лицензии и native binaries входят в release inventory.

Порядок admission: актуальная identity → membership/Role/lifecycle → короткий
media grant → LiveKit connection. Подключённый transport не равен разрешению
говорить. Server mute, потеря speak/join, ban и freeze применяются к media,
не только к UI. Voice revocation использует канонические 2s p95 / 5s max;
[game federation](../architecture/game-federation.md) обязана обеспечить тот же
предел или не включается для voice.

Смена устройств, unplug USB, Bluetooth profile change, отсутствие разрешения и
возврат из background имеют явные события. Нельзя автоматически снять mute после
reconnect или включить input по умолчанию, если пользователь выбрал другой.

**Игра и Voice одновременно:** одна логическая voice session на аккаунт по
канону multi-profile. Central Voice admission coordinator атомарно резервирует
одну room/session для аккаунта до выдачи любого media grant на master или node.
Listen-only join тоже занимает эту session; одна capture lease сама по себе
не запрещает слушать вторую комнату. При недоступности coordinator новый join
отклоняется. Capture lease одного device/client instance подчинена этой записи.
Второй клиент может читать текст и видеть разрешённый roster, но не открывает
вторую voice session. Передача звука требует действия игрока, fencing старого
instance и атомарного обновления admission. Без handoff coordinator пилот должен
отклонить второй join с предложением выйти из первого.

Текущий Voice design допускает reconnect с ранее выданным LiveKit bearer до его
≤60s expiry с повторным eject по SLA; изменение epoch само по себе JWT не
инвалидирует в SFU. SDK не обещает deny-before-connect поверх этого поведения.
Для строгой node authority lease нужен проверяющий admission механизм на media
стороне, описанный в game federation; одного короткого app token недостаточно.

Proximity audio — отдельная последующая capability: positions поступают от
авторитетной игры, наружу не раскрываются без необходимости, дальность и
occlusion задаёт игра. Обычный party voice работает без координат. Spatial audio
не заменяет правила доступа; отключённый звук не скрывает утечку запрещённого track.

## 7. Текст и recovery

- История/attachments используют существующие Message IDs и permission checks.
  Pending send имеет стабильный `clientMessageId`; UI reconciles его с server ID.
- Локальная очередь ограничена размером/временем. После revoke сообщения не
  отправляются автоматически при новом binding/profile.
- После reconnect сначала сверяются roster/доступные chats, затем выбранная
  история по cursor каждого чата. Realtime `s` не используется как history cursor.
- Порядок live событий может отличаться от ответа send; dedupe выполняется по
  server message ID и client ID. Повтор события не создаёт вторую строку/notification.
- Смена окружения, logout, unlink и потеря доступа очищают scoped cache и pending
  actions по retention/security policy; кеш чужого профиля не показывается.
- Разметка ограничена безопасными компонентами; произвольный HTML/JS из игры
  или сообщения не исполняется. URLs проходят allowlist соответствующего действия.

## 8. UI и accessibility

Prefab/widget должен работать с клавиатурой и controller focus, иметь различимые
mute/deafen/speaking/reconnect состояния, readable text scale и локализацию EN/RU.
Speaking indicator не передаёт состояние только цветом. Горячая клавиша PTT
переназначается, не перехватывает ввод игры без согласия. При отсутствии микрофона
доступен текст и прослушивание, если разрешено.

В клиенте явно видны выбранный профиль, игровая identity и оператор community
node. Не показывать внутренние IDs, token errors или stack traces обычному игроку;
request ID можно скопировать из diagnostics. Permission consent не прячется в
общий checkbox принятия правил игры.

## 9. Packaging, security и поддержка

Release artifact содержит version, checksum/signature, native dependency/license
inventory, supported engine/platform matrix, sample project и changelog. Release
build не логирует tokens, raw webhook bodies, личную переписку или все profile IDs.
Diagnostic bundle собирается по явному действию и редактирует чувствительные поля.
Crash/reconnect telemetry имеет app/env/sdk/platform tags, без conversation content.

SDK не открывает произвольный URL ноды, переданный игрой: endpoint discovery только
через доверенный registry. Не следует редиректам с Authorization на другой host.
Refresh credentials хранятся в OS storage; в логах и Unity PlayerPrefs их нет.

Feature negotiation отделена от semantic SDK version. Unsupported feature
возвращает typed error/fallback, а не partially functioning widget. Security
update policy, поддержка старых major versions и срок deprecation — G05; без них
пакет не объявляется стабильным публичным SDK.

## 10. Готовность SDK

Editor demo недостаточен. Gate включает packaged build, реальные два устройства,
packet loss/network switch, hotplug и permissions, revoke во время speaking,
API contract tests, native leak/crash smoke, cleanup при смене сцены, sandbox/prod
isolation и модифицированный клиент, пытающийся подставить чужой chat/profile.
Полная матрица и зависимость от backend —
[acceptance](../testing/game-integrations-acceptance.md).
