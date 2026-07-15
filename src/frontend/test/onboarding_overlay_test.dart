import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/l10n/app_localizations_ru.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_anchor_keys.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _OnboardingAtSpacesStep extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(
    completedSteps: ['save_account', 'chats_nav'],
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    state = OnboardingUiState(
      completedSteps: [...state.completedSteps, stepId],
    );
  }
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
  Future<void> dismiss() => completeStep('dismiss');

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

class _CoachMarkTourController extends OnboardingController {
  final completedSteps = <String>['save_account'];

  @override
  OnboardingUiState build() => const OnboardingUiState(
    completedSteps: ['save_account'],
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> dismiss() => completeStep('dismiss');

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

Widget _onboardingTestApp({
  required List<Override> overrides,
  required Widget child,
  Locale locale = const Locale('en'),
}) {
  return ProviderScope(
    overrides: overrides,
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: OnboardingOverlay(child: child),
    ),
  );
}

Widget _onboardingAnchorsScaffold() {
  return Scaffold(
    body: Stack(
      children: [
        Center(
          child: SizedBox(
            key: OnboardingAnchorKeys.chatsNav,
            width: 48,
            height: 48,
          ),
        ),
        Center(
          child: SizedBox(
            key: OnboardingAnchorKeys.spaces,
            width: 48,
            height: 48,
          ),
        ),
        Center(
          child: SizedBox(
            key: OnboardingAnchorKeys.matchmaking,
            width: 48,
            height: 48,
          ),
        ),
      ],
    ),
  );
}

void main() {
  testWidgets('spaces step Find a space opens global search catalog', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _onboardingTestApp(
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
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingSpacesTitle), findsOneWidget);
    await tester.tap(find.widgetWithText(TextButton, l10n.onboardingSpacesFind));
    await tester.pump();

    final overlayElement = tester.element(find.byType(OnboardingOverlay));
    final container = ProviderScope.containerOf(overlayElement);
    expect(container.read(globalSearchFocusRequestProvider), greaterThan(0));
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  testWidgets('coach-mark skip dismisses onboarding', (tester) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final recording = _RecordingOnboardingController();

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(() => recording),
        ],
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
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.widgetWithText(TextButton, l10n.onboardingSkip), findsOneWidget);
    await tester.tap(find.widgetWithText(TextButton, l10n.onboardingSkip));
    await tester.pumpAndSettle(
      const Duration(milliseconds: 100),
      EnginePhase.sendSemanticsUpdate,
      const Duration(seconds: 2),
    );

    expect(recording.completedSteps, contains('dismiss'));
    expect(recording.state.completed, isTrue);
  });

  testWidgets('coach-mark tour steps through chats, spaces, matchmaking, wrap-up', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final recording = _CoachMarkTourController();

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(() => recording),
        ],
        child: _onboardingAnchorsScaffold(),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingChatsNavTitle), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, l10n.onboardingGotIt));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingSpacesTitle), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, l10n.onboardingLater));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingMatchmakingTitle), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, l10n.onboardingLater));
    await tester.pumpAndSettle();

    expect(find.text(l10n.onboardingWrapUpTitle), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, l10n.onboardingWrapUpStart));
    await tester.pumpAndSettle();

    expect(
      recording.completedSteps,
      [
        'save_account',
        'chats_nav',
        'spaces',
        'matchmaking',
        'wrap_up',
      ],
    );
    expect(recording.state.completed, isFalse);
    expect(recording.state.currentStep, isNull);
  });

  testWidgets('onboarding coach marks use Russian l10n strings', (tester) async {
    final l10n = AppLocalizationsRu();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _onboardingTestApp(
        locale: const Locale('ru'),
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(_OnboardingAtSpacesStep.new),
        ],
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
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingSpacesTitle), findsOneWidget);
    expect(find.text(l10n.onboardingSpacesBody), findsOneWidget);
    expect(find.widgetWithText(TextButton, l10n.onboardingSkip), findsOneWidget);
    expect(find.widgetWithText(FilledButton, l10n.onboardingLater), findsOneWidget);
  });
}
