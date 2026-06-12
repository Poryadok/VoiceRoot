import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';

void main() {
  group('ChatRoomController offline cache', () {
    late InMemoryMessageCacheStore cache;

    setUp(() {
      cache = InMemoryMessageCacheStore();
    });

    test('serves cached messages when device is offline', () async {
      await cache.replaceChatMessages(
        profileId: 'prof-test',
        chatId: 'chat-1',
        messages: [_message('cached-1')],
      );

      final container = _container(
        cache: cache,
        offline: true,
        messagesClient: _OfflineFailingMessagesClient(),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.messages.single.id, 'cached-1');
      expect(state.isOfflineCache, isTrue);
      expect(state.errorMessage, isNull);
    });

    test('falls back to cache on network_error from API', () async {
      await cache.replaceChatMessages(
        profileId: 'prof-test',
        chatId: 'chat-1',
        messages: [_message('cached-2')],
      );

      final container = _container(
        cache: cache,
        messagesClient: _NetworkErrorMessagesClient(),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.messages.single.id, 'cached-2');
      expect(state.isOfflineCache, isTrue);
    });

    test('writes through to cache after successful online load', () async {
      final container = _container(
        cache: cache,
        messagesClient: _FakeMessagesClient(
          pages: [MessageListData(messages: [_message('live-1')])],
        ),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      final cached = await cache.getMessages(
        profileId: 'prof-test',
        chatId: 'chat-1',
      );
      expect(cached.single.id, 'live-1');
    });

    test('blocks send while offline', () async {
      final container = _container(
        cache: cache,
        offline: true,
        messagesClient: _FakeMessagesClient(),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      final err = await container
          .read(chatRoomControllerProvider('chat-1').notifier)
          .sendMessage('hello');
      expect(err, kChatOfflineBlockedError);
    });

    test('isolates cache by active profile', () async {
      await cache.replaceChatMessages(
        profileId: 'prof-test',
        chatId: 'chat-1',
        messages: [_message('mine')],
      );
      await cache.replaceChatMessages(
        profileId: 'prof-other',
        chatId: 'chat-1',
        messages: [_message('theirs')],
      );

      final container = _container(
        cache: cache,
        offline: true,
        messagesClient: _OfflineFailingMessagesClient(),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      expect(
        container.read(chatRoomControllerProvider('chat-1')).messages.single.id,
        'mine',
      );
    });

    test('clears cache on logout via lifecycle provider', () async {
      await cache.replaceChatMessages(
        profileId: 'prof-test',
        chatId: 'chat-1',
        messages: [_message('cached-logout')],
      );

      final container = _container(
        cache: cache,
        messagesClient: _FakeMessagesClient(),
      );
      addTearDown(container.dispose);
      container.read(messageCacheLifecycleProvider);

      await container.read(authControllerProvider.notifier).logout();
      await pumpEventQueue();

      expect(
        await cache.getMessages(profileId: 'prof-test', chatId: 'chat-1'),
        isEmpty,
      );
    });

    test('shows error when offline and cache is empty', () async {
      final container = _container(
        cache: cache,
        offline: true,
        messagesClient: _OfflineFailingMessagesClient(),
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('chat-1'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      final state = container.read(chatRoomControllerProvider('chat-1'));
      expect(state.messages, isEmpty);
      expect(state.isOfflineCache, isFalse);
    });
  });
}

ProviderContainer _container({
  required InMemoryMessageCacheStore cache,
  required VoiceMessagesClient messagesClient,
  bool offline = false,
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
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceMessagesClientProvider.overrideWithValue(messagesClient),
      realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
      messageCacheStoreProvider.overrideWithValue(cache),
      isDeviceOfflineProvider.overrideWith((ref) => offline),
    ],
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

class _FakeMessagesClient extends VoiceMessagesClient {
  _FakeMessagesClient({List<MessageListData> pages = const []})
    : _pages = [...pages],
      super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final List<MessageListData> _pages;

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
    if (_pages.isEmpty) {
      return const MessagesApiOk(MessageListData(messages: []));
    }
    return MessagesApiOk(_pages.removeAt(0));
  }
}

class _NetworkErrorMessagesClient extends VoiceMessagesClient {
  _NetworkErrorMessagesClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

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
    return const MessagesApiFailure(
      message: 'network down',
      errorCode: 'network_error',
    );
  }
}

class _OfflineFailingMessagesClient extends _NetworkErrorMessagesClient {}

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
