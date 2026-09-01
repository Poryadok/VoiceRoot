import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/profile/manage_profiles_sheet.dart';

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
  testWidgets('manage profiles deletes secondary profile via REST', (tester) async {
    var deleteCalled = false;
    final mock = MockClient((req) async {
      if (req.method == 'DELETE' &&
          req.url.path == '/api/v1/users/profiles/profile-secondary') {
        deleteCalled = true;
        return http.Response('', 200);
      }
      if (req.url.path == '/api/v1/users/profiles') {
        return http.Response(
          jsonEncode({
            'profile_list': {
              'profiles': [
                {
                  'id': 'profile-primary',
                  'account_id': 'account-1',
                  'username': 'main',
                  'discriminator': '0001',
                  'display_name': 'Main',
                  'is_primary': true,
                },
                {
                  'id': 'profile-secondary',
                  'account_id': 'account-1',
                  'username': 'alt',
                  'discriminator': '0002',
                  'display_name': 'Alt',
                  'is_primary': false,
                },
              ],
            },
          }),
          200,
        );
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
          home: const Scaffold(body: ManageProfilesSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('manage_profiles_delete_profile-secondary')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Delete'));
    await tester.pumpAndSettle();

    expect(deleteCalled, isTrue);
  });
}
