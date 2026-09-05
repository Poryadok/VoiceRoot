import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/chat/chat_archive_screen.dart';
import 'package:voice_frontend/ui/profile/profile_avatar_menu.dart';

import 'support/test_voice_token_catalog.dart';
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

class _NoopProfileSwitchRealtimeBoundary
    implements ProfileSwitchRealtimeBoundary {
  @override
  Set<String> get activeSubscriptions => const {};

  @override
  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff) async {}
}

void main() {
  testWidgets('ProfileAvatarMenuButton switches profile from rail menu', (
    tester,
  ) async {
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
        ...voiceThemeTestOverrides(),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
        realtimeAutoConnectProvider.overrideWithValue(false),
        profileSwitchRealtimeBoundaryProvider.overrideWithValue(
          _NoopProfileSwitchRealtimeBoundary(),
        ),
      ],
    );
    addTearDown(container.dispose);
    await container.read(authControllerProvider.notifier).applySession(session);
    await container.read(myProfilesProvider.future);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: ProfileAvatarMenuButton(
              profile: const VoiceProfile(
                id: 'profile-primary',
                accountId: 'account-1',
                username: 'alice',
                discriminator: '0001',
                displayName: 'Alice',
                isPrimary: true,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(ProfileAvatarMenuButton));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Gaming'));
    await tester.pumpAndSettle();

    expect(
      container.read(authControllerProvider).activeProfileId,
      'profile-alt',
    );
    container.dispose();
  });

  testWidgets('ProfileAvatarMenuButton archive opens archive screen', (
    tester,
  ) async {
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
                  'is_primary': true,
                },
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/chats' &&
          req.url.queryParameters['inbox'] == 'archive') {
        return http.Response(
          jsonEncode({
            'chat_list': {'items': []},
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final container = ProviderContainer(
      overrides: [
        ...voiceThemeTestOverrides(),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(storage),
      ],
    );
    addTearDown(container.dispose);
    await container.read(authControllerProvider.notifier).applySession(session);
    await container.read(myProfilesProvider.future);

    final router = createVoiceGoRouter(
      shellBuilder: (context, state) => Scaffold(
        body: ProfileAvatarMenuButton(
          profile: const VoiceProfile(
            id: 'profile-primary',
            accountId: 'account-1',
            username: 'alice',
            discriminator: '0001',
            displayName: 'Alice',
            isPrimary: true,
          ),
        ),
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          routerConfig: router,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(ProfileAvatarMenuButton));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Archive'));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatArchiveScreen.screenKey), findsOneWidget);
    container.dispose();
  });
}
