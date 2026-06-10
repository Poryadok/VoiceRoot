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
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/markdown_test_helpers.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Widget tests for PLAN Phase 4 reactions — emoji chips with counters.
void main() {
  Widget reactionsTestApp({
    required http.Client client,
    String chatId = 'chat-abc',
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
        selectedChatIdProvider.overrideWith((ref) => chatId),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: ChatRoomPanel(chatId: chatId)),
      ),
    );
  }

  testWidgets('ChatRoomPanel shows reaction chips with counters', (tester) async {
    tester.view.physicalSize = const Size(800, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      reactionsTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return utf8JsonResponse(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-1',
                      'chat': {'id': 'chat-abc'},
                      'sender_profile_id': 'profile-b',
                      'content': 'React here',
                      'reactions_json': jsonEncode([
                        {'emoji': '👍', 'count': 2, 'reacted_by_me': false},
                        {'emoji': '🔥', 'count': 1, 'reacted_by_me': false},
                      ]),
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expectMessagePlainText(tester, 'React here');
    expect(
      find.byKey(const ValueKey('message_reaction_msg-1_👍')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('message_reaction_msg-1_🔥')),
      findsOneWidget,
    );
    expect(find.text('2'), findsOneWidget);
  });

  testWidgets('tapping reaction chip toggles via reactions API', (tester) async {
    tester.view.physicalSize = const Size(800, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    var addCalled = false;
    await tester.pumpWidget(
      reactionsTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return utf8JsonResponse(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-1',
                      'chat': {'id': 'chat-abc'},
                      'sender_profile_id': 'profile-b',
                      'content': 'Tap react',
                      'reactions_json': jsonEncode([
                        {'emoji': '👍', 'count': 1, 'reacted_by_me': false},
                      ]),
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
            );
          }
          if (req.url.path == '/api/v1/messages/msg-1/reactions' &&
              req.method == 'POST') {
            addCalled = true;
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            expect(body['emoji'], '👍');
            return http.Response('{}', 204);
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('message_reaction_msg-1_👍')));
    await tester.pumpAndSettle();

    expect(addCalled, isTrue);
  });

  testWidgets('message actions sheet offers add reaction', (tester) async {
    tester.view.physicalSize = const Size(800, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      reactionsTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-1',
                      'chat': {'id': 'chat-abc'},
                      'sender_profile_id': 'profile-b',
                      'content': 'Long press me',
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.longPress(messagePlainTextFinder('Long press me'));
    await tester.pumpAndSettle();

    expect(find.text('Add reaction'), findsOneWidget);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  Future<void> disconnect() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  void typingStart(String chatId) {}

  @override
  void typingStop(String chatId) {}

  @override
  Future<void> markRead(String chatId, String messageId) async {}

  @override
  Future<void> deliveryAck({
    required String chatId,
    required String messageId,
    required String senderProfileId,
  }) async {}
}
