import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/state/auth_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  testWidgets('shell greys out guest-restricted actions', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: guestShellTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        )..add(
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true);
            return c;
          }),
        ),
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    for (final key in [
      const Key('dm_compose_button'),
      const Key('start_call_button'),
      const Key('friend_invite_button'),
    ]) {
      expect(find.byKey(key), findsOneWidget);
      final button = tester.widget<ElevatedButton>(find.byKey(key));
      expect(button.onPressed, isNull, reason: '$key disabled for guest');
    }
  });
}
