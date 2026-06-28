import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/in_app_notifications.dart';
import 'package:voice_frontend/state/shared_media_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';

void main() {
  group('push deep link navigation', () {
    test('navigateToChat selects chat and scrolls to message', () async {
      final container = _container();
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      container.read(inAppNotificationControllerProvider)!.onPushNotificationData(
        {
          'type': 'new_message',
          'deep_link': 'https://voice.gg/ch/c1/m/m1',
          'chat_id': 'c1',
          'message_id': 'm1',
          'sender_profile_id': 'peer-1',
        },
        navigateToChat: true,
      );
      await pumpEventQueue();

      expect(container.read(selectedChatIdProvider), 'c1');
      expect(container.read(pendingChatMessageScrollProvider('c1')), 'm1');
      expect(container.read(pendingChatMessageHighlightProvider('c1')), 'm1');
      expect(
        container.read(navigationSectionProvider),
        NavigationSection.chats,
      );
    });

    test('navigateToChat selects space chat', () async {
      final container = _container();
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      container.read(inAppNotificationControllerProvider)!.onPushNotificationData(
        {
          'type': 'new_message',
          'deep_link': 'https://voice.gg/s/space-1/c/c1',
          'chat_id': 'c1',
          'sender_profile_id': 'peer-1',
        },
        navigateToChat: true,
      );
      await pumpEventQueue();

      expect(container.read(selectedSpaceIdProvider), 'space-1');
      expect(container.read(selectedChatIdProvider), 'c1');
    });

    test('navigateToChat opens DM deep link', () async {
      final container = _container(
        chats: _FakeChatsClient(
          pages: [
            const ChatListData(
              items: [
                ChatListItem(
                  chat: VoiceChat(
                    id: 'dm-chat',
                    type: 'CHAT_TYPE_DM',
                    creatorProfileId: 'peer-1',
                  ),
                ),
              ],
            ),
          ],
          dmChatId: 'dm-chat',
        ),
      );
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      container.read(inAppNotificationControllerProvider)!.onPushNotificationData(
        {
          'type': 'new_message',
          'deep_link': 'https://voice.gg/dm/user-1',
          'chat_id': 'dm-chat',
          'sender_profile_id': 'peer-1',
        },
        navigateToChat: true,
      );
      await pumpEventQueue();

      expect(container.read(selectedChatIdProvider), 'dm-chat');
    });
  });
}

ProviderContainer _container({_FakeChatsClient? chats}) {
  final hub = _FakeRealtimeHub();
  final chatsClient =
      chats ??
      _FakeChatsClient(
        pages: [
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'c1',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
              ),
            ],
          ),
        ],
      );

  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chatsClient),
      voiceMessagesClientProvider.overrideWithValue(_FakeMessagesClient()),
      realtimeHubProvider.overrideWithValue(hub),
      notificationSoundPlayerProvider.overrideWithValue(
        const NoOpNotificationSoundPlayer(),
      ),
    ],
  );
}

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub() : super(_UnwiredRef());

  final _events = StreamController<RealtimeFrame>.broadcast();

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {
    await _events.close();
  }
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}

class _FakeChatsClient extends VoiceChatsClient {
  _FakeChatsClient({
    List<ChatListData> pages = const [],
    this.dmChatId = 'dm-chat',
  }) : _pages = [...pages],
       super(
         gateway: gatewayHttpForTest(
           MockClient((_) async => http.Response('{}', 500)),
         ),
       );

  final List<ChatListData> _pages;
  final String dmChatId;

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
  }) async {
    if (_pages.isEmpty) {
      return const ChatsApiOk(ChatListData(items: []));
    }
    return ChatsApiOk(_pages.removeAt(0));
  }

  @override
  Future<ChatsApiResult<VoiceChat>> createDm({
    required String authorization,
    required String otherProfileId,
  }) async {
    return ChatsApiOk(
      VoiceChat(
        id: dmChatId,
        type: 'CHAT_TYPE_DM',
        creatorProfileId: otherProfileId,
      ),
    );
  }
}

class _FakeMessagesClient extends VoiceMessagesClient {
  _FakeMessagesClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );
}
