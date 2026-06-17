import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/guest_nickname_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

AuthController _guestAuthController(Ref ref) {
  final controller = AuthController(
    authClient: ref.watch(voiceAuthClientProvider),
    storage: ref.watch(authSessionStorageProvider),
    guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
  );
  controller.state = const AuthState(
    session: AuthSession(
      accessToken: 'guest-access',
      refreshToken: 'guest-refresh',
      accountId: 'guest-acc',
      activeProfileId: 'guest-prof',
      expiresInSeconds: 900,
    ),
    needsGuestNickname: true,
  );
  return controller;
}

void main() {
  testWidgets('GuestNicknameScreen submits nickname and clears guest flag', (
    tester,
  ) async {
    Map<String, dynamic>? patchBody;
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me' && req.method == 'PATCH') {
        patchBody = jsonDecode(req.body) as Map<String, dynamic>;
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'guest-prof',
              'account_id': 'guest-acc',
              'username': 'guest',
              'discriminator': '0001',
              'display_name': patchBody!['display_name'],
              'bio': '',
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
    });

    late AuthController authController;
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
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith((ref) {
            authController = _guestAuthController(ref);
            return authController;
          }),
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
          home: const GuestNicknameScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('guest_nickname_screen')), findsOneWidget);

    await tester.enterText(
      find.byKey(const Key('guest_nickname_field')),
      'PlayerOne',
    );
    await tester.tap(find.byKey(const Key('guest_nickname_submit')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(patchBody?['display_name'], 'PlayerOne');
    expect(authController.state.needsGuestNickname, isFalse);
  });

  testWidgets('GuestNicknameScreen shows API error', (tester) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/users/me' && req.method == 'PATCH') {
        return http.Response('{"message":"nickname taken"}', 409);
      }
      return http.Response('Not Found', 404);
    });

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
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith(_guestAuthController),
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
          home: const GuestNicknameScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const Key('guest_nickname_field')),
      'TakenName',
    );
    await tester.tap(find.byKey(const Key('guest_nickname_submit')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.text('nickname taken'), findsOneWidget);
  });
}
