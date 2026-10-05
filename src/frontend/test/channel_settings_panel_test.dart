import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/channel_settings_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _chatId = 'channel-1';

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
        home: Scaffold(body: ChannelSettingsPanel(chatId: _chatId)),
      ),
    );
  }

  testWidgets('loads stored false values and saves only staged changes', (
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
      _switchValue(tester, ChannelSettingsPanel.threadsToggleKey),
      isFalse,
    );
    expect(
      _switchValue(tester, ChannelSettingsPanel.memberPostsToggleKey),
      isFalse,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChannelSettingsPanel.saveKey))
          .onPressed,
      isNull,
    );

    await tester.tap(find.byKey(ChannelSettingsPanel.threadsToggleKey));
    await tester.pump();
    expect(_switchValue(tester, ChannelSettingsPanel.threadsToggleKey), isTrue);
    expect(patchBodies, isEmpty);

    await tester.tap(find.byKey(ChannelSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'threads_enabled': true},
    ]);
    expect(threadsEnabled, isTrue);
    expect(_switchValue(tester, ChannelSettingsPanel.threadsToggleKey), isTrue);
    expect(
      _switchValue(tester, ChannelSettingsPanel.memberPostsToggleKey),
      isFalse,
    );
  });

  testWidgets('save failure retains the staged values and allows retry', (
    tester,
  ) async {
    final patchBodies = <Map<String, dynamic>>[];
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'admin'},
        ]);
      }
      if (req.method == 'PATCH') {
        patchBodies.add(jsonDecode(req.body) as Map<String, dynamic>);
        return http.Response(jsonEncode({'error': 'permission_denied'}), 403);
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChannelSettingsPanel.memberPostsToggleKey));
    await tester.pump();
    await tester.tap(find.byKey(ChannelSettingsPanel.saveKey));
    await tester.pumpAndSettle();

    expect(patchBodies, [
      {'allow_user_main_feed': true},
    ]);
    expect(
      _switchValue(tester, ChannelSettingsPanel.memberPostsToggleKey),
      isTrue,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(ChannelSettingsPanel.saveKey))
          .onPressed,
      isNotNull,
    );
    expect(find.text('Could not complete this action.'), findsOneWidget);
  });

  testWidgets('cancel closes the draft without writing', (tester) async {
    var patchCount = 0;
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
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
    await tester.tap(find.byKey(ChannelSettingsPanel.threadsToggleKey));
    await tester.pump();
    await tester.tap(find.byKey(ChannelSettingsPanel.cancelKey));
    await tester.pumpAndSettle();

    expect(patchCount, 0);
    expect(find.byKey(ChannelSettingsPanel.panelKey), findsNothing);
  });

  testWidgets('paged absence does not hide an owner on the next page', (
    tester,
  ) async {
    var memberPage = 0;
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        memberPage++;
        if (req.url.queryParameters['cursor'] == null) {
          return _membersResponse([
            {'profile_id': 'another-profile', 'role': 'member'},
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

    expect(memberPage, 2);
    expect(find.byKey(ChannelSettingsPanel.threadsToggleKey), findsOneWidget);
  });

  testWidgets('hides controls for members, groups, and Space channels', (
    tester,
  ) async {
    for (final scenario in [
      (type: 'CHAT_TYPE_CHANNEL', spaceId: null, role: 'member'),
      (type: 'CHAT_TYPE_GROUP', spaceId: null, role: 'owner'),
      (type: 'CHAT_TYPE_CHANNEL', spaceId: 'space-1', role: 'owner'),
    ]) {
      final client = MockClient((req) async {
        if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
          return _chatListResponse(
            chatType: scenario.type,
            spaceId: scenario.spaceId,
          );
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
      expect(find.byKey(ChannelSettingsPanel.threadsToggleKey), findsNothing);
    }
  });

  testWidgets('stale save completion does not reload under a new profile', (
    tester,
  ) async {
    final pendingPatch = Completer<http.Response>();
    var patchCount = 0;
    var chatListReads = 0;
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/chats') {
        chatListReads++;
        return _chatListResponse();
      }
      if (req.method == 'GET' && req.url.path.endsWith('/members')) {
        return _membersResponse([
          {'profile_id': 'prof-test', 'role': 'owner'},
        ]);
      }
      if (req.method == 'PATCH') {
        patchCount++;
        return pendingPatch.future;
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(testApp(client: client));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChannelSettingsPanel.threadsToggleKey));
    await tester.pump();
    await tester.tap(find.byKey(ChannelSettingsPanel.saveKey));
    await tester.pump();
    expect(patchCount, 1);
    await tester.tap(find.byKey(ChannelSettingsPanel.saveKey));
    await tester.pump();
    expect(patchCount, 1);

    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChannelSettingsPanel)),
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'new-access',
        refreshToken: 'new-refresh',
        accountId: 'new-account',
        activeProfileId: 'new-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();
    final readsAfterProfileSwitch = chatListReads;
    pendingPatch.complete(_updateResponse(threadsEnabled: true));
    await tester.pumpAndSettle();

    expect(chatListReads, readsAfterProfileSwitch);
    expect(patchCount, 1);
  });
}

bool _switchValue(WidgetTester tester, Key key) {
  return tester.widget<SwitchListTile>(find.byKey(key)).value;
}

http.Response _chatListResponse({
  bool threadsEnabled = false,
  bool memberPostsEnabled = false,
  String chatType = 'CHAT_TYPE_CHANNEL',
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

http.Response _updateResponse({
  bool threadsEnabled = false,
  bool memberPostsEnabled = false,
}) {
  return http.Response(
    jsonEncode({
      'chat': {
        'id': _chatId,
        'type': 'CHAT_TYPE_CHANNEL',
        'creator_profile_id': 'prof-test',
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
