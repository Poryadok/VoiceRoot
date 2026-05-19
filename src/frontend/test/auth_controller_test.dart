import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');

  Map<String, dynamic> sessionJson() => {
        'access_token': 'access',
        'refresh_token': 'refresh',
        'expires_in_seconds': 900,
        'account_id': 'acc-1',
        'profile_id': 'prof-1',
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
            ...sessionJson(),
            'access_token': 'access-new',
            'refresh_token': 'refresh-new',
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
