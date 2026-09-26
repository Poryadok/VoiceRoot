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
Их не нужно заново согласовывать. Конкретные недоопределённые правила ниже —
остаточные решения: согласие с направлением не задаёт числовой срок хранения,
конкретного provider или алгоритм восстановления.

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
  optional; способ server-side enforcement остаётся Q05.
- Voice Node поставляется одним bundle с single-instance компонентами;
  это не обещание HA или объединения внутренних доверенных identities.

## Остаточные вопросы

Рекомендации ниже — предложения аудиторов, не уже утверждённые ответы.
GI0 координирует их закрытие; ответ фиксируется в owning feature/API до кода,
а acceptance превращается в проверяемый тест. Независимые подзадачи не обязаны
ждать решения вопроса, которого они не касаются.

### Q01. Повторное вступление и интервалы истории

Основание: [party history](../features/game-integrations.md#партия-и-общение)
задаёт since_join, SE04 проверяет нового участника. Не определено, возвращается
ли история первого членства при rejoin, скрыт ли период отсутствия, доступны ли
цитаты и старые download URLs. Это не просто retention G04.

Предложение: хранить интервалы entitlement, не расширять историю при rejoin
автоматически. Единая проверка для history/search/quotes/threads/files. До GI1/GI5;
acceptance: join → leave → сообщения без пользователя → rejoin → проверки всех
поверхностей, затем повторный revoke.

### Q02. Новые scopes и смена владельца приложения

Основание: [API scopes](../architecture/game-integration-api.md#предлагаемые-scopes)
и node registration описывают первоначальный consent, но не передачу приложения
другой студии, изменение оператора/назначения или расширение scopes.

Предложение: versioned consent; расширение доступа и смена получателя доверия
требуют повторного согласия, старые grants не повышаются сами. Определить судьбу
queued/in-flight commands и что именно считается сменой оператора. До GI7/GI8;
acceptance: старая installation/key не получает новые права, queued actions
проверяют актуальный consent, уже admitted effect следует G13.

### Q03. Новый владелец того же game subject или персонажа

Основание: [API model](../architecture/game-integration-api.md#2-ресурсы)
сохраняет stable external key, а current owner proof не отличает передачу аккаунта
от входа прежнего владельца. Продажа персонажа, перераспределение provider subject
или recovery у провайдера не должны переносить личную историю и consent.

Предложение: отдельная ownership generation/transfer процедура, отзыв прежних
bindings без наследования private контекста. Требуется определить доступный
доверенный сигнал передачи; если provider его не даёт, нельзя обещать её
автоматическое обнаружение. До GI7/GI5; acceptance: один внешний ID, новый owner,
нет доступа к старому permanent account, ключам, истории и командам.

### Q04. Подписи edits, deletes и вложений на чужой ноде

Основание: [авторство](../architecture/game-integration-api.md#авторство-сообщений-пользователя)
покрывает отправленный envelope, но не versioned edit/delete, attachment digest
и показ старой подписанной версии вместо актуальной. Подпись не доказывает
полноту истории и сама не предотвращает сокрытие сообщений нодой.

Нужно выбрать точную гарантию: проверка происхождения каждого payload либо ещё
обнаружение rollback/equivocation. Предложение: подписанные revisions и content
digests, отдельное происхождение moderator deletion, правила invalid/unverifiable
UI; старые device public keys сохраняются для проверки авторства, но не дают
новый admission после revoke. До GI4/GI6; acceptance: modified attachment, edit
rollback, forged delete, revoked key и противоречащие версии на двух клиентах.

### Q05. Обязательное подтверждение команды на сервере

Основание: [bot actions](../features/game-bot-interactions.md) передают immutable
action IDs, но ещё не определяют proof того, что конкретная цена/последствия
подтверждены. Прямой invoke не должен обходить required confirmation.

Предложение: server-issued single-use challenge, привязанный к actor, action,
цене/state revision, expiry и consent. Определить классы опасных действий;
при изменении цены повторить подтверждение. Это доказывает protocol consent,
не физический клик человека в контролируемом разработчиком executable.
До GI2/GI3; acceptance: direct invoke без challenge, replay, другая цена/actor,
expired challenge и два одновременных подтверждения.

### Q06. Отзыв прав под нагрузкой и целостность control plane

Основание: [federation](../architecture/game-federation.md) задаёт короткую lease
и quotas, но не резерв ресурсов для revoke/freeze/lease при потоке сообщений,
roster updates и полном resync. Особенно важно для single-host Voice Node.

Предложение: отдельные очереди/лимиты и ресурсный бюджет критических control
операций; отсутствие свежей authority закрывает доступ независимо от очереди.
До GI6/GI8; acceptance: flood + большой snapshot + revoke + живой SFU, замер
полного budget. Нужно отделить корректное fail-closed от приемлемой доступности
соседних Space, а не считать массовое отключение единственным успешным исходом.

### Q07. Отображение sdk-account и ограничения профилей

Основание: [identity](../features/game-integrations.md#identity-и-приватность)
отделяет персонажа от профиля, но не задаёт постоянный profile ID sdk-account,
учёт лимита профилей при conversion и поведение при заполненном лимите target.
Глобальный profile ID/аватар может раскрыть больше, чем разрешённый игровой alias.

Нужно определить Auth/User модель principal/profile/app-visible alias, выбор
существующего target profile и сохранение исторического автора без implicit
extra profile. Проверить сериализацию roster/cards/search/presence, а не только
ник в UI. До GI7/GI4; acceptance: полный лимит профилей, скрытые профили,
две игры, conversion и нет утечки cross-app связи в публичные payload.

### Q08. Account-wide voice при конвертации и отдельных sdk-accounts

Основание: одна account-wide voice session принята, но два ещё не связанных
sdk-account могут принадлежать одному человеку. Без доказанной связи Voice
не может считать их одной identity и не должен угадывать по устройству/IP.

Предложение: до linking лимит применяется к каждому подтверждённому аккаунту;
при conversion объединённая authority проверяет обе текущие сессии и требует
handoff/conflict, без двух active sessions и незаметного unmute. До GI7/Voice;
acceptance: permanent account уже в звонке, sdk-account слушает другую комнату,
conversion одновременно с reconnect и lease renewal.

### Q09. Детальная семантика block/report и жалобы на оператора

Основание: report/block и разделение санкций приняты; не определено, скрывает ли
block текст/голос в общем managed chat, блокирует ли mentions/бота игры и куда
попадает жалоба, если обвиняемый — оператор ноды. Локальное скрытие не равно
отзыву membership и не должно ломать authoritative roster.

Предложение: явно разделить личный block, moderation ban и app/node suspension;
независимый master путь жалобы на ноду, минимальный evidence с provenance.
Не пересылать без согласия всю историю студии. До GI3/GI5/GI6; acceptance:
общая корпорация после block, blocked bot, жалоба при недоступной/злонамеренной ноде.

### Q10. Политики retention, удаления и восстановления identity

Основание: [conversion](../architecture/game-integration-api.md#конвертация-sdk-account)
оставляет retired audit reference; account deletion и сохранение signatures,
dedupe, abuse history и node backups должны быть совместимы.

Нужны конкретные сроки и схема минимальных tombstones: что остаётся после
удаления, как не воскресить старую связь из backup и как отличить повторный
вход от повторной регистрации. Правила экспорта и история при закрытии игры
также требуют G04/G10. До GI7/GI8; acceptance: delete → restore backup → повторный
provider login, отсутствие оживших private данных/ключей/старых grants.

### Q11. Кто и как впервые получает developer/node полномочия

Основание: [API](../architecture/game-integration-api.md#9-совместимость-и-admission-разработчика)
предполагает app/env registry, [node](../architecture/game-federation.md#4-регистрация-и-доверие)
— approval. Developer portal теперь вне спринта, поэтому требуется рабочий
bootstrap через Voice API/операторский процесс, а не недоступный UI.

Нужно определить approver, account/app ownership, credential issue/recovery,
node enrollment, sandbox→production и ограничения численности sdk-accounts.
Выбрать независимый provider для реальной приёмки: fake доказывает контракт,
но не интеграцию с настоящим provider. До GI7/GI8; acceptance: clean bootstrap
без прямой правки БД, чужой app ID, отозванный developer, независимый login proof.

### Q12. Совместимость, SLO и вместимость поставки

Приняты version matrix, quotas и единый bundle; остаются конкретные API/node
versions, support window, supported host/runtime, active-user/media/storage
targets, retry/retention windows, RPO/RTO и G08/G09/G13 bounds. Цена/SLA не
выводятся автоматически из решения использовать один экземпляр сервисов.

До GI8/GI9: зафиксировать измеримые значения, платформы мессенджера для проверки
и политику несовместимого upgrade. Нельзя считать media verifier реализуемым
только потому, что LiveKit выдаёт JWT: FED10 обязан доказать fencing на media
path. Длительность спринта/команда ещё не заданы; этот документ не оценивает срок.

## Границы и следующие действия

G01–G13 сохраняют решения по доменам, Q01–Q12 уточняют конкретные незамкнутые
сценарии. Это один backlog спринта Voice, а не дополнительные релизные этапы.
До dependent implementation закрыть Q01–Q11 контрактами; Q12 должен получить
проверяемые цели до нагрузочной/эксплуатационной приёмки. Проверки могут готовиться
раньше, но неизвестные значения не заменяются удобными константами без фиксации.

Приоритет обсуждения: Q03/Q07/Q11 (identity/bootstrap), Q01/Q02/Q05 (доступ и
consent), Q04/Q06/Q08 (federation/media), Q09/Q10/Q12 (эксплуатация и lifecycle).
Свой протокол шифрования не вводится: действующий
[encryption canon](../features/encryption.md) остаётся отдельным источником;
подпись авторства не означает E2E-приватность от игры или content node.

Проверка этого изменения: Markdown local links, anchors добавленных ссылок,
diff whitespace и поиск устаревших scope-утверждений. Runtime/source audit,
тесты сервисов, провайдеров, медиатрафика и движков не выполнялись.
