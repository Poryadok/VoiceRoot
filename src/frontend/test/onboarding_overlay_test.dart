import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_anchor_keys.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_overlay.dart';

import 'support/auth_test_overrides.dart';

class _OnboardingAtSpacesStep extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(
    completedSteps: ['save_account', 'chats_nav'],
  );

  @override
  Future<void> load() async {}
}

class _RecordingOnboardingController extends OnboardingController {
  final completedSteps = <String>[];

  @override
  OnboardingUiState build() => const OnboardingUiState(
    completedSteps: ['save_account', 'chats_nav'],
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    completedSteps.add(stepId);
    if (stepId == 'dismiss') {
      state = const OnboardingUiState(completed: true);
      return;
    }
    state = OnboardingUiState(
      completedSteps: [...state.completedSteps, stepId],
    );
  }
}

void main() {
  testWidgets('spaces step Find a space opens global search catalog', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((request) async {
              if (request.url.path.endsWith('/onboarding')) {
                return http.Response(
                  jsonEncode({
                    'onboarding_state': {
                      'completed': false,
                      'completed_steps': ['save_account', 'chats_nav'],
                    },
                  }),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          onboardingControllerProvider.overrideWith(_OnboardingAtSpacesStep.new),
        ],
        child: MaterialApp(
          home: OnboardingOverlay(
            child: Scaffold(
              body: Center(
                child: SizedBox(
                  key: OnboardingAnchorKeys.spaces,
                  width: 48,
                  height: 48,
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Spaces'), findsOneWidget);
    await tester.tap(find.text('Find a space'));
    await tester.pump();

    final overlayElement = tester.element(find.byType(OnboardingOverlay));
    final container = ProviderScope.containerOf(overlayElement);
    expect(container.read(globalSearchFocusRequestProvider), greaterThan(0));
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  testWidgets('coach-mark skip dismisses onboarding', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final recording = _RecordingOnboardingController();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(() => recording),
        ],
        child: MaterialApp(
          home: OnboardingOverlay(
            child: Scaffold(
              body: Center(
                child: SizedBox(
                  key: OnboardingAnchorKeys.spaces,
                  width: 48,
                  height: 48,
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.text('Skip'));
    await tester.pump();

    expect(recording.completedSteps, contains('dismiss'));
    expect(recording.state.completed, isTrue);
  });
}
