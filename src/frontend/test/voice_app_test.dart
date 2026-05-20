import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/auth_test_overrides.dart';

List<Override> voiceAppTestOverrides({required http.Client client}) => [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://localhost:9999'),
      ),
      httpClientProvider.overrideWithValue(client),
    ];

void main() {
  testWidgets('shows Gateway ok when /health returns 200', (tester) async {
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
        child: VoiceApp(locale: const Locale('en')),
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
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(testDiscoverHintStorage),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: ''),
          ),
        ],
        child: VoiceApp(locale: const Locale('en')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('missing base URL'), findsOneWidget);
  });

  testWidgets('session bar shows @handle when profile loads', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((request) async {
            if (request.url.path == '/health') {
              return http.Response('OK', 200);
            }
            if (request.url.path == '/api/v1/users/profiles/prof-test') {
              return http.Response(
                jsonEncode({
                  'profile': {
                    'id': 'prof-test',
                    'account_id': 'acc-test',
                    'username': 'voiceuser',
                    'discriminator': '4242',
                    'display_name': 'Voice User',
                    'locale': 'en',
                    'theme': 'dark',
                    'is_primary': true,
                    'verification_type': 'none',
                  },
                }),
                200,
              );
            }
            return http.Response('Not Found', 404);
          }),
        ),
        child: VoiceApp(locale: const Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('@voiceuser#4242'), findsOneWidget);
    expect(find.textContaining('Profile:'), findsNothing);
  });

  testWidgets('chat list shows backend unavailable on 404', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((request) async {
            if (request.url.path == '/health') {
              return http.Response('OK', 200);
            }
            if (request.url.path == '/api/v1/chats') {
              return http.Response('not found', 404);
            }
            return http.Response('Not Found', 404);
          }),
        ),
        child: VoiceApp(locale: const Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chat_list_unavailable')), findsOneWidget);
    expect(
      find.textContaining('Start the full API stack'),
      findsOneWidget,
    );
    expect(find.text('No conversations yet'), findsNothing);
  });
}
