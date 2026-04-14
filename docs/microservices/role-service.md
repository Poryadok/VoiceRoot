# Role Service

## Обзор

Управление ролями и правами в пространствах. Иерархия ролей, гранулярные права, оверрайды по текстовым чатам (`group` / `channel`) и голосовым комнатам.

**Язык**: Go
**БД**: PostgreSQL `role_db`

## Ответственность

- Предустановленные роли: Owner, Admin, Moderator, Member, Guest
- Кастомные роли
- Иерархия ролей (позиция определяет приоритет)
- 32+ типов прав (send_messages, manage_channels, ban_members, etc.)
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

## Типы прав

```
// General
VIEW_CHANNEL, MANAGE_CHANNEL, MANAGE_SPACE, MANAGE_ROLES,
MANAGE_INVITES, MANAGE_EMOJIS, VIEW_AUDIT_LOG,

// Membership
KICK_MEMBERS, BAN_MEMBERS, MANAGE_NICKNAMES,

// Text
SEND_MESSAGES, SEND_MEDIA, EMBED_LINKS, ATTACH_FILES,
ADD_REACTIONS, USE_EXTERNAL_EMOJIS, MENTION_EVERYONE,
MANAGE_MESSAGES, READ_MESSAGE_HISTORY, PIN_MESSAGES,

// Threads
CREATE_THREADS, SEND_THREAD_MESSAGES, MANAGE_THREADS,

// Voice
CONNECT, SPEAK, VIDEO, SCREEN_SHARE, MUTE_MEMBERS,
DEAFEN_MEMBERS, MOVE_MEMBERS, USE_PTT, PRIORITY_SPEAKER,

// Moderation
MANAGE_REPORTS, TIMEOUT_MEMBERS, SLOW_MODE,

// Bots
MANAGE_BOTS,

// Matchmaking
MANAGE_MM_CONFIG
```

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
2. Base = @everyone (Member) permissions
3. Для каждой роли пользователя (по позиции):
   Base |= role.permissions
4. Применить оверрайды целевого узла (chat или voice_room):
   Base &= ~node_override.deny
   Base |= node_override.allow
5. Admin → все права кроме Owner-specific
```

## Публикуемые события (→ NATS)

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


