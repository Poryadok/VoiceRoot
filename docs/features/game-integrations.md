# Интеграции игр: API, SDK и сообщества

## Статус и границы документа

**Proposed target.** Первый ограниченный Auth bootstrap описан в GAME-AUTH-01;
это не готовая игровая интеграция, обе конвертации и остальные runtime gates
ещё не завершены.
Владелец запросил описание игрового SDK, федерации для игр и взаимодействия
игры с людьми через ботов. Это разрешение на проектирование, а не включение
федерации в релиз. Очередь и release gates остаются в [PLAN](../PLAN.md).

Требования владельца: общение под подтверждённой игровой/Voice-личностью;
автоматическая группа команды; Unity и Unreal; постоянные сообщества MMO;
доступ из мессенджера вне игры; события и команды игровому backend;
возможность разместить коммуникации игры на собственной ноде.

Уточнение scope владельца: один спринт реализует **Voice** — backend, мессенджер,
внешние контракты, game federation и Voice Node distribution. Assets/SDK для
Unity/Unreal/других движков и developer tools — отдельная задача. Их дизайн
сохранён для совместимости; в спринте Voice внешнего потребителя представляет
внутренний protocol/media test client, а игровой backend — controlled adapter.

Все новые маршруты, SDK-методы, scopes, таблицы и числовые defaults в связанных
документах — **предлагаемый контракт**, не описание доступного API. Существующее
поведение отмечено отдельно. Решения, требующие выбора владельца перед
реализацией, собраны в [матрице решений](../testing/game-integrations-acceptance.md#решения-перед-активацией).

## Карта спецификации

| Документ | Содержание |
|---|---|
| Этот файл | Пользовательские сценарии, модель продукта и границы |
| [Game Integration API](../architecture/game-integration-api.md) | Identity, scopes, ресурсы, REST, ошибки, консистентность |
| [Game SDK](../sdk/game-sdk.md) | Unity/Unreal, lifecycle, audio, packaging, onboarding |
| [Игровые боты](game-bot-interactions.md) | Source audit, команды, карточки, доставка и безопасность |
| [Федерация для игр](../architecture/game-federation.md) | Нода игры, master, authority, routing, recovery |
| [Единый спринт и acceptance](../testing/game-integrations-acceptance.md) | Подзадачи, зависимости, общий результат, тесты и открытые решения |
| [Повторный аудит решений](../testing/game-integrations-design-audit.md) | Принятые правила, остаточные вопросы и границы доказательств |

## Обещание продукта

Игрок входит в команду и получает её текстовый чат и голос внутри игры. При
подключении Voice тот же разрешённый контекст доступен в мессенджере. Игра может
сообщать о событиях и принимать разрешённые команды, пока графический клиент
закрыт. Разработчик использует единый внешний контракт независимо от размещения
сообщества на основной инфраструктуре или зарегистрированной ноде.

Голос не требует установленного приложения Voice. Использование чата не требует
включения микрофона. SDK не является игровым netcode, сервером симуляции,
античитом, системой экономики или обязательной заменой игрового matchmaking.
Встроенный Voice matchmaking подключается отдельным адаптером; создание лобби
само по себе не запускает поиск команды.

## Три независимых уровня

| Уровень | Зачем | Требует следующего уровня |
|---|---|---|
| Game API + SDK | Identity, лобби, текст, голос, presence, приглашения | Нет |
| Bot interactions | События игры и приказы из Voice | Нет; может работать без SDK движка |
| Game federation | Собственные content/media/storage студии в общей сети Voice | Нет; API/SDK работают и на master |

`game_id` из каталога матчмейкинга не является credential или developer
application. Одна игра может иметь несколько приложений/окружений, а бот — быть
одной из возможностей приложения. Обладатель каталожной записи не получает
права управлять аккаунтами игроков или чужими Space.

## Сценарий HerdTrip: кооператив на 2–4 игроков

### Рекомендуемый первый вход для любой игры

Решение владельца: при первом входе стоит сразу сообщить «Общение в игре
работает через Voice» и показать возможность подключить свой аккаунт. Это
рекомендация UX для разработчика игры, а не обязательный отдельный экран SDK.
Например: «Продолжить с игровым профилем» и «Подключить Voice»; второй путь
позволяет войти или зарегистрироваться. Установка мессенджера не требуется.

Начальный выбор одинаков для всех: игра не знает, существует ли у человека
аккаунт Voice. SDK различает подтверждённую связь и её отсутствие, а не
«зарегистрирован / не зарегистрирован». Нельзя определять аккаунт по email,
нику или установленному приложению. При следующих входах восстанавливается
ранее подтверждённый binding, пока он действует. Consent и выбор профиля
не заменяются сообщением о том, что общение обслуживает Voice.

Для входа без связи применяется отдельный `sdk-account`, описанный
ниже. Эта capability и оба пути конвертации обязательны для общего результата
спринта; linked-only сборка не считается завершённой поставкой и не имитирует
этот путь через `guest`.

### Партия и общение

Референс владельца: друзья проводят стадо через горы; быстрые разговоры нужны
при разделении команды, подготовке переправы и спасении животных. Конкретные
игровые механики не входят в Voice.

1. Игра авторизует игрока своим способом. SDK сообщает, доступно ли общение.
2. Игрок подключает выбранный профиль Voice либо использует `sdk-account`,
   если эта будущая capability включена для приложения.
3. Доверенный backend сообщает состав лобби. Voice создаёт коммуникационную
   сессию идемпотентно, возвращает её группу и временную голосовую комнату.
4. Игрок видит участников и кнопку подключения звука. Первое использование
   микрофона требует понятного согласия и системного разрешения. Сохранённая
   настройка авто-входа в голос не снимает mute/deafen и не меняет устройство.
5. В ходе игры состав обновляется, выход/исключение отзывает доступ. Сетевой
   reconnect восстанавливает разрешённое состояние без нового чата.
6. После забега временная сессия закрывается. Команда может явно сохранить
   общение: создаётся постоянная группа с согласившимися участниками, либо
   продолжается уже существующая постоянная party-группа.

**Party и match — разные объекты.** Пять забегов одной компании не должны
создавать пять постоянных групп. `party` имеет стабильный внешний ключ;
`match` имеет собственный жизненный цикл и может ссылаться на party. Сохранение
общения не добавляет людей в друзья и не открывает личные сообщения автоматически.

Для временной match-сессии доступ к её истории заканчивается в
`closed_at + 30 days` (граница исключительная). Закрытие match не удаляет и не
закрывает общий party-чат, на который ссылаются другие match: этот ресурс и
его обычное хранение остаются у Chat и party. Явный keep-group включает в
создаваемую постоянную группу только согласившихся участников текущей
сессии. При продолжении существующей party-группы участник подключается к ней
только после собственного согласия. История match в постоянную группу не
копируется.

Для `since_join` сервер сохраняет интервалы членства. В интервале
`[joined_at, revoked_at)` сообщение доступно, если его неизменяемый
`created_at` попадает в этот интервал; `joined_at` включён, `revoked_at`
исключён. Повторное вступление начинает новый интервал и не возвращает доступ
к промежутку отсутствия или прежнему интервалу. Это же правило действует для
истории, поиска, цитат и контекста тредов, метаданных вложений и проверки
скачивания файла в момент получения.

Для команды без выделенного сервера нужен Voice-managed session broker с
проверяемым внешним login ticket. Клиентский host не получает серверный secret
или право произвольно выдавать доступ другим игровым аккаунтам. Roster
обновляет только аутентифицированный игровой backend или broker. Принятый
полный roster получает lease 60 секунд от commit в GIS DB; повтор той же
revision и тех же данных lease не продлевает. Более низкая revision — stale
no-op, та же revision с иным телом — conflict; неполная страница или ошибка
получения не означает пустой roster.

Передача host проходит только полным roster следующей revision через CAS;
назначенный successor уже присутствует в этом roster. Удаление прежнего host
и предоставление session-control полномочий successor применяются атомарно.
Эти полномочия не меняют Voice roles и не дают Voice Owner. Клиентские
заявления о передаче отклоняются. Если roster удаляет host без successor,
его control полномочия отзываются сразу; сессия закрывается при истечении
текущего lease. В момент `lease_expires_at` новые admission, reconnect,
governed reads и writes запрещаются; активный Voice/media доступ fencing
завершается не позднее чем через 5 секунд. Повторная доставка старого roster
не продлевает lease.

## Сценарий MMO: корпорации, гильдии и флот

Игра имеет независимые корпорации. Каждая привязана к Space; внутри — общие,
офицерские и специализированные группы/каналы, постоянные голосовые комнаты.
Флот — временная коммуникационная сессия, а не новая корпорация.

| Контекст | Источник допуска | Доступ вне игрового клиента |
|---|---|---|
| Корпорация | Подтверждённый состав и роли игры | Да, пока членство действительно |
| Офицерский чат | Managed role + overrides Voice | Да, без обхода роли |
| Флот / raid | Текущий roster сессии | По политике конкретного типа сессии |
| Локальный чат игровой зоны | Подтверждённое присутствие персонажа | Нет по умолчанию; сервер игры может явно разрешить |
| Друзья после матча | Согласованная постоянная группа Voice | Да, независимо от следующего матча |

Backend игры передаёт версионированные снимки состава. Ранги игры сопоставляются
только с разрешёнными managed roles; они не назначают системного Owner и не
отменяют баны Voice. Смена главы корпорации не равна передаче владельца Space:
применяется защищённый ownership flow Voice или отдельное будущее решение G03.

Если несколько персонажей игрока дают доступ к одному Space, хранится причина
каждого grant. Удаление персонажа пересчитывает оставшиеся причины; нельзя
оставить его офицерское право только потому, что другой персонаж ещё рядовой.
Политика объединения ролей либо ограничения одним персонажем задаётся до
активации интеграции (G02). Публично альты не связываются автоматически.

## Сценарий Dejavu: игра продолжается через сообщения

Референс владельца: несколько воплощений выполняют задачи офлайн. Найденное
существо, завершённая работа или открытый проход создают содержательное событие.
Пользователь выбирает действие из мессенджера; вечером продолжает ту же историю
в игровом клиенте.

Пример целевой карточки:

> **Охотник · Dejavu · игровое приложение**
>
> Найден железный олень. Приручение расходует 20 единиц корма.
>
> [Приручить] [Отметить место] [Открыть игру]

Команду исполняет backend игры, проверяя владение персонажем, состояние события,
ресурсы и срок действия. Voice показывает подтверждённый результат. При таймауте
результат неизвестен, пока не получен status/receipt; нельзя написать «олень
приручён» только потому, что webhook принят.

Подробная модель — [игровые боты](game-bot-interactions.md). Slash-команды и
карточки используют один command handler игры. Свободный текст/LLM — опциональная
интерпретация на стороне игры, не доверенный источник права или результата.
Ни один сценарий не должен требовать реакции на push для предотвращения потери:
безопасное игровое поведение при отсутствии ответа определяет игра.

## Identity и приватность

- Аккаунт Voice принадлежит Auth, профиль — User. Игра видит выбранный профиль,
  а не все личности аккаунта. Контакты не импортируются без отдельного scope.
- Игровой аккаунт подтверждается backend/провайдером игры; произвольный ник,
  Steam ID или email в клиентском JSON не являются доказательством владения.
- Персонаж — сущность игры, привязанная к игровому аккаунту. Это не дополнительный
  Voice-профиль, не бот на каждого персонажа и не обход лимита 2/5 профилей.
- `sdk-account` — будущая capability с отдельной политикой.
  Текущий `guest` не равнозначен ей; не создаём без согласия полноценные аккаунты.
- Привязка существующего Voice требует доказательства обеих сторон. Нельзя
  объединять аккаунты по одинаковому email/нику или молча переносить историю.
- Выбор профиля в SDK явный. Переключение профиля мессенджера не меняет identity
  уже открытой игровой сессии. Смена binding отзывает старые grant и соединения.
- Отзыв интеграции прекращает её доступ, presence и новые команды. Он не удаляет
  автоматически обычные дружеские группы, созданные отдельно от managed roster.
- Блокировка аккаунта, санкции, privacy и жизненный цикл Space действуют на SDK,
  бот и federated node так же, как на обычный клиент.

### sdk-account и переход в постоянный аккаунт

Технические решения G01/Q03/Q07/Q08/Q10/Q11 первого Auth-среза закреплены в
[GAME-AUTH-01](../architecture/game-integration-api.md#замороженный-auth-identity-slice-game-auth-01).
Первый provider — Google OIDC через Voice-owned audience, отдельный ключ каждого
устройства и одноразовый server nonce. Developer credential не подтверждает
игрока. Bootstrap credential не даёт доступа к чатам/голосу; оба conversion
пути, User profile и public Gateway activation остаются следующими slices.
Google identity сама по себе не подтверждает принадлежность персонажа игре.

**Принято владельцем:** `sdk-account` — отдельный тип аккаунта Voice для SDK-интеграции,
не текущий гостевой аккаунт
Voice и не переименование `guest`. Основание доверия — проверенное владение
игровым аккаунтом через независимо проверяемый provider flow или вход Voice.
Заверения backend разработчика достаточны для игрового контекста, но не для
удостоверения авторства игрока. Device key связывается с аккаунтом независимо
от service credential студии. Это позволяет участвовать
в разрешённых игровых чатах, голосе и managed communities, но не доказывает
личность человека, email, возраст или право на произвольные ресурсы Voice.
Права выдаются в пределах приложения, членства и scopes; полномочия текущего
guest и механика `ConvertGuest` не наследуются автоматически.

«Временная» означает отсутствие самостоятельного постоянного входа в Voice,
а не новый аккаунт на каждый запуск или матч. Повторное доказательство того же
game subject в том же app/env возвращает тот же `sdk-account`. Срок хранения
и восстановление после потери игрового входа остаются решениями G01/G04.

Обязательны оба пути конвертации:

- **В новый постоянный аккаунт:** пользователь явно регистрируется и подтверждает
  требуемые Voice credentials. Его игровая continuity сохраняется по принятой
  политике истории; повторно вступать во все текущие игровые чаты не требуется.
- **В существующий постоянный аккаунт:** пользователь доказывает владение
  `sdk-account` и постоянным аккаунтом Voice, выбирает профиль, видит переносимые связи и
  конфликты, затем подтверждает переход. Это полноценная операция конвертации,
  а не оставление двух независимых активных identities с новой ссылкой между ними.

Второй путь переносит допустимый игровой контекст в выбранный профиль, но не
объединяет безусловно все профили, друзей, истории и роли. Историческое авторство
сохраняется; доступ к истории, приватность и санкции перепроверяются. Конфликт
уже занятого binding требует явного разрешения без перезаписи чужой связи.
Технический протокол и recovery описаны в
[Game Integration API](../architecture/game-integration-api.md#конвертация-sdk-account).
Необходимость обоих путей согласована; точные правила конфликтов, хранения и
восстановления ещё должны быть закрыты в G01 до включения capability.

## Пользовательские экраны

### Принятые владельцем продуктовые правила

Следующие направления согласованы; точные wire formats, лимиты и unresolved
сценарии перечислены в design audit и G01–G13, а не считаются решёнными молча.

| Вопрос | Принятое правило |
|---|---|
| Создание sdk-account | Только независимо проверяемый вход; если такого способа у игры нет, подключение Voice. Service credential студии не удостоверяет игрока |
| Граница аккаунта | Отдельный sdk-account на app/env/provider subject; разные игры не связывают людей автоматически. Объединение доступа — по выбору владельца через permanent account |
| Конвертация | Proof обеих сторон, выбранный профиль и preview конфликтов; без overwrite занятой связи и автоматического union ролей/истории |
| Устройства и recovery | Свой ключ у каждого устройства, независимая авторизация нового, отдельный отзыв. Provider login не восстанавливает permanent Voice credentials |
| Публичная личность | Выбранное имя в игровом контексте и явная принадлежность игре; скрытые профили/персонажи не раскрываются. Политика concurrent alt roles остаётся G02 |
| Власть игры | Игра подтверждает roster и разрешённые ranks; Space Owner меняется отдельным защищённым flow. Voice ban нельзя отменить roster sync |
| Срок жизни общения | Match, managed community и добровольная постоянная группа различны. Выход из игры/корпорации не удаляет независимую группу друзей |
| Доступ разработчика | Раздельные scopes на roster, bot messages, чтение чата и команды; нет общего inbox/contacts. Оператор content node имеет доступ к локально обрабатываемому plaintext |
| Жалобы и санкции | Доступный report/block, разделение ответственности Voice/студии, лимиты на app и user. Player/app/node sanctions имеют разные последствия |
| Отказы | Ограниченная по времени authority; нет новых privileged действий без проверки. В UI честно показана недоступность, не успех или вечный stale access |
| Закрытие игры | Дружеские группы независимы; export/backup/delete и ответственность оператора определены до запуска. Нет обещания восстановить на master отсутствующую там историю |
| Команды | Видны персонаж, цена и последствия; опасные действия требуют дополнительного подтверждения по принятой policy. Повтор не дублирует effect |
| Уведомления | Отдельное согласие по категориям, quiet hours и подавление дубликатов между клиентами; уведомление не исполняет команду |
| Совместный запуск | Одна account-wide voice session, явная передача между клиентами и сохранение mute; listen-only тоже учитывается |
| Совместимость | Version matrix/minimum versions/support policy, понятный отказ без unsafe fallback. Engine-specific matrix относится к задаче инструментов |

### Поверхности Voice и будущих инструментов

| Поверхность | Обязательные состояния и действия |
|---|---|
| Подключение игры | Проверенный разработчик, название/окружение, выбранный профиль, конкретные permissions, отмена |
| Встроенная party-панель | Roster, кто говорит, mute/deafen/PTT, громкость участников, reconnect, выход |
| Текстовый чат | История разрешённого периода, pending/failed send, retry без дубля, block/report |
| Voice → подключённые игры | Профиль, связанные персонажи только владельцу, scopes, уведомления, отключить |
| Игровой Space | Игра/оператор ноды, managed роли, недоступность, правила вступления и обжалования |
| Игровая карточка | Автор-приложение, персонаж, цена/последствия, expiry, pending/result/error |
| Приглашение в игру | Игра, команда, истечение, «Открыть игру»; при отсутствии установки — понятный fallback |

SDK поставляет заменяемые UI-компоненты. Студия может оформить их в стиле игры,
но не скрывать permissions, mute, источник карточки, ошибки и жалобы.
Неизвестные компоненты показываются безопасным текстовым fallback без кнопок.

## Эксплуатация и экономика

Managed Voice и нода студии используют одинаковый capability contract. Студия
выбирает размещение; большое количество чатов само по себе не требует федерации.
Для self-hosted ноды нужны резервные копии, обновления, media capacity, мониторинг
и moderation contact. Регистрация ноды не выдаёт безлимитные ресурсы master.

Production setup is separate from sandbox and starts in a pending environment;
its policy is configured independently. Pending production is not usable for
identity admission or service credentials. Activation waits for reviewed operator
approval, independent provider/user-proof acceptance, and out-of-band secret
provisioning. The current staged workflow and open gates are specified in the
[Game Integration API contract](../architecture/game-integration-api.md#t11-staged-production-admission-development-only).

Self-hosted поставка — единый **Voice Node** bundle: один экземпляр необходимых
сервисов и инфраструктуры, единые установка/config/version/update/backup.
Внутренние микросервисные контракты сохраняются; оператор не собирает весь Voice
вручную. Базовая нода рассчитана на один хост без HA и без обязательных реплик.
Подробности — [дистрибутив федерации](../architecture/game-federation.md#единый-дистрибутив-voice-node).

Квоты привязаны к приложению/окружению/получателю/ресурсу, а не только к токену.
В portal видны usage, причины 429, ошибки доставки и лимиты. Цены, бесплатные
пакеты, retention и SLA не выдумываются в этой спецификации: G04/G07.
Нельзя обещать экономию без измерения media traffic, support и хранения.

## Основа и новые зависимости

Ниже — source inventory на feature target `b894441983b81fdce2964159bed9a123689b07e3`
(2026-09-28), а не release sign-off. `Partial` означает, что в коде есть
ограниченный slice с указанными тестами; это не доказывает общий Voice-owned
acceptance. PR [#550](https://github.com/Poryadok/VoiceRoot/pull/550) открыт на
этом base и ещё не входит в перечисленные ниже source paths: T31 orchestration
остаётся pending.

| Capability | Статус на target | Source и test evidence | Что не доказано / отсутствует |
|---|---|---|---|
| Game Integration Service registry, app/environment authority, credentials, binding и resource mapping | Partial | [HTTP API](../../src/backend/gameintegration/main.go), [application](../../src/backend/gameintegration/internal/httpapi/applications.go), [credentials](../../src/backend/gameintegration/internal/httpapi/credentials.go), [binding authority](../../src/backend/gameintegration/internal/httpapi/binding_authority.go), [resource mapping](../../src/backend/gameintegration/internal/httpapi/resource_mapping_authorization.go); например [bootstrap](../../src/backend/gameintegration/internal/httpapi/bootstrap_integration_test.go), [authority](../../src/backend/gameintegration/internal/httpapi/binding_authority_test.go), [resource mapping test](../../src/backend/gameintegration/internal/httpapi/resource_mapping_authorization_test.go) | Это bounded service foundation; end-to-end game party/match path и release admission ещё не закрыты. |
| Auth guest conversion и sdk-account identity/conversion | Partial | [guest OTP acceptance](../../src/backend/auth/src/main/java/voice/backend/auth/service/TransactionalGuestConversionOtpAcceptance.java), [SDK identity](../../src/backend/auth/src/main/java/voice/backend/auth/sdkidentity/SdkIdentityService.java), [SDK conversion](../../src/backend/auth/src/main/java/voice/backend/auth/sdkidentity/SdkConversionService.java); [guest OTP acceptance test](../../src/backend/auth/src/test/java/voice/backend/auth/GuestConversionOtpAcceptanceJdbcIntegrationTest.java), [SDK identity JDBC test](../../src/backend/auth/src/test/java/voice/backend/auth/sdkidentity/SdkIdentityJdbcIntegrationTest.java), [SDK conversion JDBC test](../../src/backend/auth/src/test/java/voice/backend/auth/sdkidentity/SdkConversionJdbcIntegrationTest.java) | Эти Auth-owned slices не доказывают обе game-account conversion вертикали со всеми сервисными receipts и client recovery. |
| Bot interaction and message/event delivery | Partial | [slash interaction](../../src/backend/bot/internal/grpcsvc/interaction.go), [outbox delivery](../../src/backend/bot/internal/grpcsvc/interaction_outbox.go), [message recipient store](../../src/backend/bot/internal/store/message_delivery.go); [durable/restart tests](../../src/backend/bot/internal/grpcsvc/interaction_durable_test.go), [authority and fencing](../../src/backend/bot/internal/grpcsvc/interaction_authority_test.go), [read-history scope](../../src/backend/bot/internal/consumer/message_events_test.go) | Durable webhook slash slice exists. Production polling ACK, rich action cards, signed Game Event ingress and proactive opt-in DM remain incomplete; see [current Bot source audit](game-bot-interactions.md#проверка-текущей-реализации). |
| Chat managed resources and Space lifecycle | Partial | [GIS-only Chat RPC](../../src/backend/chat/internal/grpcsvc/gameintegration_chat.go), [managed chat store](../../src/backend/chat/internal/store/managed_chats.go), [managed chat tests](../../src/backend/chat/internal/store/managed_chats_integration_test.go), [Space voice-room access](../../src/backend/space/internal/grpcsvc/voice_room_access.go), [Space access contract test](../../src/backend/space/internal/grpcsvc/voice_room_access_contract_test.go) | Idempotent managed-chat create/roster sync exists. MMO corporation-to-Space binding and end-to-end role/media enforcement are not proven by these tests. |
| GIS party/match orchestration (T31) | Absent on this target | The [T31 requirement](../testing/game-integrations-exec-plan.md) has a pending [PR #550](https://github.com/Poryadok/VoiceRoot/pull/550); it is not part of target `b894441983b81fdce2964159bed9a123689b07e3`. | Chat + Voice resource staging, receipts, compensation, reconciliation and activate-after-ready are not integrated at this snapshot. |
| Voice game-session admission and media | Partial | [game-session provisioning](../../src/backend/voice/internal/grpcsvc/game_session_provisioning.go), [game principal](../../src/backend/voice/internal/gameprincipal/interceptor.go), [LiveKit room lifecycle](../../src/backend/voice/internal/livekit/room_lifecycle.go); [provisioning contract](../../src/backend/voice/internal/grpcsvc/game_session_contract_test.go), [game provision store](../../src/backend/voice/internal/gameprovision/store_integration_test.go), [principal verification](../../src/backend/voice/internal/gameprincipal/interceptor_test.go) | Contract/store coverage is not proof of an actual game-client media session, account-wide admission/revocation, or device handoff. |
| Federation authority and node runtime | Partial | [Federation API/runtime](../../src/backend/federation/api.go), [authority](../../src/backend/federation/authority.go), [store](../../src/backend/federation/store.go); [authority tests](../../src/backend/federation/authority_test.go), [clean-start acceptance](../../src/backend/federation/q11_acceptance_test.go), [runtime/profile guard](../../src/backend/federation/runtime_test.go), [contracts](../../protos/voice/s2s/v1/federation_management.proto) | Authority foundation exists. Full node transport/Voice Node bundle distribution, restoration and measured fencing are not proven; federation deployment remains outside `G0–G4`. |
| Gateway routing for game-owned flows | Partial | Existing [SDK authorization routes](../../src/backend/gateway/sdk_authorization_routes.go), [Bot routes](../../src/backend/gateway/transcode_bots.go), and [matchmaking routes](../../src/backend/gateway/transcode_matchmaking.go); tests include [SDK route tests](../../src/backend/gateway/sdk_authorization_routes_test.go) and [matchmaking route tests](../../src/backend/gateway/transcode_matchmaking_test.go) | Existing SDK-auth/Bot/matchmaking routes do not establish the complete game party, resource mapping, event or command surface through Gateway. |
| Flutter game surfaces | Partial | Flutter has [SDK authorization UI/client](../../src/frontend/lib/ui/sdk/sdk_authorization_screens.dart), [game catalog/matchmaking client](../../src/frontend/lib/backend/matchmaking_client.dart), [SDK widget tests](../../src/frontend/test/sdk_authorization_widget_test.dart), and [catalog tests](../../src/frontend/test/game_catalog_screen_test.dart). | Game-session party integration is incomplete in Flutter. |
| Unity/Unreal SDK distributions | Absent / out of scope | Engine packages and distributable SDK assets are excluded from the Voice sprint in [PLAN](../PLAN.md) and the scope of this feature. | No Unity/Unreal runtime package or engine-client acceptance is claimed. |

Таблица фиксирует наличие source/test files; проверки перечисленных tests не
запускались этим docs-only audit. Live provider/device tests и staging runtime
acceptance исключены из sprint scope и не заявляются. Общий acceptance и
ограничения релиза задают [game acceptance](../testing/game-integrations-acceptance.md)
и [PLAN](../PLAN.md).

## Проверка ценности

Измерять отдельно успешное общение внутри игры и добровольное продолжение в
Voice. Автосозданная identity не считается активным пользователем мессенджера.
Для SDK важны время до первого работающего лобби, voice join success, reconnect,
ошибки устройств и трудозатраты студии; для бота — полезные завершённые решения,
отключения уведомлений и корректность последствий; для ноды — доступность,
отзыв прав, стоимость и восстановление. Пороги пилота фиксируются до сбора данных.

## Внешние референсы

Проверено 2026-09-26; это источники сравнения, не требования Voice и не импорт
Discord API. Публичный Social SDK показывает лобби, привязку identity и
Unity/Unreal; коммуникации требуют approval. Полезный вывод для нашего дизайна:
разделить игровой backend, пользовательский SDK и бот, сделать capabilities и
условия активации видимыми. Не переносить чужие заявления об engagement на Voice.

- [Discord Social SDK overview](https://discord.com/developers/docs/social-sdk/index.html).
- [Communication requirements](https://docs.discord.com/developers/discord-social-sdk/core-concepts/communication-features).
- [SDK Terms](https://support-dev.discord.com/hc/en-us/articles/30225844245271-Discord-Social-SDK-Terms).
- [Discord components](https://docs.discord.com/developers/components/reference).

Собственная реализация строится по требованиям Voice; использование чужого SDK
для конкурентных экспериментов не является частью задачи.
