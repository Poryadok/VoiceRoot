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
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/profile/create_profile_sheet.dart';

import 'support/test_voice_token_catalog.dart';

class _MemoryAuthStorage implements AuthSessionStorage {
  AuthSession? _session;

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async => _session = session;
}

void main() {
  testWidgets('create profile sends work preset in POST body', (tester) async {
    String? capturedBody;
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/users/profiles' && req.method == 'POST') {
        capturedBody = req.body;
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'profile-new',
              'account_id': 'account-1',
              'username': 'user',
              'discriminator': '1234',
              'display_name': 'Work Alt',
              'is_primary': false,
              'accent_color': '#7EC8E3',
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/auth/switch-profile') {
        return http.Response(
          jsonEncode({
            'access_token': 'after',
            'refresh_token': 'refresh',
            'expires_in_seconds': 900,
            'account_id': 'account-1',
            'profile_id': 'profile-new',
          }),
          200,
        );
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
      authorizationProvider: () => 'Bearer before',
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
          myProfilesProvider.overrideWith((_) async => const []),
          subscriptionTierProvider.overrideWith((_) => 'free'),
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
                accessToken: 'before',
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
          home: const Scaffold(body: CreateProfileSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(CreateProfileSheet.displayNameFieldKey),
      'Work Alt',
    );
    await tester.tap(find.text('Work'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(CreateProfileSheet.submitKey));
    await tester.pumpAndSettle();

    expect(capturedBody, isNotNull);
    final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
    expect(body['preset'], 'work');
    expect(body['display_name'], 'Work Alt');
  });
}
