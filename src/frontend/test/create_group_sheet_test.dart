import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/create_group_friends_provider.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/chat/create_group_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget testApp({
    required Widget home,
    required http.Client client,
    bool useApiFriendPages = false,
    List<Override> extraOverrides = const [],
  }) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        friendsListProvider.overrideWith(
          (ref) async => const FriendsListData(
            friends: ['friend-a', 'friend-b', 'friend-c'],
          ),
        ),
        if (!useApiFriendPages)
          createGroupFriendsProvider.overrideWith(
            (ref) async => const ['friend-a', 'friend-b', 'friend-c'],
          ),
        profileProvider.overrideWith((ref, profileId) async {
          return _testProfiles[profileId];
        }),
        ...extraOverrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  testWidgets('CreateGroupSheet exposes friend search', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateGroupSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async => http.Response('{}', 404)),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(
      find.widgetWithText(TextField, 'Search by name or @username'),
      findsOneWidget,
    );
  });

  testWidgets('desktop CreateGroup is a dialog with close and cancel actions', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final triggerFocus = FocusNode();
    addTearDown(triggerFocus.dispose);
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            focusNode: triggerFocus,
            onPressed: () => CreateGroupSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((request) async => http.Response('{}', 404)),
      ),
    );
    await tester.pumpAndSettle();

    triggerFocus.requestFocus();
    await tester.pump();
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.byType(Dialog), findsOneWidget);
    expect(find.byType(BottomSheet), findsNothing);
    expect(find.byKey(CreateGroupSheet.closeKey), findsOneWidget);
    expect(find.byKey(CreateGroupSheet.cancelKey), findsOneWidget);

    await tester.tap(find.byKey(CreateGroupSheet.cancelKey));
    await tester.pumpAndSettle();

    expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
    expect(triggerFocus.hasFocus, isTrue);
  });

  testWidgets('mobile CreateGroup fills the viewport and exposes close', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateGroupSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((request) async => http.Response('{}', 404)),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.byType(BottomSheet), findsOneWidget);
    expect(find.byType(Dialog), findsNothing);
    expect(tester.getSize(find.byType(BottomSheet)).height, greaterThan(800));
    expect(find.byKey(CreateGroupSheet.closeKey), findsOneWidget);
    expect(find.byKey(CreateGroupSheet.cancelKey), findsNothing);

    await tester.tap(find.byKey(CreateGroupSheet.closeKey));
    await tester.pumpAndSettle();
    expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
  });

  testWidgets(
    'DM group creation keeps the original peer and invites a friend',
    (tester) async {
      final requests = <http.Request>[];
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) => TextButton(
              onPressed: () => CreateGroupSheet.show(
                context,
                requiredMemberProfileId: 'dm-peer',
              ),
              child: const Text('open'),
            ),
          ),
          client: MockClient((request) async {
            requests.add(request);
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': 'group-from-dm',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Weekend plans',
                    'creator_profile_id': 'profile-me',
                  },
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats/group-from-dm/members') {
              return http.Response('', 204);
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('dm-peer')),
        findsOneWidget,
      );
      final requiredPeer = tester.widget<CheckboxListTile>(
        find.byKey(CreateGroupSheet.memberTileKey('dm-peer')),
      );
      expect(requiredPeer.value, isTrue);
      expect(requiredPeer.onChanged, isNull);
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-a')),
        findsOneWidget,
      );
      expect(
        tester
            .widget<FilledButton>(find.byKey(CreateGroupSheet.submitKey))
            .onPressed,
        isNull,
      );

      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Weekend plans',
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
      await tester.pump();
      expect(
        find.byKey(CreateGroupSheet.submitKey).hitTestable(),
        findsOneWidget,
      );
      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      final inviteRequest = requests.singleWhere(
        (request) =>
            request.method == 'POST' &&
            request.url.path == '/api/v1/chats/group-from-dm/members',
      );
      final profileIds =
          (jsonDecode(inviteRequest.body)
                  as Map<String, dynamic>)['profile_ids']
              as List<dynamic>;
      expect(profileIds, containsAll(['dm-peer', 'friend-a']));
      expect(profileIds, hasLength(2));
      expect(
        requests.where(
          (request) =>
              request.method == 'POST' && request.url.path == '/api/v1/chats',
        ),
        hasLength(1),
      );
    },
  );

  testWidgets('DM group invalid form and dismissal make no requests', (
    tester,
  ) async {
    final requests = <http.Request>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateGroupSheet.show(
              context,
              requiredMemberProfileId: 'dm-peer',
            ),
            child: const Text('open'),
          ),
        ),
        client: MockClient((request) async {
          requests.add(request);
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateGroupSheet.nameFieldKey), 'Plans');
    expect(
      tester
          .widget<FilledButton>(find.byKey(CreateGroupSheet.submitKey))
          .onPressed,
      isNull,
    );
    expect(requests, isEmpty);

    Navigator.of(tester.element(find.byKey(CreateGroupSheet.sheetKey))).pop();
    await tester.pumpAndSettle();
    expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
    expect(requests, isEmpty);
  });

  testWidgets('DM group sheet blocks submission after viewer profile changes', (
    tester,
  ) async {
    final calls = <String>[];
    late ProviderContainer container;
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) {
            container = ProviderScope.containerOf(context);
            return TextButton(
              onPressed: () => CreateGroupSheet.show(
                context,
                requiredMemberProfileId: 'dm-peer',
                expectedViewerProfileId: 'prof-test',
              ),
              child: const Text('open'),
            );
          },
        ),
        client: MockClient((request) async {
          calls.add('${request.method} ${request.url.path}');
          return http.Response('{}', 500);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateGroupSheet.nameFieldKey), 'Squad');
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
    await tester.pump();

    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'profile-b-token',
        refreshToken: 'profile-b-refresh',
        accountId: 'acc-test',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();

    expect(
      tester
          .widget<FilledButton>(find.byKey(CreateGroupSheet.submitKey))
          .onPressed,
      isNull,
    );
    expect(
      calls.where((call) => call.startsWith('POST /api/v1/chats')),
      isEmpty,
    );
    expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
  });

  testWidgets(
    'searching across names and handles preserves hidden selections on submit',
    (tester) async {
      final requests = <http.Request>[];
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) => TextButton(
              onPressed: () => CreateGroupSheet.show(context),
              child: const Text('open'),
            ),
          ),
          useApiFriendPages: true,
          client: MockClient((request) async {
            requests.add(request);
            if (request.method == 'GET' &&
                request.url.path == '/api/v1/friends') {
              if (request.url.queryParameters['cursor'] == null) {
                return http.Response(
                  jsonEncode({
                    'friends': [
                      {'profile_id': 'friend-a'},
                      {'profile_id': 'friend-b'},
                    ],
                    'next_cursor': 'page-2',
                  }),
                  200,
                );
              }
              expect(request.url.queryParameters['cursor'], 'page-2');
              return http.Response(
                jsonEncode({
                  'friends': [
                    {'profile_id': 'friend-c'},
                  ],
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': 'group-search',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Squad',
                    'creator_profile_id': 'profile-me',
                  },
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats/group-search/members') {
              return http.Response('', 204);
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Squad',
      );
      await tester.pump();
      final search = find.byKey(CreateGroupSheet.searchFieldKey);

      await tester.enterText(search, '@charlie');
      await tester.pumpAndSettle();
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-c')),
        findsOneWidget,
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-c')));
      await tester.pump();

      await tester.enterText(search, 'alice');
      await tester.pumpAndSettle();
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-a')),
        findsOneWidget,
      );
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-c')),
        findsNothing,
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
      await tester.pump();

      await tester.enterText(search, 'no-such-friend');
      await tester.pumpAndSettle();
      expect(find.text('No profiles found'), findsOneWidget);
      expect(
        find.byKey(CreateGroupSheet.submitKey).hitTestable(),
        findsOneWidget,
      );

      await tester.enterText(search, '');
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<CheckboxListTile>(
              find.byKey(CreateGroupSheet.memberTileKey('friend-c')),
            )
            .value,
        isTrue,
      );
      expect(
        tester
            .widget<CheckboxListTile>(
              find.byKey(CreateGroupSheet.memberTileKey('friend-a')),
            )
            .value,
        isTrue,
      );
      expect(
        find.text('Select at least 2 friends to create a group.'),
        findsNothing,
      );
      expect(
        tester
            .widget<FilledButton>(find.byKey(CreateGroupSheet.submitKey))
            .onPressed,
        isNotNull,
      );
      expect(
        find.byKey(CreateGroupSheet.submitKey).hitTestable(),
        findsOneWidget,
      );
      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      expect(
        requests.map((request) => '${request.method} ${request.url.path}'),
        contains('POST /api/v1/chats'),
      );
      final createRequest = requests.singleWhere(
        (request) =>
            request.method == 'POST' && request.url.path == '/api/v1/chats',
      );
      final inviteRequest = requests.singleWhere(
        (request) =>
            request.method == 'POST' &&
            request.url.path == '/api/v1/chats/group-search/members',
      );
      expect(jsonDecode(createRequest.body), containsPair('name', 'Squad'));
      expect(jsonDecode(inviteRequest.body), {
        'profile_ids': ['friend-c', 'friend-a'],
      });
      expect(
        requests
            .where(
              (request) =>
                  request.method == 'GET' &&
                  request.url.path == '/api/v1/friends',
            )
            .map((request) => request.url.queryParameters['cursor'])
            .toList(),
        [null, 'page-2'],
      );
    },
  );

  testWidgets(
    'profile lookup failures block partial search results and retry',
    (tester) async {
      var failBob = true;
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) => TextButton(
              onPressed: () => CreateGroupSheet.show(context),
              child: const Text('open'),
            ),
          ),
          client: MockClient((request) async => http.Response('{}', 404)),
          extraOverrides: [
            profileProvider.overrideWith((ref, profileId) async {
              if (profileId == 'friend-b' && failBob) {
                throw Exception('private profile upstream detail');
              }
              return _testProfiles[profileId];
            }),
          ],
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateGroupSheet.searchFieldKey),
        'bob',
      );
      await tester.pumpAndSettle();

      expect(find.text('No profiles found'), findsNothing);
      expect(find.text('private profile upstream detail'), findsNothing);
      expect(find.byIcon(Icons.cloud_off_outlined), findsOneWidget);
      expect(find.text('Try again'), findsOneWidget);

      failBob = false;
      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(find.text('Bob Example'), findsOneWidget);
    },
  );

  testWidgets(
    'later friend page failure hides partial results and retry restarts paging',
    (tester) async {
      final requests = <http.Request>[];
      var pageTwoAvailable = false;
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) => TextButton(
              onPressed: () => CreateGroupSheet.show(context),
              child: const Text('open'),
            ),
          ),
          useApiFriendPages: true,
          client: MockClient((request) async {
            requests.add(request);
            if (request.method == 'GET' &&
                request.url.path == '/api/v1/friends') {
              if (request.url.queryParameters['cursor'] == null) {
                return http.Response(
                  jsonEncode({
                    'friends': [
                      {'profile_id': 'friend-a'},
                      {'profile_id': 'friend-b'},
                    ],
                    'next_cursor': 'page-2',
                  }),
                  200,
                );
              }
              expect(request.url.queryParameters['cursor'], 'page-2');
              if (!pageTwoAvailable) {
                return http.Response('{}', 503);
              }
              return http.Response(
                jsonEncode({
                  'friends': [
                    {'profile_id': 'friend-c'},
                  ],
                }),
                200,
              );
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-a')),
        findsNothing,
      );
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-b')),
        findsNothing,
      );
      expect(find.text('Try again'), findsOneWidget);
      expect(
        requests
            .where((request) => request.url.path == '/api/v1/friends')
            .map((request) => request.url.queryParameters['cursor'])
            .toList(),
        [null, 'page-2'],
      );

      pageTwoAvailable = true;
      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();

      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-a')),
        findsOneWidget,
      );
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-b')),
        findsOneWidget,
      );
      expect(
        find.byKey(CreateGroupSheet.memberTileKey('friend-c')),
        findsOneWidget,
      );
      expect(
        requests
            .where((request) => request.url.path == '/api/v1/friends')
            .map((request) => request.url.queryParameters['cursor'])
            .toList(),
        [null, 'page-2', null, 'page-2'],
      );
    },
  );

  testWidgets('ChatListPanel opens create group sheet', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ChatListPanel.createGroupKey));
    await tester.pumpAndSettle();

    expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
    expect(find.text('New group'), findsOneWidget);
  });

  testWidgets('CreateGroupSheet submits group create + invite', (tester) async {
    final calls = <String>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateGroupSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async {
          calls.add('${req.method} ${req.url.path}');
          if (req.method == 'POST' && req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat': {
                  'id': 'group-new',
                  'type': 'CHAT_TYPE_GROUP',
                  'name': 'Squad',
                  'creator_profile_id': 'profile-me',
                },
              }),
              200,
            );
          }
          if (req.method == 'POST' &&
              req.url.path == '/api/v1/chats/group-new/members') {
            return http.Response('', 204);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(CreateGroupSheet.nameFieldKey), 'Squad');
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
    await tester.pump();

    await tester.tap(find.byKey(CreateGroupSheet.submitKey));
    await tester.pumpAndSettle();

    expect(calls, contains('POST /api/v1/chats'));
    expect(calls, contains('POST /api/v1/chats/group-new/members'));
    expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
  });

  testWidgets(
    'replays an ambiguous create with the same request id and rotates it for a new form attempt',
    (tester) async {
      final createBodies = <Map<String, dynamic>>[];
      final createdGroups = <String, String>{};
      var createAttempts = 0;
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) => TextButton(
              onPressed: () => CreateGroupSheet.show(context),
              child: const Text('open'),
            ),
          ),
          client: MockClient((request) async {
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              final body = jsonDecode(request.body) as Map<String, dynamic>;
              createBodies.add(body);
              final requestId = body['request_id'] as String;
              final chatId = createdGroups.putIfAbsent(
                requestId,
                () => 'group-${createdGroups.length + 1}',
              );
              createAttempts++;
              if (createAttempts <= 3) {
                // The server may have committed, but the client lost its response.
                return http.Response(jsonEncode({'error': 'unavailable'}), 503);
              }
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': chatId,
                    'type': 'CHAT_TYPE_GROUP',
                    'name': body['name'],
                    'creator_profile_id': 'profile-me',
                  },
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats/group-2/members') {
              return http.Response('', 204);
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Squad',
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
      await tester.pump();

      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();
      final firstRequestId = createBodies.single['request_id'];
      expect(firstRequestId, isA<String>());
      expect(firstRequestId, isNotEmpty);

      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      // Editing the form creates a new logical request; retrying that attempt
      // must keep its key and exact CreateChat body.
      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Updated squad',
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-c')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      expect(createBodies, hasLength(4));
      expect(createBodies[0], createBodies[1]);
      expect(
        createBodies[1]['request_id'],
        isNot(createBodies[2]['request_id']),
      );
      expect(createBodies[2], createBodies[3]);
      expect(createdGroups, hasLength(2));
      expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
    },
  );

  testWidgets(
    'keeps the CreateChat request id across same-session 401 token refresh',
    (tester) async {
      final createBodies = <Map<String, dynamic>>[];
      final createAuthorizations = <String?>[];
      late ProviderContainer container;
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) {
              container = ProviderScope.containerOf(context);
              return TextButton(
                onPressed: () => CreateGroupSheet.show(context),
                child: const Text('open'),
              );
            },
          ),
          client: MockClient((request) async {
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              createBodies.add(
                jsonDecode(request.body) as Map<String, dynamic>,
              );
              createAuthorizations.add(request.headers['authorization']);
              if (createBodies.length == 1) {
                return http.Response(
                  jsonEncode({'error': 'invalid_token'}),
                  401,
                );
              }
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': 'group-after-refresh',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Refresh-safe group',
                    'creator_profile_id': 'prof-test',
                  },
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/auth/refresh') {
              expect(
                (jsonDecode(request.body)
                    as Map<String, dynamic>)['refresh_token'],
                'test-refresh',
              );
              return http.Response(
                jsonEncode({
                  'session': {
                    'access_token': 'test-access-renewed',
                    'refresh_token': 'test-refresh-renewed',
                    'expires_in_seconds': 900,
                    'account_id': 'acc-test',
                    'profile_id': 'prof-test',
                  },
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path ==
                    '/api/v1/chats/group-after-refresh/members') {
              expect(
                request.headers['authorization'],
                'Bearer test-access-renewed',
              );
              return http.Response('', 204);
            }
            return http.Response('{}', 404);
          }),
          extraOverrides: [
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Refresh-safe group',
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
      await tester.pump();

      final authController = container.read(authControllerProvider.notifier);
      final originalIdentity = authController.gatewayRequestIdentity!;
      final originalInstallGeneration = authController.sessionInstallGeneration;
      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      expect(createBodies, hasLength(2));
      expect(createBodies[1], createBodies[0]);
      expect(createBodies[0]['request_id'], isNotEmpty);
      expect(createAuthorizations, [
        'Bearer test-access',
        'Bearer test-access-renewed',
      ]);
      expect(authController.gatewayRequestIdentity, originalIdentity);
      expect(
        authController.sessionInstallGeneration,
        originalInstallGeneration,
      );
      expect(container.read(selectedChatIdProvider), 'group-after-refresh');
      expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
    },
  );

  testWidgets(
    'does not replay a CreateChat after identity replacement during 401 refresh',
    (tester) async {
      final createBodies = <Map<String, dynamic>>[];
      final refreshStarted = Completer<void>();
      final finishRefresh = Completer<http.Response>();
      late ProviderContainer container;
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) {
              container = ProviderScope.containerOf(context);
              return TextButton(
                onPressed: () => CreateGroupSheet.show(context),
                child: const Text('open'),
              );
            },
          ),
          client: MockClient((request) async {
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              createBodies.add(
                jsonDecode(request.body) as Map<String, dynamic>,
              );
              return http.Response(jsonEncode({'error': 'invalid_token'}), 401);
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/auth/refresh') {
              refreshStarted.complete();
              return finishRefresh.future;
            }
            return http.Response('{}', 404);
          }),
          extraOverrides: [
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Identity-bound group',
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
      await tester.pump();

      final authController = container.read(authControllerProvider.notifier);
      final originalIdentity = authController.gatewayRequestIdentity!;
      final originalInstallGeneration = authController.sessionInstallGeneration;
      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pump();
      await refreshStarted.future;
      await container
          .read(authControllerProvider.notifier)
          .applySession(
            const AuthSession(
              accessToken: 'replacement-access',
              refreshToken: 'replacement-refresh',
              accountId: 'acc-test',
              activeProfileId: 'profile-b',
              expiresInSeconds: 900,
            ),
          );
      expect(
        authController.gatewayRequestIdentity!.generation,
        isNot(originalIdentity.generation),
      );
      expect(
        authController.sessionInstallGeneration,
        greaterThan(originalInstallGeneration),
      );
      finishRefresh.complete(
        http.Response(
          jsonEncode({
            'session': {
              'access_token': 'stale-refreshed-access',
              'refresh_token': 'stale-refreshed-refresh',
              'expires_in_seconds': 900,
              'account_id': 'acc-test',
              'profile_id': 'prof-test',
            },
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();

      expect(createBodies, hasLength(1));
      expect(createBodies.single['request_id'], isA<String>());
      expect(
        container.read(authControllerProvider).activeProfileId,
        'profile-b',
      );
      expect(
        container.read(selectedChatIdProvider),
        isNot('group-after-refresh'),
      );
      expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
    },
  );

  testWidgets('does not reuse a create request id after profile changes', (
    tester,
  ) async {
    final createBodies = <Map<String, dynamic>>[];
    var createAttempts = 0;
    late ProviderContainer container;
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) {
            container = ProviderScope.containerOf(context);
            return TextButton(
              onPressed: () => CreateGroupSheet.show(context),
              child: const Text('open'),
            );
          },
        ),
        client: MockClient((request) async {
          if (request.method == 'POST' && request.url.path == '/api/v1/chats') {
            final body = jsonDecode(request.body) as Map<String, dynamic>;
            createBodies.add(body);
            createAttempts++;
            if (createAttempts == 1) {
              return http.Response(jsonEncode({'error': 'unavailable'}), 503);
            }
            return http.Response(
              jsonEncode({
                'chat': {
                  'id': 'group-profile-b',
                  'type': 'CHAT_TYPE_GROUP',
                  'name': body['name'],
                  'creator_profile_id': 'profile-b',
                },
              }),
              200,
            );
          }
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/chats/group-profile-b/members') {
            return http.Response('', 204);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateGroupSheet.nameFieldKey), 'Squad');
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.submitKey));
    await tester.pumpAndSettle();

    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'profile-b-token',
        refreshToken: 'profile-b-refresh',
        accountId: 'acc-test',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.submitKey));
    await tester.pumpAndSettle();

    expect(createBodies, hasLength(2));
    expect(createBodies[0]['request_id'], isNot(createBodies[1]['request_id']));
    expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
  });

  testWidgets(
    'CreateGroupSheet retries failed member invite on the created group',
    (tester) async {
      final requests = <http.Request>[];
      var inviteAttempts = 0;
      late ProviderContainer container;
      await tester.pumpWidget(
        testApp(
          home: Builder(
            builder: (context) {
              container = ProviderScope.containerOf(context);
              return TextButton(
                onPressed: () => CreateGroupSheet.show(context),
                child: const Text('open'),
              );
            },
          ),
          client: MockClient((request) async {
            requests.add(request);
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': 'group-retry',
                    'type': 'CHAT_TYPE_GROUP',
                    'name': 'Squad',
                    'creator_profile_id': 'profile-me',
                  },
                }),
                200,
              );
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats/group-retry/members') {
              inviteAttempts++;
              if (inviteAttempts == 1) {
                return http.Response(
                  jsonEncode({'message': 'private_member_diagnostic'}),
                  503,
                );
              }
              return http.Response('', 204);
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(CreateGroupSheet.nameFieldKey),
        'Squad',
      );
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
      await tester.pump();
      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      expect(
        requests.where(
          (request) =>
              request.method == 'POST' && request.url.path == '/api/v1/chats',
        ),
        hasLength(1),
      );
      expect(inviteAttempts, 1);
      expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
      expect(find.text('Could not complete this action.'), findsOneWidget);
      expect(find.text('private_member_diagnostic'), findsNothing);
      expect(find.text('Try again'), findsOneWidget);

      await tester.tap(find.byKey(CreateGroupSheet.submitKey));
      await tester.pumpAndSettle();

      final createRequests = requests
          .where(
            (request) =>
                request.method == 'POST' && request.url.path == '/api/v1/chats',
          )
          .toList(growable: false);
      final inviteRequests = requests
          .where(
            (request) =>
                request.method == 'POST' &&
                request.url.path == '/api/v1/chats/group-retry/members',
          )
          .toList(growable: false);
      expect(createRequests, hasLength(1));
      expect(inviteRequests, hasLength(2));
      expect(
        (jsonDecode(inviteRequests[0].body)
            as Map<String, dynamic>)['profile_ids'],
        ['friend-a', 'friend-b'],
      );
      expect(inviteRequests[1].body, inviteRequests[0].body);
      expect(container.read(selectedChatIdProvider), 'group-retry');
      expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
    },
  );

  testWidgets('CreateGroupSheet hides upstream failure details', (
    tester,
  ) async {
    final calls = <String>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateGroupSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async {
          calls.add('${req.method} ${req.url.path}');
          if (req.method == 'POST' && req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({'message': 'private_group_gateway_diagnostic'}),
              500,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<FilledButton>(find.byKey(CreateGroupSheet.submitKey))
          .onPressed,
      isNull,
    );
    expect(calls, isNot(contains('POST /api/v1/chats')));

    await tester.enterText(find.byKey(CreateGroupSheet.nameFieldKey), 'Squad');
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
    await tester.pump();
    expect(
      find.byKey(CreateGroupSheet.submitKey).hitTestable(),
      findsOneWidget,
    );

    await tester.tap(find.byKey(CreateGroupSheet.submitKey));
    await tester.pumpAndSettle();

    expect(calls, contains('POST /api/v1/chats'));
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('private_group_gateway_diagnostic'), findsNothing);
    expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
  });

  testWidgets('CreateGroupSheet retains the local not-authenticated message', (
    tester,
  ) async {
    final calls = <String>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () => CreateGroupSheet.show(context),
            child: const Text('open'),
          ),
        ),
        client: MockClient((req) async {
          calls.add('${req.method} ${req.url.path}');
          return http.Response('{}', 500);
        }),
        extraOverrides: [authorizationHeaderProvider.overrideWithValue(null)],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(CreateGroupSheet.nameFieldKey), 'Squad');
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-a')));
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.memberTileKey('friend-b')));
    await tester.pump();
    await tester.tap(find.byKey(CreateGroupSheet.submitKey));
    await tester.pumpAndSettle();

    expect(
      find.text('Could not create group: not_authenticated'),
      findsOneWidget,
    );
    expect(
      calls.where((call) => call.startsWith('POST /api/v1/chats')),
      isEmpty,
    );
    expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
  });
}

final _testProfiles = <String, VoiceProfile>{
  'friend-a': const VoiceProfile(
    id: 'friend-a',
    accountId: 'account-a',
    username: 'alice',
    discriminator: '0001',
    displayName: 'Alice Example',
  ),
  'friend-b': const VoiceProfile(
    id: 'friend-b',
    accountId: 'account-b',
    username: 'bob',
    discriminator: '0002',
    displayName: 'Bob Example',
  ),
  'friend-c': const VoiceProfile(
    id: 'friend-c',
    accountId: 'account-c',
    username: 'charlie',
    discriminator: '0003',
    displayName: 'Charlie Example',
  ),
};

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
