import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/discover_hint_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('login shows @handle and discover hint snackbar', (tester) async {
    final hintStorage = InMemoryDiscoverHintStorage();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(hintStorage),
          onboardingControllerProvider.overrideWith(
            () => _CompletedOnboardingController(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/login') {
                return http.Response(
                  jsonEncode({
                    'session': {
                      'access_token': 'a',
                      'refresh_token': 'r',
                      'expires_in_seconds': 900,
                      'account_id': 'acc',
                      'profile_id': 'prof-42',
                    },
                  }),
                  200,
                );
              }
              if (request.url.path == '/api/v1/users/profiles') {
                return http.Response(
                  jsonEncode({
                    'profile_list': {
                      'profiles': [
                        {
                          'id': 'prof-42',
                          'account_id': 'acc',
                          'username': 'alice',
                          'discriminator': '0001',
                          'display_name': 'Alice',
                          'locale': 'en',
                          'theme': 'dark',
                          'is_primary': true,
                          'verification_type': 'none',
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              if (request.url.path == '/api/v1/users/profiles/prof-42') {
                return http.Response(
                  jsonEncode({
                    'profile': {
                      'id': 'prof-42',
                      'account_id': 'acc',
                      'username': 'alice',
                      'discriminator': '0001',
                      'display_name': 'Alice',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
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
    await tester.enterText(find.byKey(AuthScreen.passwordFieldKey), 'secret12');
    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('auth_session_profile')), findsOneWidget);
    expect(find.text('@alice#0001'), findsOneWidget);
    expect(find.byKey(const Key('social_discover_hint')), findsOneWidget);
    expect(find.text('Find people — use the icon on the left'), findsOneWidget);
    expect(find.byKey(AuthScreen.screenKey), findsNothing);
    expect(await hintStorage.wasShown(), isTrue);
  });

  testWidgets('register shows localized validation_failed from API', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
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
    await tester.enterText(find.byKey(AuthScreen.passwordFieldKey), 'short');
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
      find.text('Use a valid email and a password of at least 8 characters.'),
      findsOneWidget,
    );
    expect(find.byKey(const Key('auth_error')), findsOneWidget);
  });

  testWidgets('login shows localized invalid_credentials', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
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
    await tester.enterText(find.byKey(AuthScreen.passwordFieldKey), 'secret12');
    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Incorrect email or password.'), findsOneWidget);
  });

  testWidgets('login shows localized rate_limited on 429', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
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
    await tester.enterText(find.byKey(AuthScreen.passwordFieldKey), 'secret12');
    await tester.tap(find.byKey(AuthScreen.loginButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.text('Too many attempts. Please wait and try again.'),
      findsOneWidget,
    );
  });

  testWidgets('empty submit shows localized empty fields error', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
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
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
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

  for (final recovery in [
    EmailVerificationRecoveryState.emailPending,
    EmailVerificationRecoveryState.promotionPending,
  ]) {
    testWidgets('${recovery.name} exposes localized pending logout', (
      tester,
    ) async {
      final storage = InMemoryAuthSessionStorage();
      final session = const AuthSession(
        accessToken: 'restricted-access',
        refreshToken: 'restricted-refresh',
        accountId: 'acc',
        activeProfileId: 'prof',
        expiresInSeconds: 900,
        accountType: 'guest',
      );
      await storage.write(session);
      String? logoutAuthorization;
      final container = ProviderContainer(
        overrides: [
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(storage),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/logout') {
                logoutAuthorization = request.headers['authorization'];
                return http.Response('', 204);
              }
              if (request.url.path == '/api/v1/auth/refresh') {
                return http.Response(
                  jsonEncode({
                    'session': {
                      'access_token': 'restricted-access',
                      'refresh_token': 'restricted-refresh',
                      'expires_in_seconds': 900,
                      'account_id': 'acc',
                      'profile_id': 'prof',
                      'account_type': 'guest',
                    },
                  }),
                  200,
                );
              }
              if (request.url.path == '/api/v1/auth/verification-status') {
                return http.Response(
                  jsonEncode({
                    'state':
                        recovery == EmailVerificationRecoveryState.emailPending
                        ? 'EMAIL_PENDING'
                        : 'PROMOTION_PENDING',
                  }),
                  200,
                );
              }
              if (request.url.path == '/health')
                return http.Response('ok', 200);
              return http.Response('not found', 404);
            }),
          ),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            locale: const Locale('en'),
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const AuthScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();
      container.read(authControllerProvider.notifier).state = AuthState(
        session: session,
        isGuest: true,
        emailVerificationRecoveryState: recovery,
      );
      await tester.pumpAndSettle();

      expect(find.byKey(AuthScreen.pendingLogoutButtonKey), findsOneWidget);
      expect(find.text('Log out'), findsOneWidget);
      expect(find.byKey(AuthScreen.loginButtonKey), findsNothing);
      expect(
        find.byKey(AuthScreen.verificationButtonKey),
        recovery == EmailVerificationRecoveryState.emailPending
            ? findsOneWidget
            : findsNothing,
      );
      expect(
        find.byKey(AuthScreen.promotionRetryButtonKey),
        recovery == EmailVerificationRecoveryState.promotionPending
            ? findsOneWidget
            : findsNothing,
      );

      await tester.tap(find.byKey(AuthScreen.pendingLogoutButtonKey));
      await tester.pumpAndSettle();
      expect(logoutAuthorization, 'Bearer restricted-access');
      expect(await storage.read(), isNull);
      expect(find.byKey(AuthScreen.loginButtonKey), findsOneWidget);
    });
  }
}

class _CompletedOnboardingController extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(completed: true);
}
