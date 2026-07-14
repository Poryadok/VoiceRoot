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
  const config = GatewayConfig(baseUrl: 'http://api.test');

  Map<String, dynamic> sessionJson() => {
    'session': {
      'access_token': 'access',
      'refresh_token': 'refresh',
      'expires_in_seconds': 900,
      'account_id': 'acc-1',
      'profile_id': 'prof-1',
    },
  };

  ProviderContainer buildContainer({
    required MockClient mock,
    AuthSessionStorage? storage,
  }) {
    return ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(config),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(
          storage ?? InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
      ],
    );
  }

  test('login persists session and exposes active profile_id', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/login') {
        return http.Response(jsonEncode(sessionJson()), 200);
      }
      return http.Response('not found', 404);
    });
    final storage = InMemoryAuthSessionStorage();
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);

    final controller = container.read(authControllerProvider.notifier);
    await controller.login(email: 'u@x.com', password: 'pw');

    final state = container.read(authControllerProvider);
    expect(state.session?.activeProfileId, 'prof-1');
    expect(state.session?.accessToken, 'access');
    expect(await storage.read(), state.session);
  });

  test('restore refreshes stored session', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        return http.Response(
          jsonEncode({
            'session': {
              ...(sessionJson()['session'] as Map<String, dynamic>),
              'access_token': 'access-new',
              'refresh_token': 'refresh-new',
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });
    final storage = InMemoryAuthSessionStorage();
    await storage.write(
      const AuthSession(
        accessToken: 'old-access',
        refreshToken: 'refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
      ),
    );
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);

    await container.read(authControllerProvider.notifier).restore();
    final state = container.read(authControllerProvider);
    expect(state.isRestoring, isFalse);
    expect(state.session?.accessToken, 'access-new');
    expect(state.session?.activeProfileId, 'prof-1');
  });

  test('login stores errorKey for invalid_credentials', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/login') {
        return http.Response(jsonEncode({'error': 'invalid_credentials'}), 401);
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock);
    addTearDown(container.dispose);

    await container
        .read(authControllerProvider.notifier)
        .login(email: 'u@x.com', password: 'password1');

    final state = container.read(authControllerProvider);
    expect(state.session, isNull);
    expect(state.errorKey, 'invalid_credentials');
  });

  test('login maps 429 without error body to rate_limited', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/login') {
        return http.Response('', 429);
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock);
    addTearDown(container.dispose);

    await container
        .read(authControllerProvider.notifier)
        .login(email: 'u@x.com', password: 'password1');

    expect(container.read(authControllerProvider).errorKey, 'rate_limited');
  });

  test('convertGuest sends user-entered password', () async {
    const guestPassword = 'guest-auto-password-1';
    const userPassword = 'user-chosen-password1';
    String? convertBody;
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword(guestPassword);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/convert-guest') {
        convertBody = req.body;
        return http.Response(
          jsonEncode({
            'session': {
              ...(sessionJson()['session'] as Map<String, dynamic>),
              'access_token': 'access-converted',
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });
    final container = ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(config),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
        guestCredentialsStorageProvider.overrideWithValue(guestStorage),
      ],
    );
    addTearDown(container.dispose);

    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(
      session: AuthSession(
        accessToken: 'access',
        refreshToken: 'refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
      ),
      isGuest: true,
    );

    final err = await controller.convertGuest(
      email: 'guest@example.com',
      password: userPassword,
    );
    expect(err, isNull);
    expect(convertBody, isNotNull);
    expect(convertBody, contains(userPassword));
    expect(convertBody, isNot(contains(guestPassword)));
  });

  test('logout clears session', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/logout') {
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });
    final storage = InMemoryAuthSessionStorage();
    await storage.write(
      const AuthSession(
        accessToken: 'access',
        refreshToken: 'refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
      ),
    );
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);

    await container.read(authControllerProvider.notifier).restore();
    await container.read(authControllerProvider.notifier).logout();

    expect(container.read(authControllerProvider).session, isNull);
    expect(await storage.read(), isNull);
  });
}
