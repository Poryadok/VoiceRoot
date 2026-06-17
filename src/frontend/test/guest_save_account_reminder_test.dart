import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/bootstrap/voice_app_bootstrap.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/guest_save_account_reminder.dart';

import 'support/guest_bootstrap_test_helpers.dart';

const _returningGuestAccountId = 'b2c3d4e5-f6a7-8901-bcde-f12345678901';

void main() {
  testWidgets('returning guest sees save-account reminder at most once per day', (
    tester,
  ) async {
    final sessionStorage = InMemoryAuthSessionStorage();
    await sessionStorage.write(
      const AuthSession(
        accessToken: 'guest-access',
        refreshToken: 'guest-refresh',
        accountId: _returningGuestAccountId,
        activeProfileId: 'guest-prof',
        expiresInSeconds: 900,
      ),
    );
    final guestStorage = InMemoryGuestCredentialsStorage();
    await guestStorage.writePassword('guest-password-12345678');
    await guestStorage.markNicknameCompleted(_returningGuestAccountId);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...guestBootstrapOverrides(
            onRequest: (request) async {
              if (request.url.path == '/api/v1/auth/refresh') {
                return http.Response(
                  jsonEncode({
                    'session': {
                      'access_token': 'guest-access',
                      'refresh_token': 'guest-refresh',
                      'expires_in_seconds': 900,
                      'account_id': _returningGuestAccountId,
                      'profile_id': 'guest-prof',
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
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            },
          ),
          authSessionStorageProvider.overrideWithValue(sessionStorage),
          guestCredentialsStorageProvider.overrideWithValue(guestStorage),
          guestSaveAccountReminderVisibleProvider.overrideWith((ref) async => true),
        ],
        child: const VoiceAppBootstrap(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('guest_save_account_reminder')), findsOneWidget);
  });
}
