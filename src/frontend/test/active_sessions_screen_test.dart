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
import 'package:voice_frontend/ui/settings/active_sessions_screen.dart';

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
  testWidgets('active sessions screen revokes other device', (tester) async {
    var revokeCalled = false;
    final mock = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/auth/sessions') {
        return http.Response(
          jsonEncode({
            'sessions': [
              {
                'id': 'sess-current',
                'device_info_json': '{"platform":"flutter"}',
                'current': true,
              },
              {
                'id': 'sess-other',
                'device_info_json': '{"platform":"web"}',
                'current': false,
              },
            ],
          }),
          200,
        );
      }
      if (req.method == 'POST' &&
          req.url.path == '/api/v1/auth/sessions/sess-other/revoke') {
        revokeCalled = true;
        return http.Response('', 204);
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
          home: const ActiveSessionsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ActiveSessionsScreen.sessionRowKey('sess-other')), findsOneWidget);
    expect(
      find.byKey(ActiveSessionsScreen.revokeButtonKey('sess-other')),
      findsOneWidget,
    );
    expect(
      find.byKey(ActiveSessionsScreen.revokeButtonKey('sess-current')),
      findsNothing,
    );

    await tester.tap(
      find.byKey(ActiveSessionsScreen.revokeButtonKey('sess-other')),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(revokeCalled, isTrue);
  });
}
