# Chat Service

## Обзор

Управление сущностями чатов: DM (1:1), текстовые **группы** и **каналы** (`type = group` \| `channel`, в т.ч. вне спейса; одинаковая модель, разные дефолты), пользовательские папки.

**Язык**: Go
**БД**: PostgreSQL `chat_db`

## Ответственность

- Создание и управление DM-чатами
- Создание и управление группами (до 500 участников)
- Создание и управление текстовыми **группами** и **каналами** (одна модель `chats`; `space_id` опционален; в спейсе — плейсмент через Space)
- Участники: для чатов **без** `space_id` — `chat_members`; для чатов **в** спейсе — наследование от `space_members` + роли/оверрайды (см. [DATA_MODEL.md](../DATA_MODEL.md))
- Папки чатов (All / DM / Groups / Channels / Spaces / пользовательские)
- Список активных чатов (до 100)
- Мьют / архивация чатов
- Slow mode (таймер между сообщениями)

## API (gRPC)

```protobuf
service ChatService {
  // DM
  rpc CreateDM(CreateDMRequest) returns (Chat);
  rpc GetDM(GetDMRequest) returns (Chat); // find existing or create

  // Группы
  rpc CreateGroup(CreateGroupRequest) returns (Chat);
  rpc UpdateGroup(UpdateGroupRequest) returns (Chat);
  rpc DeleteGroup(DeleteGroupRequest) returns (Empty);

  // Текстовые каналы (в т.ч. standalone; привязка к спейсу — совместно с Space)
  rpc CreateTextChannel(CreateTextChannelRequest) returns (Chat);
  rpc UpdateTextChannel(UpdateTextChannelRequest) returns (Chat);
  rpc DeleteTextChannel(DeleteTextChannelRequest) returns (Empty);

  // Участники
  rpc AddMembers(AddMembersRequest) returns (Empty);
  rpc RemoveMember(RemoveMemberRequest) returns (Empty);
  rpc LeaveChat(LeaveChatRequest) returns (Empty);
  rpc ListMembers(ListMembersRequest) returns (MemberList);

  // Список чатов
  rpc ListChats(ListChatsRequest) returns (ChatList);
  rpc GetChat(GetChatRequest) returns (Chat);

  // Папки
  rpc ListFolders(ListFoldersRequest) returns (FolderList);
  rpc CreateFolder(CreateFolderRequest) returns (Folder);
  rpc UpdateFolder(UpdateFolderRequest) returns (Folder);
  rpc DeleteFolder(DeleteFolderRequest) returns (Empty);

  // Действия
  rpc MuteChat(MuteChatRequest) returns (Empty);
  rpc ArchiveChat(ArchiveChatRequest) returns (Empty);
}
```

## Модель данных

```
chats
├── id (UUID)
├── type (dm | group | channel)
├── space_id (nullable — группа/канал в спейсе)
├── name (nullable)
├── avatar_url (nullable)
├── topic (nullable, часто канал)
├── creator_profile_id
├── slow_mode_seconds (0 = off)
├── last_message_at
├── created_at
└── updated_at

chat_members
├── chat_id (FK)
├── profile_id (FK)
├── role (owner | admin | member)
├── joined_at
├── muted_until (nullable)
├── is_archived (bool)
└── UNIQUE(chat_id, profile_id)

folders
├── id (UUID)
├── profile_id (FK)
├── name
├── type (system | custom)
├── filter_config (jsonb) -- правила фильтрации
├── sort_order (int)
└── created_at

folder_chats (для custom folders)
├── folder_id (FK)
├── chat_id (FK)
└── added_at
```

## Публикуемые события (→ NATS)

| Событие               | Данные                             |
|-----------------------|------------------------------------|
| `chat.created`        | chat_id, type, creator_id, members |
| `chat.updated`        | chat_id, changed_fields            |
| `chat.deleted`        | chat_id                            |
| `chat.member_added`   | chat_id, profile_id, added_by      |
| `chat.member_removed` | chat_id, profile_id, removed_by    |
| `chat.member_left`    | chat_id, profile_id                |

## Зависимости

- **Social Service** — проверка блокировок при создании DM
- **User Service** — получение профилей участников
- **Subscription Service** — лимиты на количество участников группы
- **Space Service** — при создании текстового чата (`group` \| `channel`) в спейсе: узел **`space_tree_nodes`** (`kind=text_chat`) после создания строки `chats`


