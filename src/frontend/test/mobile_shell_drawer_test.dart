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

  testWidgets('MobileShellDrawer retries a failed Quick Access load', (
    tester,
  ) async {
    var attempts = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          chatFoldersProvider.overrideWith(
            (_) async => const FolderListData(folders: []),
          ),
          quickAccessListProvider.overrideWith((_) async {
            attempts++;
            if (attempts == 1) throw Exception('private quick access detail');
            return const QuickAccessListData(
              items: [
                VoiceQuickAccessItem(
                  chatId: 'chat-qa-1',
                  chat: VoiceChat(
                    id: 'chat-qa-1',
                    type: 'CHAT_TYPE_DM',
                    creatorProfileId: 'profile-1',
                    name: 'Favorite DM',
                  ),
                ),
              ],
            );
          }),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Consumer(
            builder: (context, ref, _) => Scaffold(
              drawer: const MobileShellDrawer(onOpenSettings: _ignore),
              body: Column(
                children: [
                  Text(ref.watch(selectedChatIdProvider) ?? 'none'),
                  Builder(
                    builder: (context) => ElevatedButton(
                      onPressed: () => Scaffold.of(context).openDrawer(),
                      child: const Text('open'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byKey(MobileShellDrawer.drawerKey)),
    )!;
    expect(attempts, 1);
    expect(find.text(l10n.chatListLoadError), findsOneWidget);
    expect(find.text('private quick access detail'), findsNothing);

    await tester.tap(find.byKey(const Key('mobile_drawer_quick_access_retry')));
    await tester.pumpAndSettle();

    expect(attempts, 2);
    expect(find.text('Favorite DM'), findsOneWidget);
    await tester.tap(find.text('Favorite DM'));
    await tester.pumpAndSettle();
    expect(find.text('chat-qa-1'), findsOneWidget);
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
