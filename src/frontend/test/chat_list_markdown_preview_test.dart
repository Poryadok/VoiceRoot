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
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('ChatListPanel shows plain markdown preview from API', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/chats') {
                return http.Response(
                  jsonEncode({
                    'chat_list': {
                      'items': [
                        {
                          'chat': {
                            'id': 'chat-md',
                            'type': 'CHAT_TYPE_DM',
                            'creator_profile_id': 'profile-a',
                          },
                          'last_message_preview': 'bold preview',
                          'unread_count': 1,
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('bold preview'), findsOneWidget);
    expect(find.text('**bold**'), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
