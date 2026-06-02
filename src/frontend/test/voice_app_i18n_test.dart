import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  testWidgets('locale ru shows Russian gateway ok when /health returns 200', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((request) async {
            if (request.url.path == '/health') {
              return http.Response('OK', 200);
            }
            return http.Response('Not Found', 404);
          }),
        ),
        child: const VoiceApp(locale: Locale('ru')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('Шлюз: ок'), findsOneWidget);
  });

  testWidgets('locale ru shows Russian message when base URL missing', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('x', 404)),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: ''),
          ),
        ],
        child: const VoiceApp(locale: Locale('ru')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('не указан базовый URL'), findsOneWidget);
  });
}
