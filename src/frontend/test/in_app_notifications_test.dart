import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/in_app_notifications.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/message_requests_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

/// Unit tests for in-app notifications (sound + badge, no FCM).
///
/// Production: [InAppNotificationController] in `lib/state/in_app_notifications.dart`.
void main() {
  group('InAppNotificationController', () {
    test(
      'friend_request refreshes incoming invitations without reload',
      () async {
        final hub = _FakeRealtimeHub();
        final friends = _RequestFriendsClient();
        final container = _container(
          sound: _RecordingSoundPlayer(),
          hub: hub,
          friends: friends,
        );
        addTearDown(container.dispose);
        final subscription = container.listen(
          friendRequestsProvider,
          (_, _) {},
        );
        addTearDown(subscription.close);
        container.read(inAppNotificationControllerProvider);
        expect(
          (await container.read(friendRequestsProvider.future)).incoming,
          isEmpty,
        );

        friends.requestVisible = true;
        hub.emit(
          const RealtimeFrame(
            op: 'notification',
            data: {
              'type': 'friend_request',
              'friend_request_id': 'request-1',
              'sender_profile_id': 'peer-1',
            },
          ),
        );
        await pumpEventQueue();

        expect((await container.read(friendRequestsProvider.future)).incoming, [
          'peer-1',
        ]);
      },
    );

    test('friend_removed refreshes the friend list without reload', () async {
      final hub = _FakeRealtimeHub();
      final friends = _RequestFriendsClient()..friendsVisible = true;
      final container = _container(
        sound: _RecordingSoundPlayer(),
        hub: hub,
        friends: friends,
      );
      addTearDown(container.dispose);
      final subscription = container.listen(friendsListProvider, (_, _) {});
      addTearDown(subscription.close);
      container.read(inAppNotificationControllerProvider);

      expect((await container.read(friendsListProvider.future)).friends, [
        'peer-1',
      ]);
      friends.friendsVisible = false;
      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {'type': 'friend_removed', 'friend_profile_id': 'peer-1'},
        ),
      );
      await pumpEventQueue();

      expect(
        (await container.read(friendsListProvider.future)).friends,
        isEmpty,
      );
      expect(friends.friendListCalls, greaterThanOrEqualTo(2));
    });

    test('reconnect refreshes friend requests missed while offline', () async {
      final hub = _FakeRealtimeHub();
      final friends = _RequestFriendsClient();
      final container = _container(
        sound: _RecordingSoundPlayer(),
        hub: hub,
        friends: friends,
      );
      addTearDown(container.dispose);
      final subscription = container.listen(friendRequestsProvider, (_, _) {});
      addTearDown(subscription.close);
      container.read(inAppNotificationControllerProvider);
      expect(
        (await container.read(friendRequestsProvider.future)).incoming,
        isEmpty,
      );

      friends.requestVisible = true;
      final session = container.read(authControllerProvider).session!;
      container
          .read(realtimeHelloBindingProvider.notifier)
          .state = RealtimeHelloBinding(
        generation: 1,
        bindingGeneration: 1,
        profileId: session.activeProfileId,
        authorization: session.authorizationHeader,
      );
      await pumpEventQueue();

      expect((await container.read(friendRequestsProvider.future)).incoming, [
        'peer-1',
      ]);
    });

    test('message_request updates the requests inbox and in-app row', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final chats = _RequestChatsClient();
      final container = _container(sound: sound, hub: hub, chats: chats);
      addTearDown(container.dispose);

      container.read(inAppNotificationControllerProvider);
      expect(
        (await container.read(
          messageRequestsSummaryProvider.future,
        )).pendingCount,
        0,
      );
      await container.read(inboxReconcilerProvider.notifier).reconcile();
      chats.requestVisible = true;

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'message_request',
            'chat_id': 'new-request',
            'message_id': 'first-message',
            'sender_profile_id': 'peer-1',
          },
        ),
      );
      await pumpEventQueue();

      expect(
        (await container.read(
          messageRequestsSummaryProvider.future,
        )).pendingCount,
        1,
      );
      expect(
        container
            .read(inboxReconcilerProvider)
            .snapshotFor('prof-test')![InboxScope.requests]
            .items
            .map((item) => item.chatId),
        contains('new-request'),
      );
      expect(
        container.read(inAppNotificationCenterProvider).items.single.type,
        'message_request',
      );
    });

    test(
      'chat_update reveals a newly created DM request without reload',
      () async {
        final hub = _FakeRealtimeHub();
        final chats = _RequestChatsClient();
        final container = _container(
          sound: _RecordingSoundPlayer(),
          hub: hub,
          chats: chats,
        );
        addTearDown(container.dispose);
        container.read(inAppNotificationControllerProvider);
        expect(
          (await container.read(
            messageRequestsSummaryProvider.future,
          )).pendingCount,
          0,
        );
        chats.requestVisible = true;
        hub.emit(
          const RealtimeFrame(
            op: 'chat_update',
            data: {
              'chat_id': 'new-request',
              'profile_id': 'prof-test',
              'change': 'joined',
            },
          ),
        );
        await pumpEventQueue();
        expect(
          (await container.read(
            messageRequestsSummaryProvider.future,
          )).pendingCount,
          1,
        );
        expect(
          container
              .read(inboxReconcilerProvider)
              .snapshotFor('prof-test')![InboxScope.requests]
              .items
              .single
              .chatId,
          'new-request',
        );
      },
    );

    test(
      'inbox bucket update removes accepted DM request without reload',
      () async {
        final hub = _FakeRealtimeHub();
        final chats = _RequestChatsClient()..requestVisible = true;
        final container = _container(
          sound: _RecordingSoundPlayer(),
          hub: hub,
          chats: chats,
        );
        addTearDown(container.dispose);
        container.read(inAppNotificationControllerProvider);
        expect(
          (await container.read(
            messageRequestsSummaryProvider.future,
          )).pendingCount,
          1,
        );
        chats.requestVisible = false;
        chats.mainVisible = true;
        hub.emit(
          const RealtimeFrame(
            op: 'chat_update',
            data: {
              'chat_id': 'new-request',
              'profile_id': 'prof-test',
              'change': 'inbox_bucket_changed',
            },
          ),
        );
        await pumpEventQueue();
        expect(
          (await container.read(
            messageRequestsSummaryProvider.future,
          )).pendingCount,
          0,
        );
        expect(
          container
              .read(inboxReconcilerProvider)
              .snapshotFor('prof-test')![InboxScope.main]
              .items
              .single
              .chatId,
          'new-request',
        );
        expect(
          container.read(chatListControllerProvider).items.single.chatId,
          'new-request',
        );
      },
    );

    test(
      'notification for non-selected chat bumps unread and plays sound',
      () async {
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
      },
    );

    test(
      'live message and read events reconcile the rendered inbox snapshot',
      () async {
        final sound = _RecordingSoundPlayer();
        final hub = _FakeRealtimeHub();
        final chats = _MutableInboxChatsClient();
        final container = _container(sound: sound, hub: hub, chats: chats);
        addTearDown(container.dispose);

        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();
        container.read(inAppNotificationControllerProvider);
        expect(
          container
              .read(inboxReconcilerProvider)
              .snapshotFor('prof-test')![InboxScope.main]
              .items
              .single
              .lastMessagePreview,
          'before',
        );

        chats.showLatestMessage = true;
        hub.emit(
          const RealtimeFrame(
            op: 'notification',
            data: {
              'type': 'new_message',
              'chat_id': 'chat-other',
              'message_id': 'msg-new',
              'sender_profile_id': 'peer-1',
            },
          ),
        );
        await pumpEventQueue();

        var item = container
            .read(inboxReconcilerProvider)
            .snapshotFor('prof-test')![InboxScope.main]
            .items
            .single;
        expect(item.lastMessagePreview, 'latest message');
        expect(item.unreadCount, 2);

        chats.markLatestMessageRead = true;
        hub.emit(
          const RealtimeFrame(
            op: 'mark_read',
            data: {'chat_id': 'chat-other', 'message_id': 'msg-new'},
          ),
        );
        await pumpEventQueue();

        item = container
            .read(inboxReconcilerProvider)
            .snapshotFor('prof-test')![InboxScope.main]
            .items
            .single;
        expect(item.lastMessagePreview, 'latest message');
        expect(item.unreadCount, 0);
        expect(chats.mainCalls, greaterThanOrEqualTo(3));
      },
    );

    test(
      'message_create alone does not bump unread (notification is canonical)',
      () async {
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
      },
    );

    test(
      'archive activity bumps only the archived badge without sound',
      () async {
        final sound = _RecordingSoundPlayer();
        final hub = _FakeRealtimeHub();
        final chats = _FakeChatsClient(
          pages: const [
            ChatListData(
              items: [
                ChatListItem(
                  chat: VoiceChat(
                    id: 'chat-other',
                    type: 'CHAT_TYPE_DM',
                    creatorProfileId: 'peer-1',
                  ),
                ),
              ],
            ),
            ChatListData(
              items: [
                ChatListItem(
                  chat: VoiceChat(
                    id: 'chat-archived',
                    type: 'CHAT_TYPE_DM',
                    creatorProfileId: 'peer-archive',
                  ),
                ),
              ],
            ),
          ],
        );
        final container = _container(sound: sound, hub: hub, chats: chats);
        addTearDown(container.dispose);

        container.read(chatListControllerProvider);
        await pumpEventQueue();
        final archiveSubscription = container.listen<ChatListState>(
          chatArchiveListControllerProvider,
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(archiveSubscription.close);
        await container
            .read(chatArchiveListControllerProvider.notifier)
            .loadInitial();
        container.read(inAppNotificationControllerProvider);

        hub.emit(
          const RealtimeFrame(
            op: 'archive_activity',
            data: {'chat_id': 'chat-archived'},
          ),
        );
        await pumpEventQueue();

        final archived = container
            .read(chatArchiveListControllerProvider)
            .items
            .single;
        expect(archived.unreadCount, 1);
        expect(sound.newMessagePlays, 0);
        expect(sound.reactionPlays, 0);
        expect(sound.mentionPlays, 0);
      },
    );

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

    test(
      'notification creates an unread center row and mark_read converges it',
      () async {
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
              'type': 'mention',
              'chat_id': 'chat-other',
              'message_id': 'msg-mention',
              'sender_profile_id': 'peer-1',
            },
          ),
        );
        await pumpEventQueue();

        expect(container.read(inAppNotificationCenterProvider).unreadCount, 1);
        expect(
          container
              .read(inAppNotificationCenterProvider)
              .items
              .single
              .messageId,
          'msg-mention',
        );

        hub.emit(
          const RealtimeFrame(
            op: 'mark_read',
            data: {'chat_id': 'chat-other', 'message_id': 'msg-mention'},
          ),
        );
        await pumpEventQueue();

        expect(container.read(inAppNotificationCenterProvider).unreadCount, 0);
        expect(
          container.read(inAppNotificationCenterProvider).items.single.isRead,
          isTrue,
        );
      },
    );

    test('archive activity never creates a notification-center row', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub);
      addTearDown(container.dispose);

      container.read(inAppNotificationControllerProvider);
      hub.emit(
        const RealtimeFrame(
          op: 'archive_activity',
          data: {'chat_id': 'chat-archived'},
        ),
      );
      await pumpEventQueue();

      expect(container.read(inAppNotificationCenterProvider).items, isEmpty);
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
      expect(container.read(inAppNotificationCenterProvider).items, isEmpty);
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

    test('mention notification plays mention sound when enabled', () async {
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
            'type': 'mention',
            'chat_id': 'chat-other',
            'message_id': 'msg-mention',
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
      expect(sound.mentionPlays, 1);
      expect(sound.newMessagePlays, 0);
    });

    test('global mute disables sound but badge still updates', () async {
      final sound = _RecordingSoundPlayer();
      final hub = _FakeRealtimeHub();
      final container = _container(sound: sound, hub: hub, soundEnabled: false);
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
  VoiceChatsClient? chats,
  VoiceFriendsClient? friends,
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
      if (friends != null)
        voiceFriendsClientProvider.overrideWithValue(friends),
      voiceMessagesClientProvider.overrideWithValue(_FakeMessagesClient()),
      realtimeHubProvider.overrideWithValue(hub),
      notificationSoundPlayerProvider.overrideWithValue(sound),
      inAppNotificationsSoundEnabledProvider.overrideWith(
        (ref) => soundEnabled,
      ),
    ],
  );
}

class _RecordingSoundPlayer implements NotificationSoundPlayer {
  var newMessagePlays = 0;
  var reactionPlays = 0;
  var mentionPlays = 0;

  @override
  void playNewMessage() => newMessagePlays++;

  @override
  void playReaction() => reactionPlays++;

  @override
  void playMention() => mentionPlays++;
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
    String? folderId,
  }) async {
    calls.add(cursor);
    if (_pages.isEmpty) {
      return const ChatsApiOk(ChatListData(items: []));
    }
    return ChatsApiOk(_pages.removeAt(0));
  }
}

class _MutableInboxChatsClient extends VoiceChatsClient {
  _MutableInboxChatsClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  bool showLatestMessage = false;
  bool markLatestMessageRead = false;
  int mainCalls = 0;

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
    String? folderId,
  }) async {
    if (inbox == null || inbox == 'main') mainCalls++;
    if (inbox != null && inbox != 'main') {
      return const ChatsApiOk(ChatListData(items: []));
    }
    return ChatsApiOk(
      ChatListData(
        items: [
          inboxChatItem(
            'chat-other',
            preview: showLatestMessage ? 'latest message' : 'before',
            unreadCount: markLatestMessageRead
                ? 0
                : showLatestMessage
                ? 2
                : 1,
          ),
        ],
      ),
    );
  }
}

class _RequestChatsClient extends VoiceChatsClient {
  _RequestChatsClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  bool requestVisible = false;
  bool mainVisible = false;

  @override
  Future<ChatsApiResult<ChatListData>> listChats({
    required String authorization,
    String? cursor,
    int? pageSize,
    String? inbox,
    String? folderId,
  }) async => ChatsApiOk(
    ChatListData(
      items: inbox == 'requests' && requestVisible
          ? [inboxChatItem('new-request', inbox: 'requests')]
          : inbox == 'main' && mainVisible
          ? [inboxChatItem('new-request', inbox: 'main')]
          : const [],
    ),
  );
}

class _RequestFriendsClient extends VoiceFriendsClient {
  _RequestFriendsClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  bool requestVisible = false;
  bool friendsVisible = false;
  int friendListCalls = 0;

  @override
  Future<FriendsApiResult<FriendsListData>> listFriends({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    friendListCalls++;
    return FriendsApiOk(
      FriendsListData(friends: friendsVisible ? const ['peer-1'] : const []),
    );
  }

  @override
  Future<FriendsApiResult<FriendRequestsData>> listFriendRequests({
    required String authorization,
  }) async => FriendsApiOk(
    FriendRequestsData(
      incoming: requestVisible ? ['peer-1'] : const [],
      outgoing: const [],
    ),
  );
}

class _FakeMessagesClient extends VoiceMessagesClient {
  _FakeMessagesClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );
}
