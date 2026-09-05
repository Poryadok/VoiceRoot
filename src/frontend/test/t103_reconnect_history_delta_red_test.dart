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
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/gateway_test_client.dart';

/// T103 RED regression for the A1 reconnect/history boundary.
///
/// The test uses the real [RealtimeHub] and [ChatRoomController] lifecycle.
/// Closing the first transport stream schedules the Hub timer. Its retry keeps
/// the link in `reconnecting` until the accepted `hello` emits `connected`, so
/// the selected room requests exactly one REST delta with `last_message_id`;
/// a mounted passive room remains history-silent.
void main() {
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
  _ReconnectHistoryHarness() {
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
  late final AuthController auth;
  late final ProviderContainer container;
  late final RealtimeHub hub;
  ProviderSubscription<ChatRoomState>? _selectedRoom;
  ProviderSubscription<ChatRoomState>? _passiveRoom;

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

  Future<void> dispose() async {
    _selectedRoom?.close();
    _passiveRoom?.close();
    container.dispose();
    await transport.dispose();
  }
}

class _MessagesScript extends VoiceMessagesClient {
  _MessagesScript()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

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
  }) async {
    final call = _GetMessagesCall(
      authorization: authorization,
      chatId: chatId,
      lastMessageId: lastMessageId,
    );
    calls.add(call);
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
  }) async => const MessagesApiOk(null);
}

VoiceMessage _message(String id) => VoiceMessage(
  id: id,
  chatId: 'chat-selected',
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
