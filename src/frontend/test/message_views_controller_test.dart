import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/message_cache_providers.dart';
import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

class _ViewsClient extends FakeVoiceMessagesClient {
  int calls = 0;
  final response = Completer<MessagesApiResult<int>>();
  @override
  Future<MessagesApiResult<int>> recordMessageView({
    required String authorization,
    required String chatId,
    required String messageId,
  }) {
    calls++;
    return response.future;
  }
}

void main() {
  test(
    'late viewport response cannot update another profile history or cache',
    () async {
      final client = _ViewsClient();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          voiceMessagesClientProvider.overrideWithValue(client),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        chatRoomControllerProvider('chat'),
        (_, _) {},
      );
      addTearDown(subscription.close);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat';
      final controller = container.read(
        chatRoomControllerProvider('chat').notifier,
      );
      controller.state = const ChatRoomState(
        historyProfileId: 'prof-test',
        messages: [
          VoiceMessage(
            id: 'message',
            chatId: 'chat',
            senderProfileId: 'peer',
            content: 'hello',
            viewCount: 0,
          ),
        ],
      );
      final pending = controller.recordMessageView('message');
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'other-token',
          refreshToken: 'refresh',
          accountId: 'account',
          activeProfileId: 'other-profile',
          expiresInSeconds: 900,
        ),
      );
      client.response.complete(const MessagesApiOk(99));
      await pending;
      expect(controller.state.messages.single.viewCount, 0);
      final cached = await container
          .read(messageCacheStoreProvider)
          .getMessages(profileId: 'other-profile', chatId: 'chat');
      expect(cached, isEmpty);
    },
  );
  test(
    'viewport retries deduplicate in flight and preserve durable cache',
    () async {
      final client = _ViewsClient();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          voiceMessagesClientProvider.overrideWithValue(client),
        ],
      );
      addTearDown(container.dispose);
      final subscription = container.listen(
        chatRoomControllerProvider('chat'),
        (_, _) {},
      );
      addTearDown(subscription.close);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat';
      final controller = container.read(
        chatRoomControllerProvider('chat').notifier,
      );
      controller.state = const ChatRoomState(
        historyProfileId: 'prof-test',
        messages: [
          VoiceMessage(
            id: 'message',
            chatId: 'chat',
            senderProfileId: 'peer',
            content: 'hello',
            viewCount: 0,
          ),
        ],
      );
      final pending = controller.recordMessageView('message');
      await controller.recordMessageView('message');
      expect(client.calls, 1);
      client.response.complete(const MessagesApiOk(8));
      await pending;
      expect(controller.state.messages.single.viewCount, 8);
      await controller.recordMessageView('message');
      expect(client.calls, 1);
      final cached = await container
          .read(messageCacheStoreProvider)
          .getMessages(profileId: 'prof-test', chatId: 'chat');
      expect(cached.single.viewCount, 8);
      controller.state = controller.state.copyWith(isOfflineCache: true);
      await controller.recordMessageView('another');
      expect(client.calls, 1);
    },
  );
}
