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

- Все `.proto` — в **этом же** репозитории, единый корень контрактов: `protos/` или `api/` (один вариант на весь репо).
- В CI: линт и формат контрактов (**buf** или эквивалент); при изменении публичных API — проверка обратной совместимости. **Breaking change** допускается только с явным описанием в PR и согласованным порядком выката потребителей.
- Сгенерированный код (Go/Java): коммит в git **или** генерация при сборке — один способ выбирает команда и фиксирует в Makefile / доке сборки.

### Идентификаторы в телах сообщений (`account_id` / `profile_id` / `user_id`)

Правила из [DATA_MODEL.md](DATA_MODEL.md) переносим в контракты так:

- В **телах gRPC-сообщений и событий JetStream** для ссылки на аккаунт используем поле **`account_id`** (UUID строкой), а не `user_id`, чтобы не путать с JWT claim `user_id` (историческое имя для того же `accounts.id`).
- Исключение: сообщения, которые **буквально отражают JWT** после интроспекции (например `TokenClaims.user_id` в Auth) — имя поля совпадает с именем claim.
- Адресация уведомлений пользователю в продуктовом API — по **`profile_id`**, если речь не про сырой account-level relay (S2S / внутренние вызовы).

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


