# Space Service

## Обзор

Управление пространствами (аналог Discord-серверов): **дерево sidebar** — единая таблица **`space_tree_nodes`** (текстовые чаты `group`/`channel` и **голосовые комнаты** в одном порядке сортировки), категории, инвайты, участники, шаблоны.

**Язык**: Go
**БД**: PostgreSQL `space_db`

## Ответственность

- CRUD пространств
- Видимость: public / invite-only / private
- Категории; **голосовые комнаты** (`voice_rooms` — сущность); **дерево** — `space_tree_nodes` (`kind`: текстовый чат → `chat_id` из Chat, или голос → `voice_room_id`)
- Системный канал (welcome, rules)
- Инвайт-ссылки (expiry, usage limits)
- Проверка при входе (phone / CAPTCHA / вопросы / ручное одобрение)
- Участники (join, leave, ban, kick)
- Лимиты: **50 узлов дерева** (текст + голос в сумме) free / 500 Pro; 50 участников free / 5000 Pro
- Каталог публичных пространств (поиск, ранжирование)
- Space-level matchmaking конфигурация
- Шаблоны пространств
- Аудит-лог действий (узлы дерева, голосовые комнаты, баны, изменение ролей)
- Передача владения
- Slow mode на уровне текстового чата (`group` \| `channel`) — в данных **Chat** (`chats.slow_mode_seconds`); Space может дублировать отображение/кэш при необходимости
- Бан пользователя (с сохранением сообщений)

## API (gRPC)

```protobuf
service SpaceService {
  // Пространства
  rpc CreateSpace(CreateSpaceRequest) returns (Space);
  rpc UpdateSpace(UpdateSpaceRequest) returns (Space);
  rpc DeleteSpace(DeleteSpaceRequest) returns (Empty);
  rpc GetSpace(GetSpaceRequest) returns (Space);
  rpc ListMySpaces(ListMySpacesRequest) returns (SpaceList);
  rpc SearchPublicSpaces(SearchRequest) returns (SpaceList);

  // Голосовые комнаты (сущность) + дерево sidebar (текст и голос в одном слое)
  rpc CreateVoiceRoom(CreateVoiceRoomRequest) returns (VoiceRoom);
  rpc UpdateVoiceRoom(UpdateVoiceRoomRequest) returns (VoiceRoom);
  rpc DeleteVoiceRoom(DeleteVoiceRoomRequest) returns (Empty); // каскад на узел в space_tree_nodes
  rpc UpsertTreeNode(UpsertTreeNodeRequest) returns (SpaceTreeNode); // text_chat (chat_id) или voice_room (voice_room_id)
  rpc RemoveTreeNode(RemoveTreeNodeRequest) returns (Empty);
  rpc CreateCategory(CreateCategoryRequest) returns (Category);
  rpc UpdateCategory(UpdateCategoryRequest) returns (Category);
  rpc DeleteCategory(DeleteCategoryRequest) returns (Empty);
  rpc ReorderSpaceTree(ReorderRequest) returns (Empty); // только space_tree_nodes: порядок и категории для текста и голоса

  // Инвайты
  rpc CreateInvite(CreateInviteRequest) returns (Invite);
  rpc RevokeInvite(RevokeInviteRequest) returns (Empty);
  rpc GetInvite(GetInviteRequest) returns (Invite);
  rpc ListInvites(ListInvitesRequest) returns (InviteList);
  rpc JoinByInvite(JoinByInviteRequest) returns (SpaceMembership);

  // Участники
  rpc JoinSpace(JoinSpaceRequest) returns (SpaceMembership);
  rpc LeaveSpace(LeaveSpaceRequest) returns (Empty);
  rpc KickMember(KickMemberRequest) returns (Empty);
  rpc BanMember(BanMemberRequest) returns (Empty);
  rpc UnbanMember(UnbanMemberRequest) returns (Empty);
  rpc ListMembers(ListMembersRequest) returns (MemberList);
  rpc ListBans(ListBansRequest) returns (BanList);
  rpc TransferOwnership(TransferRequest) returns (Empty);

  // Шаблоны
  rpc ListTemplates(Empty) returns (TemplateList);
  rpc CreateFromTemplate(CreateFromTemplateRequest) returns (Space);

  // Аудит
  rpc GetAuditLog(GetAuditLogRequest) returns (AuditLogList);
}
```

## Модель данных

```
spaces
├── id (UUID)
├── name
├── description (text)
├── icon_url
├── banner_url
├── visibility (public | invite_only | private)
├── owner_profile_id
├── member_count (denormalized counter)
├── is_verified (bool)
├── verification_type (none | personal | organization)
├── entry_requirement (none | phone | captcha | questions | manual)
├── entry_questions (jsonb, nullable)
├── mm_config (jsonb — space-level matchmaking settings)
├── created_at
└── updated_at

voice_rooms
├── id (UUID)
├── space_id (FK)
├── name
├── created_at
└── updated_at

space_tree_nodes
├── id (UUID)
├── space_id (FK)
├── category_id (FK, nullable)
├── kind (text_chat | voice_room)
├── chat_id (nullable — Chat, group|channel)
├── voice_room_id (nullable — FK → voice_rooms)
├── sort_order (int)
├── is_system (bool — только text_chat)
├── created_at
└── updated_at

categories
├── id (UUID)
├── space_id (FK)
├── name
├── sort_order (int)
└── created_at

space_members
├── space_id (FK)
├── profile_id (FK)
├── joined_at
├── nickname (nullable, space-specific)
└── UNIQUE(space_id, profile_id)

space_bans
├── space_id (FK)
├── account_id (FK)
├── banned_by (profile_id)
├── reason (text, nullable)
├── banned_at
└── UNIQUE(space_id, account_id)

invites
├── id (UUID)
├── space_id (FK)
├── code (string, unique)
├── creator_profile_id
├── max_uses (nullable)
├── use_count (int)
├── expires_at (nullable)
├── created_at
└── revoked_at (nullable)

audit_log
├── id (UUID)
├── space_id (FK)
├── actor_profile_id
├── action (string — voice_room_created, tree_node_upserted, tree_node_removed, member_banned, role_updated, ...)
├── target_type (string)
├── target_id (UUID)
├── details (jsonb)
└── created_at
```

## Публикуемые события (→ NATS)

| Событие                 | Данные                          |
|-------------------------|---------------------------------|
| `space.created`         | space_id, owner_id              |
| `space.updated`         | space_id, changed_fields        |
| `space.deleted`         | space_id                        |
| `space.member_joined`   | space_id, profile_id            |
| `space.member_left`     | space_id, profile_id            |
| `space.member_banned`   | space_id, account_id, banned_by |
| `space.voice_room_created`   | space_id, voice_room_id         |
| `space.voice_room_deleted`   | space_id, voice_room_id         |
| `space.tree_node_upserted`   | space_id, node_id, kind, chat_id?, voice_room_id? |
| `space.tree_node_removed`    | space_id, node_id               |
| `space.invite_created`  | space_id, invite_code           |

## Зависимости

- **Chat Service** — создание/удаление строки текстового чата (`chats`, `group` \| `channel`); Space ведёт **`space_tree_nodes`** (`kind = text_chat`)
- **Role Service** — проверка прав при операциях (в т.ч. `chat_overrides` по `chat_id` и `voice_room_overrides`)
- **Subscription Service** — лимиты узлов дерева (текст + голос) и участников (free vs Pro)
- **User Service** — профили участников
- **Social Service** — проверка блокировок при join


