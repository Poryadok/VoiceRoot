import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/bootstrap/voice_app_bootstrap.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/ui/auth/auth_screen.dart';

import 'support/guest_bootstrap_test_helpers.dart';

void main() {
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
