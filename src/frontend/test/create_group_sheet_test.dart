import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
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

  testWidgets('ChatListPanel opens create group sheet', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({'chat_list': {'items': []}}),
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

    expect(calls, contains('POST /api/v1/chats'));
    expect(calls, contains('POST /api/v1/chats/group-new/members'));
    expect(find.byKey(CreateGroupSheet.sheetKey), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}
