import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_deferred_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';

void main() {
  group('ChatListController', () {
    test('loads chats after auth becomes available', () async {
      final chats = _FakeChatsClient(
        pages: [
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-restored',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
                lastMessagePreview: 'restored',
              ),
            ],
          ),
        ],
      );
      final storage = InMemoryAuthSessionStorage();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(storage),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          voiceChatsClientProvider.overrideWithValue(chats),
          voiceMessagesClientProvider.overrideWithValue(_FakeMessagesClient()),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
        ],
      );
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      expect(container.read(chatListControllerProvider).items, isEmpty);
      expect(chats.calls, isEmpty);

      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'access',
          refreshToken: 'refresh',
          accountId: 'acc-1',
          activeProfileId: 'prof-1',
          expiresInSeconds: 900,
        ),
      );
      await pumpEventQueue();

      final state = container.read(chatListControllerProvider);
      expect(state.items.map((item) => item.chatId), ['chat-restored']);
      expect(chats.calls, hasLength(1));
    });

    test('reloads chats when session access token changes', () async {
      final chats = _FakeChatsClient(
        pages: [
          const ChatListData(items: []),
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-after-convert',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
              ),
            ],
          ),
        ],
      );
      final storage = InMemoryAuthSessionStorage();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(storage),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          voiceChatsClientProvider.overrideWithValue(chats),
          voiceMessagesClientProvider.overrideWithValue(_FakeMessagesClient()),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
        ],
      );
      addTearDown(container.dispose);

      const guestSession = AuthSession(
        accessToken: 'guest-access',
        refreshToken: 'guest-refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
        accountType: 'guest',
      );
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: guestSession,
      );
      container.read(chatListControllerProvider);
      await pumpEventQueue();
      expect(chats.calls, hasLength(1));

      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'regular-access',
          refreshToken: 'regular-refresh',
          accountId: 'acc-1',
          activeProfileId: 'prof-1',
          expiresInSeconds: 900,
          accountType: 'regular',
        ),
      );
      await pumpEventQueue();

      final state = container.read(chatListControllerProvider);
      expect(state.items.map((item) => item.chatId), ['chat-after-convert']);
      expect(chats.calls, hasLength(2));
    });

    test('loads the next chat page and keeps existing rows', () async {
      final chats = _FakeChatsClient(
        pages: [
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-1',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
                lastMessagePreview: 'first',
              ),
            ],
            nextCursor: 'cursor-2',
          ),
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-2',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-2',
                ),
                lastMessagePreview: 'second',
              ),
            ],
          ),
        ],
      );
      final container = _container(
        chatsClient: chats,
        messagesClient: _FakeMessagesClient(),
        realtimeHubBuilder: _FakeRealtimeHub.new,
      );
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      expect(container.read(chatListControllerProvider).items, hasLength(1));

      await container.read(chatListControllerProvider.notifier).loadMore();

      final state = container.read(chatListControllerProvider);
      expect(state.items.map((item) => item.chatId), ['chat-1', 'chat-2']);
      expect(chats.calls.map((call) => call.cursor), [null, 'cursor-2']);
    });
  });

  group('ChatRoomController', () {
    test(
      'fetches REST delta on realtime message_create and de-duplicates',
      () async {
        late _FakeRealtimeHub hub;
        final messages = _FakeMessagesClient(
          pages: [
            MessageListData(messages: [_message('msg-1')]),
            MessageListData(messages: [_message('msg-1'), _message('msg-2')]),
          ],
        );
        final container = _container(
          chatsClient: _FakeChatsClient(),
          messagesClient: messages,
          realtimeHubBuilder: (ref) => hub = _FakeRealtimeHub(ref),
        );
        addTearDown(container.dispose);

        final sub = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-1'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(sub.close);
        await pumpEventQueue();

        hub.addFrame(
          const RealtimeFrame(
            op: 'message_create',
            data: {'chat_id': 'chat-1', 'message_id': 'msg-2'},
            sequence: 2,
          ),
        );
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.messages.map((message) => message.id), ['msg-1', 'msg-2']);
        expect(messages.getCalls.last.afterMessageId, 'msg-1');
      },
    );

    test('clears deferred bot interaction on message_create', () async {
      late _FakeRealtimeHub hub;
      final messages = _FakeMessagesClient(
        pages: [
          MessageListData(messages: [_message('msg-1')]),
          MessageListData(messages: [_message('msg-1'), _message('msg-2')]),
        ],
      );
      final container = _container(
        chatsClient: _FakeChatsClient(),
        messagesClient: messages,
        realtimeHubBuilder: (ref) => hub = _FakeRealtimeHub(ref),
      );
      addTearDown(container.dispose);

      container
          .read(deferredBotInteractionProvider('chat-1').notifier)
          .setDeferred(botName: 'PingBot', interactionToken: 'tok-1');
      expect(
        container.read(deferredBotInteractionProvider('chat-1'))?.botName,
        'PingBot',
      );

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      hub.addFrame(
        const RealtimeFrame(
          op: 'message_create',
          data: {'chat_id': 'chat-1', 'message_id': 'msg-2'},
          sequence: 2,
        ),
      );
      await pumpEventQueue();

      expect(container.read(deferredBotInteractionProvider('chat-1')), isNull);
    });

    test('uses last_message_id for reconnect catch-up', () async {
      final messages = _FakeMessagesClient(
        pages: [
          MessageListData(messages: [_message('msg-1')]),
          MessageListData(messages: [_message('msg-2')]),
        ],
      );
      final container = _container(
        chatsClient: _FakeChatsClient(),
        messagesClient: messages,
        realtimeHubBuilder: _FakeRealtimeHub.new,
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.reconnecting;
      await pumpEventQueue();
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connected;
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.messages.map((message) => message.id), ['msg-1', 'msg-2']);
      expect(messages.getCalls.last.lastMessageId, 'msg-1');
    });

    test(
      'loads older messages by cursor and preserves current messages',
      () async {
        final messages = _FakeMessagesClient(
          pages: [
            MessageListData(
              messages: [_message('msg-2')],
              nextCursor: 'older-cursor',
              hasMore: true,
            ),
            MessageListData(messages: [_message('msg-1'), _message('msg-2')]),
          ],
        );
        final container = _container(
          chatsClient: _FakeChatsClient(),
          messagesClient: messages,
          realtimeHubBuilder: _FakeRealtimeHub.new,
        );
        addTearDown(container.dispose);

        final sub = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-1'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(sub.close);
        await pumpEventQueue();

        await container
            .read(chatRoomControllerProvider('chat-1').notifier)
            .loadOlderMessages();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.messages.map((message) => message.id), ['msg-1', 'msg-2']);
        expect(messages.getCalls.last.cursor, 'older-cursor');
      },
    );

    test('createGroupWithMembers creates group, invites, and selects chat', () async {
      final chats = _FakeChatsClient();
      late _FakeRealtimeHub hub;
      final container = _container(
        chatsClient: chats,
        messagesClient: _FakeMessagesClient(),
        realtimeHubBuilder: (ref) => hub = _FakeRealtimeHub(ref),
      );
      addTearDown(container.dispose);

      final err = await container
          .read(chatActionsProvider)
          .createGroupWithMembers(
            name: 'Squad',
            memberProfileIds: const ['friend-a', 'friend-b'],
          );
      await pumpEventQueue();

      expect(err, isNull);
      expect(chats.createGroupCalls, ['Squad']);
      expect(chats.addMembersCalls, hasLength(1));
      expect(chats.addMembersCalls.first.$1, 'group-created');
      expect(chats.addMembersCalls.first.$2, ['friend-a', 'friend-b']);
      expect(container.read(selectedChatIdProvider), 'group-created');
      expect(hub.subscribedChats, ['group-created']);
    });

    test('tracks typing profiles from typing WS frames', () async {
      late _FakeRealtimeHub hub;
      final container = _container(
        chatsClient: _FakeChatsClient(),
        messagesClient: _FakeMessagesClient(
          pages: [MessageListData(messages: [_message('msg-1')])],
        ),
        realtimeHubBuilder: (ref) => hub = _FakeRealtimeHub(ref),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      hub.addFrame(
        const RealtimeFrame(
          op: 'typing',
          data: {
            'chat_id': 'chat-1',
            'profile_id': 'peer-1',
            'kind': 'start',
          },
        ),
      );
      await pumpEventQueue();

      expect(
        container.read(chatRoomControllerProvider('chat-1')).typingProfileIds,
        contains('peer-1'),
      );
    });

    test('tracks delivery receipts from WS frames', () async {
      late _FakeRealtimeHub hub;
      final messages = _FakeMessagesClient(
        pages: [
          MessageListData(
            messages: [
              VoiceMessage(
                id: 'msg-mine',
                chatId: 'chat-1',
                senderProfileId: 'prof-test',
                content: 'mine',
                createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
              ),
            ],
          ),
        ],
      );
      final container = _container(
        chatsClient: _FakeChatsClient(),
        messagesClient: messages,
        realtimeHubBuilder: (ref) => hub = _FakeRealtimeHub(ref),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      hub.addFrame(
        const RealtimeFrame(
          op: 'message_delivered',
          data: {'chat_id': 'chat-1', 'message_id': 'msg-mine'},
        ),
      );
      await pumpEventQueue();

      var state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.deliveredMessageIds, contains('msg-mine'));

      hub.addFrame(
        const RealtimeFrame(
          op: 'message_read',
          data: {'chat_id': 'chat-1', 'message_id': 'msg-mine'},
        ),
      );
      await pumpEventQueue();

      state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.readMessageIds, contains('msg-mine'));
    });

    test(
      'marks the latest loaded message read and fans out over realtime',
      () async {
        late _FakeRealtimeHub hub;
        final messages = _FakeMessagesClient(
          pages: [
            MessageListData(messages: [_message('msg-9')]),
          ],
        );
        final container = _container(
          chatsClient: _FakeChatsClient(),
          messagesClient: messages,
          realtimeHubBuilder: (ref) => hub = _FakeRealtimeHub(ref),
        );
        addTearDown(container.dispose);

        final sub = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-1'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(sub.close);
        await pumpEventQueue();

        expect(messages.markReadCalls.single.messageId, 'msg-9');
        expect(hub.markReadCalls.single, ('chat-1', 'msg-9'));
      },
    );
  });
}

ProviderContainer _container({
  required VoiceChatsClient chatsClient,
  required VoiceMessagesClient messagesClient,
  required RealtimeHub Function(Ref ref) realtimeHubBuilder,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async {
          return http.Response('{}', 404);
        }),
      ),
      voiceChatsClientProvider.overrideWithValue(chatsClient),
      voiceMessagesClientProvider.overrideWithValue(messagesClient),
      realtimeHubProvider.overrideWith(realtimeHubBuilder),
      messageCacheStoreProvider.overrideWithValue(InMemoryMessageCacheStore()),
      isDeviceOfflineProvider.overrideWith((ref) => false),
    ],
  );
}

VoiceMessage _message(String id) {
  return VoiceMessage(
    id: id,
    chatId: 'chat-1',
    senderProfileId: 'peer-1',
    content: 'message $id',
    createdAt: DateTime.parse(
      '2024-01-01T00:00:00Z',
    ).add(Duration(seconds: int.parse(id.split('-').last))),
  );
}

class _ChatListCall {
  const _ChatListCall(this.cursor);
  final String? cursor;
}

class _FakeChatsClient extends VoiceChatsClient {
  _FakeChatsClient({List<ChatListData> pages = const []})
    : _pages = [...pages],
      super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final List<ChatListData> _pages;
  final calls = <_ChatListCall>[];
  final createGroupCalls = <String>[];
  final addMembersCalls = <(String, List<String>)>[];

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
  }) async {
    calls.add(_ChatListCall(cursor));
    if (_pages.isEmpty) {
      return const ChatsApiOk(ChatListData(items: []));
    }
    return ChatsApiOk(_pages.removeAt(0));
  }

  @override
  Future<ChatsApiResult<VoiceChat>> createGroup({
    required String authorization,
    required String name,
  }) async {
    createGroupCalls.add(name);
    return ChatsApiOk(
      VoiceChat(
        id: 'group-created',
        type: 'CHAT_TYPE_GROUP',
        creatorProfileId: 'prof-1',
        name: name,
      ),
    );
  }

  @override
  Future<ChatsApiResult<void>> addGroupMembers({
    required String authorization,
    required String chatId,
    required List<String> profileIds,
  }) async {
    addMembersCalls.add((chatId, profileIds));
    return const ChatsApiOk(null);
  }
}

class _MessageGetCall {
  const _MessageGetCall({this.afterMessageId, this.lastMessageId, this.cursor});

  final String? afterMessageId;
  final String? lastMessageId;
  final String? cursor;
}

class _MarkReadCall {
  const _MarkReadCall({required this.chatId, required this.messageId});

  final String chatId;
  final String messageId;
}

class _FakeMessagesClient extends VoiceMessagesClient {
  _FakeMessagesClient({List<MessageListData> pages = const []})
    : _pages = [...pages],
      super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final List<MessageListData> _pages;
  final getCalls = <_MessageGetCall>[];
  final markReadCalls = <_MarkReadCall>[];

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) async {
    getCalls.add(
      _MessageGetCall(
        afterMessageId: afterMessageId,
        lastMessageId: lastMessageId,
        cursor: cursor,
      ),
    );
    if (_pages.isEmpty) {
      return const MessagesApiOk(MessageListData(messages: []));
    }
    return MessagesApiOk(_pages.removeAt(0));
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async {
    markReadCalls.add(
      _MarkReadCall(chatId: chatId, messageId: lastReadMessageId),
    );
    return const MessagesApiOk(null);
  }
}

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub(super.ref);

  final _events = StreamController<RealtimeFrame>.broadcast();
  final subscribedChats = <String>[];
  final markReadCalls = <(String, String)>[];

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {
    subscribedChats.add(chatId);
  }

  @override
  void markRead(String chatId, String messageId) {
    markReadCalls.add((chatId, messageId));
  }

  void addFrame(RealtimeFrame frame) {
    _events.add(frame);
  }

  @override
  Future<void> dispose() async {
    await _events.close();
  }
}
