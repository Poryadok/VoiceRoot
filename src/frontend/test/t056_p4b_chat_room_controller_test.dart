import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/message_cache/message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/gateway_test_client.dart';

void main() {
  group('T056 P4b ChatRoomController terminal DM state', () {
    test('REST DELETED preserves loaded rows and paging', () async {
      final cache = _RecordingCacheStore();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(
            ids: const ['msg-1'],
            cursor: 'cursor-older',
            hasMore: true,
            peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
          ),
        ],
      );
      final container = _container(messages: messages, cache: cache);
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isTrue);
      expect(state.messages.map((message) => message.id), ['msg-1']);
      expect(state.nextCursor, 'cursor-older');
      expect(state.hasMore, isTrue);
      expect(await cache.cachedIds(), ['msg-1']);
    });

    test('empty delta still processes response-level DELETED', () async {
      final hub = _TestRealtimeHub();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(ids: const ['msg-1']),
          _page(peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED),
        ],
      );
      final container = _container(
        messages: messages,
        cache: _RecordingCacheStore(),
        realtimeHub: hub,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      hub.addFrame(
        const RealtimeFrame(
          op: 'message_create',
          data: {'chat_id': 'chat-1', 'message_id': 'new-message'},
        ),
      );
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isTrue);
      expect(state.messages.map((message) => message.id), ['msg-1']);
      expect(messages.calls.last.afterMessageId, 'msg-1');
    });

    test('older DELETED does not change loaded history or cursor', () async {
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(ids: const ['msg-2'], cursor: 'cursor-current', hasMore: true),
          _page(
            ids: const ['msg-1'],
            cursor: 'cursor-should-not-apply',
            hasMore: false,
            peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
          ),
        ],
      );
      final container = _container(
        messages: messages,
        cache: _RecordingCacheStore(),
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      await container
          .read(chatRoomControllerProvider('chat-1').notifier)
          .loadOlderMessages();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isTrue);
      expect(state.messages.map((message) => message.id), ['msg-2']);
      expect(state.nextCursor, 'cursor-current');
      expect(state.hasMore, isTrue);
      expect(state.isLoadingOlder, isFalse);
    });

    test(
      'repeated REST and live deletion notifications are idempotent',
      () async {
        final hub = _TestRealtimeHub();
        final cache = _RecordingCacheStore();
        final messages = _ScriptedMessagesClient(
          pages: [
            _page(ids: const ['msg-1']),
            _page(
              ids: const ['msg-1'],
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
            _page(
              ids: const ['msg-1'],
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
          ],
        );
        final container = _container(
          messages: messages,
          cache: cache,
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        final subscription = await _loadRoom(container);
        addTearDown(subscription.close);
        final controller = container.read(
          chatRoomControllerProvider('chat-1').notifier,
        );
        await controller.loadInitial();
        await controller.loadInitial();
        await pumpEventQueue();
        final writesBeforeLive = cache.replaceCalls.length;

        for (var i = 0; i < 2; i++) {
          hub.addFrame(
            const RealtimeFrame(
              op: 'dm_peer_deleted',
              data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
            ),
          );
        }
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), ['msg-1']);
        expect(cache.replaceCalls.length, writesBeforeLive);
        expect(await cache.cachedIds(), ['msg-1']);
      },
    );

    test('cached history alone never infers deleted peer', () async {
      final cache = _RecordingCacheStore();
      await cache.replaceChatMessages(
        profileId: 'profile-a',
        chatId: 'chat-1',
        messages: [_message('cached-1')],
      );
      final messages = _ScriptedMessagesClient();
      final container = _container(
        messages: messages,
        cache: cache,
        offline: true,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.messages.map((message) => message.id), ['cached-1']);
      expect(state.isOfflineCache, isTrue);
      expect(state.isDmPeerDeleted, isFalse);
      expect(messages.calls, isEmpty);
    });

    test(
      'late profile-A DELETED response cannot affect profile-B room',
      () async {
        final auth = _MutableAuthController();
        final pending = Completer<MessagesApiResult<MessageListData>>();
        final messages = _ScriptedMessagesClient(pending: pending);
        final cache = _RecordingCacheStore();
        final container = _container(
          auth: auth,
          messages: messages,
          cache: cache,
        );
        addTearDown(container.dispose);

        final subscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('chat-1'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        await pumpEventQueue();

        final controller = container.read(
          chatRoomControllerProvider('chat-1').notifier,
        );
        auth.state = _authState('profile-b', 'access-b');
        controller.state = ChatRoomState(
          messages: [_message('profile-b-message')],
          nextCursor: 'profile-b-cursor',
          hasMore: true,
        );
        pending.complete(
          MessagesApiOk(
            _page(
              ids: const ['profile-a-message'],
              cursor: 'profile-a-cursor',
              hasMore: true,
              peerState: messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
            ),
          ),
        );
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isFalse);
        expect(state.messages.map((message) => message.id), [
          'profile-b-message',
        ]);
        expect(state.nextCursor, 'profile-b-cursor');
        expect(state.hasMore, isTrue);
        expect(cache.replaceCalls, isEmpty);
      },
    );

    test('old-profile and nonselected live events are ignored', () async {
      final hub = _TestRealtimeHub();
      final auth = _MutableAuthController();
      final messages = _ScriptedMessagesClient(
        pages: [
          _page(ids: const ['msg-1']),
        ],
      );
      final container = _container(
        auth: auth,
        messages: messages,
        cache: _RecordingCacheStore(),
        realtimeHub: hub,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      auth.state = _authState('profile-b', 'access-b');
      for (final frame in [
        const RealtimeFrame(
          op: 'dm_peer_deleted',
          data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
        ),
        const RealtimeFrame(
          op: 'dm_peer_deleted',
          data: {'chat_id': 'other-chat', 'recipient_profile_id': 'profile-b'},
        ),
        const RealtimeFrame(op: 'dm_peer_deleted', data: {'chat_id': 'chat-1'}),
      ]) {
        hub.addFrame(frame);
      }
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isFalse);
      expect(state.messages.map((message) => message.id), ['msg-1']);
    });

    test(
      'live DELETED requires current profile, chat, and loaded history',
      () async {
        final hub = _TestRealtimeHub();
        final messages = _ScriptedMessagesClient(
          pages: [
            _page(ids: const ['msg-1']),
          ],
        );
        final container = _container(
          messages: messages,
          cache: _RecordingCacheStore(),
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        final subscription = await _loadRoom(container);
        addTearDown(subscription.close);
        hub.addFrame(
          const RealtimeFrame(
            op: 'dm_peer_deleted',
            data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
          ),
        );
        await pumpEventQueue();

        final state = container.read(chatRoomControllerProvider('chat-1'));
        expect(state.isDmPeerDeleted, isTrue);
        expect(state.messages.map((message) => message.id), ['msg-1']);
      },
    );

    test('live DELETED is ignored before any history is loaded', () async {
      final hub = _TestRealtimeHub();
      final messages = _ScriptedMessagesClient(pages: [_page()]);
      final container = _container(
        messages: messages,
        cache: _RecordingCacheStore(),
        realtimeHub: hub,
      );
      addTearDown(container.dispose);

      final subscription = await _loadRoom(container);
      addTearDown(subscription.close);
      hub.addFrame(
        const RealtimeFrame(
          op: 'dm_peer_deleted',
          data: {'chat_id': 'chat-1', 'recipient_profile_id': 'profile-a'},
        ),
      );
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.isDmPeerDeleted, isFalse);
      expect(state.messages, isEmpty);
    });
  });
}

ProviderContainer _container({
  required _ScriptedMessagesClient messages,
  required _RecordingCacheStore cache,
  _MutableAuthController? auth,
  _TestRealtimeHub? realtimeHub,
  bool offline = false,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(
        (ref) => auth ?? _MutableAuthController(),
      ),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceMessagesClientProvider.overrideWithValue(messages),
      realtimeHubProvider.overrideWithValue(realtimeHub ?? _TestRealtimeHub()),
      messageCacheStoreProvider.overrideWithValue(cache),
      isDeviceOfflineProvider.overrideWith((ref) => offline),
    ],
  );
}

Future<ProviderSubscription<ChatRoomState>> _loadRoom(
  ProviderContainer container,
) async {
  final subscription = container.listen<ChatRoomState>(
    chatRoomControllerProvider('chat-1'),
    (_, _) {},
    fireImmediately: true,
  );
  await pumpEventQueue();
  return subscription;
}

MessageListData _page({
  List<String> ids = const [],
  String? cursor,
  bool hasMore = false,
  messaging_pb.DmPeerState? peerState,
}) {
  return MessageListData(
    messages: ids.map(_message).toList(growable: false),
    nextCursor: cursor,
    hasMore: hasMore,
    dmPeerState: peerState,
  );
}

VoiceMessage _message(String id) {
  return VoiceMessage(
    id: id,
    chatId: 'chat-1',
    senderProfileId: 'peer-1',
    content: 'message $id',
    createdAt: DateTime.parse('2024-01-01T00:00:00Z'),
  );
}

AuthState _authState(String profileId, String accessToken) {
  return AuthState(
    session: AuthSession(
      accessToken: accessToken,
      refreshToken: 'refresh-$profileId',
      accountId: 'account-1',
      activeProfileId: profileId,
      expiresInSeconds: 900,
    ),
  );
}

class _MutableAuthController extends AuthController {
  _MutableAuthController()
    : super(
        authClient: VoiceAuthClient(
          gateway: gatewayHttpForTest(
            MockClient((_) async => http.Response('{}', 404)),
          ),
        ),
        storage: InMemoryAuthSessionStorage(),
        guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
      ) {
    state = _authState('profile-a', 'access-a');
  }
}

class _ScriptedMessagesClient extends VoiceMessagesClient {
  _ScriptedMessagesClient({
    List<MessageListData> pages = const [],
    this.pending,
  }) : _pages = [...pages],
       super(
         gateway: gatewayHttpForTest(
           MockClient((_) async => http.Response('{}', 500)),
         ),
       );

  final List<MessageListData> _pages;
  final Completer<MessagesApiResult<MessageListData>>? pending;
  final calls = <_GetMessagesCall>[];

  @override
  Future<MessagesApiResult<MessageListData>> getMessages({
    required String authorization,
    required String chatId,
    String? afterMessageId,
    String? beforeMessageId,
    String? lastMessageId,
    String? cursor,
    int? pageSize,
  }) {
    calls.add(
      _GetMessagesCall(
        authorization: authorization,
        chatId: chatId,
        afterMessageId: afterMessageId,
        lastMessageId: lastMessageId,
        cursor: cursor,
      ),
    );
    if (pending != null) return pending!.future;
    if (_pages.isEmpty) {
      return Future.value(const MessagesApiOk(MessageListData(messages: [])));
    }
    return Future.value(MessagesApiOk(_pages.removeAt(0)));
  }

  @override
  Future<MessagesApiResult<void>> markRead({
    required String authorization,
    required String chatId,
    required String lastReadMessageId,
  }) async => const MessagesApiOk(null);
}

class _GetMessagesCall {
  const _GetMessagesCall({
    required this.authorization,
    required this.chatId,
    this.afterMessageId,
    this.lastMessageId,
    this.cursor,
  });

  final String authorization;
  final String chatId;
  final String? afterMessageId;
  final String? lastMessageId;
  final String? cursor;
}

class _TestRealtimeHub extends RealtimeHub {
  _TestRealtimeHub() : super(_UnwiredRef());

  final _events = StreamController<RealtimeFrame>.broadcast();

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  void addFrame(RealtimeFrame frame) => _events.add(frame);

  @override
  Future<void> dispose() async => _events.close();
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}

class _CacheWrite {
  const _CacheWrite({
    required this.profileId,
    required this.chatId,
    required this.messages,
  });

  final String profileId;
  final String chatId;
  final List<VoiceMessage> messages;
}

class _RecordingCacheStore implements MessageCacheStore {
  final _delegate = InMemoryMessageCacheStore();
  final replaceCalls = <_CacheWrite>[];

  Future<List<String>> cachedIds() async => (await _delegate.getMessages(
    profileId: 'profile-a',
    chatId: 'chat-1',
  )).map((message) => message.id).toList(growable: false);

  @override
  Future<void> clearAll() => _delegate.clearAll();

  @override
  Future<void> clearProfile(String profileId) =>
      _delegate.clearProfile(profileId);

  @override
  Future<List<VoiceMessage>> getMessages({
    required String profileId,
    required String chatId,
  }) => _delegate.getMessages(profileId: profileId, chatId: chatId);

  @override
  Future<void> replaceChatMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    replaceCalls.add(
      _CacheWrite(
        profileId: profileId,
        chatId: chatId,
        messages: List<VoiceMessage>.unmodifiable(messages),
      ),
    );
    await _delegate.replaceChatMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
  }

  @override
  Future<void> upsertMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) => _delegate.upsertMessages(
    profileId: profileId,
    chatId: chatId,
    messages: messages,
  );
}
