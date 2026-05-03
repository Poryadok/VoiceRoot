import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

void main() {
  testWidgets('shows Gateway ok when /health returns 200', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://localhost:9999'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/health') {
                return http.Response('OK', 200);
              }
              return http.Response('Not Found', 404);
            }),
          ),
        ],
        child: const VoiceApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('Gateway: ok'), findsOneWidget);
    expect(find.byKey(const Key('gateway_status_text')), findsOneWidget);
  });

  testWidgets('shows failure when base URL missing', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: ''),
          ),
        ],
        child: const VoiceApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('missing base URL'), findsOneWidget);
  });
}
