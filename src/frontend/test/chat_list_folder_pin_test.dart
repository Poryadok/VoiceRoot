import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('Chat row actions show pin when folder selected', (tester) async {
    const folderId = 'folder-custom';
    const chatId = 'chat-pin-1';
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith(
          (ref) => FakeVoiceChatsClient(
            pages: [
              ChatListData(
                items: [
                  ChatListItem(
                    chat: VoiceChat(
                      id: chatId,
                      type: 'CHAT_TYPE_GROUP',
                      creatorProfileId: 'p1',
                      name: 'Pin Target',
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'Custom', folderType: 'custom'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
      ],
    );
    addTearDown(container.dispose);
    container.read(selectedChatFolderIdProvider.notifier).state = folderId;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(find.text('Pin Target'));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListBody.pinActionKey(chatId)), findsOneWidget);
    expect(
      find.byKey(ChatListBody.quickAccessActionKey(chatId)),
      findsOneWidget,
    );
    expect(
      find.byKey(ChatListBody.removeFromFolderActionKey(chatId)),
      findsOneWidget,
    );
    expect(find.byKey(ChatListBody.archiveActionKey(chatId)), findsOneWidget);
  });

  testWidgets('Group row can be archived and added to a custom folder', (
    tester,
  ) async {
    const chatId = 'chat-group-1';
    const folderId = 'folder-custom';
    final chats = _TrackingVoiceChatsClient(
      pages: [
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Group Target',
              ),
            ),
          ],
        ),
        ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: chatId,
                type: 'CHAT_TYPE_GROUP',
                creatorProfileId: 'p1',
                name: 'Group Target',
              ),
            ),
          ],
        ),
      ],
    );
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        onboardingControllerProvider.overrideWith(
          TestCompletedOnboardingController.new,
        ),
        voiceChatsClientProvider.overrideWith((ref) => chats),
        chatFoldersProvider.overrideWith(
          (_) async => FolderListData(
            folders: [
              VoiceFolder(id: folderId, name: 'Custom', folderType: 'custom'),
            ],
          ),
        ),
        quickAccessListProvider.overrideWith(
          (_) async => const QuickAccessListData(items: []),
        ),
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
          home: const Scaffold(body: ChatListBody(showHeader: false)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(find.text('Group Target'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatListBody.addToFolderActionKey(chatId)));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(Key('chat_list_add_to_folder_$folderId')));
    await tester.pumpAndSettle();
    expect(chats.added, [(folderId, chatId)]);

    await tester.longPress(find.text('Group Target'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatListBody.archiveActionKey(chatId)));
    await tester.pumpAndSettle();
    expect(chats.archived, [chatId]);
  });
}

class _TrackingVoiceChatsClient extends FakeVoiceChatsClient {
  _TrackingVoiceChatsClient({required super.pages});

  final List<(String, String)> added = [];
  final List<String> archived = [];

  @override
  Future<ChatsApiResult<void>> addChatToFolder({
    required String authorization,
    required String folderId,
    required String chatId,
  }) async {
    added.add((folderId, chatId));
    return const ChatsApiOk(null);
  }

  @override
  Future<ChatsApiResult<void>> archiveChat({
    required String authorization,
    required String chatId,
    required bool archived,
  }) async {
    if (archived) this.archived.add(chatId);
    return const ChatsApiOk(null);
  }
}
