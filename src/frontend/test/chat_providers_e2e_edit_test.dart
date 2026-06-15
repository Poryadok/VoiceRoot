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
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/e2e_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart' show gatewayHttpForTest;

void main() {
  test('editMessage re-encrypts E2E content before API call', () async {
    String? editedContent;
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
          _TrackingEditMessagesClient(onEdit: (content) => editedContent = content),
        ),
        realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
        messageCacheStoreProvider.overrideWithValue(
          InMemoryMessageCacheStore(),
        ),
        isDeviceOfflineProvider.overrideWith((ref) => false),
        chatListControllerProvider.overrideWith(_E2eEnabledChatListController.new),
        dmPeerProfileByChatIdProvider.overrideWith(
          (ref) => {'e2e-chat-edit': 'peer-b'},
        ),
        e2eMessageServiceProvider.overrideWithValue(
          _EncryptingE2eMessageService(),
        ),
      ],
    );
    addTearDown(container.dispose);

    final notifier = container.read(chatRoomControllerProvider('e2e-chat-edit').notifier);
    notifier.state = ChatRoomState(
      messages: const [
        VoiceMessage(
          id: 'msg-e2e-1',
          chatId: 'e2e-chat-edit',
          senderProfileId: 'prof-test',
          content: 'cipher-v1',
          isE2e: true,
        ),
      ],
    );

    final err = await notifier.editMessage('msg-e2e-1', 'edited-plain');
    expect(err, isNull);
    expect(editedContent, 'encrypted:edited-plain');
  });
}

class _E2eEnabledChatListController extends ChatListController {
  _E2eEnabledChatListController(super.ref) : super() {
    state = const ChatListState(
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'e2e-chat-edit',
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

class _EncryptingE2eMessageService extends E2eMessageService {
  _EncryptingE2eMessageService() : super();

  @override
  Future<String> encryptOutgoing({
    required String localProfileId,
    required String peerProfileId,
    required String plaintext,
    String? authorization,
    String? chatId,
  }) async {
    return 'encrypted:$plaintext';
  }
}

class _TrackingEditMessagesClient extends VoiceMessagesClient {
  _TrackingEditMessagesClient({required this.onEdit})
      : super(
          gateway: gatewayHttpForTest(
            MockClient((_) async => http.Response(
              '{"message":{"id":"msg-e2e-1","content":"encrypted:edited-plain","is_e2e":true}}',
              200,
              headers: {'content-type': 'application/json'},
            )),
          ),
        );

  final void Function(String content) onEdit;

  @override
  Future<MessagesApiResult<VoiceMessage>> editMessage({
    required String authorization,
    required String messageId,
    required String content,
  }) async {
    onEdit(content);
    return MessagesApiOk(
      VoiceMessage(
        id: messageId,
        chatId: 'e2e-chat-edit',
        senderProfileId: 'prof-test',
        content: content,
        isE2e: true,
      ),
    );
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
