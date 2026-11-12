import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/register_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

Widget _testApp({
  required http.Client client,
}) {
  return ProviderScope(
    overrides: [
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      guestCredentialsStorageProvider.overrideWithValue(
        InMemoryGuestCredentialsStorage(),
      ),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const RegisterScreen(),
    ),
  );
}

Widget _loginStandInApp() {
  return ProviderScope(
    overrides: [
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      guestCredentialsStorageProvider.overrideWithValue(
        InMemoryGuestCredentialsStorage(),
      ),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((request) async => http.Response('not found', 404)),
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const _LoginStandIn(),
    ),
  );
}

void main() {
  testWidgets('register screen shows all v3 controls', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(_testApp(client: MockClient((_) async {
      return http.Response('not found', 404);
    })));
    await tester.pumpAndSettle();

    expect(find.byKey(RegisterScreen.screenKey), findsOneWidget);
    expect(find.text('Create your account'), findsOneWidget);
    expect(find.text('Fill in the details to create your account.'),
      findsOneWidget,
    );
    expect(find.byKey(RegisterScreen.typePickerKey), findsOneWidget);
    expect(find.text('Account type'), findsOneWidget);
    expect(find.text('Personal'), findsOneWidget);
    expect(find.text('Organization'), findsOneWidget);
    expect(find.byKey(RegisterScreen.displayNameFieldKey), findsOneWidget);
    expect(find.byKey(RegisterScreen.tagFieldKey), findsOneWidget);
    expect(find.byKey(RegisterScreen.emailFieldKey), findsOneWidget);
    expect(find.byKey(RegisterScreen.passwordFieldKey), findsOneWidget);
    expect(find.byKey(RegisterScreen.confirmPasswordFieldKey), findsOneWidget);
    expect(find.byKey(RegisterScreen.submitButtonKey), findsOneWidget);
    expect(find.byKey(RegisterScreen.backButtonKey), findsOneWidget);
    expect(find.text('Back to login'), findsOneWidget);
  });

  testWidgets('account type switch changes display name and tag copy', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(_testApp(client: MockClient((_) async {
      return http.Response('not found', 404);
    })));
    await tester.pumpAndSettle();

    expect(find.text('Display name'), findsOneWidget);
    expect(find.text('Profile tag'), findsOneWidget);

    await tester.tap(find.text('Organization'));
    await tester.pumpAndSettle();

    expect(find.text('Organization name'), findsOneWidget);
    expect(find.text('Organization tag'), findsOneWidget);
    expect(find.text('Display name'), findsNothing);
    expect(find.text('Profile tag'), findsNothing);

    await tester.tap(find.text('Personal'));
    await tester.pumpAndSettle();

    expect(find.text('Display name'), findsOneWidget);
    expect(find.text('Profile tag'), findsOneWidget);
    expect(find.text('Organization name'), findsNothing);
    expect(find.text('Organization tag'), findsNothing);
  });

  testWidgets('tag field previews the free tag base', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(_testApp(client: MockClient((_) async {
      return http.Response('not found', 404);
    })));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'myprofile');
    await tester.tap(find.text('Organization'));
    await tester.pumpAndSettle();

    expect(find.text('Organization tag'), findsOneWidget);
    expect(find.text('Free tag: @myprofile'), findsOneWidget);
    expect(
      find.text(
        'If this tag is taken, the system adds a short ending.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('missing required fields show localized field errors', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'validpass',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'validpass',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Enter a display name.'), findsOneWidget);
    expect(find.text('Enter a tag.'), findsOneWidget);
    expect(registerCalls, isZero);
    expect(find.text('Account created. Sign in to enter.'), findsNothing);
  });

  testWidgets('empty email blocks submit with localized error', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'validpass',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'validpass',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Enter your email.'), findsOneWidget);
    expect(registerCalls, isZero);
  });

  testWidgets('display name over the limit shows localized error', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'a' * 33,
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'validpass',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'validpass',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.text('Display name must be 32 characters or fewer.'),
      findsOneWidget,
    );
    expect(registerCalls, isZero);
  });

  testWidgets('short password blocks submit with localized error', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(find.byKey(RegisterScreen.passwordFieldKey), 'short');
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'short',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text('Password must be at least 8 characters.'), findsWidgets);
    expect(registerCalls, isZero);
  });

  testWidgets('confirm-password mismatch shows correction and keeps fields', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
        return http.Response(
          jsonEncode({
            'session': {
              'access_token': 'a',
              'refresh_token': 'r',
              'expires_in_seconds': 900,
              'account_id': 'acc',
              'profile_id': 'prof-1',
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'goodpassword1',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'goodpassword2',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(find.text("Passwords don't match."), findsOneWidget);
    expect(registerCalls, isZero);

    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'goodpassword1',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(registerCalls, 1);
    expect(find.text("Passwords don't match."), findsNothing);
    expect(find.text('Account created. Sign in to enter.'), findsOneWidget);
  });

  testWidgets('register submit posts only email and password', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    Map<String, dynamic>? registerBody;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerBody = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'session': {
              'access_token': 'a',
              'refresh_token': 'r',
              'expires_in_seconds': 900,
              'account_id': 'acc',
              'profile_id': 'prof-1',
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'goodpassword1',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'goodpassword1',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(registerBody, isNotNull);
    expect(registerBody?['email'], 'user@example.com');
    expect(registerBody?['password'], 'goodpassword1');
    expect(registerBody!.containsKey('display_name'), isFalse);
    expect(registerBody!.containsKey('tag'), isFalse);
    expect(find.byKey(RegisterScreen.submitButtonKey), findsNothing);
    expect(find.text('Account created. Sign in to enter.'), findsOneWidget);
  });

  testWidgets('register submit stays blocked until fields are complete', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(registerCalls, isZero);
    expect(find.text('Account created. Sign in to enter.'), findsNothing);
  });

  testWidgets('register shows localized validation_failed from API', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
        return http.Response(
          jsonEncode({'error': 'validation_failed'}),
          400,
        );
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'goodpassword1',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'goodpassword1',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(registerCalls, 1);
    expect(
      find.text(
        'Use a valid email and a password of at least 8 characters.',
      ),
      findsOneWidget,
    );
    expect(find.text('Account created. Sign in to enter.'), findsNothing);
  });

  testWidgets('registration conflict and rate limit stay localized', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    var statusCode = 409;
    var errorCode = 'registration_conflict';
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        return http.Response(
          jsonEncode({'error': errorCode}),
          statusCode,
        );
      }
      return http.Response('not found', 404);
    });

    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    Future<void> fillAndSubmit() async {
      await tester.enterText(
        find.byKey(RegisterScreen.displayNameFieldKey),
        'Alice',
      );
      await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
      await tester.enterText(
        find.byKey(RegisterScreen.emailFieldKey),
        'user@example.com',
      );
      await tester.enterText(
        find.byKey(RegisterScreen.passwordFieldKey),
        'goodpassword1',
      );
      await tester.enterText(
        find.byKey(RegisterScreen.confirmPasswordFieldKey),
        'goodpassword1',
      );
      await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    }

    await fillAndSubmit();
    await tester.pumpAndSettle();
    expect(
      find.text(
        "This email can't be used. Try another one or sign in "
        'if you already have an account.',
      ),
      findsOneWidget,
    );

    statusCode = 429;
    errorCode = 'rate_limited';
    await fillAndSubmit();
    await tester.pumpAndSettle();
    expect(
      find.text('Too many attempts. Please wait and try again.'),
      findsOneWidget,
    );
  });

  testWidgets('unknown register failure hides diagnostics behind neutral copy'
    , (tester,
  ) async {
    bindLargeTestViewport(tester);
    const diagnostic = 'private-registration-stack-detail';
    var registerCalls = 0;
    final client = MockClient((request) async {
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/register') {
        registerCalls++;
        return http.Response(
          jsonEncode({
            'error': 'unknown_registration_failure',
            'message': diagnostic,
          }),
          500,
        );
      }
      return http.Response('not found', 404);
    });
    await tester.pumpWidget(_testApp(client: client));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(RegisterScreen.displayNameFieldKey),
      'Alice',
    );
    await tester.enterText(find.byKey(RegisterScreen.tagFieldKey), 'alicebase');
    await tester.enterText(
      find.byKey(RegisterScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.passwordFieldKey),
      'goodpassword1',
    );
    await tester.enterText(
      find.byKey(RegisterScreen.confirmPasswordFieldKey),
      'goodpassword1',
    );
    await tester.tap(find.byKey(RegisterScreen.submitButtonKey));
    await tester.pumpAndSettle();

    expect(registerCalls, 1);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text('unknown_registration_failure'), findsNothing);
    expect(find.text(diagnostic), findsNothing);
    expect(find.text('Account created. Sign in to enter.'), findsNothing);
  });

  testWidgets('back button returns to the login form', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(_loginStandInApp());
    await tester.pumpAndSettle();

    expect(find.text('Stand-in login form'), findsOneWidget);
    expect(find.byKey(RegisterScreen.screenKey), findsNothing);

    await tester.tap(find.byKey(_LoginStandIn.pushButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(RegisterScreen.screenKey), findsOneWidget);
    expect(find.text('Create your account'), findsOneWidget);

    await tester.tap(find.byKey(RegisterScreen.backButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(RegisterScreen.screenKey), findsNothing);
    expect(find.text('Stand-in login form'), findsOneWidget);
  });

  testWidgets('register type picker supports keyboard focus', (
    tester,
  ) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(_testApp(client: MockClient((_) async {
      return http.Response('not found', 404);
    })));
    await tester.pumpAndSettle();

    final organizationLabel = find.text('Organization');
    final focus = Focus.of(tester.element(organizationLabel));
    focus.requestFocus();
    await tester.pump();
    expect(focus.hasFocus, isTrue);

    await tester.tap(find.text('Organization'));
    await tester.pumpAndSettle();

    expect(find.text('Organization name'), findsOneWidget);
  });
}

class _LoginStandIn extends StatelessWidget {
  const _LoginStandIn();

  static const Key pushButtonKey = Key('test_login_stand_in_push');

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: Column(
          key: const Key('test_login_stand_in'),
          children: [
            const Text('Stand-in login form'),
            ElevatedButton(
              key: pushButtonKey,
              onPressed: () {
                Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (_) => const RegisterScreen(),
                  ),
                );
              },
              child: const Text('Register here'),
            ),
          ],
        ),
      ),
    );
  }
}
