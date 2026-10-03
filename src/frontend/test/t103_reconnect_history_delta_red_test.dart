import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

/// T103 RED regression for the A1 reconnect/history boundary.
///
/// The test uses the real [RealtimeHub] and [ChatRoomController] lifecycle.
/// Closing the first transport stream schedules the Hub timer. Its retry keeps
/// the link in `reconnecting` until the accepted `hello` emits `connected`, so
/// the selected room requests exactly one REST delta with `last_message_id`;
/// a mounted passive room remains history-silent.
void main() {
  test(
    'accepted reconnect composes paginated inbox retry with selected typed history and read cursors',
    () async {
      final chats = InboxReconcilerChatsFake(
        profileByAuthorization: const {
          'Bearer access-a': 'profile-a',
          'Bearer access-b': 'profile-b',
        },
      );
      _enqueueInboxPage(
        chats,
        inbox: 'main',
        items: [
          _chatItem('chat-selected', 'CHAT_TYPE_DM'),
          _chatItem('chat-group', 'CHAT_TYPE_GROUP'),
        ],
        nextCursor: 'main-page-2',
      );
      _enqueueInboxPage(
        chats,
        inbox: 'requests',
        items: [inboxChatItem('request-chat', inbox: 'requests')],
      );
      _enqueueInboxPage(
        chats,
        inbox: 'archive',
        items: [inboxChatItem('archive-chat', inbox: 'archive')],
      );
      chats
        ..enqueue(
          const InboxChatPageScript(
            inbox: 'main',
            cursor: 'main-page-2',
            profileId: 'profile-a',
            authorization: 'Bearer access-a',
            result: ChatsApiFailure(
              message: 'later page unavailable',
              statusCode: 503,
            ),
          ),
        )
        ..enqueue(
          InboxChatPageScript(
            inbox: 'main',
            cursor: 'main-page-2',
            profileId: 'profile-a',
            authorization: 'Bearer access-a',
            result: ChatsApiOk(
              ChatListData(
                items: [_chatItem('chat-channel', 'CHAT_TYPE_CHANNEL')],
              ),
            ),
          ),
        );

      final harness = _ReconnectHistoryHarness(chats: chats);
      addTearDown(harness.dispose);
      final reconciler = harness.container.read(
        inboxReconcilerProvider.notifier,
      );
      await harness.mountRooms();
      await harness.connectInitial();
      await pumpEventQueue();

      expect(chats.calls, isNotEmpty);
      expect(chats.unmatchedCalls, isEmpty);
      var profileSnapshot = harness.container
          .read(inboxReconcilerProvider)
          .profileSnapshots['profile-a']!;
      expect(
        profileSnapshot[InboxScope.main].items.map((item) => item.chatId),
        ['chat-selected', 'chat-group'],
      );
      expect(profileSnapshot[InboxScope.main].failedCursor, 'main-page-2');
      expect(profileSnapshot[InboxScope.main].errorStatusCode, 503);
      expect(
        profileSnapshot[InboxScope.requests].items.single.chatId,
        'request-chat',
      );
      expect(
        profileSnapshot[InboxScope.archive].items.single.chatId,
        'archive-chat',
      );

      harness.messages
        ..enqueueHistory(
          'chat-group',
          MessageListData(
            messages: [_message('group-baseline', chatId: 'chat-group')],
          ),
        )
        ..enqueueHistory(
          'chat-channel',
          MessageListData(
            messages: [_message('channel-baseline', chatId: 'chat-channel')],
          ),
        )
        ..enqueueHistory(
          'chat-channel',
          MessageListData(
            messages: [_message('channel-delta', chatId: 'chat-channel')],
          ),
        );
      harness.selectAndWatchRoom('chat-group');
      await pumpEventQueue();
      harness.selectAndWatchRoom('chat-channel');
      await pumpEventQueue();

      await reconciler.retry(InboxScope.main);
      profileSnapshot = harness.container
          .read(inboxReconcilerProvider)
          .profileSnapshots['profile-a']!;
      expect(
        profileSnapshot[InboxScope.main].items.map((item) => item.chatId),
        ['chat-selected', 'chat-group', 'chat-channel'],
      );
      expect(profileSnapshot[InboxScope.main].isComplete, isTrue);

      _enqueueInboxPage(
        chats,
        inbox: 'main',
        items: [
          _chatItem('chat-selected', 'CHAT_TYPE_DM'),
          _chatItem('chat-group', 'CHAT_TYPE_GROUP'),
        ],
        nextCursor: 'reconnect-main-page-2',
      );
      _enqueueInboxPage(
        chats,
        inbox: 'requests',
        items: [inboxChatItem('request-chat-refreshed', inbox: 'requests')],
      );
      _enqueueInboxPage(
        chats,
        inbox: 'archive',
        items: [inboxChatItem('archive-chat-refreshed', inbox: 'archive')],
      );
      chats.enqueue(
        InboxChatPageScript(
          inbox: 'main',
          cursor: 'reconnect-main-page-2',
          profileId: 'profile-a',
          authorization: 'Bearer access-a',
          manual: true,
          result: ChatsApiOk(
            ChatListData(
              items: [_chatItem('chat-channel', 'CHAT_TYPE_CHANNEL')],
            ),
          ),
        ),
      );
      await harness.connection(0).closeFrames();
      await harness.transport.waitForConnect(1);
      harness.connection(1).addHello();
      await pumpEventQueue();

      final pendingPage = chats.findCall(
        inbox: 'main',
        cursor: 'reconnect-main-page-2',
        profileId: 'profile-a',
        authorization: 'Bearer access-a',
      )!;
      expect(pendingPage.completed, isFalse);
      final callsDuringReconnect = chats.calls.length;
      final joinedRefresh = reconciler.reconcile();
      final coalescedRefresh = reconciler.reconcile();
      expect(identical(joinedRefresh, coalescedRefresh), isTrue);
      expect(chats.calls, hasLength(callsDuringReconnect));

      final reconnectHistory = harness.messages.calls.last;
      expect(reconnectHistory.chatId, 'chat-channel');
      expect(reconnectHistory.lastMessageId, 'channel-baseline');
      expect(
        harness.messages.calls.where((call) => call.chatId == 'chat-passive'),
        isEmpty,
      );
      expect(
        harness.messages.markReadCalls,
        containsAll([
          ('chat-selected', 'selected-baseline'),
          ('chat-group', 'group-baseline'),
          ('chat-channel', 'channel-baseline'),
          ('chat-channel', 'channel-delta'),
        ]),
      );
      profileSnapshot = harness.container
          .read(inboxReconcilerProvider)
          .profileSnapshots['profile-a']!;
      expect(
        profileSnapshot[InboxScope.main].items.map((item) => item.chatId),
        ['chat-selected', 'chat-group', 'chat-channel'],
      );
      expect(profileSnapshot[InboxScope.main].isLoading, isTrue);
      expect(
        profileSnapshot[InboxScope.requests].items.single.chatId,
        'request-chat-refreshed',
      );
      expect(
        profileSnapshot[InboxScope.archive].items.single.chatId,
        'archive-chat-refreshed',
      );

      final bPages = [
        ('main', [inboxChatItem('profile-b-main')]),
        ('requests', [inboxChatItem('profile-b-request', inbox: 'requests')]),
        ('archive', [inboxChatItem('profile-b-archive', inbox: 'archive')]),
      ];
      for (final (inbox, items) in bPages) {
        _enqueueInboxPage(
          chats,
          inbox: inbox,
          items: items,
          profileId: 'profile-b',
          authorization: 'Bearer access-b',
        );
      }
      harness.auth.state = const AuthState(
        session: AuthSession(
          accessToken: 'access-b',
          refreshToken: 'refresh-b',
          accountId: 'account-b',
          activeProfileId: 'profile-b',
          expiresInSeconds: 900,
        ),
      );
      final profileSwitch = harness.hub.reconnectWithNewSession();
      await harness.transport.waitForConnect(2);
      await profileSwitch;
      harness.connection(2).addHello();
      await pumpEventQueue();
      expect(
        harness.container.read(realtimeHelloBindingProvider)?.authorization,
        'Bearer access-b',
      );

      await chats.completeCall(
        pendingPage,
        result: ChatsApiOk(
          ChatListData(items: [_chatItem('chat-channel', 'CHAT_TYPE_CHANNEL')]),
        ),
      );
      await joinedRefresh;
      await pumpEventQueue();
      profileSnapshot = harness.container
          .read(inboxReconcilerProvider)
          .profileSnapshots['profile-b']!;
      expect(
        profileSnapshot[InboxScope.main].items.map((item) => item.chatId),
        ['profile-b-main'],
      );
      expect(profileSnapshot[InboxScope.main].isComplete, isTrue);
      expect(
        profileSnapshot[InboxScope.requests].items.single.chatId,
        'profile-b-request',
      );
      expect(
        profileSnapshot[InboxScope.archive].items.single.chatId,
        'profile-b-archive',
      );
      expect(chats.unmatchedCalls, isEmpty);
      expect(chats.pendingScripts, 0);
    },
  );

  test(
    'real transport reconnect catches up selected history once after accepted hello',
    () async {
      final harness = _ReconnectHistoryHarness();
      addTearDown(harness.dispose);

      await harness.mountRooms();
      await harness.connectInitial();

      expect(harness.messages.calls, hasLength(1));
      expect(harness.messages.calls.single, _call('chat-selected'));
      expect(
        harness.container
            .read(chatRoomControllerProvider('chat-selected'))
            .messages
            .map((message) => message.id),
        ['selected-baseline'],
      );
      expect(
        harness.container
            .read(chatRoomControllerProvider('chat-passive'))
            .messages,
        isEmpty,
      );
      final initialHello = harness.container.read(realtimeHelloBindingProvider);
      expect(initialHello?.profileId, 'profile-a');
      expect(initialHello?.authorization, 'Bearer access-a');

      await harness.connection(0).closeFrames();
      await harness.transport.waitForConnect(1);
      harness.connection(1).addHello();
      await pumpEventQueue();

      expect(harness.transport.sessions, [_session(), _session()]);
      expect(
        harness.container.read(authControllerProvider).session,
        _session(),
      );
      expect(
        harness.container.read(realtimeLinkStatusProvider),
        RealtimeLinkStatus.connected,
      );
      final reconnectHello = harness.container.read(
        realtimeHelloBindingProvider,
      );
      expect(reconnectHello?.profileId, 'profile-a');
      expect(reconnectHello?.authorization, 'Bearer access-a');
      expect(reconnectHello?.generation, greaterThan(initialHello!.generation));

      expect(harness.messages.calls, hasLength(2));
      expect(
        harness.messages.calls.last,
        _call('chat-selected', lastMessageId: 'selected-baseline'),
      );
      expect(
        harness.messages.calls.where((call) => call.chatId == 'chat-passive'),
        isEmpty,
      );
      expect(
        harness.container
            .read(chatRoomControllerProvider('chat-selected'))
            .messages
            .map((message) => message.id),
        ['selected-baseline', 'selected-delta'],
      );
    },
  );
}

AuthSession _session() => const AuthSession(
  accessToken: 'access-a',
  refreshToken: 'refresh-a',
  accountId: 'account-a',
  activeProfileId: 'profile-a',
  expiresInSeconds: 900,
);

_GetMessagesCall _call(String chatId, {String? lastMessageId}) =>
    _GetMessagesCall(
      authorization: 'Bearer access-a',
      chatId: chatId,
      lastMessageId: lastMessageId,
    );

class _ReconnectHistoryHarness {
  _ReconnectHistoryHarness({InboxReconcilerChatsFake? chats})
    : chats = chats ?? InboxReconcilerChatsFake() {
    auth = AuthController(
      authClient: VoiceAuthClient(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 404)),
        ),
      ),
      storage: InMemoryAuthSessionStorage(),
      guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
    )..state = AuthState(session: _session());
    container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith((_) => auth),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((_) async => http.Response('{}', 404)),
        ),
        gatewayHttpClientProvider.overrideWithValue(
          gatewayHttpForTest(MockClient((_) async => http.Response('{}', 404))),
        ),
        voiceMessagesClientProvider.overrideWithValue(messages),
        voiceChatsClientProvider.overrideWithValue(this.chats),
        chatListControllerProvider.overrideWith(_NoAutoChatListController.new),
        chatListProvider.overrideWith(
          (_) async => const ChatListData(items: []),
        ),
        messageCacheStoreProvider.overrideWithValue(
          InMemoryMessageCacheStore(),
        ),
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeTransportFactoryProvider.overrideWithValue(transport),
      ],
    );
    hub = container.read(realtimeHubProvider);
  }

  final transport = _ControlledTransportFactory();
  final messages = _MessagesScript();
  final InboxReconcilerChatsFake chats;
  late final AuthController auth;
  late final ProviderContainer container;
  late final RealtimeHub hub;
  ProviderSubscription<ChatRoomState>? _selectedRoom;
  ProviderSubscription<ChatRoomState>? _passiveRoom;
  final _otherRooms = <ProviderSubscription<ChatRoomState>>[];

  _ControlledConnection connection(int attempt) =>
      transport.connection(attempt);

  Future<void> connectInitial() async {
    final connecting = hub.ensureConnected();
    await transport.waitForConnect(0);
    await connecting;
    connection(0).addHello();
    await pumpEventQueue();
    expect(
      container.read(realtimeLinkStatusProvider),
      RealtimeLinkStatus.connected,
    );
  }

  Future<void> mountRooms() async {
    container.read(selectedChatIdProvider.notifier).state = 'chat-selected';
    _selectedRoom = container.listen<ChatRoomState>(
      chatRoomControllerProvider('chat-selected'),
      (_, _) {},
      fireImmediately: true,
    );
    _passiveRoom = container.listen<ChatRoomState>(
      chatRoomControllerProvider('chat-passive'),
      (_, _) {},
      fireImmediately: true,
    );
    await pumpEventQueue();
  }

  void selectAndWatchRoom(String chatId) {
    container.read(selectedChatIdProvider.notifier).state = chatId;
    _otherRooms.add(
      container.listen<ChatRoomState>(
        chatRoomControllerProvider(chatId),
        (_, _) {},
        fireImmediately: true,
      ),
    );
  }

  Future<void> dispose() async {
    _selectedRoom?.close();
    _passiveRoom?.close();
    for (final room in _otherRooms) {
      room.close();
    }
    container.dispose();
    await transport.dispose();
  }
}

class _NoAutoChatListController extends ChatListController {
  _NoAutoChatListController(super.ref);

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _MessagesScript extends VoiceMessagesClient {
  _MessagesScript()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final calls = <_GetMessagesCall>[];
  final markReadCalls = <(String, String)>[];
  final _historyPages = <String, List<MessageListData>>{};

  void enqueueHistory(String chatId, MessageListData page) {
    _historyPages.putIfAbsent(chatId, () => []).add(page);
  }

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
    final call = _GetMessagesCall(
      authorization: authorization,
      chatId: chatId,
      lastMessageId: lastMessageId,
    );
    calls.add(call);
    final pages = _historyPages[chatId];
    if (pages != null && pages.isNotEmpty) {
      return MessagesApiOk(pages.removeAt(0));
    }
    if (call == _call('chat-selected')) {
      return MessagesApiOk(
        MessageListData(messages: [_message('selected-baseline')]),
      );
    }
    if (call == _call('chat-selected', lastMessageId: 'selected-baseline')) {
      return MessagesApiOk(
        MessageListData(messages: [_message('selected-delta')]),
      );
    }
    return const MessagesApiOk(MessageListData(messages: []));
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async {
    markReadCalls.add((chatId, lastReadMessageId));
    return const MessagesApiOk(null);
  }
}

void _enqueueInboxPage(
  InboxReconcilerChatsFake chats, {
  required String inbox,
  required List<ChatListItem> items,
  String? nextCursor,
  String profileId = 'profile-a',
  String authorization = 'Bearer access-a',
}) {
  chats.enqueue(
    InboxChatPageScript(
      inbox: inbox,
      cursor: null,
      profileId: profileId,
      authorization: authorization,
      result: ChatsApiOk(ChatListData(items: items, nextCursor: nextCursor)),
    ),
  );
}

ChatListItem _chatItem(String id, String type) => ChatListItem(
  chat: VoiceChat(id: id, type: type, creatorProfileId: 'peer-$id'),
  inbox: 'main',
  unreadCount: 1,
);

VoiceMessage _message(String id, {String chatId = 'chat-selected'}) =>
    VoiceMessage(
      id: id,
      chatId: chatId,
      senderProfileId: 'peer-a',
      content: id,
      createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
    );

class _GetMessagesCall {
  const _GetMessagesCall({
    required this.authorization,
    required this.chatId,
    this.lastMessageId,
  });

  final String authorization;
  final String chatId;
  final String? lastMessageId;

  @override
  bool operator ==(Object other) =>
      other is _GetMessagesCall &&
      other.authorization == authorization &&
      other.chatId == chatId &&
      other.lastMessageId == lastMessageId;

  @override
  int get hashCode => Object.hash(authorization, chatId, lastMessageId);
}

class _ControlledTransportFactory implements RealtimeTransportFactory {
  final connections = <_ControlledConnection>[];
  final sessions = <AuthSession>[];
  final _connectStarted = <Completer<void>>[];

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    sessions.add(session);
    final connection = _ControlledConnection(connections.length);
    connections.add(connection);
    _connectStarted.add(connection.connectStarted);
    return connection;
  }

  _ControlledConnection connection(int attempt) => connections[attempt];

  Future<void> waitForConnect(int attempt) async {
    while (_connectStarted.length <= attempt) {
      await Future<void>.delayed(Duration.zero);
    }
    await _connectStarted[attempt].future;
  }

  Future<void> dispose() async {
    for (final connection in connections) {
      await connection.closeFrames();
    }
  }
}

class _ControlledConnection extends VoiceRealtimeConnection {
  _ControlledConnection(this.attempt)
    : super(uri: Uri.parse('ws://transport.test/ws'), headers: const {});

  final int attempt;
  final connectStarted = Completer<void>();
  final _frames = StreamController<RealtimeFrame>.broadcast(sync: true);

  @override
  Stream<RealtimeFrame> get events => _frames.stream;

  @override
  Future<void> connect() async {
    if (!connectStarted.isCompleted) connectStarted.complete();
  }

  @override
  Future<void> dispose() async {}

  void addHello() => _frames.add(const RealtimeFrame(op: 'hello', sequence: 1));

  Future<void> closeFrames() async {
    if (!_frames.isClosed) await _frames.close();
  }
}
