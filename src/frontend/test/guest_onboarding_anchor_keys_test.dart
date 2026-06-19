import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_anchor_keys.dart';

import 'support/auth_test_overrides.dart';

void main() {
  testWidgets('guest shell exposes onboarding coach-mark anchor keys', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: guestShellTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        )..add(
          authControllerProvider.overrideWith((ref) {
            final c = authenticatedAuthController(ref);
            c.state = c.state.copyWith(isGuest: true, needsGuestNickname: false);
            return c;
          }),
        ),
        child: const VoiceApp(locale: Locale('en')),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(OnboardingAnchorKeys.chatsNav), findsOneWidget);
    expect(find.byKey(OnboardingAnchorKeys.spaces), findsOneWidget);
    expect(find.byKey(OnboardingAnchorKeys.saveAccountStep), findsOneWidget);

    await tester.tap(find.text('Friends'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.byKey(OnboardingAnchorKeys.matchmaking), findsOneWidget);
  });
}
