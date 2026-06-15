import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/message_cache/in_memory_message_cache_store.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/e2e/e2e_exceptions.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/e2e_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart' show gatewayHttpForTest;

/// Batch E2E-A red test: encrypt failure must not fall back to plaintext API send.
void main() {
  group('ChatRoomController E2E send', () {
    test('encrypt failure returns error and does not call messages API', () async {
      var sendCalled = false;
      final container = ProviderContainer(
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
          voiceMessagesClientProvider.overrideWithValue(
            _TrackingMessagesClient(onSend: () => sendCalled = true),
          ),
          realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
          messageCacheStoreProvider.overrideWithValue(
            InMemoryMessageCacheStore(),
          ),
          isDeviceOfflineProvider.overrideWith((ref) => false),
          chatListControllerProvider.overrideWith(_E2eEnabledChatListController.new),
          dmPeerProfileByChatIdProvider.overrideWith(
            (ref) => {'e2e-chat-send': 'peer-b'},
          ),
          e2eMessageServiceProvider.overrideWithValue(
            _FailingE2eMessageService(),
          ),
        ],
      );
      addTearDown(container.dispose);

      final sub = container.listen<ChatRoomState>(
        chatRoomControllerProvider('e2e-chat-send'),
        (_, _) {},
        fireImmediately: true,
      );
      addTearDown(sub.close);
      await pumpEventQueue();

      final err = await container
          .read(chatRoomControllerProvider('e2e-chat-send').notifier)
          .sendMessage('secret');

      expect(err, isNotNull);
      expect(sendCalled, isFalse);
      final state = container.read(chatRoomControllerProvider('e2e-chat-send'));
      expect(state.isSending, isFalse);
      expect(state.errorMessage, isNotNull);
    });
  });
}

class _E2eEnabledChatListController extends ChatListController {
  _E2eEnabledChatListController(super.ref) : super() {
    state = const ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'e2e-chat-send',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            e2eEnabled: true,
          ),
          dmPeerProfileId: 'peer-b',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _FailingE2eMessageService extends E2eMessageService {
  _FailingE2eMessageService() : super();

  @override
  Future<String> encryptOutgoing({
    required String localProfileId,
    required String peerProfileId,
    required String plaintext,
    String? authorization,
    String? chatId,
  }) async {
    throw E2eEncryptException('e2e_encrypt_failed');
  }
}

class _TrackingMessagesClient extends VoiceMessagesClient {
  _TrackingMessagesClient({required this.onSend})
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final VoidCallback onSend;

  @override
  Future<MessagesApiResult<VoiceMessage>> sendMessage({
    required String authorization,
    required String chatId,
    required String content,
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? threadParentId,
    String? clientMessageId,
    bool isE2e = false,
  }) async {
    onSend();
    return const MessagesApiFailure(message: 'should not be called');
  }
}

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

typedef VoidCallback = void Function();
