# Social Service

## Обзор

Управление социальным графом: друзья, контакты, блокировки.

**Язык**: Go
**БД**: PostgreSQL `social_db`

## Ответственность

- Два уровня связей: контакт (одностороннее добавление) и друг (двустороннее подтверждение)
- Заявки в друзья (отправка, принятие, отклонение)
- Списки: друзья, избранное, заблокированные
- Методы добавления: по username, телефону, QR-коду, из пространства, из истории ММ
- Синхронизация телефонных контактов
- Блокировка **аккаунта** (все профили заблокированной стороны): в gRPC — `BlockAccount` / `UnblockAccount`, в теле запроса — `blocked_account_id` (= `accounts.id`), блокирующий — из контекста аутентификации (см. [DATA_MODEL.md](../DATA_MODEL.md))
- Friends-of-friends (1 уровень глубины) для приватности

## API (gRPC)

Канон: [`protos/voice/social/v1/social.proto`](../../protos/voice/social/v1/social.proto). Заявки в друзья: `SendFriendInvitation` / `AcceptFriendInvitation` / `DeclineFriendInvitation`; все ответы — уникальные `*Response` (см. buf STANDARD в [REPOSITORIES.md](../REPOSITORIES.md)).

```protobuf
service SocialService {
  // Друзья
  rpc SendFriendInvitation(SendFriendInvitationRequest) returns (SendFriendInvitationResponse);
  rpc AcceptFriendInvitation(AcceptFriendInvitationRequest) returns (AcceptFriendInvitationResponse);
  rpc DeclineFriendInvitation(DeclineFriendInvitationRequest) returns (DeclineFriendInvitationResponse);
  rpc RemoveFriend(RemoveFriendRequest) returns (RemoveFriendResponse);
  rpc ListFriends(ListFriendsRequest) returns (ListFriendsResponse);
  rpc ListFriendRequests(ListFriendRequestsRequest) returns (ListFriendRequestsResponse);

  // Контакты
  rpc AddContact(AddContactRequest) returns (AddContactResponse);
  rpc RemoveContact(RemoveContactRequest) returns (RemoveContactResponse);
  rpc ListContacts(ListContactsRequest) returns (ListContactsResponse);
  rpc SyncPhoneContacts(SyncPhoneContactsRequest) returns (SyncPhoneContactsResponse);

  // Избранное
  rpc SetFavorite(SetFavoriteRequest) returns (SetFavoriteResponse);
  rpc ListFavorites(ListFavoritesRequest) returns (ListFavoritesResponse);

  // Блокировки (уровень аккаунта)
  rpc BlockAccount(BlockAccountRequest) returns (BlockAccountResponse);
  rpc UnblockAccount(UnblockAccountRequest) returns (UnblockAccountResponse);
  rpc ListBlocked(ListBlockedRequest) returns (ListBlockedResponse);
  rpc IsBlocked(IsBlockedRequest) returns (IsBlockedResponse); // internal

  // Граф
  rpc AreFriends(AreFriendsRequest) returns (AreFriendsResponse); // internal
  rpc GetFriendsOfFriends(GetFriendsOfFriendsRequest) returns (GetFriendsOfFriendsResponse); // internal, 1 level
}
```

## Модель данных

```
friendships
├── id (UUID)
├── requester_profile_id (UUID, logical ref → user_db.profiles.id)
├── target_profile_id (UUID, logical ref → user_db.profiles.id)
├── status (pending | accepted | declined)
├── created_at
└── updated_at

contacts
├── id (UUID)
├── owner_profile_id (UUID, logical ref → user_db.profiles.id)
├── target_profile_id (UUID, logical ref → user_db.profiles.id)
├── source (manual | phone_sync | space | matchmaking)
├── is_favorite (bool)
├── created_at
└── updated_at

blocks
├── id (UUID)
├── blocker_account_id (UUID, logical ref → auth_db.accounts.id) -- блокировка на уровне аккаунта
├── blocked_account_id (UUID, logical ref → auth_db.accounts.id)
├── created_at
└── UNIQUE(blocker_account_id, blocked_account_id)
```

### V1 (Фаза 0-1) — детальный профиль для DDL

В первой волне миграций используются `friendships` и `blocks`.
`contacts` откладывается отдельной миграцией после ядра DM/friends.

```
friendships
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── requester_profile_id UUID NOT NULL -- logical ref → user_db.profiles.id
├── target_profile_id UUID NOT NULL -- logical ref → user_db.profiles.id
├── status VARCHAR(16) NOT NULL CHECK (status IN ('pending','accepted','declined'))
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

blocks
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── blocker_account_id UUID NOT NULL -- logical ref → auth_db.accounts.id
├── blocked_account_id UUID NOT NULL -- logical ref → auth_db.accounts.id
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Индексы v1:
- `UNIQUE INDEX friendships_pair_uq ON friendships(requester_profile_id, target_profile_id)`
- `INDEX friendships_target_status_idx (target_profile_id, status, created_at DESC)` для входящих заявок
- `INDEX friendships_requester_status_idx (requester_profile_id, status, created_at DESC)` для исходящих и списка друзей
- `UNIQUE INDEX blocks_pair_uq ON blocks(blocker_account_id, blocked_account_id)`
- `INDEX blocks_blocked_account_idx (blocked_account_id)` для обратной проверки блоков

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`social.events`** (матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                  | Данные                                 |
|--------------------------|----------------------------------------|
| `social.friend_request`  | requester_id, target_id                |
| `social.friend_accepted` | profile_id_a, profile_id_b             |
| `social.friend_removed`  | profile_id_a, profile_id_b             |
| `social.contact_added`   | owner_id, target_id, source            |
| `social.user_blocked`    | blocker_account_id, blocked_account_id |
| `social.user_unblocked`  | blocker_account_id, blocked_account_id |
| `social.contacts_synced` | owner_id, matched_count                |

## Зависимости

- **User Service** — получение профилей для списков
- **Auth Service** — маппинг profile_id → account_id (для блокировок)


