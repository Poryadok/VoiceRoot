import 'dart:async';
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
import 'package:voice_frontend/state/auth_providers.dart';
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

class _DelayedOnboardingController extends OnboardingController {
  _DelayedOnboardingController({
    required this.completedSteps,
    required this.delayedStep,
  });

  final List<String> completedSteps;
  final String delayedStep;
  final completion = Completer<void>();

  @override
  OnboardingUiState build() => OnboardingUiState(
    completedSteps: completedSteps,
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    if (stepId == delayedStep) await completion.future;
    state = OnboardingUiState(
      completedSteps: [...state.completedSteps, stepId],
    );
  }
}

class _FailedOnboardingController extends OnboardingController {
  _FailedOnboardingController({required this.completedSteps});

  final List<String> completedSteps;
  var completeCalls = 0;

  @override
  OnboardingUiState build() => OnboardingUiState(
    completedSteps: completedSteps,
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    completeCalls++;
  }
}

AuthController _guestAuthController(Ref ref) {
  final controller = authenticatedAuthController(ref);
  controller.state = controller.state.copyWith(isGuest: true);
  return controller;
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
  testWidgets('spaces step opens search for a known space', (
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

  testWidgets('coach-mark tour defers matchmaking until its navigation trigger', (
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

    final overlayElement = tester.element(find.byType(OnboardingOverlay));
    final container = ProviderScope.containerOf(overlayElement);
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
    expect(find.text(l10n.onboardingMatchmakingTitle), findsNothing);

    expect(
      recording.completedSteps,
      [
        'save_account',
        'chats_nav',
        'spaces',
      ],
    );
    expect(recording.state.completed, isFalse);
    expect(recording.state.currentStep, OnboardingStep.matchmaking);
  });

  testWidgets('matchmaking coach-mark appears when social navigation is active', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final controller = _DelayedOnboardingController(
      completedSteps: ['save_account', 'chats_nav', 'spaces'],
      delayedStep: 'matchmaking',
    );

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          navigationSectionProvider.overrideWith(
            (ref) => NavigationSection.social,
          ),
          onboardingControllerProvider.overrideWith(() => controller),
        ],
        child: _onboardingAnchorsScaffold(),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingMatchmakingTitle), findsOneWidget);
  });

  testWidgets('coach-mark waits for delayed completion before showing next step', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final delayed = _DelayedOnboardingController(
      completedSteps: ['save_account'],
      delayedStep: 'chats_nav',
    );

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(() => delayed),
        ],
        child: _onboardingAnchorsScaffold(),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.widgetWithText(FilledButton, l10n.onboardingGotIt));
    await tester.pump();
    expect(find.text(l10n.onboardingChatsNavTitle), findsNothing);

    delayed.completion.complete();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text(l10n.onboardingSpacesTitle), findsOneWidget);
  });

  testWidgets('coach-mark reappears when completion fails', (tester) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final failed = _FailedOnboardingController(
      completedSteps: ['save_account'],
    );

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(() => failed),
        ],
        child: _onboardingAnchorsScaffold(),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(find.widgetWithText(FilledButton, l10n.onboardingGotIt));
    await tester.pump();

    expect(failed.completeCalls, 1);
    expect(find.text(l10n.onboardingChatsNavTitle), findsOneWidget);
  });

  testWidgets('secondary coach CTA waits for completion before the next step', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final delayed = _DelayedOnboardingController(
      completedSteps: ['save_account', 'chats_nav'],
      delayedStep: 'spaces',
    );

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          onboardingControllerProvider.overrideWith(() => delayed),
        ],
        child: _onboardingAnchorsScaffold(),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    await tester.tap(
      find.widgetWithText(TextButton, l10n.onboardingSpacesFind),
    );
    await tester.pump();
    expect(find.text(l10n.onboardingMatchmakingTitle), findsNothing);

    delayed.completion.complete();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.text(l10n.onboardingMatchmakingTitle), findsNothing);

    final overlayElement = tester.element(find.byType(OnboardingOverlay));
    final container = ProviderScope.containerOf(overlayElement);
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  testWidgets('guest auto-skip does not retry a failed completion', (tester) async {
    final failed = _FailedOnboardingController(completedSteps: const []);

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          authControllerProvider.overrideWith(_guestAuthController),
          onboardingControllerProvider.overrideWith(() => failed),
        ],
        child: _onboardingAnchorsScaffold(),
      ),
    );
    await tester.pump();
    await tester.pump();

    expect(failed.completeCalls, 1);
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

  test('onboarding copy describes the profile and invite-only space flow', () {
    expect(
      AppLocalizationsEn().onboardingSaveAccountBody,
      'Choose a nickname and add an avatar so people can recognize you.',
    );
    expect(
      AppLocalizationsEn().onboardingSpacesBody,
      "Spaces are communities with channels and voice rooms. Search for a space you know, join with a friend's invite, or create your own.",
    );
    expect(AppLocalizationsEn().onboardingSpacesFind, 'Open search');
    expect(
      AppLocalizationsRu().onboardingSaveAccountBody,
      'Укажи ник и добавь аватар, чтобы тебя было легко узнать.',
    );
    expect(
      AppLocalizationsRu().onboardingSpacesBody,
      'Спейсы — это сообщества с каналами и войс-чатами. Ищи знакомые спейсы в поиске, вступай по инвайту от друга или создай свой.',
    );
    expect(AppLocalizationsRu().onboardingSpacesFind, 'Открыть поиск');
  });
}
