import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_drawer.dart';

void main() {
  testWidgets('MobileShellDrawer lists folders and quick access', (tester) async {
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
    expect(find.byKey(const Key('mobile_drawer_manage_folders')), findsOneWidget);

    await tester.tap(find.byKey(const Key('mobile_drawer_settings')));
    await tester.pumpAndSettle();
    expect(settingsOpened, isTrue);
  });
}
