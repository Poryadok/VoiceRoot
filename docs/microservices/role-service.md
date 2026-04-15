# Role Service

## Обзор

Управление ролями и правами в пространствах. Иерархия ролей, гранулярные права, оверрайды по текстовым чатам (`group` / `channel`) и голосовым комнатам.

**Язык**: Go
**БД**: PostgreSQL `role_db`

## Ответственность

- Предустановленные роли: Owner, Admin, Moderator, Member, Guest
- Кастомные роли
- Иерархия ролей (позиция определяет приоритет)
- Права как **набор именованных флагов** в `bigint` bitmask (`roles.permissions`); идентификаторы — `SCREAMING_SNAKE_CASE`, без имён из сторонних продуктов (см. раздел ниже)
- Оверрайды прав для узла спейса: отдельно для текстового чата (`chat_id`) и голосовой комнаты (`voice_room_id`)
- Назначение ролей участникам пространства
- Верификационные роли (автоматические по статусу верификации)
- Voice chat organizer роль
- Проверка прав (вычисление effective permissions)

## API (gRPC)

```protobuf
service RoleService {
  // Роли
  rpc CreateRole(CreateRoleRequest) returns (Role);
  rpc UpdateRole(UpdateRoleRequest) returns (Role);
  rpc DeleteRole(DeleteRoleRequest) returns (Empty);
  rpc ListRoles(ListRolesRequest) returns (RoleList);
  rpc ReorderRoles(ReorderRolesRequest) returns (Empty);

  // Назначение
  rpc AssignRole(AssignRoleRequest) returns (Empty);
  rpc RevokeRole(RevokeRoleRequest) returns (Empty);
  rpc GetMemberRoles(GetMemberRolesRequest) returns (RoleList);

  // Оверрайды прав узла спейса (текстовый чат = group | channel)
  rpc SetChatOverride(SetChatOverrideRequest) returns (Empty);
  rpc RemoveChatOverride(RemoveChatOverrideRequest) returns (Empty);
  rpc GetChatOverrides(GetChatOverridesRequest) returns (OverrideList);
  rpc SetVoiceRoomOverride(SetVoiceRoomOverrideRequest) returns (Empty);
  rpc RemoveVoiceRoomOverride(RemoveVoiceRoomOverrideRequest) returns (Empty);
  rpc GetVoiceRoomOverrides(GetVoiceRoomOverridesRequest) returns (OverrideList);

  // Проверка прав (internal, вызывается другими сервисами)
  rpc CheckPermission(CheckPermissionRequest) returns (CheckPermissionResponse);
  rpc GetEffectivePermissions(GetEffectiveRequest) returns (PermissionSet);
}
```

## Идентификаторы прав (bitmask)

Один канонический набор имён для **ролей в спейсе**, **оверрайдов** и проверок в сервисах. В манифестах ботов (`scopes`) используются **в основном те же строки**, плюс исключение `DM_SEND` (только боты; см. [features/bots.md](../features/bots.md)).

### Спейс (глобально по `space_id`)

| Константа | Назначение |
|-----------|------------|
| `SPACE_VIEW` | Видеть спейс и базовую информацию |
| `SPACE_MANAGE_SETTINGS` | Название, иконка, видимость, правила входа и пр. |
| `SPACE_MANAGE_ROLES` | Создавать/редактировать/удалять роли ниже своей позиции (политика иерархии) |
| `SPACE_MANAGE_INVITES` | Создавать и отзывать инвайт-ссылки |
| `SPACE_VIEW_AUDIT_LOG` | Читать аудит-лог |
| `SPACE_MANAGE_CUSTOM_EMOJIS` | Кастомные эмодзи спейса |
| `SPACE_MANAGE_BOTS` | Добавлять/удалять ботов, их scopes |
| `SPACE_MANAGE_MATCHMAKING` | Настройки матчмейкинга спейса |
| `SPACE_VIEW_MEMBER_LIST` | Видеть список участников спейса (ростер, не контент чатов) |

### Участники спейса

| Константа | Назначение |
|-----------|------------|
| `MEMBER_KICK` | Исключить участника |
| `MEMBER_BAN` | Бан / разбан |
| `MEMBER_MANAGE_NICKNAMES` | Менять ник в спейсе |
| `MEMBER_ASSIGN_ROLES` | Назначать и снимать с участников роли **ниже своей** позиции в иерархии (без создания/удаления определений ролей — это `SPACE_MANAGE_ROLES`) |

### Создание текстовых чатов в спейсе

| Константа | Назначение |
|-----------|------------|
| `TEXT_CHAT_CREATE_IN_SPACE` | Создавать новые строки `chats` с `type = group` \| `channel` и узлы дерева (совместно с Space) |

### Текстовый чат (`chat_id`, `group` \| `channel`)

| Константа | Назначение |
|-----------|------------|
| `TEXT_CHAT_VIEW` | Видеть чат в списке и открывать |
| `TEXT_CHAT_MANAGE_SETTINGS` | Тема, slow mode, настройки чата |
| `TEXT_CHAT_SEND_MESSAGES` | Писать сообщения (если политика чата разрешает от своего имени) |
| `TEXT_CHAT_SEND_MEDIA` | Вложения медиа |
| `TEXT_CHAT_EMBED_LINKS` | Превью / встраивание ссылок |
| `TEXT_CHAT_ATTACH_FILES` | Файлы |
| `TEXT_CHAT_ADD_REACTIONS` | Реакции |
| `TEXT_CHAT_USE_EXTERNAL_EMOJIS` | Внешние эмодзи |
| `TEXT_CHAT_MENTION_ALL_ONLINE` | Упоминание всех онлайн в этом чате (`@here`) |
| `TEXT_CHAT_MENTION_ALL_IN_CHAT` | Упоминание всех участников чата (`@everyone`) |
| `TEXT_CHAT_MANAGE_MESSAGES` | Удалять/закреплять чужие сообщения |
| `TEXT_CHAT_READ_HISTORY` | Читать историю (если выключено — виден только «с момента входа») |
| `TEXT_CHAT_PIN_MESSAGES` | Закреплять сообщения |
| `TEXT_CHAT_CREATE_THREADS` | Создавать треды |
| `TEXT_CHAT_SEND_IN_THREADS` | Писать в тредах |
| `TEXT_CHAT_MANAGE_THREADS` | Модерировать треды |
| `TEXT_CHAT_SET_SLOW_MODE` | Выставлять slow mode на чате |

### Голосовая комната (`voice_room_id`)

| Константа | Назначение |
|-----------|------------|
| `VOICE_JOIN` | Подключаться к комнате |
| `VOICE_SPEAK` | Аудио от себя |
| `VOICE_VIDEO` | Видео |
| `VOICE_SCREEN_SHARE` | Демонстрация экрана |
| `VOICE_MUTE_OTHERS` | Мьютить других |
| `VOICE_DEAFEN_OTHERS` | Deafen других |
| `VOICE_MOVE_OTHERS` | Переносить между комнатами |
| `VOICE_USE_PTT` | Push-to-talk, если включён режим |
| `VOICE_PRIORITY_SPEAKER` | Приоритетный говорящий |

### Модерация (спейс)

| Константа | Назначение |
|-----------|------------|
| `MODERATION_MANAGE_REPORTS` | Жалобы по контенту спейса |
| `MODERATION_TIMEOUT_MEMBERS` | Таймаут участника |

**Владелец спейса** обходит проверки (или эквивалент «все флаги»); точные биты и порядок фиксируются в коде при первой миграции bitmask — здесь зафиксированы **имена**, а не номера битов.

### Манифест бота (`scopes` в JSON)

Те же строковые константы, плюс **только для ботов** (не хранятся в bitmask роли участника):

| Константа | Назначение |
|-----------|------------|
| `DM_SEND` | Писать пользователю в DM — только в ответ на его действие (v1; см. [bots.md](../features/bots.md)) |

Остальные возможности бота задаются теми же именами, что и права участника, например `TEXT_CHAT_SEND_MESSAGES`, `TEXT_CHAT_CREATE_IN_SPACE`, `MEMBER_ASSIGN_ROLES`, `SPACE_VIEW_MEMBER_LIST`. Привилегированное чтение истории для бота — строка `TEXT_CHAT_READ_HISTORY` с отдельной политикой в Bot Service (предупреждение в UI при установке).

## Модель данных

```
roles
├── id (UUID)
├── space_id (FK)
├── name
├── color (hex, nullable)
├── is_system (bool) -- Owner, Admin, Moderator, Member, Guest
├── position (int) -- иерархия, выше = больше приоритет
├── permissions (bigint bitmask)
├── is_mentionable (bool)
├── created_at
└── updated_at

member_roles
├── space_id (FK)
├── profile_id (FK)
├── role_id (FK)
├── assigned_at
├── assigned_by (profile_id)
└── UNIQUE(space_id, profile_id, role_id)

chat_overrides
├── chat_id (FK) -- chats.id, type = group | channel
├── role_id (FK)
├── allow (bigint bitmask) -- явно разрешённые права
├── deny (bigint bitmask)  -- явно запрещённые права
└── UNIQUE(chat_id, role_id)

voice_room_overrides
├── voice_room_id (FK)
├── role_id (FK)
├── allow (bigint bitmask) -- явно разрешённые права
├── deny (bigint bitmask)  -- явно запрещённые права
└── UNIQUE(voice_room_id, role_id)
```

## Вычисление effective permissions

```
1. Если Owner → все права
2. Base = права дефолтной роли участника спейса (роль «участник» / «все»)
3. Для каждой роли пользователя (по позиции):
   Base |= role.permissions
4. Применить оверрайды целевого узла (chat или voice_room):
   Base &= ~node_override.deny
   Base |= node_override.allow
5. Admin → все права кроме Owner-specific
```

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`role.events`** (матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие             | Данные                            |
|---------------------|-----------------------------------|
| `role.created`      | space_id, role_id, name           |
| `role.updated`      | space_id, role_id, changed_fields |
| `role.deleted`      | space_id, role_id                 |
| `role.assigned`     | space_id, profile_id, role_id     |
| `role.revoked`      | space_id, profile_id, role_id     |
| `role.chat_override_set`  | chat_id, role_id       |
| `role.voice_override_set` | voice_room_id, role_id |

## Зависимости

- **Space Service** — валидация `space_id`, `voice_room_id`
- **Chat Service** — валидация `chat_id` для текстового чата (`group` \| `channel`) при оверрайдах
- **Federation Service** — синхронизация ролей при S2S (SyncSnapshot)


