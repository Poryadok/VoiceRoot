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
import 'package:voice_frontend/ui/profile/profile_switcher.dart';

import 'support/voice_test_theme.dart';

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
  test('profile switcher updates session profile_id', () async {
    const before = AuthSession(
      accessToken: 'before',
      refreshToken: 'refresh',
      expiresInSeconds: 900,
      accountId: 'account-1',
      activeProfileId: 'profile-primary',
    );
    const after = AuthSession(
      accessToken: 'after',
      refreshToken: 'refresh-rotated',
      expiresInSeconds: 900,
      accountId: 'account-1',
      activeProfileId: 'profile-alt',
    );
    expect(after.activeProfileId, isNot(equals(before.activeProfileId)));

    final storage = _MemoryAuthStorage();
    await storage.write(before);

    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/auth/switch-profile') {
        expect(req.method, 'POST');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['profile_id'], 'profile-alt');
        return http.Response(
          jsonEncode({
            'access_token': after.accessToken,
            'refresh_token': after.refreshToken,
            'account_id': after.accountId,
            'profile_id': after.activeProfileId,
            'expires_in_seconds': after.expiresInSeconds,
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
                  'username': 'alice',
                  'discriminator': '0001',
                  'display_name': 'Alice',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
                {
                  'id': 'profile-alt',
                  'account_id': 'account-1',
                  'username': 'alice2',
                  'discriminator': '0002',
                  'display_name': 'Gaming',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': false,
                  'verification_type': 'none',
                },
              ],
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final container = ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);

    await container.read(authControllerProvider.notifier).applySession(before);
    final err = await container
        .read(authControllerProvider.notifier)
        .switchActiveProfile('profile-alt');
    expect(err, isNull);

    final session = container.read(authControllerProvider).session;
    expect(session?.activeProfileId, 'profile-alt');
    expect(session?.accessToken, 'after');
  });

  testWidgets('ProfileSwitcher dropdown switches active profile', (tester) async {
    final storage = _MemoryAuthStorage();
    const session = AuthSession(
      accessToken: 'token',
      refreshToken: 'refresh',
      expiresInSeconds: 900,
      accountId: 'account-1',
      activeProfileId: 'profile-primary',
    );
    await storage.write(session);

    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/users/profiles') {
        return http.Response(
          jsonEncode({
            'profile_list': {
              'profiles': [
                {
                  'id': 'profile-primary',
                  'account_id': 'account-1',
                  'username': 'alice',
                  'discriminator': '0001',
                  'display_name': 'Alice',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
                {
                  'id': 'profile-alt',
                  'account_id': 'account-1',
                  'username': 'gaming',
                  'discriminator': '0002',
                  'display_name': 'Gaming',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': false,
                  'verification_type': 'none',
                },
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/auth/switch-profile') {
        return http.Response(
          jsonEncode({
            'access_token': 'token-new',
            'refresh_token': 'refresh',
            'account_id': 'account-1',
            'profile_id': 'profile-alt',
            'expires_in_seconds': 900,
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final container = ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);
    await container.read(authControllerProvider.notifier).applySession(session);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ProfileSwitcher()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ProfileSwitcher.switcherKey), findsOneWidget);
    await tester.tap(find.byKey(ProfileSwitcher.switcherKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Gaming').last);
    await tester.pumpAndSettle();

    expect(
      container.read(authControllerProvider).activeProfileId,
      'profile-alt',
    );
    container.dispose();
  });

  testWidgets('ProfileSwitcher shows guest nickname when profile list is empty',
      (tester) async {
    const guestProfileId = '7ff61e3a-27d9-44be-a636-a3e94f2a5265';
    final mock = MockClient((req) async {
      if (req.url.path == '/api/v1/users/profiles') {
        return http.Response('unavailable', 503);
      }
      if (req.url.path == '/api/v1/users/profiles/$guestProfileId') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': guestProfileId,
              'account_id': 'guest-acc',
              'username': 'playerone',
              'discriminator': '0042',
              'display_name': 'PlayerOne',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    late AuthController authController;
    final container = ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(_MemoryAuthStorage()),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        authControllerProvider.overrideWith((ref) {
          authController = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          authController.state = const AuthState(
            session: AuthSession(
              accessToken: 'guest-access',
              refreshToken: 'guest-refresh',
              accountId: 'guest-acc',
              activeProfileId: guestProfileId,
              expiresInSeconds: 900,
            ),
            isGuest: true,
          );
          return authController;
        }),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ProfileSwitcher()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('PlayerOne'), findsOneWidget);
    expect(find.textContaining('Profile:'), findsNothing);
    container.dispose();
  });
}
