# Chat Service

## Обзор

Управление сущностями чатов: DM (1:1), текстовые **группы** и **каналы** (`type = group` \| `channel`, в т.ч. вне спейса; одинаковая модель, разные дефолты), пользовательские папки.

**Язык**: Go
**БД**: PostgreSQL `chat_db`

## Ответственность

- Создание и управление DM-чатами
- Создание и управление **текстовыми групповыми чатами** (`type = group` \| `channel`, до 500 участников на чат вне спейса по продукту; одна модель API); `space_id` опционален; в спейсе узел **`space_tree_nodes`** — совместно с Space
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

  // Текстовые групповые чаты (group | channel) — один набор RPC; тип задаётся в запросе (`ChatType`)
  rpc CreateChat(CreateChatRequest) returns (Chat);   // type = group | channel; space_id optional
  rpc UpdateChat(UpdateChatRequest) returns (Chat);
  rpc DeleteChat(DeleteChatRequest) returns (Empty);

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
├── chat_id (UUID, logical ref → chats.id)
├── profile_id (UUID, logical ref → user_db.profiles.id; без меж-БД REFERENCES)
├── role (owner | admin | member)
├── joined_at
├── muted_until (nullable)
├── is_archived (bool)
└── UNIQUE(chat_id, profile_id)

folders
├── id (UUID)
├── profile_id (UUID, logical ref → user_db.profiles.id)
├── name
├── type (system | custom)
├── filter_config (jsonb) -- правила фильтрации
├── sort_order (int)
└── created_at

folder_chats (для custom folders)
├── folder_id (UUID, logical ref → folders.id)
├── chat_id (UUID, logical ref → chats.id)
└── added_at
```

### V1 (Фаза 0-1) — детальный профиль для DDL

В первой волне миграций Chat ограничен DM-сценарием:
- `chats` с `type = dm`
- `chat_members`
- пользовательские папки (`folders`, `folder_chats`) отложены и не входят в v1.

```
chats
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── type VARCHAR(16) NOT NULL CHECK (type = 'dm')
├── space_id UUID NULL -- в v1 всегда NULL
├── name TEXT NULL
├── avatar_url TEXT NULL
├── topic TEXT NULL
├── creator_profile_id UUID NOT NULL -- logical ref → user_db.profiles.id
├── slow_mode_seconds INTEGER NOT NULL DEFAULT 0 CHECK (slow_mode_seconds = 0)
├── last_message_at TIMESTAMPTZ NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

chat_members
├── chat_id UUID NOT NULL -- logical ref → chats.id
├── profile_id UUID NOT NULL -- logical ref → user_db.profiles.id
├── role VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member'))
├── joined_at TIMESTAMPTZ NOT NULL DEFAULT now()
├── muted_until TIMESTAMPTZ NULL
├── is_archived BOOLEAN NOT NULL DEFAULT false
└── PRIMARY KEY (chat_id, profile_id)
```

Индексы v1:
- `INDEX chat_members_profile_id_idx (profile_id, joined_at DESC)` для `ListChats`
- `INDEX chats_last_message_at_idx (last_message_at DESC)` для сортировки диалогов
- `INDEX chats_creator_profile_id_idx (creator_profile_id)`

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`chat.events`** (совместно с Space для событий дерева/спейса; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

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


