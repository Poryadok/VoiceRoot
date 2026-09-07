import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_drawer.dart';

import 'support/fake_voice_api_clients.dart';

void main() {
  testWidgets('MobileShellDrawer lists folders and quick access', (
    tester,
  ) async {
    var settingsOpened = false;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          chatFoldersProvider.overrideWith(
            (_) async => const FolderListData(
              folders: [
                VoiceFolder(id: 'f1', name: 'All', folderType: 'system'),
              ],
            ),
          ),
          quickAccessListProvider.overrideWith(
            (_) async => const QuickAccessListData(items: []),
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            drawer: MobileShellDrawer(
              onOpenSettings: () => settingsOpened = true,
            ),
            body: Builder(
              builder: (context) => ElevatedButton(
                onPressed: () => Scaffold.of(context).openDrawer(),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.byKey(MobileShellDrawer.drawerKey), findsOneWidget);
    expect(find.text('All'), findsOneWidget);
    expect(find.text('No favorites yet'), findsOneWidget);
    expect(
      find.byKey(const Key('mobile_drawer_manage_folders')),
      findsOneWidget,
    );

    await tester.tap(find.byKey(const Key('mobile_drawer_settings')));
    await tester.pumpAndSettle();
    expect(settingsOpened, isTrue);
  });

  testWidgets(
    'MobileShellDrawer persists a reordered canonical quick access list',
    (tester) async {
      final chats = _RecordingQuickAccessClient();
      const qaData = QuickAccessListData(
        items: [
          VoiceQuickAccessItem(chatId: 'chat-qa-1'),
          VoiceQuickAccessItem(chatId: 'chat-qa-2'),
        ],
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authorizationHeaderProvider.overrideWithValue('Bearer test'),
            voiceChatsClientProvider.overrideWithValue(chats),
            chatFoldersProvider.overrideWith(
              (_) async => const FolderListData(folders: []),
            ),
            quickAccessListProvider.overrideWith((_) async => qaData),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              drawer: const MobileShellDrawer(onOpenSettings: _ignore),
              body: Builder(
                builder: (context) => ElevatedButton(
                  onPressed: () => Scaffold.of(context).openDrawer(),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      final list = tester.widget<ReorderableListView>(
        find.byKey(const Key('mobile_drawer_quick_access_reorder')),
      );
      list.onReorder(1, 0);
      await tester.pumpAndSettle();

      expect(chats.reorderedChatIds, ['chat-qa-2', 'chat-qa-1']);
    },
  );
}

void _ignore() {}

class _RecordingQuickAccessClient extends FakeVoiceChatsClient {
  List<String>? reorderedChatIds;

  @override
  Future<ChatsApiResult<void>> reorderQuickAccess({
    required String authorization,
    required List<String> chatIds,
  }) async {
    reorderedChatIds = List.of(chatIds);
    return const ChatsApiOk<void>(null);
  }
}
