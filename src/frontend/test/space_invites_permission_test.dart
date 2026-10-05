import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_tree_column.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'keeps invite management unavailable while permission is pending',
    (tester) async {
      final permission = Completer<bool>();
      const inviteQuery = (
        spaceId: 'space-1',
        permission: SpacePermissions.spaceManageInvites,
        chatId: null,
        voiceRoomId: null,
      );
      const botsQuery = (
        spaceId: 'space-1',
        permission: SpacePermissions.spaceManageBots,
        chatId: null,
        voiceRoomId: null,
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            spaceProvider('space-1').overrideWith(
              (ref) async => const VoiceSpace(
                id: 'space-1',
                name: 'Test Space',
                visibility: 'private',
                ownerProfileId: 'owner',
              ),
            ),
            spaceTreeProvider('space-1').overrideWith(
              (ref) async => const SpaceTreeData(
                categories: [],
                nodes: [],
                voiceRooms: [],
              ),
            ),
            spacePermissionProvider(
              inviteQuery,
            ).overrideWith((ref) => permission.future),
            spacePermissionProvider(
              botsQuery,
            ).overrideWith((ref) async => true),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: SpaceTreeColumn(
                spaceId: 'space-1',
                onTextChatSelected: (_) {},
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byKey(const Key('space_invites_action')), findsNothing);
      permission.completeError(StateError('private permission diagnostic'));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('space_invites_action')), findsNothing);
      expect(
        find.textContaining('private permission diagnostic'),
        findsNothing,
      );
    },
  );

  testWidgets('does not expose invite management without its permission', (
    tester,
  ) async {
    const inviteQuery = (
      spaceId: 'space-1',
      permission: SpacePermissions.spaceManageInvites,
      chatId: null,
      voiceRoomId: null,
    );
    const botsQuery = (
      spaceId: 'space-1',
      permission: SpacePermissions.spaceManageBots,
      chatId: null,
      voiceRoomId: null,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceProvider('space-1').overrideWith(
            (ref) async => const VoiceSpace(
              id: 'space-1',
              name: 'Test Space',
              visibility: 'private',
              ownerProfileId: 'owner',
            ),
          ),
          spaceTreeProvider('space-1').overrideWith(
            (ref) async =>
                const SpaceTreeData(categories: [], nodes: [], voiceRooms: []),
          ),
          spacePermissionProvider(
            inviteQuery,
          ).overrideWith((ref) async => false),
          spacePermissionProvider(botsQuery).overrideWith((ref) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SpaceTreeColumn(
              spaceId: 'space-1',
              onTextChatSelected: (_) {},
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('space_invites_action')), findsNothing);
  });
}
