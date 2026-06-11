import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/group_members_sheet.dart';

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

  testWidgets('group room shows members sheet with owner badge and kick', (
    tester,
  ) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(400, 800));
    const chatId = 'group-roles-1';
    const ownerId = 'prof-test';
    const memberId = 'profile-member';

    await tester.pumpWidget(
      testApp(
        home: MediaQuery(
          data: const MediaQueryData(size: Size(400, 800)),
          child: const SizedBox(
            width: 400,
            height: 800,
            child: ChatRoomPanel(chatId: chatId),
          ),
        ),
        client: MockClient((req) async {
          final path = req.url.path;
          if (path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': chatId,
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': ownerId,
                        'name': 'Roles squad',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (path == '/api/v1/chats/$chatId/messages') {
            return http.Response(
              jsonEncode({'message_list': {'messages': []}}),
              200,
            );
          }
          if (path == '/api/v1/chats/$chatId/members') {
            return http.Response(
              jsonEncode({
                'member_list': {
                  'members': [
                    {'profile_id': ownerId, 'role': 'owner'},
                    {'profile_id': memberId, 'role': 'member'},
                  ],
                },
              }),
              200,
            );
          }
          if (path.startsWith('/api/v1/users/profiles/')) {
            final id = path.split('/').last;
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': id,
                  'account_id': 'acc-$id',
                  'handle': '@$id',
                  'display_name': id,
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.groupMembersKey), findsOneWidget);
    await tester.tap(find.byKey(ChatRoomPanel.groupMembersKey));
    await tester.pumpAndSettle();

    expect(find.byKey(GroupMembersSheet.sheetKey), findsOneWidget);
    expect(find.text('Owner'), findsOneWidget);
    expect(
      find.byKey(GroupMembersSheet.kickMemberKey(memberId)),
      findsOneWidget,
    );
    expect(
      find.byKey(GroupMembersSheet.kickMemberKey(ownerId)),
      findsNothing,
    );
    expect(find.byKey(GroupMembersSheet.ownerLeaveHintKey), findsOneWidget);
    expect(find.byKey(GroupMembersSheet.leaveKey), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> disconnect() async {}
}
