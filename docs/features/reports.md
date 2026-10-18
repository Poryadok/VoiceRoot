# Reports — репорты и модерация

Система жалоб для глобального сервера (не внутри спейса — у спейсов своя модерация через роли).

## Что можно репортить

| Объект       | Контекст                                |
|--------------|-----------------------------------------|
| Пользователь | Профиль, поведение в ММ, поведение в ЛС |
| Сообщение    | Конкретное сообщение в любом чате       |
| Спейс        | Нарушение правил платформы              |
| Стори        | Конкретная стори (медиа/текст); см. [stories.md](stories.md) |

## Категории жалоб

- **Спам** — нежелательные рекламные сообщения, массовые рассылки
- **Домогательство / Харрасмент** — угрозы, оскорбления, преследование
- **Оскорбительный контент** — призывы к насилию, дискриминация, NSFW вне разрешённых зон
- **Фейк / Выдача себя за другого** — имитация известного лица или другого пользователя
- **Читерство / Токсик в ММ** — специфичная для геймерской аудитории категория
- **Другое** — произвольный комментарий (обязателен)

## API

Один endpoint на все типы объектов (не отдельные `ReportUser` / `ReportMessage` / `ReportSpace`):

| Слой | Контракт |
|------|----------|
| HTTP (клиент) | `POST /api/v1/moderation/reports` → **202 Accepted** |
| gRPC | `ModerationService.CreateReport(CreateReportRequest)` → `CreateReportResponse` |

Тело запроса: `target_type`, `target_id`, `category`, опционально `description` (обязателен для `other`), `evidence_json`.

| `target_type` | `target_id` |
|---------------|-------------|
| `user` | `profile_id` |
| `message` | `message_id` |
| `space` | `space_id` |
| `story` | `story_id` |

Категории: `spam`, `harassment`, `offensive`, `fake`, `cheating` (алиас `mm_toxic` на HTTP), `other`.

Репортер не может пожаловаться на свой профиль (`target_type=user` + свой `profile_id`). Статус репорта репортеру не отдаётся.

## Флоу репорта

### Приём по ID без раскрытия существования (BE-137)

Решение владельца: жалобу может подать любой знающий ID; приём не раскрывает,
существует ли объект. Это относится к глобальным жалобам, включая private,
недоступные репортёру и уже удалённые объекты. Проверка права просмотра не
является условием приёма. Существующий запрет жалобы на собственный активный
профиль сохраняется и не расширяется на другие собственные объекты.

Для аутентифицированного синтаксически корректного запроса сервер сначала
надёжно сохраняет обращение и возвращает `202` с `report.id` — только квитанцией
о приёме. Другие поля `report` не заполняются. Ответ не подтверждает существование,
пригодность, постановку в очередь, статус или применение санкции. Проверки
владельцев объектов, automod и публикация событий не входят в цепочку запроса;
недоступность владельца не меняет успешный приём при исправном локальном store.
Ошибка сохранения не выдаётся за успешный приём. Ошибки сессии, входных данных
и запрет собственного активного профиля остаются явными.

Уточнение P3 (BE-137 c11/c12): до сохранения проверяется только текущая
защищённая Auth authority самого заявителя — account, непрозрачный subject,
ревизия жизненного цикла и session epoch. Это не проверка цели. При недоступной
authority заявителя возвращается одинаковая общая ошибка для всех целей;
удалённый заявитель или устаревшая сессия не могут создать новое обращение.
Текущая LIVE-учётная запись с ограничениями не получает нового запрета жалоб.
Открытие приёма после запуска/восстановления требует актуального импорта
terminal fences; запоздалый LIVE-ответ не восстанавливает удалённые связи.

Отдельный durable worker проверяет минимальный результат владельца объекта.
Только подтверждённый живой объект допускает жалобу к staff-очереди, счётчикам
и обычным правилам automod. Подтверждённые отсутствие/удаление завершают внутреннюю
проверку без санкций и удаляют description/evidence обращения. Недоступность
зависимости сохраняет обращение для повторной проверки. Эти внутренние исходы
не возвращаются репортёру через HTTP, WS, push или новый polling endpoint.
Квитанция не даёт права читать жалобу. Технический контракт и migration provenance
определены в [Moderation Service](../microservices/moderation-service.md).

Форма квитанции, durable очередь проверки и минимизация недействительных
обращений — автономные технические решения BE-137 по делегированному мандату;
они не добавляют Space-local модерацию или привилегию чтения чужого контента.

1. Пользователь нажимает "Пожаловаться" (ПКМ на сообщении / пользователе / спейсе / стори)
2. Выбирает категорию
3. Опционально: добавляет комментарий (до 500 символов)
4. Репорт отправлен → уведомление: "Жалоба принята. Мы рассмотрим её в ближайшее время"
5. Автор репорта не получает обновлений по статусу (во избежание раскрытия решений)

## Обработка

**Автоматика (первая линия):**
- **Порог авто-блокировки**: относительный — 1% от аудитории объекта, но не меньше 10 репортов за 24 часа; при превышении — временная блокировка / shadow ban до ручной проверки; относительный порог защищает от координированных атак (договориться в другом канале и массово зарепортить)
- **Shadow ban**: пользователь видит свои сообщения и действия, другие их не видят; контент не доставляется; пользователь не знает, что забанен
- Очевидный спам (паттерн сообщений) → авто-мут первый раз на час, второй раз навсегда; после автомута первые 10 сообщений проверяются на паттерн

### BE-046: accepted authoritative threshold target

The automatic report threshold is `max(10, ceil(N/100))` distinct verified
submission accounts on one target user profile over `(t-24h,t]`; it is not an
OR/minimum rule or an environment population estimate. Only VALID unresolved
reports with a current authoritative account binding contribute. Duplicate
receipts/categories/profiles from one account count once, and the target's
own account counts zero without changing the opaque submission receipt.

The autonomous metric `POTENTIAL_PUBLIC_PROFILE_ACCOUNTS_V1` counts distinct
other nondeleted Auth accounts with at least one nondeleted User profile.
Guests and frozen profiles count once; target owner, deleted/purged accounts
and accounts without a live profile are excluded. Blocks, presence,
suspensions and per-field settings do not shrink this potential denominator;
this changes no profile read privacy. It avoids lowering a safety threshold
by manipulating personal restrictions and makes no Sybil-resistance claim.

User and Auth must provide complete positive owner observations, with the
target binding rechecked and the action committed within 30 seconds of the
first observation. Unknown/partial/stale authority never uses a fallback.
Authoritative zero audience and all message/Space/Story targets remain
manual-only. A qualifying user case creates one account shadow ban pending
review, never an Auth status suspension or an inferred ban of a content owner.

One account transaction commits the machine-issued sanction, case, log and
stable event outbox. A closed review episode records a cutoff so old evidence
cannot immediately recreate the ban. Receipt acceptance/manual review survives
authority outage; bounded persisted work retries until the evidence ages out.
Deletion/restore/erasure use the existing subject lifecycle and terminal fences.
The [implementation plan](../testing/report-threshold-exec-plan.md) defines
the exact producer, concurrency and privacy checks. This accepted target does
not claim its runtime activation, CI, deployment or acceptance.
### BE-045: committed-message detector

Уточнённый account-scoped контракт BE-045: пять успешно сохранённых сообщений с
одним нормализованным текстовым паттерном за десять минут либо три ссылки в одном
сообщении дают один hit. Evidence не переиспользуется: шестое одинаковое сообщение
не создаёт второй hit, десятое может создать его из пяти новых сообщений. Окно
повторного hit — тридцать дней по времени источника; mute действует час от момента
применения. Второй hit создаёт `perm_ban`, а не shadow ban. Профили и принадлежащие
аккаунту Bots разделяют историю паттерна; Bot owner проверяется у Bot Service.
Пустой текст и E2E не хешируются, но успешно сохранённые сообщения занимают позиции
первых десяти после mute; после десятой позиции детектор продолжает работать.
Обработка асинхронная после commit, без ретроактивного отказа уже сохранённому
сообщению. Числовая политика взята из решения c2; episode/Bot/retention решения c9
приняты агентом по последующей делегации пользователя. Полная privacy, expiry,
rotation и purge семантика — [контракт и ExecPlan](../testing/async-automod-exec-plan.md).

**Ручная модерация:**
- Глобальные модераторы платформы рассматривают жалобы в очереди
- Глобальный модератор — отдельная роль с обязательным подтверждённым телефоном
- Приоритет: харрасмент > фейк > спам > прочее
- Репорты на публичные спейсы — **отдельная очередь** (не смешивается с репортами на пользователей/сообщения)

## Санкции

| Тяжесть | Меры                      |
|---------|---------------------------|
| Лёгкая  | Предупреждение            |
| Средняя | Временный бан (1–30 дней) |
| Тяжёлая | Перманентный бан аккаунта |

- Бан в ММ — отдельная мера, не связанная с баном аккаунта (см. [matchmaking.md](matchmaking.md))

## Апелляция

- Пользователь может подать апелляцию через форму в профиле (раздел "Аккаунт")
- Один раз на санкцию
- Срок рассмотрения: до 7 рабочих дней



