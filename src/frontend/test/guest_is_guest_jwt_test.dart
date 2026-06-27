import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

void main() {
  test('restore marks isGuest from JWT account_type without guest_password heuristic', () async {
    final sessionStorage = InMemoryAuthSessionStorage();
    const session = AuthSession(
      accessToken: 'ey.test.guest',
      refreshToken: 'refresh',
      accountId: 'acc-guest',
      activeProfileId: 'prof-guest',
      expiresInSeconds: 900,
    );
    await sessionStorage.write(session);

    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(sessionStorage),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((request) async {
            if (request.url.path == '/api/v1/auth/refresh') {
              return http.Response(
                jsonEncode({
                  'session': {
                    'access_token': 'ey.test.guest',
                    'refresh_token': 'refresh',
                    'expires_in_seconds': 900,
                    'account_id': 'acc-guest',
                    'profile_id': 'prof-guest',
                    'account_type': 'guest',
                  },
                }),
                200,
              );
            }
            if (request.url.path == '/api/v1/users/me') {
              return http.Response(
                jsonEncode({
                  'profile': {
                    'id': 'prof-guest',
                    'account_id': 'acc-guest',
                    'display_name': 'Player',
                  },
                  'account_type': 'guest',
                }),
                200,
              );
            }
            return http.Response('not found', 404);
          }),
        ),
      ],
    );
    addTearDown(container.dispose);

    await container.read(authControllerProvider.notifier).restore();
    final auth = container.read(authControllerProvider);

    expect(auth.isGuest, isTrue);
    expect(auth.isAuthenticated, isTrue);
  });
}
