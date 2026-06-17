import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

const _guestAccountId = 'a1b2c3d4-e5f6-7890-abcd-ef1234567890';
const _guestSession = AuthSession(
  accessToken: 'guest-access',
  refreshToken: 'guest-refresh',
  accountId: _guestAccountId,
  activeProfileId: 'guest-prof',
  expiresInSeconds: 900,
);

void main() {
  test('isPlaceholderGuestDisplayName matches account uuid display name', () {
    expect(
      isPlaceholderGuestDisplayName(
        accountId: _guestAccountId,
        displayName: _guestAccountId,
      ),
      isTrue,
    );
    expect(
      isPlaceholderGuestDisplayName(
        accountId: _guestAccountId,
        displayName: 'PlayerOne',
      ),
      isFalse,
    );
  });

  test('restore sets needsGuestNickname for guest without completed nickname', () async {
    final sessionStorage = InMemoryAuthSessionStorage();
    await sessionStorage.write(_guestSession);
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword('guest-password-12345678');

    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(sessionStorage),
        guestCredentialsStorageProvider.overrideWithValue(guestStorage),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((request) async {
            if (request.url.path == '/api/v1/auth/refresh') {
              return http.Response(
                jsonEncode({
                  'session': {
                    'access_token': 'guest-access',
                    'refresh_token': 'guest-refresh',
                    'expires_in_seconds': 900,
                    'account_id': _guestAccountId,
                    'profile_id': 'guest-prof',
                  },
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

    expect(auth.isAuthenticated, isTrue);
    expect(auth.isGuest, isTrue);
    expect(auth.needsGuestNickname, isTrue);
    expect(await guestStorage.isNicknameCompleted(_guestAccountId), isFalse);
  });
}
