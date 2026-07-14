import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/state/auth_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  testWidgets('save-account reminder opens convert-guest modal', (tester) async {
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

    expect(find.byKey(const Key('guest_save_account_reminder')), findsOneWidget);
    await tester.tap(find.byKey(const Key('guest_save_account_reminder_cta')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.byKey(const Key('guest_convert_modal')), findsOneWidget);
    expect(find.byKey(const Key('guest_convert_email')), findsOneWidget);
    expect(find.byKey(const Key('guest_convert_password')), findsOneWidget);
  });
}
