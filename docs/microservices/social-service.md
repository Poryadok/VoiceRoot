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
- Блокировка пользователей (применяется ко всему аккаунту, включая все профили)
- Friends-of-friends (1 уровень глубины) для приватности

## API (gRPC)

```protobuf
service SocialService {
  // Друзья
  rpc SendFriendRequest(FriendRequest) returns (Empty);
  rpc AcceptFriendRequest(FriendRequest) returns (Empty);
  rpc DeclineFriendRequest(FriendRequest) returns (Empty);
  rpc RemoveFriend(FriendRequest) returns (Empty);
  rpc ListFriends(ListFriendsRequest) returns (FriendList);
  rpc ListFriendRequests(ListRequestsRequest) returns (FriendRequestList);

  // Контакты
  rpc AddContact(AddContactRequest) returns (Empty);
  rpc RemoveContact(RemoveContactRequest) returns (Empty);
  rpc ListContacts(ListContactsRequest) returns (ContactList);
  rpc SyncPhoneContacts(SyncRequest) returns (SyncResponse);

  // Избранное
  rpc SetFavorite(SetFavoriteRequest) returns (Empty);
  rpc ListFavorites(ListFavoritesRequest) returns (FriendList);

  // Блокировки
  rpc BlockUser(BlockRequest) returns (Empty);
  rpc UnblockUser(BlockRequest) returns (Empty);
  rpc ListBlocked(ListBlockedRequest) returns (BlockedList);
  rpc IsBlocked(IsBlockedRequest) returns (IsBlockedResponse); // internal

  // Граф
  rpc AreFriends(AreFriendsRequest) returns (AreFriendsResponse); // internal
  rpc GetFriendsOfFriends(GetFoFRequest) returns (ProfileIdList); // internal, 1 level
}
```

## Модель данных

```
friendships
├── id (UUID)
├── requester_profile_id (FK)
├── target_profile_id (FK)
├── status (pending | accepted | declined)
├── created_at
└── updated_at

contacts
├── id (UUID)
├── owner_profile_id (FK)
├── target_profile_id (FK)
├── source (manual | phone_sync | space | matchmaking)
├── is_favorite (bool)
├── created_at
└── updated_at

blocks
├── id (UUID)
├── blocker_account_id (FK) -- блокировка на уровне аккаунта
├── blocked_account_id (FK)
├── created_at
└── UNIQUE(blocker_account_id, blocked_account_id)
```

## Публикуемые события (→ NATS)

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


