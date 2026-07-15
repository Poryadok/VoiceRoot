import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/voice_room_providers.dart';
import 'package:voice_frontend/ui/space/space_tree_panel.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

SpaceTreeData _treeWithVoiceRoom() {
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
        id: 'node-voice',
        spaceId: 'space-1',
        categoryId: 'cat-1',
        kind: 'voice_room',
        voiceRoomId: 'vr-1',
        sortOrder: 0,
        displayName: 'Lobby',
      ),
    ],
    voiceRooms: [
      VoiceRoomData(id: 'vr-1', spaceId: 'space-1', name: 'Lobby'),
    ],
  );
}

/// spaces.md: voice room tap joins room and shows connected participants.
void main() {
  testWidgets('voice node tap joins room and shows participants', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spacePermissionProvider.overrideWith((ref, query) async => true),
          spaceTreeProvider('space-1').overrideWith((_) async {
            return _treeWithVoiceRoom();
          }),
          voiceRoomParticipantsProvider('vr-1').overrideWith((_) async {
            return const [
              VoiceRoomParticipant(
                profileId: 'profile-a',
                displayName: 'Alice',
              ),
              VoiceRoomParticipant(
                profileId: 'profile-b',
                displayName: 'Bob',
              ),
            ];
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

    expect(find.text('Lobby'), findsOneWidget);
    await tester.tap(find.byKey(SpaceTreePanel.nodeKey('node-voice')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('voice_room_participants_vr-1')), findsOneWidget);
    expect(find.text('Alice'), findsOneWidget);
    expect(find.text('Bob'), findsOneWidget);
  });
}

void _noop(String _) {}
