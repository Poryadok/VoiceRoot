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

void main() {
  Widget pinsTestApp({required http.Client client, String chatId = 'chat-abc'}) {
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

  testWidgets('ChatRoomPanel shows pinned messages bar', (tester) async {
    tester.view.physicalSize = const Size(800, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      pinsTestApp(
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
                      'content': 'Pinned announcement',
                      'is_pinned': true,
                      'reactions_json': '[]',
                      'mentions_json': '[]',
                      'attachments_json': '[]',
                      'type': 'regular',
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
            );
          }
          if (req.url.path == '/api/v1/chats/chat-abc/pinned-messages') {
            return utf8JsonResponse(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-1',
                      'chat': {'id': 'chat-abc'},
                      'sender_profile_id': 'profile-b',
                      'content': 'Pinned announcement',
                      'is_pinned': true,
                      'reactions_json': '[]',
                      'mentions_json': '[]',
                      'attachments_json': '[]',
                      'type': 'regular',
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

    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsOneWidget);
    expect(find.textContaining('pinned message'), findsOneWidget);
    expectMessagePlainText(tester, 'Pinned announcement');
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
