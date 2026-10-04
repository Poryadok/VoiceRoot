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
import 'package:voice_frontend/ui/space/space_chat_slow_mode_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('slow mode update failure hides upstream details', (
    tester,
  ) async {
    const raw = 'internal gateway key=slow-mode-secret';
    await tester.pumpWidget(
      ProviderScope(
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
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/chats' &&
                  request.method == 'GET') {
                return http.Response(
                  jsonEncode({
                    'chat_list': {'items': []},
                  }),
                  200,
                );
              }
              if (request.url.path == '/api/v1/chats/test-slow-mode' &&
                  request.method == 'PATCH') {
                return http.Response(
                  jsonEncode({'error': 'internal_error', 'message': raw}),
                  500,
                );
              }
              return http.Response('{}', 200);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                onPressed: () => SpaceChatSlowModeSheet.show(
                  context,
                  chatId: 'test-slow-mode',
                ),
                child: const Text('Open slow mode'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Open slow mode'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('slow_mode_option_5')));
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceChatSlowModeSheet.sheetKey), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining('slow-mode-secret'), findsNothing);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
