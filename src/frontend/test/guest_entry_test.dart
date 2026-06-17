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
import 'package:voice_frontend/state/guest_bootstrap_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/guest_bootstrap_test_helpers.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _guestAccountId = guestBootstrapAccountId;

void main() {
  testWidgets('bootstrap auto-registers guest on web without manual tap', (
    tester,
  ) async {
    var guestRegisterCalls = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...guestBootstrapOverrides(
            onRequest: (request) async {
              if (request.url.path == '/api/v1/auth/register') {
                guestRegisterCalls++;
                final body = jsonDecode(request.body) as Map<String, dynamic>;
                expect(body['guest'], isTrue);
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
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            },
          ),
          webGuestAutoRegisterEnabledProvider.overrideWithValue(true),
        ],
        child: const VoiceAppBootstrap(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      guestRegisterCalls,
      1,
      reason: 'web bootstrap must auto registerGuest when no session',
    );
    expect(find.byKey(AuthScreen.screenKey), findsNothing);
    expect(find.byKey(const Key('guest_nickname_screen')), findsOneWidget);
  });

  testWidgets('continue as guest registers and shows nickname screen', (
    tester,
  ) async {
    var guestRegisterCalls = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: guestBootstrapOverrides(
          onRequest: (request) async {
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
                    'account_id': _guestAccountId,
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
          },
        ),
        child: const VoiceAppBootstrap(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(AuthScreen.continueGuestButtonKey), findsOneWidget);
    await tester.tap(find.byKey(AuthScreen.continueGuestButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pumpAndSettle();

    expect(guestRegisterCalls, 1);
    expect(find.byKey(const Key('guest_nickname_screen')), findsOneWidget);
    expect(find.byKey(AuthScreen.emailFieldKey), findsNothing);
  });
}
