import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/ui/shell/manage_folders_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/voice_test_theme.dart';

class _ReorderChatsClient extends FakeVoiceChatsClient {
  _ReorderChatsClient({this.failAtUpdate});

  final int? failAtUpdate;
  Future<void> Function()? afterFirstUpdate;
  bool rejectOldBearer = false;
  final folders = <VoiceFolder>[
    const VoiceFolder(
      id: 'a',
      name: 'Alpha',
      folderType: 'custom',
      sortOrder: 5,
    ),
    const VoiceFolder(
      id: 'b',
      name: 'Bravo',
      folderType: 'custom',
      sortOrder: 6,
    ),
    const VoiceFolder(
      id: 'c',
      name: 'Charlie',
      folderType: 'custom',
      sortOrder: 7,
    ),
  ];
  final updates = <(String, int)>[];

  @override
  Future<ChatsApiResult<FolderListData>> listFolders({
    required String authorization,
  }) async {
    final ordered = [...folders]
      ..sort((a, b) => a.sortOrder.compareTo(b.sortOrder));
    return ChatsApiOk(FolderListData(folders: ordered));
  }

  @override
  Future<ChatsApiResult<VoiceFolder>> updateFolder({
    required String authorization,
    required String folderId,
    String? name,
    int? sortOrder,
    String? filterConfigJson,
  }) async {
    updates.add((folderId, sortOrder!));
    if (rejectOldBearer && authorization == 'Bearer test-access') {
      return const ChatsApiFailure(message: 'revoked token');
    }
    if (updates.length == failAtUpdate) {
      return const ChatsApiFailure(message: 'reorder failed');
    }
    final index = folders.indexWhere((folder) => folder.id == folderId);
    final old = folders[index];
    final updated = VoiceFolder(
      id: old.id,
      name: old.name,
      folderType: old.folderType,
      sortOrder: sortOrder,
    );
    folders[index] = updated;
    if (updates.length == 1) await afterFirstUpdate?.call();
    return ChatsApiOk(updated);
  }
}

void main() {
  test(
    'profile switch stops writes and reports partial order after token revoke',
    () async {
      final chats = _ReorderChatsClient();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          voiceChatsClientProvider.overrideWithValue(chats),
        ],
      );
      addTearDown(container.dispose);
      chats.afterFirstUpdate = () async {
        chats.rejectOldBearer = true;
        await container
            .read(authControllerProvider.notifier)
            .applySession(
              const AuthSession(
                accessToken: 'other-profile-token',
                refreshToken: 'other-refresh',
                expiresInSeconds: 900,
                accountId: 'acc-test',
                activeProfileId: 'other-profile',
              ),
            );
      };

      final error = await container
          .read(folderActionsProvider)
          .reorderCustomFolders([
            chats.folders[2],
            chats.folders[0],
            chats.folders[1],
          ]);

      expect(error, kFolderReorderMayBePartial);
      expect(chats.updates, [('c', 5)]);
      expect(chats.folders.map((folder) => folder.sortOrder), [5, 6, 5]);
    },
  );

  testWidgets('Edit folders reorders custom folders and persists their order', (
    tester,
  ) async {
    final chats = _ReorderChatsClient();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        voiceChatsClientProvider.overrideWithValue(chats),
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
          home: const Scaffold(body: ManageFoldersSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final list = tester.widget<ReorderableListView>(
      find.byType(ReorderableListView),
    );
    list.onReorder(2, 0);
    await tester.pumpAndSettle();

    expect(chats.updates, [('c', 5), ('a', 6), ('b', 7)]);
    final rows = find.byType(ListTile);
    expect(
      [
        tester.widget<ListTile>(rows.at(0)).key,
        tester.widget<ListTile>(rows.at(1)).key,
        tester.widget<ListTile>(rows.at(2)).key,
      ],
      [
        const Key('manage_folder_c'),
        const Key('manage_folder_a'),
        const Key('manage_folder_b'),
      ],
    );
  });

  testWidgets('failed folder reorder restores earlier persisted writes', (
    tester,
  ) async {
    final chats = _ReorderChatsClient(failAtUpdate: 2);
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        voiceChatsClientProvider.overrideWithValue(chats),
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
          home: const Scaffold(body: ManageFoldersSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    tester
        .widget<ReorderableListView>(find.byType(ReorderableListView))
        .onReorder(2, 0);
    await tester.pumpAndSettle();

    expect(chats.updates, [('c', 5), ('a', 6), ('c', 7)]);
    expect(find.text('reorder failed'), findsOneWidget);
    final rows = find.byType(ListTile);
    expect(
      [
        tester.widget<ListTile>(rows.at(0)).key,
        tester.widget<ListTile>(rows.at(1)).key,
        tester.widget<ListTile>(rows.at(2)).key,
      ],
      [
        const Key('manage_folder_a'),
        const Key('manage_folder_b'),
        const Key('manage_folder_c'),
      ],
    );
  });

  testWidgets('folder controls are disabled while reorder is saving', (
    tester,
  ) async {
    final chats = _ReorderChatsClient();
    final pending = Completer<void>();
    chats.afterFirstUpdate = () => pending.future;
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 404)),
        ),
        authorizationHeaderProvider.overrideWithValue('Bearer test'),
        voiceChatsClientProvider.overrideWithValue(chats),
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
          home: const Scaffold(body: ManageFoldersSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    tester
        .widget<ReorderableListView>(find.byType(ReorderableListView))
        .onReorder(2, 0);
    await tester.pump();

    expect(
      tester
          .widget<IconButton>(
            find.byKey(const Key('manage_folders_create_button')),
          )
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<IconButton>(find.byKey(const Key('manage_folder_delete_a')))
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<ReorderableDragStartListener>(
            find.byType(ReorderableDragStartListener).first,
          )
          .enabled,
      isFalse,
    );

    pending.complete();
    await tester.pumpAndSettle();
  });
}
