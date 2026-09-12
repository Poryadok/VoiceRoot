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
import 'package:voice_frontend/ui/settings/verification_settings_sheet.dart';

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
  testWidgets('shows and mutates links only for the selected profile', (
    tester,
  ) async {
    var twitchLinked = true;
    var youtubeLinkStarted = false;
    var twitchUnlinked = false;
    final mock = MockClient((request) async {
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/auth/linked-accounts') {
        return http.Response(
          jsonEncode({
            'linked_accounts': [
              if (twitchLinked)
                {
                  'platform': 'twitch',
                  'profile_id': 'profile-selected',
                  'external_id': 'tw-1',
                  'external_login': 'selected-twitch',
                },
              {
                'platform': 'youtube',
                'profile_id': 'profile-other',
                'external_id': 'yt-1',
                'external_login': 'other-youtube',
              },
            ],
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/linked-accounts/youtube/link') {
        youtubeLinkStarted = true;
        return http.Response(
          jsonEncode({'authorization_url': 'https://youtube.test/oauth'}),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/auth/linked-accounts/twitch/unlink') {
        twitchLinked = false;
        twitchUnlinked = true;
        return http.Response('', 204);
      }
      return http.Response('not found', 404);
    });
    final gateway = GatewayHttpClient(
      httpClient: mock,
      config: const GatewayConfig(baseUrl: 'http://api.test'),
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
                accessToken: 'access-token',
                refreshToken: 'refresh-token',
                expiresInSeconds: 900,
                accountId: 'account-1',
                activeProfileId: 'profile-selected',
              ),
            );
            return controller;
          }),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(body: VerificationSettingsSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(VerificationSettingsSheet.selectedProfileKey),
      findsOneWidget,
    );
    expect(find.textContaining('profile-selected'), findsOneWidget);
    expect(find.text('selected-twitch'), findsOneWidget);
    expect(find.text('other-youtube'), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsNothing);
    expect(
      find.byKey(VerificationSettingsSheet.youtubeLinkKey),
      findsOneWidget,
    );

    await tester.tap(find.byKey(VerificationSettingsSheet.youtubeLinkKey));
    await tester.pumpAndSettle();
    expect(youtubeLinkStarted, isTrue);

    await tester.tap(find.byKey(const ValueKey('verification_twitch_unlink')));
    await tester.pumpAndSettle();
    expect(twitchUnlinked, isTrue);
    expect(find.text('selected-twitch'), findsNothing);
    expect(find.byKey(VerificationSettingsSheet.twitchLinkKey), findsOneWidget);
  });
}
