import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  for (final language in ['en', 'ru']) {
    for (final failure in [
      'transport',
      'provider',
      'http503',
      'missing base',
    ]) {
      testWidgets(
        '$language gateway $failure is safe and recovers on refresh',
        (tester) async {
          const voipChannel = MethodChannel('voice/voip');
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(voipChannel, (_) async => null);
          addTearDown(
            () => TestDefaultBinaryMessengerBinding
                .instance
                .defaultBinaryMessenger
                .setMockMethodCallHandler(voipChannel, null),
          );
          final requests = <http.Request>[];
          var failed = true;
          final privateMarker = 'private_health_${language}_$failure';
          final container = ProviderContainer(
            overrides: [
              ...voiceAppTestOverrides(
                client: MockClient((request) async {
                  if (request.url.path != '/health') {
                    return http.Response('{}', 404);
                  }
                  requests.add(request);
                  if (failed && failure == 'transport') {
                    throw http.ClientException(privateMarker, request.url);
                  }
                  return http.Response(
                    failed ? privateMarker : 'OK',
                    failed ? 503 : 200,
                  );
                }),
              ),
              if (failure == 'provider' || failure == 'missing base')
                gatewayHealthProvider.overrideWith((ref) async {
                  if (!failed) return const GatewayHealthOk();
                  if (failure == 'provider') throw StateError(privateMarker);
                  return const GatewayHealthFailure(
                    kGatewayMissingBaseUrlDetail,
                  );
                }),
            ],
          );
          addTearDown(container.dispose);
          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: VoiceApp(locale: Locale(language)),
            ),
          );
          await tester.pumpAndSettle();
          final l10n = AppLocalizations.of(
            tester.element(find.byKey(const Key('gateway_status_text'))),
          )!;
          final expected = failure == 'missing base'
              ? l10n.gatewayMissingBaseUrl
              : l10n.gatewayStatusFailure(l10n.backendUnavailable);
          expect(find.text(expected), findsOneWidget);
          expect(find.textContaining(privateMarker), findsNothing);
          expect(find.textContaining('Bad state:'), findsNothing);
          expect(find.textContaining('ClientException'), findsNothing);
          if (failure == 'transport') {
            final health = await container.read(gatewayHealthProvider.future);
            expect(
              (health as GatewayHealthFailure).message,
              contains(privateMarker),
            );
          }
          final expectedRequests =
              failure == 'transport' || failure == 'http503' ? 1 : 0;
          expect(requests.length, expectedRequests);
          for (final request in requests) {
            expect(request.method, 'GET');
            expect(request.url.toString(), 'http://localhost:9999/health');
            expect(request.headers['X-Request-Id'], isNotEmpty);
            expect(request.headers['Authorization'], isNull);
          }
          await tester.pump(const Duration(seconds: 1));
          expect(
            requests.length,
            expectedRequests,
            reason: 'no automatic retry',
          );
          failed = false;
          container.invalidate(gatewayHealthProvider);
          await tester.pumpAndSettle();
          expect(find.byKey(const Key('gateway_status_text')), findsNothing);
          expect(requests.length, expectedRequests * 2);
          expect(find.byKey(const Key('settings_open')), findsOneWidget);
        },
      );
    }
  }

  testWidgets('locale ru hides gateway bar when /health returns 200', (
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
    expect(find.textContaining('Шлюз: ок'), findsNothing);
    expect(find.text('Личные сообщения').first, findsOneWidget);
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
