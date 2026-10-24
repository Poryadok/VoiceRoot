# Roles — ролевая модель и права

Канонический список имён прав и bitmask — [microservices/role-service.md](../microservices/role-service.md) (раздел «Идентификаторы прав»).

## Базовая иерархия в спейсе

Владелец → Администратор → Модератор → Участник → Гость

## Кастомные роли

Можно создавать роли с произвольным набором флагов из канонического перечисления (`roles.permissions`).

## Поведение по умолчанию

- **Дефолтная роль**: «Участник» — выдаётся при вступлении в спейс; настраивается владельцем или админом
- **Admin**: все обычные права, включая управление нижестоящими кастомными ролями; явный deny оверрайда действует и на Admin. Это не даёт Owner identity или права менять Owner через generic API.
- **Рассмотрение заявок в спейс** (принятый target): отдельное делегируемое
  `SPACE_APPROVE_JOIN_REQUESTS` даёт чтение заявок/ответов и approve/reject;
  по умолчанию доступно Owner, Admin и Moderator. У Member и Guest этого права
  по умолчанию нет. Кастомная роль может получить его по обычным правилам
  назначения прав. Управление инвайтами, настройками и банами не заменяет это
  право. См. [правила входа](spaces.md#верификация-при-вступлении).
- **Наследование прав**: объединение флагов по всем ролям участника; при конфликте **явный deny в оверрайде перебивает allow**; приоритет ролей (позиция в списке) задаётся в настройках спейса и влияет на действия, завязанные на иерархию (например `MEMBER_ASSIGN_ROLES`)

## Права (гранулярно, именами из Role Service)

Примеры (не исчерпывающий список — см. role-service):

- `TEXT_CHAT_SEND_MESSAGES`, `TEXT_CHAT_EMBED_LINKS`, `TEXT_CHAT_ATTACH_FILES`
- `TEXT_CHAT_CREATE_IN_SPACE` — создавать новые текстовые чаты в спейсе (`group` \| `channel`) вместе с узлом дерева
- `MEMBER_KICK`, `MEMBER_BAN`, `MEMBER_ASSIGN_ROLES`
- `SPACE_MANAGE_ROLES`, `SPACE_MANAGE_SETTINGS`, `SPACE_MANAGE_BOTS`, `SPACE_MANAGE_MATCHMAKING`
- `VOICE_JOIN`, `VOICE_SPEAK`, `VOICE_MUTE_OTHERS`, …
- `TEXT_CHAT_MENTION_ALL_IN_CHAT`, `TEXT_CHAT_MENTION_ALL_ONLINE` — упоминания всех в чате и всех онлайн в чате (синтаксис в клиенте может быть `@everyone` / `@here`)
- `TEXT_CHAT_MANAGE_MESSAGES`, `TEXT_CHAT_PIN_MESSAGES`, `TEXT_CHAT_SET_SLOW_MODE`

## Оверрайды по текстовому чату и голосовой комнате

Для конкретного `chat_id` (`group` \| `channel`) или `voice_room_id` права роли можно усилить или запретить через `chat_overrides` / `voice_room_overrides` (поля `allow` / `deny` bitmask). См. [DATA_MODEL.md](../DATA_MODEL.md).

## Верификационные роли

Роли, выдаваемые автоматически при верификации участника (например, «подтверждён по Steam», «ранг Diamond+»).

## Организатор войс-чата

- Это постоянная роль в спейсе
- Любой участник с ролью организатора автоматически имеет права организатора при входе в войс-чат
- Права: управлять микрофонами участников, выдавать/забирать слово, включать режим «поднять руку»

Для accepted target BE-273 управление чужим микрофоном требует текущих
`VOICE_MUTE_OTHERS` в точной канонической комнате и `SPACE_VIEW_MEMBER_LIST`
для выбора/раскрытия адресата. Owner bypass разрешает только Role; отдельной
Voice-иерархии или иммунитета нет. Ограничение `moderator_muted` хранит Voice,
не меняя Role grants; оно запрещает собственное аудио адресата, включая
screen-share audio. Снятие не включает микрофон и не выдаёт `VOICE_SPEAK`.
Self-target запрещён. Timeout остаётся отдельной admission/media проверкой и
не отнимает сам по себе moderator administration rights. См.
[moderator mute](voice-chat.md#moderator-mute-в-space-accepted-target-be-273-не-реализовано).

## DM от незнакомцев

Кто может писать пользователю в DM — настраивается **самим пользователем** в настройках приватности (не на уровне спейса).

## Permanent Space retirement (P3)

После `PURGE_DECIDED` только Space workload identity вызывает защищённый
`RetireSpace` с `protocol_version=1`, Space/deletion IDs, generation,
`purge_decided_at` и immutable manifest binding. Role отказывает, пока любая
ownership-v2 операция `PREPARED`; затем одной транзакцией записывает permanent
retirement fence и compact receipt, удаляет ordinary Role rows и публикует
последнюю policy invalidation. Точный replay возвращает сохранённый receipt,
изменённый запрос для того же Space не может заменить fence.

После commit все delayed ownership v1/v2, ordinary permission, bootstrap и
identifier-reuse paths сначала проверяют retirement fence. Старые ownership
receipts могут быть удалены только после retirement и отсутствия `PREPARED`;
fence и compact receipt не имеют time-based expiry. Restore происходит только
до `PURGE_DECIDED`, поэтому permanent retirement с ним не пересекается.

Role хранит compact evidence в `role_space_retirement_receipts`, а full
request/receipt bytes — минимум 30 дней от database `retired_at`. Вызывать
`RetireSpace` можно только через authenticated TLS listener; ordinary listener
возвращает `UNAVAILABLE` до входа в handler.
