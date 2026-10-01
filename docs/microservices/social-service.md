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
- Для отображения списка блокировок `BlockAccount` может передать `blocked_profile_id` — именно открытый пользователем профиль. Social до записи проверяет через User, что этот профиль принадлежит `blocked_account_id`, и сохраняет полученные от User `display_name`, `username`, `discriminator` как снимок. `ListBlocked` возвращает снимок вместе с `blocked_account_id`; `UnblockAccount` использует только ID аккаунта. Social не выбирает другой профиль аккаунта. У старых блокировок без ID профиля поля снимка пусты.
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
├── is_favorite (bool; projection of profile_favorites for this contact)
├── created_at
└── updated_at

profile_favorites
├── owner_profile_id (UUID, logical ref → user_db.profiles.id)
├── favorite_profile_id (UUID, logical ref → user_db.profiles.id)
├── created_at
├── updated_at
└── PRIMARY KEY(owner_profile_id, favorite_profile_id)

blocks
├── id (UUID)
├── blocker_account_id (UUID, logical ref → auth_db.accounts.id) -- блокировка на уровне аккаунта
├── blocked_account_id (UUID, logical ref → auth_db.accounts.id)
├── blocked_profile_id (UUID, nullable; профиль, который блокирующий видел)
├── blocked_display_name / blocked_username / blocked_discriminator (nullable snapshot)
├── created_at
└── UNIQUE(blocker_account_id, blocked_account_id)
```

### V1 (core DM scope) — детальный профиль для DDL

В первой волне миграций используются `friendships` и `blocks`.
`contacts` и независимые `profile_favorites` добавляются отдельными миграциями после ядра DM/friends. Значения `contacts.is_favorite` переносятся в `profile_favorites`; последующие изменения избранного обновляют проекцию контакта, если такая строка существует.

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
├── blocked_profile_id UUID NULL -- added by migration 000003
├── blocked_display_name, blocked_username, blocked_discriminator TEXT NULL -- added by migration 000003
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()

friend_accept_outbox (migration 000004)
├── friendship_id UUID PRIMARY KEY REFERENCES friendships(id) ON DELETE CASCADE
├── requester_profile_id / target_profile_id UUID NOT NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── delivered_at TIMESTAMPTZ NULL

friend_request_outbox (migration 000006)
├── friendship_id UUID PRIMARY KEY REFERENCES friendships(id) ON DELETE CASCADE -- request_id
├── event_id UUID NOT NULL -- stable JetStream message identity for retries
├── requester_profile_id / target_profile_id UUID NOT NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
├── delivered_at TIMESTAMPTZ NULL
└── cancelled_at TIMESTAMPTZ NULL -- request was accepted/declined before notification dispatch
```

`AcceptFriendInvitation` commits the accepted friendship and its outbox row in one `social_db` transaction. A Social worker retries `social.friend_accepted` publication after NATS failures; Chat's durable consumer moves an existing DM request to `main` only if the pair remains friends. The consumer is idempotent, so publish success followed by a worker crash may safely replay. Deploy migration 000004 before the updated Social service; rollback of 000004 discards undelivered rows and therefore requires draining the outbox first.

`SendFriendInvitation` commits the pending friendship and its request-outbox row in one `social_db` transaction. A Social worker retries `social.friend_request` publication independently of the caller RPC. `FriendRequest.request_id` is the persisted friendship row ID; `SocialStreamEvent.event_id` and the JetStream `Nats-Msg-Id` come from the outbox row and stay stable across retries. The `social_events` stream deduplicates that message identity within its configured 24-hour duplicate window; delivery is at-least-once, and retries outside the window may be stored again. Re-sending a pending or declined request keeps its request ID and creates a new event ID for that explicit send. Accepting or declining cancels a request event that has not started dispatch. A publish already acknowledged by JetStream cannot be retracted if the Social process or database commit fails before marking the outbox row delivered; clients must reconcile notifications against the authoritative request list. Deploy migration 000006 and the updated `social_events` stream config before the updated Social service; rollback requires draining the request outbox first.

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

### Privacy service principals

Social calls User `GetPrivacySettings` and Space `AreCoMembers` on their
dedicated TLS listeners using request-bound RS256 service credentials. Only
`service:social` may use those protected privacy methods. Each attempt has a
fresh request ID and JWT ID; incoming identity metadata is never forwarded.
User/Space verify the signature, exact RPC/request hash/audience, expiry and
shared Redis replay admission before accessing their stores.

Social's `GetPrivacySettings`, `GetProfile` and `ListProfileIDsForAccount`
lookups use the User protected TLS listener with a fresh request-bound RS256
service credential. The protected listener exposes exactly these three User
methods to `service:social`; Social has no ordinary User 9090 lookup path.
Friends, phone contacts and account-level block cascades therefore fail closed
when signing, TLS, verification, replay admission, or the lookup response is
unavailable or malformed. This does not authorize raw metadata on either
listener.
Deployment and rotation: [DEPLOYMENT.md](../DEPLOYMENT.md#social-privacy-principals).

- **User Service** — получение профилей для списков
- **Auth Service** — маппинг profile_id → account_id (для блокировок)
