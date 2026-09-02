import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';
import 'package:voice_frontend/ui/auth/password_reset_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

Widget _testApp({required Widget home, required http.Client client}) {
  return ProviderScope(
    overrides: [
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: home,
    ),
  );
}

Widget _authScreenApp({required http.Client client}) {
  return ProviderScope(
    overrides: [
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
    ],
    child: const VoiceApp(locale: Locale('en')),
  );
}

void main() {
  testWidgets('forgot password opens password reset screen', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(
      _authScreenApp(
        client: MockClient((request) async {
          if (request.url.path == '/health') {
            return http.Response('ok', 200);
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(AuthScreen.forgotPasswordButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(PasswordResetScreen.screenKey), findsOneWidget);
  });

  testWidgets('send reset link advances to code entry step', (tester) async {
    bindLargeTestViewport(tester);
    var sendCalled = false;
    await tester.pumpWidget(
      _testApp(
        home: const PasswordResetScreen(),
        client: MockClient((req) async {
          if (req.method == 'POST' &&
              req.url.path == '/api/v1/auth/otp/send') {
            sendCalled = true;
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            expect(body['email'], 'user@example.com');
            expect(body['otp_type'], 'password_reset');
            return http.Response('', 204);
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(PasswordResetScreen.emailFieldKey),
      'user@example.com',
    );
    await tester.tap(find.byKey(PasswordResetScreen.sendLinkButtonKey));
    await tester.pumpAndSettle();

    expect(sendCalled, isTrue);
    expect(find.byKey(PasswordResetScreen.codeFieldKey), findsOneWidget);
    expect(find.byKey(PasswordResetScreen.newPasswordFieldKey), findsOneWidget);
    expect(
      find.byKey(PasswordResetScreen.confirmPasswordFieldKey),
      findsOneWidget,
    );
  });

  testWidgets('reset password submits and returns to login', (tester) async {
    bindLargeTestViewport(tester);
    var resetCalled = false;
    await tester.pumpWidget(
      _testApp(
        home: const PasswordResetScreen(initialEmail: 'user@example.com'),
        client: MockClient((req) async {
          if (req.method == 'POST' &&
              req.url.path == '/api/v1/auth/otp/send') {
            return http.Response('', 204);
          }
          if (req.method == 'POST' &&
              req.url.path == '/api/v1/auth/password/reset') {
            resetCalled = true;
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            expect(body['email'], 'user@example.com');
            expect(body['code'], '123456');
            expect(body['new_password'], 'newpass99');
            return http.Response('', 204);
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(PasswordResetScreen.sendLinkButtonKey));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(PasswordResetScreen.codeFieldKey),
      '123456',
    );
    await tester.enterText(
      find.byKey(PasswordResetScreen.newPasswordFieldKey),
      'newpass99',
    );
    await tester.enterText(
      find.byKey(PasswordResetScreen.confirmPasswordFieldKey),
      'newpass99',
    );
    await tester.tap(find.byKey(PasswordResetScreen.resetButtonKey));
    await tester.pumpAndSettle();

    expect(resetCalled, isTrue);
    expect(
      find.text('Password reset. You can sign in with your new password.'),
      findsOneWidget,
    );
  });

  testWidgets('password mismatch shows localized error', (tester) async {
    bindLargeTestViewport(tester);
    await tester.pumpWidget(
      _testApp(
        home: const PasswordResetScreen(initialEmail: 'user@example.com'),
        client: MockClient((req) async {
          if (req.method == 'POST' &&
              req.url.path == '/api/v1/auth/otp/send') {
            return http.Response('', 204);
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(PasswordResetScreen.sendLinkButtonKey));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(PasswordResetScreen.codeFieldKey),
      '123456',
    );
    await tester.enterText(
      find.byKey(PasswordResetScreen.newPasswordFieldKey),
      'newpass99',
    );
    await tester.enterText(
      find.byKey(PasswordResetScreen.confirmPasswordFieldKey),
      'otherpass',
    );
    await tester.tap(find.byKey(PasswordResetScreen.resetButtonKey));
    await tester.pump();

    expect(find.text("Passwords don't match."), findsOneWidget);
  });
}
