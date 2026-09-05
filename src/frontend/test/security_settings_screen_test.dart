import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/settings/security_settings_screen.dart';

import 'support/test_voice_token_catalog.dart';

class _MemoryAuthStorage implements AuthSessionStorage {
  @override
  Future<void> clear() async {}

  @override
  Future<AuthSession?> read() async => null;

  @override
  Future<void> write(AuthSession session) async {}
}

void main() {
  testWidgets('security settings deletes account after password confirm', (
    tester,
  ) async {
    var deleteCalled = false;
    final mock = MockClient((req) async {
      if (req.method == 'POST' &&
          req.url.path == '/api/v1/auth/delete-account') {
        deleteCalled = true;
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['password'], 'secret');
        return http.Response('', 204);
      }
      if (req.method == 'POST' && req.url.path == '/api/v1/auth/logout') {
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });

    final gateway = GatewayHttpClient(
      httpClient: mock,
      config: const GatewayConfig(baseUrl: 'http://api.test'),
      authorizationProvider: () => 'Bearer token',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          gatewayHttpClientProvider.overrideWithValue(gateway),
          voiceAuthClientProvider.overrideWithValue(
            VoiceAuthClient(gateway: gateway),
          ),
          authControllerProvider.overrideWith((ref) {
            final controller = AuthController(
              authClient: ref.watch(voiceAuthClientProvider),
              storage: ref.watch(authSessionStorageProvider),
              guestCredentialsStorage: ref.watch(
                guestCredentialsStorageProvider,
              ),
            );
            controller.state = const AuthState(
              session: AuthSession(
                accessToken: 'token',
                refreshToken: 'refresh',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-primary',
              ),
            );
            return controller;
          }),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const SecuritySettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(
      find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
      120,
      scrollable: find
          .descendant(
            of: find.byKey(SecuritySettingsScreen.screenKey),
            matching: find.byType(Scrollable),
          )
          .first,
    );
    await tester.tap(find.byKey(SecuritySettingsScreen.deleteAccountButtonKey));
    await tester.pumpAndSettle();

    expect(
      find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(SecuritySettingsScreen.deleteAccountPasswordKey),
      'secret',
    );
    await tester.tap(
      find.descendant(
        of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
        matching: find.text('Delete'),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(deleteCalled, isTrue);
  });

  testWidgets(
    'totp_required keeps deletion pending and retries with authenticator code',
    (tester) async {
      var deleteAttempts = 0;
      var logoutCalled = false;
      late AuthController controller;
      final mock = MockClient((req) async {
        if (req.method == 'POST' &&
            req.url.path == '/api/v1/auth/delete-account') {
          deleteAttempts++;
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['password'], 'secret');
          if (deleteAttempts == 1) {
            expect(body.containsKey('totp_code'), isFalse);
            return http.Response(jsonEncode({'error': 'totp_required'}), 401);
          }
          if (deleteAttempts == 2) {
            expect(body['totp_code'], 'invalid-code');
            return http.Response(jsonEncode({'error': 'invalid_totp'}), 401);
          }
          expect(body['totp_code'], 'backup-code-123');
          return http.Response('', 204);
        }
        if (req.method == 'POST' && req.url.path == '/api/v1/auth/logout') {
          logoutCalled = true;
          return http.Response('', 204);
        }
        return http.Response('not found', 404);
      });

      final gateway = GatewayHttpClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
        authorizationProvider: () => 'Bearer token',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            gatewayHttpClientProvider.overrideWithValue(gateway),
            voiceAuthClientProvider.overrideWithValue(
              VoiceAuthClient(gateway: gateway),
            ),
            authControllerProvider.overrideWith((ref) {
              controller = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              controller.state = const AuthState(
                session: AuthSession(
                  accessToken: 'token',
                  refreshToken: 'refresh',
                  expiresInSeconds: 900,
                  accountId: 'account-1',
                  activeProfileId: 'profile-primary',
                ),
              );
              return controller;
            }),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const SecuritySettingsScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.scrollUntilVisible(
        find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
        120,
        scrollable: find
            .descendant(
              of: find.byKey(SecuritySettingsScreen.screenKey),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.tap(
        find.byKey(SecuritySettingsScreen.deleteAccountButtonKey),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountPasswordKey),
        'secret',
      );
      await tester.tap(
        find.descendant(
          of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
          matching: find.text('Delete'),
        ),
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 1);
      expect(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel('Authenticator or backup code'),
        findsOneWidget,
      );
      expect(
        find.text(
          'Enter the code from your authenticator app or a backup code.',
        ),
        findsOneWidget,
      );
      expect(logoutCalled, isFalse);

      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        'invalid-code',
      );
      await tester.tap(
        find.descendant(
          of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
          matching: find.text('Delete'),
        ),
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 2);
      expect(
        find.text('Invalid authenticator or backup code.'),
        findsOneWidget,
      );
      expect(logoutCalled, isFalse);

      await tester.enterText(
        find.byKey(SecuritySettingsScreen.deleteAccountTotpKey),
        'backup-code-123',
      );
      await tester.tap(
        find.descendant(
          of: find.byKey(SecuritySettingsScreen.deleteAccountDialogKey),
          matching: find.text('Delete'),
        ),
      );
      await tester.pumpAndSettle();

      expect(deleteAttempts, 3);
      expect(logoutCalled, isTrue);
      expect(controller.state.session, isNull);
    },
  );
}
