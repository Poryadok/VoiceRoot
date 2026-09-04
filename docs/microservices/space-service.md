# Space Service

## Обзор

Управление пространствами (аналог Discord-серверов): **дерево sidebar** — единая таблица **`space_tree_nodes`** (текстовые чаты `group`/`channel` и **голосовые комнаты** в одном порядке сортировки), категории, инвайты, участники, шаблоны.

**Язык**: Go
**БД**: PostgreSQL `space_db`

Пул advisory lease должен подключаться напрямую к PostgreSQL или через session pooling; transaction-mode PgBouncer не поддерживается, потому что lock привязан к сессии.

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
  rpc UpdateSpaceMmConfig(UpdateSpaceMmConfigRequest) returns (Space); // ✓ shipped
  rpc DeleteSpace(DeleteSpaceRequest) returns (Empty);               // current hard-delete handler; must become schedule-delete
  rpc RestoreSpace(RestoreSpaceRequest) returns (Space);             // target: owner restore within 7 days
  rpc GetSpace(GetSpaceRequest) returns (Space);
  rpc ListMySpaces(ListMySpacesRequest) returns (SpaceList);
  rpc SearchPublicSpaces(SearchRequest) returns (SpaceList);         // ✗ unimplemented

  // Голосовые комнаты (сущность) + дерево sidebar (текст и голос в одном слое)
  rpc CreateVoiceRoom(CreateVoiceRoomRequest) returns (VoiceRoom);
  rpc UpdateVoiceRoom(UpdateVoiceRoomRequest) returns (VoiceRoom);
  rpc DeleteVoiceRoom(DeleteVoiceRoomRequest) returns (Empty); // каскад на узел в space_tree_nodes
  rpc UpsertTreeNode(UpsertTreeNodeRequest) returns (SpaceTreeNode); // text_chat (chat_id) или voice_room (voice_room_id)
  rpc RemoveTreeNode(RemoveTreeNodeRequest) returns (Empty);
  rpc ListSpaceTree(ListSpaceTreeRequest) returns (SpaceTreeList);   // ✓ shipped
  rpc CreateCategory(CreateCategoryRequest) returns (Category);
  rpc UpdateCategory(UpdateCategoryRequest) returns (Category);
  rpc DeleteCategory(DeleteCategoryRequest) returns (Empty);
  rpc ReorderSpaceTree(ReorderRequest) returns (Empty); // только space_tree_nodes: порядок и категории для текста и голоса
  rpc PinTreeNode(PinTreeNodeRequest) returns (SpaceTreeNode);   // ✓ shipped
  rpc UnpinTreeNode(UnpinTreeNodeRequest) returns (SpaceTreeNode); // ✓ shipped

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
  rpc TimeoutMember(TimeoutMemberRequest) returns (Empty);           // ✓ shipped
  rpc RemoveMemberTimeout(RemoveMemberTimeoutRequest) returns (Empty); // ✓ shipped
  rpc TransferOwnership(TransferRequest) returns (Empty);            // ✓ backend gRPC (Role Owner reassign fail-closed + compensated)
  rpc AddBotMember(AddBotMemberRequest) returns (SpaceMembership);   // ✓ shipped
  rpc RemoveBotMember(RemoveBotMemberRequest) returns (Empty);       // ✓ shipped

  // Шаблоны
  rpc ListTemplates(Empty) returns (TemplateList);                   // ✗ unimplemented
  rpc CreateFromTemplate(CreateFromTemplateRequest) returns (Space); // ✗ unimplemented

  // Аудит
  rpc GetAuditLog(GetAuditLogRequest) returns (AuditLogList);        // ✓ backend gRPC

  // S2S / internal
  rpc AreCoMembers(AreCoMembersRequest) returns (AreCoMembersResponse); // ✓ shipped
  rpc SyncSpaceProSubscription(SyncSpaceProSubscriptionRequest) returns (Empty); // ✓ shipped
}
```

### Implementation status (proto vs handlers)

Источник истины: [protos/voice/space/v1/space.proto](../../protos/voice/space/v1/space.proto). Сводка по `src/backend/space/internal/grpcsvc/`:

| RPC | Proto | Handler | Notes |
|-----|-------|---------|-------|
| CreateSpace, UpdateSpace, GetSpace, ListMySpaces | ✓ | ✓ | |
| UpdateSpaceMmConfig | ✓ | ✓ | MM config on space |
| DeleteSpace | ✓ | ✓ | Current owner-only hard delete is obsolete; target schedules 7-day hidden/frozen recovery window |
| RestoreSpace | ✗ | ✗ | Target owner-only restore during recovery window |
| SearchPublicSpaces | ✓ | ✗ | Catalog search backlog |
| Create/Update/Delete VoiceRoom | ✓ | ✓ | |
| UpsertTreeNode, RemoveTreeNode, ReorderSpaceTree | ✓ | ✓ | Tree pin fields shipped in migration `000007_tree_pin` |
| **ListSpaceTree** | ✓ | ✓ | **Omitted from earlier doc inventory** |
| Create/Update/Delete Category | ✓ | ✓ | |
| PinTreeNode, UnpinTreeNode | ✓ | ✓ | Migration `000007_tree_pin`; handlers and event payload shipped |
| CreateInvite, GetInvite, JoinByInvite | ✓ | ✓ | |
| **RevokeInvite, ListInvites** | ✓ | ✓ | **Owner-only** today (`requireSpaceOwner`) — normative target: role with `MANAGE_INVITES` |
| JoinSpace, LeaveSpace | ✓ | ✓ | Composable AND entry policy and invite-safe verifier pipeline remain backlog — [todo/backend.md](../todo/backend.md) |
| KickMember, BanMember, UnbanMember, ListMembers, ListBans | ✓ | ✓ | |
| TimeoutMember, RemoveMemberTimeout | ✓ | ✓ | |
| TransferOwnership | ✓ | ✓ | Backend owner→member path; Owner role Assign/Revoke fail-closed when Roles wired; failed role or audit step compensates the Owner transition and database owner; audit/event only after success. Password/2FA confirmation and Gateway/Flutter lifecycle UX remain backlog |
| AddBotMember, RemoveBotMember | ✓ | ✓ | |
| ListTemplates, CreateFromTemplate | ✓ | ✗ | |
| GetAuditLog | ✓ | ✓ | `created_at DESC, id DESC`; opaque timestamp+UUID keyset cursor; default 50/max 100; exact `SPACE_VIEW_AUDIT_LOG` check, owner-only fallback only when Role Service is unwired. Filters and REST/Flutter surfaces remain backlog |
| AreCoMembers | ✓ | ✓ | S2S co-membership check |
| SyncSpaceProSubscription | ✓ | ✓ | Subscription sync |

`GetAuditLog` читает только строки запрошенного `space_id` и возвращает все поля `AuditLogEntry`. Ошибка Role Service закрывает доступ (`UNAVAILABLE`), явный deny даёт `PERMISSION_DENIED`, malformed cursor — `INVALID_ARGUMENT`. Наличие RPC не означает полноту аудита: writers для части действий, фильтры по actor/action и клиентские REST/Flutter поверхности остаются в [backend backlog](../todo/backend.md).

**Invite permissions (code vs spec):** shipped handlers gate `RevokeInvite` / `ListInvites` on **space owner** only. Product spec allows admins with invite-management permission — align handlers when Role Service integration lands; until then document owner-only as **partial shipment**.

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
├── allow_guests (bool, default false)
├── entry_policy_version (int)
├── entry_policy (jsonb: enabled phone/captcha/questions/manual + versioned config)
├── mm_config (jsonb — space-level matchmaking settings)
├── deletion_scheduled_at (nullable)
├── purge_after (nullable; deletion_scheduled_at + 7 days)
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
├── is_pinned (bool, default false) — pinned nodes sort above unpinned within same category
├── pin_order (int, nullable) — ordering among pinned nodes
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

### Pin tree node

Закреп узла sidebar — UX [spaces.md](../features/spaces.md) § Pin элемента дерева. Поля и handlers shipped в migration `000007_tree_pin`.

```protobuf
message PinTreeNodeRequest {
  string space_id = 1;
  string node_id = 2;
}
message UnpinTreeNodeRequest {
  string space_id = 1;
  string node_id = 2;
}
```

**Rules:** pinned nodes render above unpinned in same `category_id`; `ReorderSpaceTree` respects pin group; audit `space.tree_node_upserted` includes **`is_pinned`**, **`pin_order`** (R2-A15). ≠ Quick Access (profile rail) ≠ folder pin (Chat inbox).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`chat.events`** (совместно с Chat; события спейса и дерева — те же потребители; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                          |
|-------------------------|---------------------------------|
| `space.created`         | space_id, owner_id              |
| `space.updated`         | space_id, changed_fields        |
| `space.deletion_scheduled` | space_id, owner_id, purge_after |
| `space.restored`        | space_id, owner_id              |
| `space.deleted`         | space_id (only after converged irreversible purge) |
| `space.member_joined`   | space_id, profile_id            |
| `space.member_left`     | space_id, profile_id            |
| `space.member_banned`   | space_id, account_id, banned_by |
| `space.voice_room_created`   | space_id, voice_room_id         |
| `space.voice_room_deleted`   | space_id, voice_room_id         |
| `space.tree_node_upserted`   | space_id, node_id, kind, chat_id?, voice_room_id?, **is_pinned**, **pin_order** |
| `space.tree_node_removed`    | space_id, node_id               |
| `space.invite_created`  | space_id, invite_code           |

## Зависимости

- **Chat Service** — создание/удаление строки текстового чата (`chats`, `group` \| `channel`); Space ведёт **`space_tree_nodes`** (`kind = text_chat`)
- **Role Service** — проверка прав при операциях (в т.ч. `chat_overrides` по `chat_id` и `voice_room_overrides`)
- **Subscription Service** — лимиты узлов дерева (текст + голос) и участников (free vs Pro)
- **User Service** — профили участников
- **Social Service** — проверка блокировок при join
