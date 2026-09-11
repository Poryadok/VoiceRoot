# Spaces — спейсы

## Создание

- Создать спейс может любой зарегистрированный пользователь
- В будущем возможно ограничение: только с подтверждённым телефоном

## Вступление

Способы:
- По инвайт-ссылке
- Поиск публичных спейсов по названию / теме / игре
- Каталог спейсов (как серверы в Discord)
- QR-код

Guest admission работает fail-closed: `allow_guests=false` по умолчанию. Владелец
или администратор должен явно разрешить гостей; даже после этого гость вступает
только по действующему invite и не обходит entry requirements, бан, member limit,
роли или отдельный `allow_guests=false` группового чата.

## Видимость

| Тип        | Виден в поиске | Вход             |
|------------|----------------|------------------|
| Публичный  | Да             | Свободный        |
| По инвайту | Да             | Только по ссылке |
| Приватный  | Нет            | Только по ссылке |

## Структура внутри спейса

- **Текстовые группы и каналы** — строки в **Chat** (`chats`, `type = group` \| `channel`); в sidebar — узел **`space_tree_nodes`** с `kind = text_chat`
- **Голосовые комнаты** — сущность в **`voice_rooms`**; в sidebar — узел **`space_tree_nodes`** с `kind = voice_room` (тот же механизм **категории и сортировки**, что и у текста)
- Категории (папки для группировки текстовых чатов и голосовых комнат, как в Discord)
- Системный текстовый чат (любой `group` \| `channel` может быть отмечен как `is_system` в узле **`space_tree_nodes`** — для объявлений от имени спейса)

## Инвайт-ссылки

- Ограничение по сроку действия и/или количеству использований (как в Discord)
- Кто может генерировать инвайты — настраивается по ролям

## Верификация при вступлении

Требования составляются в versioned `entry_policy`; все включённые требования
применяются по **AND** и должны быть выполнены:

- Подтверждённый телефон
- Капча
- Ответы на вопросы (скрининг, как в Telegram)
- Одобрение модератором вручную

Порядок проверки: ban/member-limit/guest admission → phone → captcha → questions →
manual approval. Invite не обходит требования. Если включено ручное одобрение,
успешное выполнение остальных шагов создаёт pending join request; membership
появляется только после approve. Гость не может пройти policy с обязательным phone.
Ошибка или недоступность verifier закрывает вход, но не расходует одноразовый invite.

## Лимиты участников

- Бесплатно: **50 участников**
- Space Pro: **5000 участников** — см. [subscription.md](subscription.md)

## Матчмейкинг внутри спейса

- Спейс может задавать собственные параметры поиска для ММ
- Спейс может включить верификацию данных (например, через Steam)
- Реализуется после глобального ММ — см. [matchmaking.md](matchmaking.md)

## Каталог и управление

- **Ранжирование в каталоге**: сначала верифицированные спейсы, затем по количеству участников
- **Передача владения**: владелец передаёт спейс только уже состоящему в нём участнику. До передачи обязательно подтверждается пароль; если на account включена 2FA, обязательны также TOTP **или** один backup code. Конкретный Auth→Space контракт, срок и защита от повторного использования заданы ниже.

### Контракт подтверждения передачи владения

Это утверждённый target-контракт публичной передачи. Auth уже содержит
`IssueOwnershipTransferProof`, Space-only `ConsumeOwnershipTransferProof` и lookup
durable receipt; Role содержит защищённый protocol-2 ledger, а Space — durable
journal от reservation/proof/PREPARED через необратимое решение до terminal
evidence, ordinary freeze и read-only scan готовых outbox rows. Network recovery
worker, publish/claim/ack, capability activation, публичный Space request и
Gateway/Flutter vertical ещё не реализованы. Клиент получает proof только у Auth
через authenticated user surface, а не создаёт и не проверяет его в Gateway или
Space.

1. После проверки password и, для account с включённой 2FA, TOTP либо backup code,
   Auth выпускает непрозрачный high-entropy proof. Его plaintext возвращается ровно
   этому клиенту; в durable storage Auth хранит только криптографический hash.
2. Proof привязан к account и active profile владельца, точным `space_id`,
   `new_owner_profile_id`, UUID `operation_id`, текущему `session_epoch` и
   фактически проверенным factors. TTL составляет пять минут. Auth отзывает proof
   при изменении session epoch, пароля, 2FA или другого security state account.
3. Публичный transfer request в Space содержит `space_id`, `new_owner_profile_id`,
   `operation_id` и proof. `operation_id` — UUID, выбранный клиентом для одного
   намерения передачи; proof из одного operation нельзя использовать для другого.
   Gateway передаёт authenticated actor только из verified claims, relay-ит opaque proof в Space, redacts его из logs/traces/metrics и не создаёт, не валидирует, не хранит proof и не выводит из него actor.
4. Space сначала аутентифицирует actor и проверяет exact idempotency record по
   `(actor_profile_id, operation_id)`. Completed exact replay возвращает только
   saved outcome исходному actor без повторной проверки current owner и без новых
   данных или side effects. Для новой операции Space проверяет текущего owner. Первый запрос сохраняет canonical non-secret bindings и криптографический digest proof, без plaintext proof.
   Тот же body возвращает ранее сохранённый outcome; иной body с тем же ключом
   завершается `ALREADY_EXISTS`. Space сначала резервирует protocol-2 journal и
   атомарно подтверждает trusted Auth consume с этими exact bindings, затем
   вызывает Role Prepare.
5. Auth consume допускается только trusted Space principal, атомарно помечает proof
   использованным и возвращает durable receipt, привязанный к `operation_id`.
   Повтор того же consume после сетевого сбоя возвращает этот receipt, не выдаёт
   второе разрешение и не расходует другой proof. Любая ошибка Auth/receipt или
   несовпадение binding закрывает передачу до mutation.
6. Только этот verified protocol-2 path вправе менять системную роль `Owner`:
   Space сохраняет необратимое commit/abort решение; commit следует только после
   Role Prepare и вызывает Finalize, а abort вызывает Role Abort, включая durable
   barrier до наблюдаемого Prepare. Успешные audit/ready outbox event становятся
   видимыми только после matching terminal Role receipt и локального completion;
   обычные member-role операции не могут выдать, снять или переназначить `Owner`.
   Private v1 compensation не является production fallback.

Клиентские ошибки не раскрывают, существует ли proof и почему именно он не годен:
отсутствующий/невалидный session → `UNAUTHENTICATED`; неверный password или
second factor, отсутствующий/истёкший/отозванный/уже consumed proof, либо любой
несовпавший binding → `PERMISSION_DENIED`; malformed/missing UUID fields →
`INVALID_ARGUMENT`; actor не owner, self-transfer или target не member →
`FAILED_PRECONDITION`; отсутствующий space → `NOT_FOUND`; повторный
`operation_id` с изменённым телом → `ALREADY_EXISTS`. Internal Auth/Role
unavailability остаётся `UNAVAILABLE`; остальные failure path fail closed.

**Verification before shipment:** contract tests prove required factors by account
state; all bindings and five-minute expiry; every revocation trigger; single consume
and durable same-operation receipt; Space same-body replay and changed-body
`ALREADY_EXISTS`; no mutation/audit/event on consume denial; and that direct Role
member RPCs reject `Owner` mutation while the trusted protocol-2
Prepare/decision/Finalize-or-Abort path reaches one matching terminal outcome.
- **Бан участника**: забаненный не может зайти в спейс; его сообщения остаются (не удаляются); публичный контент спейса — не видит (как Discord)
- **Slow mode для текстовых чатов** (`group` \| `channel`): настраиваемый интервал 5 сек – 6 ч (настраивается из rate limiting)
- **Шаблоны при создании**: выбор темы — "Игровое" / "Рабочее" / "Общение"; влияет на дефолтные каналы и структуру, не на функциональность

## Аудит-лог

Журнал административных действий внутри спейса — как в Discord.

**Что фиксируется:**
- Бан / кик / разбан участника (кто, кого, причина)
- Изменение ролей участника
- Создание / удаление / переименование **текстового чата** (группа или канал в Chat), **голосовой комнаты**; изменение **дерева** (`space_tree_nodes`: порядок, категории, перенос текст/голос)
- Изменение настроек спейса
- Создание / отзыв инвайт-ссылки
- Добавление / удаление роли

**Доступ:**
- Только для ролей с правом "Просматривать аудит-лог" (по умолчанию — администраторы и владелец)

**UX:**
- Список событий в хронологическом порядке (новые сверху)
- Фильтрация по типу события и по конкретному модератору
- Каждая запись: дата, кто сделал, что сделал, с кем / с чем

## Pin элемента дерева спейса

Закреп узла (`space_tree_nodes`: text chat или voice room) вверху категории или корня дерева.

| Аспект | Правило |
|--------|---------|
| **Кто может** | Участники с правом управления деревом спейса (роли спейса; по умолчанию — админы и владелец) |
| **UX** | Иконка pin на узле; pinned nodes сортируются выше обычных в той же категории |
| **Лимиты** | Без жёсткого глобального лимита; разумный UI-cap на клиенте |
| **≠ Quick Access** | Pin **элемента дерева** (text chat / voice room) — только sidebar спейса; **Quick Access** в rail — отдельная сущность профиля ([navigation.md](navigation.md), [GLOSSARY.md](../GLOSSARY.md)) |
| **≠ folder pin** | Pin **чата в inbox-папке** — другой контекст ([navigation.md](navigation.md)); не путать с pin **узла дерева** |

## Удаление Спейса

- Удаление доступно только owner и требует пароль/2FA плюс ввод имени Спейса.
- После подтверждения начинается **7-дневное recovery window**: Спейс скрыт из
  каталога и списков участников, заморожен для чтения/записи/join/invite/MM, а owner
  видит только действие восстановления.
- Восстановление в течение 7 дней возвращает прежние memberships, roles, tree,
  chats и files; новые события в замороженный Space не принимаются.
- После 7 дней выполняется необратимый purge: memberships, roles, invites, bans,
  tree, voice rooms, Space-attached chats/messages и Space-owned media удаляются;
  File Service удаляет binary objects только когда на них не осталось других refs.
- Отдельно сохраняется минимальный audit tombstone (`space_id`, owner account
  tombstone, timestamps, deletion actor/reason) по production retention policy.
- `space.deletion_scheduled` публикуется при начале окна, `space.restored` — при
  восстановлении, а `space.deleted` — только после завершённого purge. Частичный
  cross-service purge повторяется идемпотентно до convergence.

### Публичный lifecycle contract A2

Методы, JSON, ошибки и retry для leave/transfer/delete/restore, invites, tree и
аудита зафиксированы в [API Gateway](../microservices/api-gateway.md#a2-space-rest-contract-target)
как target. Удаление использует отдельный Auth proof purpose `space_delete`,
связанный с точным именем Спейса и operation_id; proof передачи владельца для
удаления непригоден. Factors, TTL и consume описаны в
[Space Service](../microservices/space-service.md#a2-public-lifecycle-and-retry-contract-target).
Это фиксация входов для следующей реализации, а не изменение статуса готовности.
