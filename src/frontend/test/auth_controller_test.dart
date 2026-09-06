import 'dart:async';
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

class _DeferredAuthSessionStorage implements AuthSessionStorage {
  _DeferredAuthSessionStorage({this.persisted});

  AuthSession? persisted;
  final Map<String, Completer<void>> _writeStarted = {};
  final Map<String, Completer<void>> _writeGates = {};

  void pauseWriteFor(String accessToken) {
    _writeGates.putIfAbsent(accessToken, Completer<void>.new);
  }

  Future<void> waitForWrite(String accessToken) {
    return _writeStarted.putIfAbsent(accessToken, Completer<void>.new).future;
  }

  void completeWriteFor(String accessToken) {
    final gate = _writeGates[accessToken];
    if (gate == null || gate.isCompleted) {
      throw StateError('No pending write for $accessToken');
    }
    gate.complete();
  }

  @override
  Future<void> clear() async {
    persisted = null;
  }

  @override
  Future<AuthSession?> read() async => persisted;

  @override
  Future<void> write(AuthSession session) async {
    final started = _writeStarted.putIfAbsent(
      session.accessToken,
      Completer<void>.new,
    );
    if (!started.isCompleted) started.complete();
    final gate = _writeGates[session.accessToken];
    if (gate != null) await gate.future;
    persisted = session;
  }
}

class _DeferredGuestCredentialsStorage extends InMemoryGuestCredentialsStorage {
  final clearStarted = Completer<void>();
  final clearGate = Completer<void>();
  var _firstClear = true;

  @override
  Future<bool> clearIfUnchanged(GuestCredentialsSnapshot snapshot) async {
    if (_firstClear) {
      _firstClear = false;
      clearStarted.complete();
      await clearGate.future;
    }
    return super.clearIfUnchanged(snapshot);
  }
}

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
    GuestCredentialsStorage? guestStorage,
  }) {
    return ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(config),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(
          storage ?? InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          guestStorage ?? InMemoryGuestCredentialsStorage(),
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
      if (req.url.path == '/api/v1/auth/otp/send') {
        return http.Response('', 204);
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

  test('guest conversion persists the verified regular SessionEnvelope and clears guest state', () async {
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword('guest-auto-password-1');
    final storage = InMemoryAuthSessionStorage();
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/convert-guest') {
        return http.Response(
          jsonEncode({
            'session': {
              ...(sessionJson()['session'] as Map<String, dynamic>),
              'access_token': 'guest-converted-access',
              'refresh_token': 'guest-converted-refresh',
              'account_type': 'guest',
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/auth/otp/send') {
        return http.Response('', 204);
      }
      if (req.url.path == '/api/v1/auth/otp/verify') {
        return http.Response(
          jsonEncode({
            'session': {
              ...(sessionJson()['session'] as Map<String, dynamic>),
              'access_token': 'regular-access',
              'refresh_token': 'regular-refresh',
              'account_type': 'regular',
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
        authSessionStorageProvider.overrideWithValue(storage),
        guestCredentialsStorageProvider.overrideWithValue(guestStorage),
      ],
    );
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(
      session: AuthSession(
        accessToken: 'guest-access',
        refreshToken: 'guest-refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
        accountType: 'guest',
      ),
      isGuest: true,
    );

    expect(
      await controller.convertGuest(
        email: 'guest@example.com',
        password: 'user-password1',
      ),
      isNull,
    );
    expect(container.read(authControllerProvider).isGuest, isTrue);
    expect(
      container.read(authControllerProvider).pendingGuestConversionEmail,
      'guest@example.com',
    );
    expect(await guestStorage.readPassword(), 'guest-auto-password-1');

    expect(await controller.verifyGuestConversionEmail('123456'), isNull);
    final state = container.read(authControllerProvider);
    expect(state.isGuest, isFalse);
    expect(state.pendingGuestConversionEmail, isNull);
    expect(state.isGuestConversionPromotionPending, isFalse);
    expect(await guestStorage.readPassword(), isNull);
    expect(await guestStorage.readPendingConversionEmail(), isNull);
    expect(await guestStorage.isGuestConversionPromotionPending(), isFalse);
    expect((await storage.read())?.accessToken, 'regular-access');
    expect((await storage.read())?.refreshToken, 'regular-refresh');
    expect((await storage.read())?.accountType, 'regular');
  });

  test('guest conversion polls guest refreshes without replaying accepted OTP', () async {
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword('guest-auto-password-1');
    var refreshes = 0;
    var verifies = 0;
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/convert-guest') {
        return http.Response(jsonEncode({'session': {...(sessionJson()['session'] as Map<String, dynamic>), 'account_type': 'guest'}}), 200);
      }
      if (req.url.path == '/api/v1/auth/otp/send') return http.Response('', 204);
      if (req.url.path == '/api/v1/auth/otp/verify') {
        verifies++;
        return http.Response('', 204);
      }
      if (req.url.path == '/api/v1/auth/refresh') {
        refreshes++;
        final type = refreshes < 4 ? 'guest' : 'regular';
        return http.Response(jsonEncode({'session': {...(sessionJson()['session'] as Map<String, dynamic>), 'account_type': type}}), 200);
      }
      return http.Response('not found', 404);
    });
    final container = ProviderContainer(overrides: [
      gatewayConfigProvider.overrideWithValue(config),
      httpClientProvider.overrideWithValue(mock),
      authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
      guestCredentialsStorageProvider.overrideWithValue(guestStorage),
    ]);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(
      session: AuthSession(
        accessToken: 'guest-access',
        refreshToken: 'guest-refresh',
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
        expiresInSeconds: 900,
        accountType: 'guest',
      ),
      isGuest: true,
    );

    await controller.convertGuest(email: 'guest@example.com', password: 'user-password1');
    expect(await controller.verifyGuestConversionEmail('123456'), isNull);
    expect(refreshes, 4);
    expect(verifies, 1);
    expect(container.read(authControllerProvider).isGuest, isFalse);
  });

  test('recreated controller resumes a 204 promotion with refresh status only', () async {
    final firstRefreshRequested = Completer<void>();
    final neverCompletes = Completer<http.Response>();
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword('guest-auto-password-1');
    await guestStorage.writePendingConversionEmail('guest@example.com');
    final storage = InMemoryAuthSessionStorage();
    const guest = AuthSession(
      accessToken: 'guest-access',
      refreshToken: 'guest-refresh',
      accountId: 'acc-1',
      activeProfileId: 'prof-1',
      expiresInSeconds: 900,
      accountType: 'guest',
    );
    await storage.write(guest);
    var verifies = 0;
    var refreshes = 0;
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/otp/verify') {
        verifies++;
        return http.Response('', 204);
      }
      if (req.url.path == '/api/v1/auth/refresh') {
        refreshes++;
        if (refreshes == 1) {
          firstRefreshRequested.complete();
          return neverCompletes.future;
        }
        if (refreshes == 2) {
          return http.Response(jsonEncode({'session': guest.toJson()}), 200);
        }
        return http.Response(jsonEncode({'session': {...guest.toJson(), 'access_token': 'regular-access', 'refresh_token': 'regular-refresh', 'account_type': 'regular'}}), 200);
      }
      return http.Response('not found', 404);
    });
    final first = buildContainer(
      mock: mock,
      storage: storage,
      guestStorage: guestStorage,
    );
    addTearDown(first.dispose);
    final firstController = first.read(authControllerProvider.notifier);
    firstController.state = const AuthState(
      session: guest,
      isGuest: true,
      pendingGuestConversionEmail: 'guest@example.com',
    );

    unawaited(firstController.verifyGuestConversionEmail('123456'));
    await firstRefreshRequested.future;
    expect(await guestStorage.isGuestConversionPromotionPending(), isTrue);

    final recreated = buildContainer(
      mock: mock,
      storage: storage,
      guestStorage: guestStorage,
    );
    addTearDown(recreated.dispose);
    final recreatedController = recreated.read(authControllerProvider.notifier);
    await recreatedController.restore();
    expect(recreated.read(authControllerProvider).isGuestConversionPromotionPending, isTrue);
    expect(
      await recreatedController.resumeGuestConversionPromotion(),
      isNull,
    );

    expect(verifies, 1);
    expect(refreshes, 3);
    expect(recreated.read(authControllerProvider).isGuest, isFalse);
  });

  test('restore preserves promotion-pending guest conversion without replaying OTP', () async {
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword('guest-auto-password-1');
    await guestStorage.writePendingConversionEmail('guest@example.com');
    await guestStorage.setGuestConversionPromotionPending(true);
    final storage = InMemoryAuthSessionStorage();
    await storage.write(const AuthSession(
      accessToken: 'guest-access',
      refreshToken: 'guest-refresh',
      accountId: 'acc-1',
      activeProfileId: 'prof-1',
      expiresInSeconds: 900,
      accountType: 'guest',
    ));
    var refreshes = 0;
    final mock = MockClient((req) async {
      if (req.url.path != '/api/v1/auth/refresh') {
        return http.Response('not found', 404);
      }
      refreshes++;
      return http.Response(jsonEncode({'session': {
        ...(sessionJson()['session'] as Map<String, dynamic>),
        'account_type': refreshes < 2 ? 'guest' : 'regular',
      }}), 200);
    });
    final container = ProviderContainer(overrides: [
      gatewayConfigProvider.overrideWithValue(config),
      httpClientProvider.overrideWithValue(mock),
      authSessionStorageProvider.overrideWithValue(storage),
      guestCredentialsStorageProvider.overrideWithValue(guestStorage),
    ]);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);

    await controller.restore();
    expect(container.read(authControllerProvider).isGuestConversionPromotionPending, isTrue);
    expect(container.read(authControllerProvider).pendingGuestConversionEmail, 'guest@example.com');
    expect(await controller.resumeGuestConversionPromotion(), isNull);
    expect(refreshes, 2);
    expect(container.read(authControllerProvider).isGuest, isFalse);
    expect(await guestStorage.readPendingConversionEmail(), isNull);
  });

  test('delayed promotion refresh cannot restore a session after logout', () async {
    final refreshRequested = Completer<void>();
    final refreshResponse = Completer<http.Response>();
    final storage = InMemoryAuthSessionStorage();
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        refreshRequested.complete();
        return refreshResponse.future;
      }
      if (req.url.path == '/api/v1/auth/logout') return http.Response('', 204);
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(
      session: AuthSession(accessToken: 'guest-access', refreshToken: 'guest-refresh', accountId: 'acc-1', activeProfileId: 'prof-1', expiresInSeconds: 900, accountType: 'guest'),
      isGuest: true,
      pendingGuestConversionEmail: 'guest@example.com',
      isGuestConversionPromotionPending: true,
    );
    final resume = controller.resumeGuestConversionPromotion();
    await refreshRequested.future;
    await controller.logout();
    refreshResponse.complete(http.Response(jsonEncode({'session': {...(sessionJson()['session'] as Map<String, dynamic>), 'account_type': 'regular'}}), 200));
    expect(await resume, 'not_authenticated');
    expect(container.read(authControllerProvider).session, isNull);
    expect(await storage.read(), isNull);
  });

  test('delayed promotion refresh cannot overwrite a profile switch', () async {
    final refreshRequested = Completer<void>();
    final refreshResponse = Completer<http.Response>();
    const switched = AuthSession(accessToken: 'profile-b-access', refreshToken: 'profile-b-refresh', accountId: 'acc-1', activeProfileId: 'profile-b', expiresInSeconds: 900, accountType: 'guest');
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        refreshRequested.complete();
        return refreshResponse.future;
      }
      if (req.url.path == '/api/v1/auth/switch-profile') return http.Response(jsonEncode(switched.toJson()), 200);
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(
      session: AuthSession(accessToken: 'guest-access', refreshToken: 'guest-refresh', accountId: 'acc-1', activeProfileId: 'prof-1', expiresInSeconds: 900, accountType: 'guest'),
      isGuest: true,
      pendingGuestConversionEmail: 'guest@example.com',
      isGuestConversionPromotionPending: true,
    );
    final resume = controller.resumeGuestConversionPromotion();
    await refreshRequested.future;
    expect(await controller.switchActiveProfile('profile-b'), isNull);
    refreshResponse.complete(http.Response(jsonEncode({'session': {...(sessionJson()['session'] as Map<String, dynamic>), 'account_type': 'regular'}}), 200));
    expect(await resume, 'not_authenticated');
    expect(container.read(authControllerProvider).session, switched);
  });

  test('logout during guest storage clear cannot restore regular promotion', () async {
    final guestStorage = _DeferredGuestCredentialsStorage();
    await guestStorage.writePassword('old-guest-password');
    await guestStorage.writePendingConversionEmail('old@example.com');
    await guestStorage.setGuestConversionPromotionPending(true);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') return http.Response(jsonEncode({'session': {...(sessionJson()['session'] as Map<String, dynamic>), 'account_type': 'regular'}}), 200);
      if (req.url.path == '/api/v1/auth/logout') return http.Response('', 204);
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, guestStorage: guestStorage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(session: AuthSession(accessToken: 'guest-access', refreshToken: 'guest-refresh', accountId: 'acc-1', activeProfileId: 'prof-1', expiresInSeconds: 900, accountType: 'guest'), isGuest: true, pendingGuestConversionEmail: 'guest@example.com', isGuestConversionPromotionPending: true);
    final resume = controller.resumeGuestConversionPromotion();
    await guestStorage.clearStarted.future;
    await controller.logout();
    await guestStorage.writePassword('new-guest-password');
    await guestStorage.writePendingConversionEmail('new@example.com');
    await guestStorage.setGuestConversionPromotionPending(true);
    guestStorage.clearGate.complete();
    expect(await resume, 'not_authenticated');
    expect(container.read(authControllerProvider).session, isNull);
    expect(await guestStorage.readPassword(), 'new-guest-password');
    expect(await guestStorage.readPendingConversionEmail(), 'new@example.com');
    expect(await guestStorage.isGuestConversionPromotionPending(), isTrue);
  });

  test('profile switch during guest storage clear cannot be overwritten', () async {
    final guestStorage = _DeferredGuestCredentialsStorage();
    await guestStorage.writePassword('old-guest-password');
    await guestStorage.writePendingConversionEmail('old@example.com');
    await guestStorage.setGuestConversionPromotionPending(true);
    const switched = AuthSession(accessToken: 'profile-b-access', refreshToken: 'profile-b-refresh', accountId: 'acc-1', activeProfileId: 'profile-b', expiresInSeconds: 900, accountType: 'guest');
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') return http.Response(jsonEncode({'session': {...(sessionJson()['session'] as Map<String, dynamic>), 'account_type': 'regular'}}), 200);
      if (req.url.path == '/api/v1/auth/switch-profile') return http.Response(jsonEncode(switched.toJson()), 200);
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, guestStorage: guestStorage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = const AuthState(session: AuthSession(accessToken: 'guest-access', refreshToken: 'guest-refresh', accountId: 'acc-1', activeProfileId: 'prof-1', expiresInSeconds: 900, accountType: 'guest'), isGuest: true, pendingGuestConversionEmail: 'guest@example.com', isGuestConversionPromotionPending: true);
    final resume = controller.resumeGuestConversionPromotion();
    await guestStorage.clearStarted.future;
    expect(await controller.switchActiveProfile('profile-b'), isNull);
    await guestStorage.writePassword('profile-b-guest-password');
    await guestStorage.writePendingConversionEmail('profile-b@example.com');
    await guestStorage.setGuestConversionPromotionPending(true);
    guestStorage.clearGate.complete();
    expect(await resume, 'not_authenticated');
    expect(container.read(authControllerProvider).session, switched);
    expect(await guestStorage.readPassword(), 'profile-b-guest-password');
    expect(await guestStorage.readPendingConversionEmail(), 'profile-b@example.com');
    expect(await guestStorage.isGuestConversionPromotionPending(), isTrue);
  });

  test('restore keeps session on network_error refresh failure', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        return http.Response('gateway unavailable', 503);
      }
      return http.Response('not found', 404);
    });
    final storage = InMemoryAuthSessionStorage();
    const saved = AuthSession(
      accessToken: 'old-access',
      refreshToken: 'refresh',
      accountId: 'acc-1',
      activeProfileId: 'prof-1',
      expiresInSeconds: 900,
    );
    await storage.write(saved);
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);

    await container.read(authControllerProvider.notifier).restore();
    final state = container.read(authControllerProvider);
    expect(state.isRestoring, isFalse);
    expect(state.session?.accessToken, 'old-access');
    expect(await storage.read(), saved);
  });

  test('restore clears session on invalid_token refresh failure', () async {
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        return http.Response(jsonEncode({'error': 'invalid_token'}), 401);
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
    expect(container.read(authControllerProvider).session, isNull);
    expect(await storage.read(), isNull);
  });

  test('convertGuest updates session before parallel refresh failure clears it', () async {
    var refreshCalls = 0;
    final guestStorage = InMemoryGuestCredentialsStorage();
    final storage = InMemoryAuthSessionStorage();
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/convert-guest') {
        return http.Response(
          jsonEncode({
            'session': {
              ...(sessionJson()['session'] as Map<String, dynamic>),
              'access_token': 'access-converted',
              'refresh_token': 'refresh-converted',
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/auth/otp/send') {
        return http.Response('', 204);
      }
      if (req.url.path == '/api/v1/auth/refresh') {
        refreshCalls++;
        return http.Response(jsonEncode({'error': 'invalid_token'}), 401);
      }
      return http.Response('not found', 404);
    });
    final container = ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(config),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
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

    final convertFuture = controller.convertGuest(
      email: 'guest@example.com',
      password: 'user-password1',
    );
    await controller.refreshOn401();
    final err = await convertFuture;

    expect(err, isNull);
    expect(refreshCalls, greaterThan(0));
    expect(
      container.read(authControllerProvider).session?.accessToken,
      'access-converted',
    );
    expect((await storage.read())?.accessToken, 'access-converted');
  });

  test('late A refresh cannot overwrite completed profile B switch', () async {
    const sessionA = AuthSession(
      accessToken: 'access-a',
      refreshToken: 'refresh-a',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    const sessionB = AuthSession(
      accessToken: 'access-b',
      refreshToken: 'refresh-b',
      accountId: 'acc-1',
      activeProfileId: 'profile-b',
      expiresInSeconds: 900,
    );
    const refreshedA = AuthSession(
      accessToken: 'access-a-refreshed',
      refreshToken: 'refresh-a-refreshed',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    final refreshRequested = Completer<void>();
    final refreshResponse = Completer<http.Response>();
    final storage = _DeferredAuthSessionStorage(persisted: sessionA);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        if (!refreshRequested.isCompleted) refreshRequested.complete();
        return refreshResponse.future;
      }
      if (req.url.path == '/api/v1/auth/switch-profile') {
        return http.Response(jsonEncode(sessionB.toJson()), 200);
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier)
      ..state = const AuthState(session: sessionA);

    final refresh = controller.refreshOn401();
    await refreshRequested.future;
    expect(await controller.switchActiveProfile('profile-b'), isNull);

    refreshResponse.complete(
      http.Response(jsonEncode(refreshedA.toJson()), 200),
    );
    expect(await refresh, isTrue);

    final current = container.read(authControllerProvider).session;
    expect(current?.activeProfileId, sessionB.activeProfileId);
    expect(current?.accessToken, sessionB.accessToken);
    expect(
      container.read(authorizationHeaderProvider),
      sessionB.authorizationHeader,
    );
    final persisted = await storage.read();
    expect(persisted?.activeProfileId, sessionB.activeProfileId);
    expect(persisted?.accessToken, sessionB.accessToken);
  });

  test('late B storage write cannot overwrite newer C switch', () async {
    const sessionA = AuthSession(
      accessToken: 'access-a',
      refreshToken: 'refresh-a',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    const sessionB = AuthSession(
      accessToken: 'access-b',
      refreshToken: 'refresh-b',
      accountId: 'acc-1',
      activeProfileId: 'profile-b',
      expiresInSeconds: 900,
    );
    const sessionC = AuthSession(
      accessToken: 'access-c',
      refreshToken: 'refresh-c',
      accountId: 'acc-1',
      activeProfileId: 'profile-c',
      expiresInSeconds: 900,
    );
    final storage = _DeferredAuthSessionStorage(persisted: sessionA)
      ..pauseWriteFor(sessionB.accessToken);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/switch-profile') {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        return switch (body['profile_id']) {
          'profile-b' => http.Response(jsonEncode(sessionB.toJson()), 200),
          'profile-c' => http.Response(jsonEncode(sessionC.toJson()), 200),
          _ => http.Response('unknown profile', 400),
        };
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier)
      ..state = const AuthState(session: sessionA);

    final switchB = controller.switchActiveProfile('profile-b');
    await storage.waitForWrite(sessionB.accessToken);
    expect(await controller.switchActiveProfile('profile-c'), isNull);

    storage.completeWriteFor(sessionB.accessToken);
    expect(await switchB, isNull);

    final current = container.read(authControllerProvider).session;
    expect(current?.activeProfileId, sessionC.activeProfileId);
    expect(current?.accessToken, sessionC.accessToken);
    expect(
      container.read(authorizationHeaderProvider),
      sessionC.authorizationHeader,
    );
    final persisted = await storage.read();
    expect(persisted?.activeProfileId, sessionC.activeProfileId);
    expect(persisted?.accessToken, sessionC.accessToken);
  });

  test('late A definitive refresh failure cannot clear completed B switch', () async {
    const sessionA = AuthSession(
      accessToken: 'access-a',
      refreshToken: 'refresh-a',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    const sessionB = AuthSession(
      accessToken: 'access-b',
      refreshToken: 'refresh-b',
      accountId: 'acc-1',
      activeProfileId: 'profile-b',
      expiresInSeconds: 900,
    );
    final refreshRequested = Completer<void>();
    final refreshResponse = Completer<http.Response>();
    final storage = _DeferredAuthSessionStorage(persisted: sessionA);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        if (!refreshRequested.isCompleted) refreshRequested.complete();
        return refreshResponse.future;
      }
      if (req.url.path == '/api/v1/auth/switch-profile') {
        return http.Response(jsonEncode(sessionB.toJson()), 200);
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier)
      ..state = const AuthState(session: sessionA);

    final refresh = controller.refreshOn401();
    await refreshRequested.future;
    expect(await controller.switchActiveProfile('profile-b'), isNull);

    refreshResponse.complete(
      http.Response(jsonEncode({'error': 'invalid_token'}), 401),
    );
    expect(await refresh, isFalse);

    final current = container.read(authControllerProvider).session;
    expect(current?.activeProfileId, sessionB.activeProfileId);
    expect(current?.accessToken, sessionB.accessToken);
    expect(
      container.read(authorizationHeaderProvider),
      sessionB.authorizationHeader,
    );
    final persisted = await storage.read();
    expect(persisted?.activeProfileId, sessionB.activeProfileId);
    expect(persisted?.accessToken, sessionB.accessToken);
  });

  test('logout prevents a delayed profile switch from restoring its session', () async {
    const sessionA = AuthSession(
      accessToken: 'access-a',
      refreshToken: 'refresh-a',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    const sessionB = AuthSession(
      accessToken: 'access-b',
      refreshToken: 'refresh-b',
      accountId: 'acc-1',
      activeProfileId: 'profile-b',
      expiresInSeconds: 900,
    );
    final storage = _DeferredAuthSessionStorage(persisted: sessionA)
      ..pauseWriteFor(sessionB.accessToken);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/switch-profile') {
        return http.Response(jsonEncode(sessionB.toJson()), 200);
      }
      if (req.url.path == '/api/v1/auth/logout') {
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier)
      ..state = const AuthState(session: sessionA);

    final switchB = controller.switchActiveProfile('profile-b');
    await storage.waitForWrite(sessionB.accessToken);
    await controller.logout();

    storage.completeWriteFor(sessionB.accessToken);
    expect(await switchB, isNull);

    expect(container.read(authControllerProvider).session, isNull);
    expect(container.read(authorizationHeaderProvider), isNull);
    expect(await storage.read(), isNull);
  });

  test('logout prevents a delayed refresh from restoring its session', () async {
    const sessionA = AuthSession(
      accessToken: 'access-a',
      refreshToken: 'refresh-a',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    const refreshedA = AuthSession(
      accessToken: 'access-a-refreshed',
      refreshToken: 'refresh-a-refreshed',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    final refreshRequested = Completer<void>();
    final refreshResponse = Completer<http.Response>();
    final storage = _DeferredAuthSessionStorage(persisted: sessionA);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/refresh') {
        if (!refreshRequested.isCompleted) refreshRequested.complete();
        return refreshResponse.future;
      }
      if (req.url.path == '/api/v1/auth/logout') {
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier)
      ..state = const AuthState(session: sessionA);

    final refresh = controller.refreshOn401();
    await refreshRequested.future;
    await controller.logout();

    refreshResponse.complete(
      http.Response(jsonEncode(refreshedA.toJson()), 200),
    );
    expect(await refresh, isTrue);

    expect(container.read(authControllerProvider).session, isNull);
    expect(container.read(authorizationHeaderProvider), isNull);
    expect(await storage.read(), isNull);
  });

  test('switch started during logout cannot restore a session', () async {
    const sessionA = AuthSession(
      accessToken: 'access-a',
      refreshToken: 'refresh-a',
      accountId: 'acc-1',
      activeProfileId: 'profile-a',
      expiresInSeconds: 900,
    );
    const sessionB = AuthSession(
      accessToken: 'access-b',
      refreshToken: 'refresh-b',
      accountId: 'acc-1',
      activeProfileId: 'profile-b',
      expiresInSeconds: 900,
    );
    final logoutRequested = Completer<void>();
    final logoutResponse = Completer<http.Response>();
    final switchResponse = Completer<http.Response>();
    final storage = _DeferredAuthSessionStorage(persisted: sessionA);
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/logout') {
        if (!logoutRequested.isCompleted) logoutRequested.complete();
        return logoutResponse.future;
      }
      if (req.url.path == '/api/v1/auth/switch-profile') {
        return switchResponse.future;
      }
      return http.Response('not found', 404);
    });
    final container = buildContainer(mock: mock, storage: storage);
    addTearDown(container.dispose);
    final controller = container.read(authControllerProvider.notifier)
      ..state = const AuthState(session: sessionA);

    final logout = controller.logout();
    await logoutRequested.future;
    final switchB = controller.switchActiveProfile('profile-b');

    logoutResponse.complete(http.Response('', 204));
    await logout;
    switchResponse.complete(http.Response(jsonEncode(sessionB.toJson()), 200));
    await switchB;

    expect(container.read(authControllerProvider).session, isNull);
    expect(container.read(authorizationHeaderProvider), isNull);
    expect(await storage.read(), isNull);
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
