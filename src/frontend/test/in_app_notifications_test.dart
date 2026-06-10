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
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/in_app_notifications.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';

/// Unit tests for Phase 4 in-app notifications (sound + badge, no FCM).
///
/// Production: [InAppNotificationController] in `lib/state/in_app_notifications.dart`.
void main() {
  group('InAppNotificationController', () {
    test('notification for non-selected chat bumps unread and plays sound', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();

      container.read(selectedChatIdProvider.notifier).state = 'chat-open';
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'new_message',
            'chat_id': 'chat-other',
            'message_id': 'msg-1',
            'sender_profile_id': 'peer-1',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 1);
      expect(sound.newMessagePlays, 1);
    });

    test('message_create alone does not bump unread (notification is canonical)', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat-open';
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'message_create',
          data: {
            'chat_id': 'chat-other',
            'message_id': 'msg-2',
            'sender_profile_id': 'peer-1',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
      expect(sound.newMessagePlays, 0);
    });

    test('notification then message_create does not double bump', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat-open';
      container.read(inAppNotificationControllerProvider);

      const payload = {
        'chat_id': 'chat-other',
        'message_id': 'msg-dedupe',
        'sender_profile_id': 'peer-1',
      };
      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {'type': 'new_message', ...payload},
        ),
      );
      hub.emit(const RealtimeFrame(op: 'message_create', data: payload));
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 1);
      expect(sound.newMessagePlays, 1);
    });

    test('own messages do not bump unread or play sound', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'message_create',
          data: {
            'chat_id': 'chat-other',
            'message_id': 'msg-own',
            'sender_profile_id': 'prof-test',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
      expect(sound.newMessagePlays, 0);
      expect(sound.reactionPlays, 0);
    });

    test('no sound when selected chat receives notification', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat-other';
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'new_message',
            'chat_id': 'chat-other',
            'message_id': 'msg-open',
            'sender_profile_id': 'peer-1',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
      expect(sound.newMessagePlays, 0);
    });

    test('mark_read WS event refreshes unread for that chat', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final chats = _FakeChatsClient(
        pages: [
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-other',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
                unreadCount: 2,
              ),
            ],
          ),
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-other',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
                unreadCount: 0,
              ),
            ],
          ),
        ],
      );
      final container = _container(sound: sound, hub: hub, chats: chats);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'mark_read',
          data: {'chat_id': 'chat-other', 'message_id': 'msg-read'},
        ),
      );
      await pumpEventQueue();

      expect(chats.calls, hasLength(2));
      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
    });

    test('reaction notification plays reaction sound when enabled', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat-open';
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'reaction',
            'chat_id': 'chat-other',
            'message_id': 'msg-react',
            'reactor_profile_id': 'peer-1',
            'emoji': '👍',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 1);
      expect(sound.reactionPlays, 1);
      expect(sound.newMessagePlays, 0);
    });

    test('own reaction does not bump unread or play sound', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'reaction',
            'chat_id': 'chat-other',
            'message_id': 'msg-own-react',
            'reactor_profile_id': 'prof-test',
            'emoji': '🔥',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
      expect(sound.reactionPlays, 0);
    });

    test('multiple rapid messages increment unread count', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(selectedChatIdProvider.notifier).state = 'chat-open';
      container.read(inAppNotificationControllerProvider);

      for (var i = 0; i < 3; i++) {
        hub.emit(
          RealtimeFrame(
            op: 'notification',
            data: {
              'type': 'new_message',
              'chat_id': 'chat-other',
              'message_id': 'msg-rapid-$i',
              'sender_profile_id': 'peer-1',
            },
          ),
        );
      }
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 3);
      expect(sound.newMessagePlays, 3);
    });

    test('mark_read does not bump unread or play sound', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final chats = _FakeChatsClient(
        pages: [
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-other',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
                unreadCount: 2,
              ),
            ],
          ),
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-other',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
                unreadCount: 0,
              ),
            ],
          ),
        ],
      );
      final container = _container(sound: sound, hub: hub, chats: chats);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'mark_read',
          data: {'chat_id': 'chat-other', 'message_id': 'msg-read'},
        ),
      );
      await pumpEventQueue();

      expect(sound.newMessagePlays, 0);
      expect(sound.reactionPlays, 0);
      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
    });

    test('notification with missing chat_id is ignored', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'new_message',
            'message_id': 'msg-no-chat',
            'sender_profile_id': 'peer-1',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 0);
      expect(sound.newMessagePlays, 0);
    });

    test('global mute disables sound but badge still updates', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(
        sound: sound,
        hub: hub,
        soundEnabled: false,
      );
      addTearDown(container.dispose);

      container.read(chatListControllerProvider);
      await pumpEventQueue();
      container.read(inAppNotificationControllerProvider);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'reaction',
            'chat_id': 'chat-other',
            'message_id': 'msg-r',
            'reactor_profile_id': 'peer-1',
            'emoji': '👍',
          },
        ),
      );
      await pumpEventQueue();

      final item = container
          .read(chatListControllerProvider)
          .items
          .firstWhere((row) => row.chatId == 'chat-other');
      expect(item.unreadCount, 1);
      expect(sound.reactionPlays, 0);
    });
  });
}

ProviderContainer _container({
  required _RecordingSoundPlayer sound,
  required _FakeRealtimeHub hub,
  _FakeChatsClient? chats,
  bool soundEnabled = true,
}) {
  final chatsClient =
      chats ??
      _FakeChatsClient(
        pages: [
          const ChatListData(
            items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-other',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-1',
                ),
              ),
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-open',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'peer-2',
                ),
              ),
            ],
          ),
        ],
      );

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
      voiceChatsClientProvider.overrideWithValue(chatsClient),
      voiceMessagesClientProvider.overrideWithValue(_FakeMessagesClient()),
      realtimeHubProvider.overrideWithValue(hub),
      notificationSoundPlayerProvider.overrideWithValue(sound),
      inAppNotificationsSoundEnabledProvider.overrideWith((ref) => soundEnabled),
    ],
  );
}

class _RecordingSoundPlayer implements NotificationSoundPlayer {
  var newMessagePlays = 0;
  var reactionPlays = 0;

  @override
  void playNewMessage() => newMessagePlays++;

  @override
  void playReaction() => reactionPlays++;
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

  void emit(RealtimeFrame frame) => _events.add(frame);

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
  _FakeChatsClient({List<ChatListData> pages = const []})
    : _pages = [...pages],
      super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final List<ChatListData> _pages;
  final calls = <String?>[];

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
  }) async {
    calls.add(cursor);
    if (_pages.isEmpty) {
      return const ChatsApiOk(ChatListData(items: []));
    }
    return ChatsApiOk(_pages.removeAt(0));
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
