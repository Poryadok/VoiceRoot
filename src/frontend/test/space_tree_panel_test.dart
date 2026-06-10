import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_tree_panel.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

SpaceTreeData _sampleTree() {
  return const SpaceTreeData(
    categories: [
      SpaceCategory(
        id: 'cat-1',
        spaceId: 'space-1',
        name: 'General',
        sortOrder: 0,
      ),
    ],
    nodes: [
      SpaceTreeNodeData(
        id: 'node-text',
        spaceId: 'space-1',
        categoryId: 'cat-1',
        kind: 'text_chat',
        linkedChatId: 'chat-1',
        sortOrder: 0,
        displayName: 'announcements',
      ),
      SpaceTreeNodeData(
        id: 'node-voice',
        spaceId: 'space-1',
        categoryId: 'cat-1',
        kind: 'voice_room',
        voiceRoomId: 'vr-1',
        sortOrder: 1,
        displayName: 'Lobby',
      ),
    ],
    voiceRooms: [
      VoiceRoomData(id: 'vr-1', spaceId: 'space-1', name: 'Lobby'),
    ],
  );
}

void main() {
  testWidgets('renders categories with text and voice nodes', (tester) async {
    String? selectedChatId;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith((_) async => _sampleTree()),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SpaceTreePanel(
              spaceId: 'space-1',
              onTextChatSelected: (id) => selectedChatId = id,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceTreePanel.panelKey), findsOneWidget);
    expect(find.byKey(SpaceTreePanel.categoryKey('cat-1')), findsOneWidget);
    expect(find.byKey(SpaceTreePanel.nodeKey('node-text')), findsOneWidget);
    expect(find.byKey(SpaceTreePanel.nodeKey('node-voice')), findsOneWidget);
    expect(find.text('announcements'), findsOneWidget);
    expect(find.text('Lobby'), findsOneWidget);

    await tester.tap(find.text('announcements'));
    await tester.pumpAndSettle();
    expect(selectedChatId, 'chat-1');
  });

  testWidgets('collapsed category hides nodes until expanded', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith((_) async {
            return SpaceTreeData(
              categories: const [
                SpaceCategory(
                  id: 'cat-1',
                  spaceId: 'space-1',
                  name: 'Hidden',
                  sortOrder: 0,
                ),
              ],
              nodes: const [
                SpaceTreeNodeData(
                  id: 'node-text',
                  spaceId: 'space-1',
                  categoryId: 'cat-1',
                  kind: 'text_chat',
                  linkedChatId: 'chat-1',
                  sortOrder: 0,
                  displayName: 'hidden-channel',
                ),
              ],
              voiceRooms: const [],
            );
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(
              spaceId: 'space-1',
              onTextChatSelected: _noop,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('hidden-channel'), findsOneWidget);
    await tester.tap(find.text('HIDDEN'));
    await tester.pumpAndSettle();
    expect(find.text('hidden-channel'), findsNothing);
    await tester.tap(find.text('HIDDEN'));
    await tester.pumpAndSettle();
    expect(find.text('hidden-channel'), findsOneWidget);
  });
}

void _noop(String _) {}
