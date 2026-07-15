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
import 'package:voice_frontend/ui/auth/guest_convert_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('save-account reminder opens convert-guest modal', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: guestShellTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        )..add(
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
        ),
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('guest_save_account_reminder')), findsOneWidget);
    await tester.tap(find.byKey(const Key('guest_save_account_reminder_cta')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.byKey(const Key('guest_convert_modal')), findsOneWidget);
    expect(find.byKey(const Key('guest_convert_email')), findsOneWidget);
    expect(find.byKey(const Key('guest_convert_password')), findsOneWidget);
  });

  testWidgets('convert sheet shows localized client validation errors', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => throw UnimplementedError()),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: GuestConvertSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Create account'));
    await tester.pump();

    expect(
      find.text('Enter your email and password.'),
      findsNWidgets(2),
    );
    expect(find.byKey(GuestConvertSheet.errorKey), findsNothing);

    await tester.enterText(
      find.byKey(GuestConvertSheet.emailFieldKey),
      'guest@example.com',
    );
    await tester.tap(find.text('Create account'));
    await tester.pump();

    expect(find.text('Enter your email and password.'), findsOneWidget);
    expect(find.byKey(GuestConvertSheet.errorKey), findsNothing);

    await tester.enterText(
      find.byKey(GuestConvertSheet.passwordFieldKey),
      'short',
    );
    await tester.tap(find.text('Create account'));
    await tester.pump();

    expect(
      find.text('Password must be at least 8 characters.'),
      findsOneWidget,
    );
    expect(find.byKey(GuestConvertSheet.errorKey), findsNothing);
  });

  testWidgets('convert sheet shows localized validation_failed from API', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          discoverHintStorageProvider.overrideWithValue(
            testDiscoverHintStorage,
          ),
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/convert-guest') {
                return http.Response(
                  jsonEncode({'error': 'validation_failed'}),
                  400,
                );
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: GuestConvertSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GuestConvertSheet.emailFieldKey),
      'guest@example.com',
    );
    await tester.enterText(
      find.byKey(GuestConvertSheet.passwordFieldKey),
      'validpass',
    );
    await tester.tap(find.text('Create account'));
    await tester.pumpAndSettle();

    expect(
      find.text('Use a valid email and a password of at least 8 characters.'),
      findsOneWidget,
    );
    expect(find.byKey(GuestConvertSheet.errorKey), findsOneWidget);
    expect(find.text('validation_failed'), findsNothing);
  });
}
