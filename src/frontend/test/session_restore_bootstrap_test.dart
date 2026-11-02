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
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/guest_bootstrap_test_helpers.dart';

void main() {
  for (final accountType in ['regular', 'guest']) {
    for (final refreshAvailable in [true, false]) {
      testWidgets(
        'restored $accountType with refresh available=$refreshAvailable and unavailable profile shows startup retry',
        (tester) async {
          await tester.binding.setSurfaceSize(const Size(1280, 800));
          addTearDown(() => tester.binding.setSurfaceSize(null));
          final storage = InMemoryAuthSessionStorage();
          final saved = AuthSession(
            accessToken: 'saved-access',
            refreshToken: 'saved-refresh',
            accountId: guestBootstrapAccountId,
            activeProfileId: 'saved-profile',
            expiresInSeconds: 900,
            accountType: accountType,
            emailVerificationRequired: false,
          );
          await storage.write(saved);
          final guestStorage = InMemoryGuestCredentialsStorage();
          await guestStorage.markNicknameCompleted(saved.accountId);
          var profileAvailable = false;
          var profileRequests = 0;
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                ...guestBootstrapOverrides(
                  onRequest: (request) async {
                    if (request.url.path == '/api/v1/auth/refresh') {
                      if (!refreshAvailable) {
                        return http.Response('refresh unavailable', 503);
                      }
                      return http.Response(
                        jsonEncode({'session': saved.toJson()}),
                        200,
                      );
                    }
                    if (request.url.path.startsWith('/api/v1/users/')) {
                      if (request.url.path ==
                          '/api/v1/users/profiles/saved-profile') {
                        profileRequests++;
                        if (profileAvailable) {
                          return http.Response(
                            jsonEncode({
                              'profile': {
                                'id': saved.activeProfileId,
                                'account_id': saved.accountId,
                                'display_name': 'Restored Player',
                                'username': 'restored',
                                'discriminator': '1234',
                              },
                            }),
                            200,
                          );
                        }
                      }
                      return http.Response('profile unavailable', 503);
                    }
                    return http.Response('not found', 404);
                  },
                ),
                authSessionStorageProvider.overrideWithValue(storage),
                guestCredentialsStorageProvider.overrideWithValue(guestStorage),
              ],
              child: const VoiceAppBootstrap(locale: Locale('en')),
            ),
          );
          await tester.pumpAndSettle();

          expect(
            find.byKey(const Key('session_restore_failure')),
            findsOneWidget,
          );
          expect(find.text('Try again'), findsOneWidget);
          expect(find.text('Profile: saved-profile'), findsNothing);
          expect(await storage.read(), saved);

          profileAvailable = true;
          await tester.tap(find.text('Try again'));
          await tester.pumpAndSettle();
          expect(profileRequests, 2);
          expect(
            find.byKey(const Key('session_restore_failure')),
            findsNothing,
          );
          expect(
            find.text(
              accountType == 'guest' ? 'Restored Player' : '@restored#1234',
            ),
            findsOneWidget,
          );
          await tester.pumpWidget(const SizedBox.shrink());
        },
      );
    }
  }

  testWidgets('restore storage failure is visible and retry recovers', (
    tester,
  ) async {
    final storage = _FailsOnceStorage();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...guestBootstrapOverrides(
            onRequest: (request) async {
              if (request.url.path == '/health') {
                return http.Response('ok', 200);
              }
              return http.Response('not found', 404);
            },
          ),
          authSessionStorageProvider.overrideWithValue(storage),
        ],
        child: const VoiceAppBootstrap(locale: Locale('en')),
      ),
    );

    await tester.pumpAndSettle();

    expect(find.textContaining('unavailable'), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
    expect(find.byKey(AuthScreen.screenKey), findsNothing);

    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(storage.readCalls, 2);
    expect(find.byKey(AuthScreen.screenKey), findsOneWidget);
  });
}

class _FailsOnceStorage implements AuthSessionStorage {
  var readCalls = 0;

  @override
  Future<void> clear() async {}

  @override
  Future<AuthSession?> read() async {
    readCalls++;
    if (readCalls == 1) throw StateError('local storage unavailable');
    return null;
  }

  @override
  Future<void> write(AuthSession session) async {}
}
