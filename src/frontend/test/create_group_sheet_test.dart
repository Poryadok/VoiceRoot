import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
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
