import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/group_settings_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _chatId = 'group-1';
const _adminTogglePrefix = GroupSettingsPanel.adminTogglePrefix;

void main() {
  Widget testApp({required http.Client client}) {
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
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: GroupSettingsPanel(chatId: _chatId)),
      ),
    );
  }

  testWidgets('stages threads and saves only the staged change', (
    tester,
  ) async {
    var threadsEnabled = false;
    var memberPostsEnabled = false;
    final patchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse(
          threadsEnabled: threadsEnabled,
          memberPostsEnabled: memberPostsEnabled,
        );
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse(memberAdditionsEnabled: false);
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      if (req.method == 'PATCH') {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        patchBodies.add(body);
        if (body.containsKey('threads_enabled')) {
          threadsEnabled = body['threads_enabled'] as bool;
        }
        if (body.containsKey('allow_user_main_feed')) {
          memberPostsEnabled = body['allow_user_main_feed'] as bool;
        }
        return _updateResponse(
          threadsEnabled: threadsEnabled,
          memberPostsEnabled: memberPostsEnabled,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(
      _switchValue(tester, GroupSettingsPanel.threadsToggleKey),
      isFalse,
    );
    expect(
      _switchValue(tester, GroupSettingsPanel.memberPostsToggleKey),
      isFalse,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupSettingsPanel.saveKey))
          .onPressed,
      isNull,
    );

    await tester.tap(find.byKey(GroupSettingsPanel.threadsToggleKey));
    await tester.pump();
    expect(
      _switchValue(tester, GroupSettingsPanel.threadsToggleKey),
      isTrue,
    );
    expect(patchBodies, isEmpty);

    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'threads_enabled': true},
    ]);
    expect(threadsEnabled, isTrue);
    expect(
      _switchValue(tester, GroupSettingsPanel.threadsToggleKey),
      isTrue,
    );
    expect(
      _switchValue(tester, GroupSettingsPanel.memberPostsToggleKey),
      isFalse,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupSettingsPanel.saveKey))
          .onPressed,
      isNull,
    );
  });

  testWidgets('inline rename field commits the name on save', (
    tester,
  ) async {
    var currentName = 'Team radio';
    final patchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse(name: currentName);
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      if (req.method == 'PATCH') {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        patchBodies.add(body);
        final name = body['name'];
        if (name is String && name.isNotEmpty) {
          currentName = name;
        }
        return _updateResponse(name: currentName);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(find.text('Team radio'), findsOneWidget);
    expect(find.byKey(GroupSettingsPanel.nameFieldKey), findsNothing);

    await tester.tap(find.byKey(GroupSettingsPanel.nameEditKey));
    await tester.pumpAndSettle();
    expect(find.byKey(GroupSettingsPanel.nameFieldKey), findsOneWidget);

    await tester.enterText(find.byKey(GroupSettingsPanel.nameFieldKey), 'Renamed group');
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(GroupSettingsPanel.nameEditKey));
    await tester.pumpAndSettle();
    expect(find.byKey(GroupSettingsPanel.nameFieldKey), findsNothing);
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupSettingsPanel.saveKey))
          .onPressed,
      isNotNull,
    );

    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'name': 'Renamed group'},
    ]);
    expect(find.text('Renamed group'), findsOneWidget);
    expect(find.byKey(GroupSettingsPanel.nameFieldKey), findsNothing);
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupSettingsPanel.saveKey))
          .onPressed,
      isNull,
    );
  });

  testWidgets('owner promotes a member to admin on save', (tester) async {
    final roles = <String, String>{
      'prof-test': 'owner',
      'u2': 'member',
    };
    final rolePatchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          for (final entry in roles.entries)
            {'profile_id': entry.key, 'role': entry.value},
        ]);
      }
      if (req.method == 'PATCH' && req.url.path.endsWith('/role')) {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        rolePatchBodies.add(body);
        final profileId =
            req.url.pathSegments[req.url.pathSegments.length - 2];
        final role = body['role'];
        if (role is String) {
          roles[profileId] = role;
        }
        return http.Response('{}', 200);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ValueKey('${_adminTogglePrefix}u2')),
      findsOneWidget,
    );
    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u2')),
      isFalse,
    );

    await tester.tap(find.byKey(ValueKey('${_adminTogglePrefix}u2')));
    await tester.pump();
    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u2')),
      isTrue,
    );

    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(rolePatchBodies, [
      {'role': 'admin'},
    ]);
    expect(roles['u2'], 'admin');
    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u2')),
      isTrue,
    );
  });

  testWidgets('admin cannot demote a peer admin', (tester) async {
    final roles = <String, String>{
      'prof-test': 'admin',
      'u1': 'owner',
      'u3': 'admin',
    };
    final rolePatchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          for (final entry in roles.entries)
            {'profile_id': entry.key, 'role': entry.value},
        ]);
      }
      if (req.method == 'PATCH' && req.url.path.endsWith('/role')) {
        rolePatchBodies.add(jsonDecode(req.body) as Map<String, dynamic>);
        return http.Response('{}', 200);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ValueKey('${_adminTogglePrefix}u3')),
      findsOneWidget,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupSettingsPanel.saveKey))
          .onPressed,
      isNull,
    );

    await tester.tap(find.byKey(ValueKey('${_adminTogglePrefix}u3')));
    await tester.pump();

    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u3')),
      isTrue,
    );
    expect(rolePatchBodies, isEmpty);
  });

  testWidgets('owner demotes an admin to member on save', (tester) async {
    final roles = <String, String>{
      'prof-test': 'owner',
      'u3': 'admin',
    };
    final rolePatchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          for (final entry in roles.entries)
            {'profile_id': entry.key, 'role': entry.value},
        ]);
      }
      if (req.method == 'PATCH' && req.url.path.endsWith('/role')) {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        rolePatchBodies.add(body);
        final profileId =
            req.url.pathSegments[req.url.pathSegments.length - 2];
        final role = body['role'];
        if (role is String) {
          roles[profileId] = role;
        }
        return http.Response('{}', 200);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u3')),
      isTrue,
    );

    await tester.tap(find.byKey(ValueKey('${_adminTogglePrefix}u3')));
    await tester.pump();
    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u3')),
      isFalse,
    );

    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(rolePatchBodies, [
      {'role': 'member'},
    ]);
    expect(roles['u3'], 'member');
    expect(
      _switchValue(tester, ValueKey('${_adminTogglePrefix}u3')),
      isFalse,
    );
  });

  testWidgets('member additions row reflects and saves the permission', (
    tester,
  ) async {
    var memberAdditionsEnabled = false;
    final patchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse(
          memberAdditionsEnabled: memberAdditionsEnabled,
        );
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      if (req.method == 'PATCH') {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        patchBodies.add(body);
        if (body.containsKey('member_additions_enabled')) {
          memberAdditionsEnabled =
              body['member_additions_enabled'] as bool;
        }
        return _updateResponse();
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(find.text('Admins only'), findsOneWidget);

    await tester.tap(find.byKey(GroupSettingsPanel.additionsToggleKey));
    await tester.pump();
    expect(
      _switchValue(tester, GroupSettingsPanel.additionsToggleKey),
      isTrue,
    );

    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'member_additions_enabled': true},
    ]);
    expect(find.text('Enabled'), findsOneWidget);
    expect(find.text('Admins only'), findsNothing);
  });

  testWidgets('failed save keeps the staged value and allows retry', (
    tester,
  ) async {
    final patchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      if (req.method == 'PATCH') {
        patchBodies.add(jsonDecode(req.body) as Map<String, dynamic>);
        return http.Response(
          jsonEncode({'error': 'permission_denied'}),
          403,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(GroupSettingsPanel.threadsToggleKey));
    await tester.pump();
    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'threads_enabled': true},
    ]);
    expect(
      _switchValue(tester, GroupSettingsPanel.threadsToggleKey),
      isTrue,
    );
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupSettingsPanel.saveKey))
          .onPressed,
      isNotNull,
    );

    await tester.tap(find.byKey(GroupSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'threads_enabled': true},
      {'threads_enabled': true},
    ]);
  });

  testWidgets('cancel closes the draft without writing', (tester) async {
    var patchCount = 0;
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      if (req.method == 'PATCH') patchCount++;
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(GroupSettingsPanel.threadsToggleKey));
    await tester.pump();
    await tester.tap(find.byKey(GroupSettingsPanel.cancelKey));
    await tester.pumpAndSettle();

    expect(patchCount, 0);
    expect(find.byKey(GroupSettingsPanel.panelKey), findsNothing);
  });

  testWidgets('paged roster authorizes the owner on the next page', (
    tester,
  ) async {
    var memberPages = 0;
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        memberPages++;
        if (req.url.queryParameters['cursor'] == null) {
          return _membersResponse([
            {'profile_id': 'u2', 'role': 'member'},
          ], nextCursor: 'next-page');
        }
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(memberPages, greaterThanOrEqualTo(2));
    expect(
      find.byKey(GroupSettingsPanel.threadsToggleKey),
      findsOneWidget,
    );
  });

  testWidgets('hides settings for members, space groups and channels', (
    tester,
  ) async {
    for (final scenario in [
      (type: 'CHAT_TYPE_GROUP', spaceId: null, role: 'member'),
      (type: 'CHAT_TYPE_GROUP', spaceId: 'space-1', role: 'owner'),
      (type: 'CHAT_TYPE_CHANNEL', spaceId: null, role: 'owner'),
    ]) {
      final client = MockClient((req) async {
        if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
          return _chatListResponse(
            chatType: scenario.type,
            spaceId: scenario.spaceId,
          );
        }
        if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
          return _permissionsResponse();
        }
        if (req.method == 'GET' && req.url.path.endsWith('/members')) {
          return _membersResponse([
            {'profile_id': 'prof-test', 'role': scenario.role},
          ]);
        }
        return http.Response('{}', 404);
      });
      await tester.pumpWidget(testApp(client: client));
      await tester.pumpAndSettle();
      expect(
        find.byKey(GroupSettingsPanel.threadsToggleKey),
        findsNothing,
      );
    }
  });

  testWidgets('load error retries and recovers the controls', (tester) async {
    var chatListReads = 0;
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        chatListReads++;
        if (chatListReads <= 2) {
          return http.Response('{}', 500);
        }
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/permissions')) {
        return _permissionsResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();

    expect(chatListReads, 2);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
    expect(
      find.byKey(GroupSettingsPanel.threadsToggleKey),
      findsNothing,
    );

    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(chatListReads, 3);
    expect(
      find.byKey(GroupSettingsPanel.threadsToggleKey),
      findsOneWidget,
    );
  });
}

bool _switchValue(WidgetTester tester, Key key) {
  return tester.widget<SwitchListTile>(find.byKey(key)).value;
}

http.Response _chatListResponse({
  bool threadsEnabled = false,
  bool memberPostsEnabled = false,
  String chatType = 'CHAT_TYPE_GROUP',
  String name = 'Team radio',
  String? spaceId,
}) {
  return http.Response(
    jsonEncode({
      'chat_list': {
        'items': [
          {
            'chat': {
              'id': _chatId,
              'type': chatType,
              'creator_profile_id': 'prof-test',
              'space_id': spaceId,
              'name': name,
              'threads_enabled': threadsEnabled,
              'allow_user_main_feed': memberPostsEnabled,
            },
          },
        ],
      },
    }),
    200,
  );
}

http.Response _permissionsResponse({
  bool canPostAsChat = true,
  bool canAddMembers = true,
  bool canManagePermissions = true,
  bool memberAdditionsEnabled = false,
}) {
  return http.Response(
    jsonEncode({
      'can_post_as_chat': canPostAsChat,
      'can_add_members': canAddMembers,
      'can_manage_permissions': canManagePermissions,
      'member_additions_enabled': memberAdditionsEnabled,
    }),
    200,
  );
}

http.Response _updateResponse({
  bool threadsEnabled = false,
  bool memberPostsEnabled = false,
  String name = 'Team radio',
}) {
  return http.Response(
    jsonEncode({
      'chat': {
        'id': _chatId,
        'type': 'CHAT_TYPE_GROUP',
        'creator_profile_id': 'prof-test',
        'name': name,
        'threads_enabled': threadsEnabled,
        'allow_user_main_feed': memberPostsEnabled,
      },
    }),
    200,
  );
}

http.Response _membersResponse(
  List<Map<String, String>> members, {
  String? nextCursor,
}) {
  return http.Response(
    jsonEncode({
      'member_list': {'members': members, 'next_cursor': nextCursor},
    }),
    200,
  );
}
