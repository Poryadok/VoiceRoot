import 'dart:convert';

import 'package:flutter/gestures.dart' show kLongPressTimeout;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/voice_room_providers.dart';
import 'package:voice_frontend/ui/space/space_tree_panel.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/auth_test_overrides.dart';
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
    voiceRooms: [VoiceRoomData(id: 'vr-1', spaceId: 'space-1', name: 'Lobby')],
  );
}

void main() {
  testWidgets('tree creation actions are hidden without tree permission', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith((_) async => _sampleTree()),
          spacePermissionProvider.overrideWith((ref, query) async => false),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceTreePanel.createCategoryButtonKey), findsNothing);
    expect(find.byKey(SpaceTreePanel.createTextChatButtonKey), findsNothing);
  });

  for (final language in ['en', 'ru']) {
    testWidgets('$language tree creation localizes missing authentication', (
      tester,
    ) async {
      var requests = 0;
      final client = MockClient((request) async {
        requests++;
        return http.Response('unexpected request', 500);
      });
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceAppTestOverrides(client: client),
            authorizationHeaderProvider.overrideWithValue(null),
            spaceTreeProvider(
              'space-1',
            ).overrideWith((_) async => _sampleTree()),
            spacePermissionProvider.overrideWith((ref, query) async => true),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: Locale(language),
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
      await tester.tap(find.byKey(SpaceTreePanel.createCategoryButtonKey));
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(SpaceTreePanel)),
      )!;
      final nameField = find.byKey(SpaceTreePanel.createNameFieldKey);
      await tester.enterText(nameField, 'Updates');
      await tester.pump();
      final submit = find.byKey(SpaceTreePanel.createSubmitButtonKey);
      expect(tester.widget<FilledButton>(submit).onPressed, isNotNull);
      expect(submit.hitTestable(), findsOneWidget);
      await tester.tap(submit);
      await tester.pumpAndSettle();

      expect(
        find.text(l10n.spaceTreeCreateError(l10n.commonActionFailed)),
        findsOneWidget,
      );
      expect(find.textContaining('not_authenticated'), findsNothing);
      expect(requests, 0);
      expect(find.text(l10n.spaceTreeCreateCategory), findsOneWidget);
      expect(tester.widget<TextField>(nameField).controller!.text, 'Updates');
      expect(tester.widget<FilledButton>(submit).onPressed, isNotNull);
    });

    testWidgets(
      '$language text chat creation localizes missing authentication',
      (tester) async {
        var requests = 0;
        final client = MockClient((request) async {
          requests++;
          return http.Response('unexpected request', 500);
        });
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              ...voiceAppTestOverrides(client: client),
              authorizationHeaderProvider.overrideWithValue(null),
              spaceTreeProvider(
                'space-1',
              ).overrideWith((_) async => _sampleTree()),
              spacePermissionProvider.overrideWith((ref, query) async => true),
            ],
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: Locale(language),
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
        await tester.tap(find.byKey(SpaceTreePanel.createTextChatButtonKey));
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(SpaceTreePanel)),
        )!;
        await tester.tap(find.byKey(SpaceTreePanel.createCategoryDropdownKey));
        await tester.pumpAndSettle();
        await tester.tap(find.text('General').last);
        await tester.pumpAndSettle();

        final nameField = find.byKey(SpaceTreePanel.createNameFieldKey);
        await tester.enterText(nameField, 'Updates');
        await tester.pump();
        final submit = find.byKey(SpaceTreePanel.createSubmitButtonKey);
        expect(tester.widget<FilledButton>(submit).onPressed, isNotNull);
        expect(submit.hitTestable(), findsOneWidget);
        await tester.tap(submit);
        await tester.pumpAndSettle();

        expect(
          find.text(l10n.spaceTreeCreateError(l10n.commonActionFailed)),
          findsOneWidget,
        );
        expect(find.textContaining('not_authenticated'), findsNothing);
        expect(requests, 0);
        expect(find.text(l10n.spaceTreeCreateTextChat), findsOneWidget);
        expect(tester.widget<TextField>(nameField).controller!.text, 'Updates');
        expect(tester.widget<FilledButton>(submit).onPressed, isNotNull);
      },
    );
  }

  testWidgets('permitted member can create a category and channel', (
    tester,
  ) async {
    final categories = <Map<String, Object?>>[];
    final nodes = <Map<String, Object?>>[];
    final postedPaths = <String>[];
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        return http.Response(
          jsonEncode({
            'categories': categories,
            'nodes': nodes,
            'voice_rooms': [],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/categories') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        postedPaths.add(request.url.path);
        categories.add({
          'id': 'cat-new',
          'space_id': 'space-1',
          'name': body['name'],
          'sort_order': 0,
        });
        return http.Response(jsonEncode({'category': categories.single}), 200);
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/chats') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        postedPaths.add(request.url.path);
        expect(body['type'], 'CHAT_TYPE_CHANNEL');
        expect(body['name'], 'announcements');
        nodes.add({
          'id': 'node-channel',
          'space_id': 'space-1',
          'category_id': null,
          'kind': 'text_chat',
          'linked_chat': {'id': 'chat-channel', 'type': 'CHAT_TYPE_CHANNEL'},
          'sort_order': 0,
          'display_name': body['name'],
        });
        return http.Response(
          jsonEncode({'space_tree_node': nodes.single}),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/tree/nodes') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        postedPaths.add(request.url.path);
        expect(body['node_id'], 'node-channel');
        expect(body['category_id'], 'cat-new');
        nodes.single['category_id'] = body['category_id'];
        return http.Response(
          jsonEncode({'space_tree_node': nodes.single}),
          200,
        );
      }
      return http.Response('unexpected request', 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async {
            return query.permission == 'TEXT_CHAT_CREATE_IN_SPACE';
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final createCategoryButton = find.byKey(
      const Key('space_tree_create_category'),
    );
    expect(createCategoryButton, findsOneWidget);
    await tester.tap(createCategoryButton);
    await tester.pumpAndSettle();
    expect(find.text('Create category'), findsOneWidget);
    await tester.enterText(
      find.byKey(const Key('space_tree_create_name')),
      'Updates',
    );
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(
            find.byKey(const Key('space_tree_create_submit')),
          )
          .onPressed,
      isNotNull,
    );
    await tester.tap(find.byKey(const Key('space_tree_create_submit')));
    await tester.pumpAndSettle();
    expect(postedPaths, ['/api/v1/spaces/space-1/categories']);
    expect(find.text('UPDATES'), findsOneWidget);

    final createChatButton = find.byKey(const Key('space_tree_create_chat'));
    expect(createChatButton, findsOneWidget);
    await tester.tap(createChatButton);
    await tester.pumpAndSettle();
    expect(find.text('Create text chat'), findsOneWidget);
    expect(
      find.byKey(SpaceTreePanel.createCategoryDropdownKey),
      findsOneWidget,
    );
    await tester.tap(find.byKey(SpaceTreePanel.createCategoryDropdownKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Updates').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('space_tree_create_type_channel')));
    await tester.enterText(
      find.byKey(const Key('space_tree_create_name')),
      'announcements',
    );
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(
            find.byKey(const Key('space_tree_create_submit')),
          )
          .onPressed,
      isNotNull,
    );
    await tester.tap(find.byKey(const Key('space_tree_create_submit')));
    await tester.pumpAndSettle();

    expect(postedPaths, [
      '/api/v1/spaces/space-1/categories',
      '/api/v1/spaces/space-1/chats',
      '/api/v1/spaces/space-1/tree/nodes',
    ]);
    expect(find.text('announcements'), findsOneWidget);
    expect(nodes.single['category_id'], 'cat-new');
  });

  testWidgets('permitted member can drag-reorder nodes within a category', (
    tester,
  ) async {
    final treeNodes = <Map<String, Object?>>[
      {
        'id': 'node-a',
        'space_id': 'space-1',
        'category_id': 'cat-1',
        'kind': 'text_chat',
        'linked_chat': {'id': 'chat-a', 'type': 'CHAT_TYPE_GROUP'},
        'sort_order': 0,
        'display_name': 'alpha',
      },
      {
        'id': 'node-b',
        'space_id': 'space-1',
        'category_id': 'cat-1',
        'kind': 'text_chat',
        'linked_chat': {'id': 'chat-b', 'type': 'CHAT_TYPE_GROUP'},
        'sort_order': 1,
        'display_name': 'beta',
      },
    ];
    List<String>? reorderedIds;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        return http.Response(
          jsonEncode({
            'categories': [
              {
                'id': 'cat-1',
                'space_id': 'space-1',
                'name': 'General',
                'sort_order': 0,
              },
            ],
            'nodes': treeNodes,
            'voice_rooms': [],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/tree/reorder') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        reorderedIds = (body['ordered_node_ids'] as List).cast<String>();
        final ordered = reorderedIds!;
        treeNodes.sort(
          (left, right) => ordered
              .indexOf(left['id']! as String)
              .compareTo(ordered.indexOf(right['id']! as String)),
        );
        return http.Response('', 204);
      }
      return http.Response('unexpected request', 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(
      tester.getCenter(find.text('beta')),
    );
    await tester.pump(kLongPressTimeout);
    await gesture.moveTo(tester.getCenter(find.text('alpha')));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(reorderedIds, ['node-b', 'node-a']);
  });

  for (final language in ['en', 'ru']) {
    for (final categorized in [false, true]) {
      for (final failure in ['denied', 'unknown', 'refresh']) {
        testWidgets('$language reorder hides $failure diagnostics and recovers '
            '(categorized=$categorized)', (tester) async {
          const secret = 'private upstream reorder diagnostic';
          final categoryId = categorized ? 'cat-1' : null;
          final nodes = [
            for (final (id, name) in [('a', 'alpha'), ('b', 'beta')])
              <String, Object?>{
                'id': 'node-$id',
                'space_id': 'space-1',
                'category_id': categoryId,
                'kind': 'text_chat',
                'linked_chat': {'id': 'chat-$id', 'type': 'CHAT_TYPE_GROUP'},
                'sort_order': id == 'a' ? 0 : 1,
                'display_name': name,
              },
          ];
          final requests = <List<String>>[];
          var failRefresh = false;
          final client = MockClient((request) async {
            if (request.method == 'GET' &&
                request.url.path == '/api/v1/spaces/space-1/tree') {
              if (failRefresh) {
                return http.Response(jsonEncode({'message': secret}), 500);
              }
              return http.Response(
                jsonEncode({
                  'categories': [
                    if (categorized)
                      {
                        'id': categoryId,
                        'space_id': 'space-1',
                        'name': 'General',
                        'sort_order': 0,
                      },
                  ],
                  'nodes': nodes,
                  'voice_rooms': [],
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/spaces/space-1/tree/reorder') {
              final body = jsonDecode(request.body) as Map<String, dynamic>;
              expect(body['space_id'], 'space-1');
              final ordered = (body['ordered_node_ids'] as List).cast<String>();
              requests.add(ordered);
              if (requests.length == 1 && failure != 'refresh') {
                return http.Response(
                  jsonEncode({'message': secret}),
                  failure == 'denied' ? 403 : 500,
                );
              }
              nodes.sort(
                (left, right) => ordered
                    .indexOf(left['id']! as String)
                    .compareTo(ordered.indexOf(right['id']! as String)),
              );
              for (var i = 0; i < nodes.length; i++) {
                nodes[i]['sort_order'] = i;
              }
              failRefresh = requests.length == 1 && failure == 'refresh';
              return http.Response('', 204);
            }
            return http.Response('unexpected request', 500);
          });
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                ...voiceAppTestOverrides(client: client),
                spacePermissionProvider.overrideWith((_, _) async => true),
              ],
              child: MaterialApp(
                theme: voiceTestTheme(),
                locale: Locale(language),
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
          final l10n = AppLocalizations.of(
            tester.element(find.byType(SpaceTreePanel)),
          )!;
          Future<void> drag(String from, String to) async {
            final gesture = await tester.startGesture(
              tester.getCenter(find.text(from)),
            );
            await tester.pump(kLongPressTimeout);
            await gesture.moveTo(tester.getCenter(find.text(to)));
            await tester.pump();
            await gesture.up();
            await tester.pumpAndSettle();
          }

          await drag('beta', 'alpha');
          expect(requests, [
            ['node-b', 'node-a'],
          ]);
          final expected = failure == 'refresh'
              ? l10n.spaceTreeCreateRefreshFailed
              : language == 'en'
              ? 'Could not reorder channels: Could not complete this action.'
              : 'Не удалось изменить порядок каналов: Не удалось выполнить действие.';
          expect(find.text(expected), findsOneWidget);
          expect(find.textContaining(secret), findsNothing);
          expect(find.textContaining('tree_refresh_failed'), findsNothing);

          if (failure == 'refresh') {
            expect(find.byKey(SpaceTreePanel.errorKey), findsOneWidget);
            failRefresh = false;
            await tester.tap(find.text(l10n.commonRetry));
            await tester.pumpAndSettle();
            expect(find.byKey(SpaceTreePanel.errorKey), findsNothing);
          }
          ScaffoldMessenger.of(
            tester.element(find.byType(SpaceTreePanel)),
          ).removeCurrentSnackBar();
          await tester.pumpAndSettle();
          await drag(
            failure == 'refresh' ? 'alpha' : 'beta',
            failure == 'refresh' ? 'beta' : 'alpha',
          );
          expect(requests, [
            ['node-b', 'node-a'],
            failure == 'refresh' ? ['node-a', 'node-b'] : ['node-b', 'node-a'],
          ]);
          expect(find.text(expected), findsNothing);
          expect(find.byKey(SpaceTreePanel.errorKey), findsNothing);
          expect(find.textContaining(secret), findsNothing);
        });
      }
    }
  }

  testWidgets('shows partial success when category assignment is denied', (
    tester,
  ) async {
    final nodes = <Map<String, Object?>>[];
    var assignmentAttempted = false;
    final requestPaths = <String>[];
    final client = MockClient((request) async {
      requestPaths.add(request.url.path);
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        return http.Response(
          jsonEncode({
            'categories': [
              {
                'id': 'cat-1',
                'space_id': 'space-1',
                'name': 'General',
                'sort_order': 0,
              },
            ],
            'nodes': nodes,
            'voice_rooms': [],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/chats') {
        nodes.add({
          'id': 'node-new',
          'space_id': 'space-1',
          'kind': 'text_chat',
          'linked_chat': {'id': 'chat-new', 'type': 'CHAT_TYPE_CHANNEL'},
          'sort_order': 0,
          'display_name': 'announcements',
        });
        return http.Response(
          jsonEncode({'space_tree_node': nodes.single}),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/tree/nodes') {
        assignmentAttempted = true;
      }
      return http.Response(
        jsonEncode({'message': 'category assignment denied'}),
        403,
      );
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SpaceTreePanel.createTextChatButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceTreePanel.createCategoryDropdownKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text('General').last);
    await tester.pumpAndSettle();
    final categoryDropdown = find.descendant(
      of: find.byKey(SpaceTreePanel.createCategoryDropdownKey),
      matching: find.byType(DropdownButton<String>),
    );
    expect(
      tester.widget<DropdownButton<String>>(categoryDropdown).value,
      'cat-1',
    );
    await tester.enterText(
      find.byKey(SpaceTreePanel.createNameFieldKey),
      'announcements',
    );
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(
            find.byKey(SpaceTreePanel.createSubmitButtonKey),
          )
          .onPressed,
      isNotNull,
    );
    await tester.tap(find.byKey(SpaceTreePanel.createSubmitButtonKey));
    await tester.pumpAndSettle();

    expect(assignmentAttempted, isTrue, reason: requestPaths.join(', '));
    expect(
      find.text(
        'Chat was created but could not be placed in the selected category: Could not complete this action.',
      ),
      findsOneWidget,
    );
    expect(find.textContaining('category assignment denied'), findsNothing);
    expect(find.text('announcements'), findsOneWidget);
  });

  testWidgets('shows the server authorization error for a denied tree write', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        return http.Response(
          jsonEncode({'categories': [], 'nodes': [], 'voice_rooms': []}),
          200,
        );
      }
      return http.Response(
        jsonEncode({'message': 'missing tree permission'}),
        403,
      );
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(SpaceTreePanel.createCategoryButtonKey));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(SpaceTreePanel.createNameFieldKey),
      'Updates',
    );
    await tester.pump();
    await tester.tap(find.byKey(SpaceTreePanel.createSubmitButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.text('Could not create item: Could not complete this action.'),
      findsOneWidget,
    );
    expect(find.textContaining('missing tree permission'), findsNothing);
  });

  testWidgets('reconciles uncertain category placement before advising retry', (
    tester,
  ) async {
    final nodes = <Map<String, Object?>>[];
    var treeReads = 0;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        treeReads++;
        return http.Response(
          jsonEncode({
            'categories': [
              {
                'id': 'cat-1',
                'space_id': 'space-1',
                'name': 'General',
                'sort_order': 0,
              },
            ],
            'nodes': nodes,
            'voice_rooms': [],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/chats') {
        nodes.add({
          'id': 'node-new',
          'space_id': 'space-1',
          'kind': 'text_chat',
          'linked_chat': {'id': 'chat-new', 'type': 'CHAT_TYPE_CHANNEL'},
          'sort_order': 0,
          'display_name': 'announcements',
        });
        return http.Response(
          jsonEncode({'space_tree_node': nodes.single}),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/tree/nodes') {
        nodes.single['category_id'] = 'cat-1';
        return http.Response(
          jsonEncode({'message': 'response timed out'}),
          503,
        );
      }
      return http.Response('unexpected request', 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceTreePanel.createTextChatButtonKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceTreePanel.createCategoryDropdownKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text('General').last);
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(SpaceTreePanel.createNameFieldKey),
      'announcements',
    );
    await tester.pump();
    await tester.tap(find.byKey(SpaceTreePanel.createSubmitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Create text chat'), findsNothing);
    expect(
      find.text(
        'Chat was created, but its category placement could not be confirmed. Refresh and check its current placement before retrying or reordering: Could not complete this action.',
      ),
      findsOneWidget,
    );
    expect(find.textContaining('response timed out'), findsNothing);
    expect(treeReads, 2, reason: 'uncertain placement triggers reconciliation');
    expect(find.text('announcements'), findsOneWidget);
  });

  testWidgets('uncertain chat creation closes the dialog and guards retry', (
    tester,
  ) async {
    var treeReads = 0;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        treeReads++;
        return http.Response(
          jsonEncode({'categories': [], 'nodes': [], 'voice_rooms': []}),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/chats') {
        return http.Response(jsonEncode({'message': 'upstream timeout'}), 503);
      }
      return http.Response('unexpected request', 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceTreePanel.createTextChatButtonKey));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(SpaceTreePanel.createNameFieldKey),
      'maybe',
    );
    await tester.pump();
    await tester.tap(find.byKey(SpaceTreePanel.createSubmitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Create text chat'), findsNothing);
    expect(
      find.text(
        'Could not confirm whether the item was created. Refresh or check the tree before trying again: Could not complete this action.',
      ),
      findsOneWidget,
    );
    expect(find.textContaining('upstream timeout'), findsNothing);
    expect(
      treeReads,
      2,
      reason: 'best-effort reconciliation follows ambiguous create',
    );
  });

  testWidgets('category creation followed by refresh failure closes the dialog', (
    tester,
  ) async {
    var treeReads = 0;
    final client = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/spaces/space-1/tree') {
        treeReads++;
        return treeReads == 1
            ? http.Response(
                jsonEncode({'categories': [], 'nodes': [], 'voice_rooms': []}),
                200,
              )
            : http.Response(
                jsonEncode({'message': 'refresh unavailable'}),
                503,
              );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/spaces/space-1/categories') {
        return http.Response(
          jsonEncode({
            'category': {
              'id': 'cat-new',
              'space_id': 'space-1',
              'name': 'Updates',
            },
          }),
          200,
        );
      }
      return http.Response('unexpected request', 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SpaceTreePanel.createCategoryButtonKey));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(SpaceTreePanel.createNameFieldKey),
      'Updates',
    );
    await tester.pump();
    await tester.tap(find.byKey(SpaceTreePanel.createSubmitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Create category'), findsNothing);
    expect(
      find.text(
        'The request succeeded, but the tree could not be refreshed. Refresh or check the tree before trying again.',
      ),
      findsOneWidget,
    );
    expect(treeReads, 2);
  });

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
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
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

  testWidgets('shows empty state when tree has no nodes', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith(
            (_) async =>
                const SpaceTreeData(categories: [], nodes: [], voiceRooms: []),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceTreePanel.emptyKey), findsOneWidget);
    expect(find.text('No channels yet'), findsOneWidget);
  });

  testWidgets('shows error state with retry when tree load fails', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith((_) async {
            throw Exception('tree failed');
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceTreePanel.errorKey), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
  });

  testWidgets('shows voice join denial reason when join is not allowed', (
    tester,
  ) async {
    var joinCalled = false;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith((_) async => _sampleTree()),
          spacePermissionProvider.overrideWith((ref, query) async {
            if (query.permission == SpacePermissions.voiceJoin) {
              return false;
            }
            return true;
          }),
          joinVoiceRoomActionProvider.overrideWith(
            (ref) => ({required voiceRoomId, required spaceId}) async {
              joinCalled = true;
            },
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.text('You need permission to join this voice room'),
      findsOneWidget,
    );

    await tester.tap(find.text('Lobby'));
    await tester.pumpAndSettle();
    expect(joinCalled, isFalse);
  });

  testWidgets('shows channel post denial reason for channel chats', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceTreeProvider('space-1').overrideWith(
            (_) async => const SpaceTreeData(
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
                  id: 'node-channel',
                  spaceId: 'space-1',
                  categoryId: 'cat-1',
                  kind: 'text_chat',
                  chatType: 'CHAT_TYPE_CHANNEL',
                  linkedChatId: 'chat-channel',
                  sortOrder: 0,
                  displayName: 'news',
                ),
              ],
              voiceRooms: [],
            ),
          ),
          spacePermissionProvider.overrideWith((ref, query) async {
            if (query.permission == SpacePermissions.textChatSendMessages) {
              return false;
            }
            return true;
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceTreePanel(spaceId: 'space-1', onTextChatSelected: _noop),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.text('You need permission to post in this channel'),
      findsOneWidget,
    );
  });
}

void _noop(String _) {}
