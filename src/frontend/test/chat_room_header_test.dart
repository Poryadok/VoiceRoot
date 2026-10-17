import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'DM to group navigation waits for selected chat metadata before showing header actions',
    (tester) async {
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
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
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          presenceProvider.overrideWith((ref, profileId) => null),
          profileProvider('peer-1').overrideWith(
            (ref) async => const VoiceProfile(
              id: 'peer-1',
              accountId: 'peer-account',
              username: 'peer',
              discriminator: '0001',
              displayName: 'Previous DM peer',
            ),
          ),
          selectedChatIdProvider.overrideWith((ref) => 'dm-chat'),
          chatListControllerProvider.overrideWith(
            _DelayedGroupMetadataChatListController.new,
          ),
          chatRoomControllerProvider(
            'dm-chat',
          ).overrideWith((ref) => _HeaderRoomController(ref, 'dm-chat')),
          chatRoomControllerProvider('group-chat').overrideWith(
            (ref) => _HeaderRoomController(
              ref,
              'group-chat',
              messages: const [
                VoiceMessage(
                  id: 'group-message',
                  chatId: 'group-chat',
                  senderProfileId: 'peer-1',
                  content: 'Group timeline message',
                ),
              ],
            ),
          ),
          groupActiveCallProvider(
            'group-chat',
          ).overrideWith((ref) async => null),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: Consumer(
                builder: (context, ref, _) =>
                    ChatRoomPanel(chatId: ref.watch(selectedChatIdProvider)!),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(_headerTitle('Previous DM peer'), findsOneWidget);
      expect(find.byKey(ChatRoomPanel.audioCallKey), findsOneWidget);
      expect(find.byKey(ChatRoomPanel.videoCallKey), findsOneWidget);

      container.read(selectedChatIdProvider.notifier).state = 'group-chat';
      await tester.pumpAndSettle();

      expect(find.byKey(ChatRoomPanel.panelKey), findsOneWidget);
      expect(_headerTitle('Previous DM peer'), findsNothing);
      expect(find.byKey(ChatRoomPanel.audioCallKey), findsNothing);
      expect(find.byKey(ChatRoomPanel.videoCallKey), findsNothing);
      expect(find.byKey(ChatRoomPanel.chatInfoKey), findsNothing);
      expect(find.byKey(ChatRoomPanel.groupMembersKey), findsNothing);
      expect(find.byKey(ChatRoomPanel.groupVoiceStartKey), findsNothing);

      final chats =
          container.read(chatListControllerProvider.notifier)
              as _DelayedGroupMetadataChatListController;
      chats.publishSelectedGroupMetadata();
      await tester.pumpAndSettle();

      expect(find.text('QA audit group'), findsOneWidget);
      expect(_headerTitle('Previous DM peer'), findsNothing);
      expect(find.byKey(ChatRoomPanel.audioCallKey), findsNothing);
      expect(find.byKey(ChatRoomPanel.videoCallKey), findsNothing);
      expect(find.byKey(ChatRoomPanel.chatInfoKey), findsOneWidget);
      expect(find.byKey(ChatRoomPanel.groupMembersKey), findsOneWidget);
      expect(find.byKey(ChatRoomPanel.groupVoiceStartKey), findsOneWidget);
    },
  );
}

Finder _headerTitle(String text) => find.byWidgetPredicate(
  (widget) =>
      widget is Text &&
      widget.data == text &&
      widget.style?.fontSize == 16,
  description: 'header title "$text"',
);

class _DelayedGroupMetadataChatListController extends ChatListController {
  _DelayedGroupMetadataChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'dm-chat',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
          ),
          dmPeerProfileId: 'peer-1',
          dmPeerDisplayName: 'Previous DM peer',
        ),
      ],
    );
  }

  void publishSelectedGroupMetadata() {
    state = state.copyWith(
      items: [
        ...state.items,
        const ChatListItem(
          chat: VoiceChat(
            id: 'group-chat',
            type: 'CHAT_TYPE_GROUP',
            creatorProfileId: 'prof-test',
            name: 'QA audit group',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _HeaderRoomController extends ChatRoomController {
  _HeaderRoomController(
    super.ref,
    super.chatId, {
    List<VoiceMessage> messages = const [],
  }) : super() {
    state = ChatRoomState(messages: messages, historyProfileId: 'prof-test');
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadScheduledMessages({bool loadMore = false}) async {}
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
