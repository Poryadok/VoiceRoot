import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/bootstrap/voice_app_bootstrap.dart';
import 'package:voice_frontend/state/auth_providers.dart';

import 'support/guest_bootstrap_test_helpers.dart';

const _returningGuestAccountId = 'b2c3d4e5-f6a7-8901-bcde-f12345678901';

void main() {
  testWidgets(
    'returning guest sees save-account reminder at most once per day',
    (tester) async {
      SharedPreferences.setMockInitialValues({
        'voice.auth.guest_reminder_first_entry.$_returningGuestAccountId': true,
      });
      var marks = 0;
      final sessionStorage = InMemoryAuthSessionStorage();
      await sessionStorage.write(
        const AuthSession(
          accessToken: 'guest-access',
          refreshToken: 'guest-refresh',
          accountId: _returningGuestAccountId,
          activeProfileId: 'guest-prof',
          expiresInSeconds: 900,
          emailVerificationRequired: false,
        ),
      );
      final guestStorage = InMemoryGuestCredentialsStorage();
      await guestStorage.writePassword('guest-password-12345678');
      await guestStorage.markNicknameCompleted(_returningGuestAccountId);
      Future<http.Response> handleRequest(http.Request request) async {
        if (request.url.path == '/api/v1/auth/refresh') {
          return http.Response(
            jsonEncode({
              'session': {
                'access_token': 'guest-access',
                'refresh_token': 'guest-refresh',
                'expires_in_seconds': 900,
                'account_id': _returningGuestAccountId,
                'profile_id': 'guest-prof',
                'email_verification_required': false,
                'account_type': 'guest',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/users/me') {
          return http.Response(
            jsonEncode({
              'profile': {
                'id': 'guest-prof',
                'account_id': _returningGuestAccountId,
                'display_name': 'ReturningGuest',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/auth/guest-reminder') {
          return http.Response(jsonEncode({'should_show': true}), 200);
        }
        if (request.url.path == '/api/v1/auth/guest-reminder/mark') {
          marks++;
          return http.Response('{}', 200);
        }
        if (request.url.path == '/health') return http.Response('ok', 200);
        return http.Response('not found', 404);
      }

      Widget app() => ProviderScope(
        overrides: [
          ...guestBootstrapOverrides(onRequest: handleRequest),
          authSessionStorageProvider.overrideWithValue(sessionStorage),
          guestCredentialsStorageProvider.overrideWithValue(guestStorage),
        ],
        child: const VoiceAppBootstrap(locale: Locale('en')),
      );

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('guest_save_account_reminder')),
        findsOneWidget,
      );
      expect(marks, 1);

      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpWidget(app());
      await tester.pumpAndSettle();
      expect(
        find.byKey(const Key('guest_save_account_reminder')),
        findsNothing,
      );
      expect(marks, 1);
    },
  );
}
