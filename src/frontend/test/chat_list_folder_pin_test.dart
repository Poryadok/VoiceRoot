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
              VoiceFolder(
                id: folderId,
                name: 'Custom',
                folderType: 'custom',
              ),
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
    expect(find.byKey(ChatListBody.quickAccessActionKey(chatId)), findsOneWidget);
  });
}
