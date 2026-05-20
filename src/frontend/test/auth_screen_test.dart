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
import 'package:voice_frontend/ui/auth/auth_screen.dart';

void main() {
  testWidgets('login shows authenticated shell with profile_id', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/login') {
                return http.Response(
                  jsonEncode({
                    'access_token': 'a',
                    'refresh_token': 'r',
                    'expires_in_seconds': 900,
                    'account_id': 'acc',
                    'profile_id': 'prof-42',
                  }),
                  200,
                );
              }
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(AuthScreen.screenKey), findsOneWidget);
    await tester.enterText(
      find.byKey(AuthScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(AuthScreen.passwordFieldKey),
      'secret12',
    );
    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('auth_session_profile')), findsOneWidget);
    expect(find.textContaining('prof-42'), findsOneWidget);
    expect(find.byKey(AuthScreen.screenKey), findsNothing);
  });

  testWidgets('register shows localized validation_failed from API', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/register') {
                return http.Response(
                  jsonEncode({'error': 'validation_failed'}),
                  400,
                );
              }
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(AuthScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(AuthScreen.passwordFieldKey),
      'short',
    );
    await tester.tap(find.byKey(AuthScreen.registerButtonKey));
    await tester.pump();

    expect(
      find.text('Password must be at least 8 characters.'),
      findsOneWidget,
    );
    expect(find.byKey(const Key('auth_error')), findsNothing);

    await tester.enterText(
      find.byKey(AuthScreen.passwordFieldKey),
      'validpass',
    );
    await tester.tap(find.byKey(AuthScreen.registerButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.text(
        'Use a valid email and a password of at least 8 characters.',
      ),
      findsOneWidget,
    );
    expect(find.byKey(const Key('auth_error')), findsOneWidget);
  });

  testWidgets('login shows localized invalid_credentials', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/login') {
                return http.Response(
                  jsonEncode({'error': 'invalid_credentials'}),
                  401,
                );
              }
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(AuthScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(AuthScreen.passwordFieldKey),
      'secret12',
    );
    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Incorrect email or password.'), findsOneWidget);
  });

  testWidgets('login shows localized rate_limited on 429', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/login') {
                return http.Response(
                  jsonEncode({'error': 'rate_limited'}),
                  429,
                );
              }
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(AuthScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(AuthScreen.passwordFieldKey),
      'secret12',
    );
    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.text('Too many attempts. Please wait and try again.'),
      findsOneWidget,
    );
  });

  testWidgets('empty submit shows localized empty fields error', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pump();

    expect(find.text('Enter your email and password.'), findsWidgets);
  });

  testWidgets('password field shows helper text', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('At least 8 characters'), findsOneWidget);
  });
}
