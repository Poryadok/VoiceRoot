# Репозитории: имена, монорепо, Protobuf

Нормативные правила. Git и ветка `master`: [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Именование на GitHub

- Имя репозитория — **PascalCase** (верхний верблюжий регистр), например `Voice`, `VoiceInfrastructure`.
- Имя — короткое, без лишних дефисов; префикс организации в имени не обязателен.
- **Go**: module path в `go.mod` — в **нижнем регистре** по конвенции языка (`github.com/<org>/voice/...`). Регистр имени репозитория на хостинге с путём модуля не обязан совпадать.

---

## Монорепозиторий

- Репозиторий `Voice`: все микросервисы бэкенда, `docs/`, корень Protobuf/gRPC — **всегда** здесь. Отдельный репозиторий на один микросервис **не** заводим.
- Клиент и админка — в `Voice`, если не принято решение о отдельном репозитории клиента (см. «Отдельный репозиторий»).
- Структура прикладного кода: **`src/frontend`** (Flutter), **`src/backend`** (Go-сервисы, gateway, общие библиотеки, SQL-миграции), **`src/admin`** (веб админки). Альтернатива `services/<имя>/` не используется в этом репозитории; точка входа и команды — [README.md](../README.md) в корне.

---

## Protobuf

- Все `.proto` — в **этом же** репозитории, единый корень контрактов: `protos/` ([`protos/buf.yaml`](../protos/buf.yaml), [`buf.work.yaml`](../buf.work.yaml)).
- В CI: линт и формат контрактов (**buf**); при изменении публичных API — проверка обратной совместимости. **Breaking change** допускается только с явным описанием в PR и согласованным порядком выката потребителей.
- Сгенерированный код: политика ниже (один источник истины — `protos/`; артефакты в git или при сборке).

### Резервирование удалённых полей

При удалении поля резервируются его номер и имя в том же полностью
квалифицированном message. Историю сверяют с текущим descriptor: уже занятый
номер нельзя резервировать или перенумеровывать ради исторической схемы.
Если старое имя свободно, допускается резервировать только имя, сохраняя
текущие поля. Удалённые messages не восстанавливают ради резервов; для домена
без подтверждённых удалений резервы не придумывают.

Аудит merged history до `698d4f064f032ea74abf3ede2017100b26c8ee69`
подтвердил свободные пары `PrivacySettings.13/show_online_include_guests`
и `BanFromMMRequest.3/until`. Дополнительно зарезервированы только свободные
исторические имена:

| Message | Зарезервированное имя |
|---|---|
| `bot.GetWebhookURLResponse`, `bot.GetChatWhitelistResponse` | `webhook_url_response`, `whitelist_response` соответственно |
| `calls.GetJoinTokenResponse`, `calls.GetVoiceStatesResponse` | `join_token_response`, `voice_state_list` соответственно |
| `chat.ChatList`, `events.RatingSubmitted` | `chats`, `profile_id` соответственно |
| `file.GetFileURLResponse`, `messaging.GetBulkReadStateResponse` | `file_url_response`, `bulk_read_state` соответственно |
| `story.GetStoryFeedResponse`, `user.GetBulkPresenceResponse` | `story_feed_response`, `bulk_presence_response` соответственно |
| `matchmaking.BanFromMMRequest`, `UnbanFromMMRequest`, `GetMMBanStatusRequest` | `account_id` |
| `notification.RelayNotificationRequest` | `user_id` |
| `s2s.AuthenticateUserResponse`, `RoleChangedEvent`, `UserBannedEvent`, `UserUnbannedEvent`, `RoleEntry`, `BanEntry`, `NotifyUserRequest` | `user_id` |

В таблице опущены общие части package `voice.<domain>.v1`. Номера этих
name-only резервов остаются заняты текущими полями; `stars` остаётся активным
именем в `RateMatchRequest` и `RatingSubmitted`. Проверка `buf breaking`
сохраняет профиль `FILE` и текущую integration base (`master`/PR base).
Это не доказательство совместимости со всеми исторически выпущенными
клиентами и не исправление прежних wire/JSON breaks при flatten/rename.
Такое исправление требует отдельного migration решения и rollout потребителей
с точным опубликованным SHA/descriptor baseline.

### Политика codegen (что коммитим, что генерим)

| Артефакт | Путь | В git | Как обновить локально | Проверка в CI |
|----------|------|-------|------------------------|---------------|
| Исходники контрактов | `protos/**/*.proto` | да | правка вручную | job `protobuf`: `buf lint`, `buf format -d`, `buf breaking` (PR vs base) |
| Go `*.pb.go` / gRPC | `src/backend/*/pb/voice/**` (вложенные `go.mod` у потребителей) | **да** | `make buf-generate` → `gen/go` (gitignored), затем **`make sync-pb-from-gen`** (или `make buf-generate-all`); альтернатива для voice: `buf generate --template buf.gen.local.yaml` | job `protobuf`: `scripts/ci/buf-go-pb-check.sh` сравнивает отслеживаемые и новые файлы в committed `pb/` trees с генерацией |
| Go scratch | `gen/go/` | **нет** (`.gitignore`) | `make buf-generate` или `make buf-generate-ci` (Docker) | `buf-generate-ci` — smoke генерации; drift-check выполняет `buf-go-pb-check` |
| Dart / Flutter | `src/frontend/lib/gen/` | **да** | `make buf-generate-dart` (`buf.gen.dart.yaml`, `protoc-gen-dart`) | job `flutter`: **`make buf-dart-check`** (реген + `git diff --exit-code`) |
| Java (Auth) | `src/backend/auth/target/generated-sources/` | **нет** | `mvn compile` / `mvn test` из `src/backend/auth` (proto-копия в `src/main/proto/`) | job `build-all` → `auth-test-ci` |

**Сборка Docker-образов Go-сервисов** использует **закоммиченные** `pb/`; `gen/go` в образ не копируется. После изменения `protos/` в PR: обновить затронутые `src/backend/*/pb/` и `src/frontend/lib/gen/`, прогнать `make buf-dart-check` и релевантные `go test`.

Команды Makefile (корень репо): `buf-lint`, `buf-format`, `buf-breaking`, `buf-generate`, `buf-go-pb-check`, `sync-pb-from-gen`, `buf-generate-all`, `buf-generate-dart`, `buf-dart-check`, `buf-ci`, `buf-breaking-ci`, `buf-generate-ci`. Паритет с [`.github/workflows/ci.yml`](../.github/workflows/ci.yml): protobuf job запускает `buf-go-pb-check`; `buf-dart-check` запускается в `flutter` / `make flutter-ci`.

### Идентификаторы в телах сообщений (`account_id` / `profile_id` / `user_id`)

Правила из [DATA_MODEL.md](DATA_MODEL.md) переносим в контракты так:

- В **телах gRPC-сообщений и событий JetStream** для ссылки на аккаунт используем поле **`account_id`** (UUID строкой), а не `user_id`, чтобы не путать с JWT claim `user_id` (историческое имя для того же `accounts.id`).
- Исключение: сообщения, которые **буквально отражают JWT** после интроспекции (например `TokenClaims.user_id` в Auth) — имя поля совпадает с именем claim.
- Адресация уведомлений пользователю в продуктовом API — по **`profile_id`**, если речь не про сырой account-level relay (S2S / внутренние вызовы).

### Общие типы, ссылки и идемпотентность

Entity IDs остаются строковыми UUID; `account_id` и `profile_id` обозначают
разные домены ([DATA_MODEL.md](DATA_MODEL.md)). Ссылкой владеет API её домена:
`voice.chat.v1.ChatRef` (`id`, optional `ChatType`) остаётся в Chat и используется
Messaging для `SendMessage`, `GetMessages`, `MarkRead` и `ForwardMessage`.
Ссылка и заявленный тип описывают объект; они не дают membership или permission.

В `common.proto` уже есть `ClientIdempotencyKey`, но наличие helper не означает
активированную реализацию идемпотентности. Формат, namespace, retention и replay
определяет конкретный RPC. В частности, `SendMessageRequest.client_message_id`
остаётся optional string (tag 3), с UUID validation и namespace
`(chat_id, verified sender_profile_id, client_message_id)` в Messaging.
Одинаковое нормализованное тело возвращает тот же результат, конфликтующее —
`ALREADY_EXISTS`; точные правила immediate receipts и отдельного scheduled
lifecycle задаёт [Messaging](microservices/messaging-service.md#идемпотентность-отправки).
Общий helper не заменяет существующие поля или их JSON/API shape.

Authority берётся из проверенного signed principal/context по
[Phase-0](ARCHITECTURE_REQUIREMENTS.md#phase-0-межсервисные-и-edge-principals-target-внедряется-по-сервисам),
а не из raw identity metadata или произвольного actor/account/profile поля тела.
Такие поля в существующем owner API — данные/проверяемые targets либо явно
ограниченные delegated inputs. Комментарий о metadata в `common.proto` сам по
себе не доказывает завершение runtime migration всех сервисов.

Новый общий actor/reference тип вводится только вместе с named enabled consumer,
owner semantics и планом совместимости. Для текущих Messaging RPC отдельный
generic actor/ref тип не требуется; `ChatRef` не переносится в common.

### Время: `google.protobuf.Timestamp` vs Unix в S2S

- **Публичные сервисы за Gateway** и общие типы — **`google.protobuf.Timestamp`** (UTC), как в [protos/voice/common/v1/common.proto](../protos/voice/common/v1/common.proto).
- **Data plane Federation** (`protos/voice/s2s/v1/s2s.proto`): допускаются **`int64` Unix epoch** секунд в потоковых кадрах и банах там, где зафиксировано в proto (компактность и исторический контракт стрима). Новые поля вне S2S предпочтительно через `Timestamp`.

### Поля `*_json`

- Допустимы для полуструктурированных данных (вложения, JWKS, device info и т.д.); источник семантики — соответствующий `docs/microservices/*.md`.
- При стабилизации домена выносить **повторяющееся ядро** в отдельные `message` / `enum` в proto, чтобы усилить проверку на границе и codegen для клиентов.

### Приоритет перевода «строковых статусов» в `enum`

Порядок для поэтапных PR (см. [DATA_MODEL.md](DATA_MODEL.md) — единый стиль статусов):

1. **Messaging** — тип сообщения (`Message.type` и родственные поля).
2. **Federation management** — статус ноды (`FederationNode.status`).
3. **Notification** — платформа / канал доставки (`RegisterDeviceRequest` и т.п.).
4. **Auth** — тип OTP (`VerifyOTPRequest.otp_type`).
5. **S2S NotifyUser** — `type` уведомления.
6. Остальные домены — по мере зрелости API.

Каждый переход с `string` на `enum` по существующему номеру поля — **breaking** для JSON и иногда для бинарного wire; нужен либо новый номер поля + deprecate, либо согласованный major/breaking PR.

Часть приоритетов уже отражена **отдельными top-level enum** в `.proto` (рядом со строковыми полями), без смены wire: `MessageKind` ([messaging.proto](../protos/voice/messaging/v1/messaging.proto)), `OtpType` ([auth.proto](../protos/voice/auth/v1/auth.proto)), `DevicePlatform` ([notification.proto](../protos/voice/notification/v1/notification.proto)), `FederationNodeRegistrationStatus` ([federation_management.proto](../protos/voice/s2s/v1/federation_management.proto)), `FederationPushEventType` ([s2s.proto](../protos/voice/s2s/v1/s2s.proto)) — для констант в клиентах и будущей миграции полей.

### buf: правила имён RPC (`STANDARD`)

В [protos/buf.yaml](../protos/buf.yaml) включён профиль **`STANDARD` без `ignore_only`**: для каждого RPC имя запроса — **`<RpcName>Request`**, ответ — **`<RpcName>Response`** (или `google.protobuf.Empty` заменён на пустой `message <RpcName>Response {}`, чтобы не переиспользовать один тип на несколько RPC).

- Полезная нагрузка лежит внутри ответа (например `RegisterResponse { AuthSession session = 1; }`, `GetChatResponse { Chat chat = 1; }`). Так buf проверяет уникальность пар типов и единый стиль для codegen.
- **Новые RPC**: соблюдать те же правила; перед merge — `make buf-ci` / `buf lint` в каталоге `protos/`.

### Realtime и protos

Публичного **gRPC**-файла для Realtime нет: клиентский поток — **WebSocket через API Gateway** к Realtime Service ([MICROSERVICES.md](MICROSERVICES.md)). Внутренние контракты Realtime при появлении описываются отдельно (REST/WS payload, Redis), а не ожидаются в `protos/voice/realtime/`.

---

## Отдельный репозиторий (только по исключениям)

**Protos** — никогда

**Инфраструктура** (Terraform, Helm, сырые манифесты k8s) — при необходимости отдельный репозиторий, например `VoiceInfrastructure`.

**Клиент** — отдельный репозиторий только при обособленном релизном цикле и команде; до такого решения клиент остаётся в монорепо.

---

## Чеклист перед созданием нового репозитория

1. Условия из раздела «Отдельный репозиторий» выполнены; иначе расширять монорепо (`Voice`) каталогом сервиса или protos.
2. Имя — PascalCase, без лишних суффиксов.
3. Ветка по умолчанию — `master`; при branch protection — без прямых push.
4. В README — одна строка назначения и ссылка на `docs/`.
5. Секреты не в git; CI — из шаблона монорепо или явно описанный минимальный workflow.

---

## Связанные документы

- [CONTRIBUTING.md](CONTRIBUTING.md) — PR, `master`
- [DEPLOYMENT.md](DEPLOYMENT.md) — выкат, артефакты
- [MICROSERVICES.md](MICROSERVICES.md) — сервисы и протоколы
- [TESTING.md](TESTING.md) — CI

