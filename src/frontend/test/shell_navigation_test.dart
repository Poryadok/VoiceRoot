import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:http/testing.dart';

import 'support/auth_test_overrides.dart';

void main() {
  group('ShellNavigation', () {
    test('exitSpace clears selectedSpaceId and side panel', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      container.read(selectedSpaceIdProvider.notifier).state = 'space-1';
      container.read(shellSidePanelProvider.notifier).state =
          ShellSidePanel.members;

      container.read(shellNavigationProvider).exitSpace();

      expect(container.read(selectedSpaceIdProvider), isNull);
      expect(container.read(shellSidePanelProvider), ShellSidePanel.none);
    });

    test('selectSpace toggles off when same space tapped again', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      container.read(shellNavigationProvider).selectSpace('space-1');
      expect(container.read(selectedSpaceIdProvider), 'space-1');

      container.read(shellNavigationProvider).selectSpace('space-1');
      expect(container.read(selectedSpaceIdProvider), isNull);
    });

    test('selectSpace clears chat when chat is not in that space', () {
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
      );
      addTearDown(container.dispose);

      container.read(selectedChatIdProvider.notifier).state = 'dm-1';
      container.read(shellNavigationProvider).selectSpace('space-1');

      expect(container.read(selectedSpaceIdProvider), 'space-1');
      expect(container.read(selectedChatIdProvider), isNull);
    });

    test('selectChatFromHome clears selected space', () {
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ],
      );
      addTearDown(container.dispose);

      container.read(selectedSpaceIdProvider.notifier).state = 'space-1';
      container.read(shellNavigationProvider).selectChatFromHome('dm-1');

      expect(container.read(selectedSpaceIdProvider), isNull);
      expect(container.read(selectedChatIdProvider), 'dm-1');
    });
  });

  group('resolveDmPeerForChatId', () {
    test('does not infer peer from messages for group chats', () {
      final peer = resolveDmPeerForChatId(
        chatId: 'group-1',
        knownPeers: const {},
        listItems: const [
          ChatListItem(
            chat: VoiceChat(
              id: 'group-1',
              type: 'CHAT_TYPE_GROUP',
              creatorProfileId: 'owner',
              name: 'Team',
            ),
          ),
        ],
        activeProfileId: 'me',
        messages: const [
          VoiceMessage(
            id: 'm1',
            chatId: 'group-1',
            senderProfileId: 'other-user',
            content: 'hi',
          ),
        ],
      );
      expect(peer, isNull);
    });
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  void ensureSubscribed(String chatId) {}

  @override
  void markRead(String chatId, String messageId) {}
}
