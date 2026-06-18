import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/bootstrap/voice_app_bootstrap.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/guest_bootstrap_test_helpers.dart';

void main() {
  testWidgets('guest first entry skips save-account onboarding reminder', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...guestBootstrapOverrides(
            onRequest: (request) async {
              if (request.url.path == '/api/v1/auth/register') {
                return http.Response(guestRegisterResponseJson(), 200);
              }
              if (request.url.path == '/api/v1/users/onboarding') {
                return http.Response(
                  jsonEncode({
                    'onboarding_state': {
                      'profile_id': 'guest-prof',
                      'completed_steps': [],
                      'completed': false,
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
          onboardingControllerProvider.overrideWith(
            () => OnboardingController(),
          ),
        ],
        child: const VoiceAppBootstrap(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(AuthScreen.continueGuestButtonKey), findsOneWidget);
    await tester.tap(find.byKey(AuthScreen.continueGuestButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('guest_nickname_screen')), findsOneWidget);
    expect(find.byKey(const Key('guest_save_account_reminder')), findsNothing);
    expect(find.byKey(const Key('onboarding_save_account_step')), findsNothing);
  });
}
