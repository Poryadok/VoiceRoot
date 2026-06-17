import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/bootstrap/voice_app_bootstrap.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';

void main() {
  testWidgets('bootstrap creates guest account and shows nickname screen', (
    tester,
  ) async {
    var guestRegisterCalls = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          onboardingControllerProvider.overrideWith(
            () => _CompletedOnboardingController(),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/auth/register') {
                guestRegisterCalls++;
                final body = jsonDecode(request.body) as Map<String, dynamic>;
                expect(body['guest'], isTrue);
                expect(body.containsKey('email'), isFalse);
                return http.Response(
                  jsonEncode({
                    'session': {
                      'access_token': 'guest-access',
                      'refresh_token': 'guest-refresh',
                      'expires_in_seconds': 900,
                      'account_id': 'guest-acc',
                      'profile_id': 'guest-prof',
                    },
                  }),
                  200,
                );
              }
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            }),
          ),
        ],
        child: const VoiceAppBootstrap(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(guestRegisterCalls, 1);
    expect(find.byKey(const Key('guest_nickname_screen')), findsOneWidget);
    expect(find.byKey(AuthScreen.emailFieldKey), findsNothing);
  });
}

class _CompletedOnboardingController extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(completed: true);
}
