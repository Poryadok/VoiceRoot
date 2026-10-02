import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/l10n/app_localizations_ru.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/guest_save_account_reminder.dart';
import 'package:voice_frontend/ui/auth/guest_save_account_reminder_banner.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_anchor_keys.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _OnboardingAtSpacesStep extends OnboardingController {
  @override
  OnboardingUiState build() => const OnboardingUiState(
    loaded: true,
    completedSteps: ['save_account', 'chats_nav'],
  );

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    state = OnboardingUiState(
      loaded: true,
      completedSteps: [...state.completedSteps, stepId],
    );
  }
}

class _RecordingOnboardingController extends OnboardingController {
  final completedSteps = <String>[];

  @override
  OnboardingUiState build() => const OnboardingUiState(
    loaded: true,
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
      state = const OnboardingUiState(completed: true, loaded: true);
      return;
    }
    state = OnboardingUiState(
      loaded: true,
      completedSteps: [...state.completedSteps, stepId],
    );
  }
}

class _CoachMarkTourController extends OnboardingController {
  final completedSteps = <String>['save_account'];

  @override
  OnboardingUiState build() =>
      const OnboardingUiState(loaded: true, completedSteps: ['save_account']);

  @override
  Future<void> load() async {}

  @override
  Future<void> dismiss() => completeStep('dismiss');

  @override
  Future<void> completeStep(String stepId) async {
    completedSteps.add(stepId);
    if (stepId == 'dismiss') {
      state = const OnboardingUiState(completed: true, loaded: true);
      return;
    }
    state = OnboardingUiState(
      loaded: true,
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
  OnboardingUiState build() =>
      OnboardingUiState(loaded: true, completedSteps: completedSteps);

  @override
  Future<void> load() async {}

  @override
  Future<void> completeStep(String stepId) async {
    if (stepId == delayedStep) await completion.future;
    state = OnboardingUiState(
      loaded: true,
      completedSteps: [...state.completedSteps, stepId],
    );
  }
}

class _FailedOnboardingController extends OnboardingController {
  _FailedOnboardingController({required this.completedSteps});

  final List<String> completedSteps;
  var completeCalls = 0;

  @override
  OnboardingUiState build() =>
      OnboardingUiState(loaded: true, completedSteps: completedSteps);

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
  testWidgets(
    'profile arriving during state load does not show an already dismissed step',
    (tester) async {
      final l10n = AppLocalizationsEn();
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final profile = Completer<VoiceProfile>();
      final requestStarted = Completer<void>();
      final response = Completer<http.Response>();

      await tester.pumpWidget(
        _onboardingTestApp(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((request) async {
                if (request.url.path == '/api/v1/users/me/onboarding') {
                  if (!requestStarted.isCompleted) requestStarted.complete();
                  return response.future;
                }
                return http.Response('{}', 404);
              }),
            ),
            activeProfileProvider.overrideWith((ref) => profile.future),
          ],
          child: const Scaffold(body: SizedBox.expand()),
        ),
      );
      await tester.pump();
      await requestStarted.future;
      profile.complete(
        const VoiceProfile(
          id: 'prof-test',
          accountId: 'acc-test',
          username: 'voiceuser',
          discriminator: '4242',
          displayName: 'Voice User',
          isPrimary: true,
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);

      response.complete(
        http.Response(
          jsonEncode({
            'onboarding_state': {
              'completed': true,
              'completed_steps': ['save_account'],
            },
          }),
          200,
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
    },
  );

  testWidgets('failed onboarding state load does not show save-account modal', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final profile = Completer<VoiceProfile>();
    final requestStarted = Completer<void>();
    final response = Completer<http.Response>();

    await tester.pumpWidget(
      _onboardingTestApp(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((request) async {
              if (request.url.path == '/api/v1/users/me/onboarding') {
                if (!requestStarted.isCompleted) requestStarted.complete();
                return response.future;
              }
              return http.Response('{}', 404);
            }),
          ),
          activeProfileProvider.overrideWith((ref) => profile.future),
        ],
        child: const Scaffold(body: SizedBox.expand()),
      ),
    );
    await tester.pump();
    await requestStarted.future;
    profile.complete(
      const VoiceProfile(
        id: 'prof-test',
        accountId: 'acc-test',
        username: 'voiceuser',
        discriminator: '4242',
        displayName: 'Voice User',
        isPrimary: true,
      ),
    );
    await tester.pump();
    response.complete(http.Response('{}', 503));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
  });

  testWidgets('regular account never sees the save-account profile modal', (
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
              if (request.url.path == '/api/v1/users/me/onboarding') {
                return http.Response(
                  jsonEncode({
                    'onboarding_state': {
                      'completed': false,
                      'completed_steps': [],
                    },
                  }),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          activeProfileProvider.overrideWith(
            (ref) async => const VoiceProfile(
              id: 'prof-test',
              accountId: 'acc-test',
              username: 'voiceuser',
              discriminator: '4242',
              displayName: 'Voice User',
              isPrimary: true,
            ),
          ),
        ],
        child: const Scaffold(body: SizedBox.expand()),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
  });

  testWidgets(
    'guest sees one guest-only save-account prompt, not profile modal',
    (tester) async {
      final l10n = AppLocalizationsEn();
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      var saveAccountStepCompletions = 0;
      await tester.pumpWidget(
        _onboardingTestApp(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((request) async {
                if (request.url.path == '/api/v1/users/me/onboarding') {
                  return http.Response(
                    jsonEncode({
                      'onboarding_state': {
                        'completed': false,
                        'completed_steps': [],
                      },
                    }),
                    200,
                  );
                }
                if (request.url.path == '/api/v1/users/me/onboarding/steps') {
                  final body =
                      jsonDecode(utf8.decode(request.bodyBytes))
                          as Map<String, dynamic>;
                  expect(body['step_id'], 'save_account');
                  saveAccountStepCompletions++;
                  return http.Response(
                    jsonEncode({
                      'onboarding_state': {
                        'completed': false,
                        'completed_steps': ['save_account'],
                      },
                    }),
                    200,
                  );
                }
                return http.Response('{}', 404);
              }),
            ),
            authControllerProvider.overrideWith(_guestAuthController),
            guestSaveAccountReminderVisibleProvider.overrideWith(
              (ref) async => true,
            ),
            activeProfileProvider.overrideWith(
              (ref) async => const VoiceProfile(
                id: 'guest-profile',
                accountId: 'guest-account',
                username: 'guestuser',
                discriminator: '1234',
                displayName: 'Guest User',
                isPrimary: true,
              ),
            ),
          ],
          child: const Scaffold(
            body: Column(
              children: [
                GuestSaveAccountReminderBanner(),
                Expanded(child: SizedBox.expand()),
              ],
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      expect(
        find.byKey(GuestSaveAccountReminderBanner.bannerKey),
        findsOneWidget,
      );
      expect(find.byKey(GuestSaveAccountReminderBanner.ctaKey), findsOneWidget);
      expect(saveAccountStepCompletions, 1);
      expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
    },
  );

  testWidgets('onboarding skip stays dismissed after app reload', (
    tester,
  ) async {
    final l10n = AppLocalizationsEn();
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    var persistedCompleted = false;
    var getCount = 0;
    var postCount = 0;
    final client = MockClient((request) async {
      expect(request.headers['authorization'], 'Bearer access-p1');
      if (request.method == 'GET' &&
          request.url.path == '/api/v1/users/me/onboarding') {
        getCount++;
        return http.Response(
          jsonEncode({
            'onboarding_state': {
              'profile_id': 'p1',
              'completed_steps': persistedCompleted
                  ? ['save_account', 'dismiss']
                  : ['save_account'],
              'completed': persistedCompleted,
            },
          }),
          200,
        );
      }
      if (request.method == 'POST' &&
          request.url.path == '/api/v1/users/me/onboarding/steps') {
        postCount++;
        final body =
            jsonDecode(utf8.decode(request.bodyBytes)) as Map<String, dynamic>;
        expect(body['step_id'], 'dismiss');
        persistedCompleted = true;
        return http.Response(
          jsonEncode({
            'onboarding_state': {
              'profile_id': 'p1',
              'completed_steps': ['dismiss'],
              'completed': true,
            },
          }),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    List<Override> overrides() => [
      ...voiceAppTestOverrides(client: client),
      authControllerProvider.overrideWith((ref) {
        final controller = authenticatedAuthController(ref);
        controller.state = controller.state.copyWith(
          session: const AuthSession(
            accessToken: 'access-p1',
            refreshToken: 'refresh-p1',
            accountId: 'account-p1',
            activeProfileId: 'p1',
            expiresInSeconds: 900,
          ),
        );
        return controller;
      }),
      activeProfileProvider.overrideWith(
        (ref) async => const VoiceProfile(
          id: 'p1',
          accountId: 'account-p1',
          username: 'voiceuser',
          discriminator: '4242',
          displayName: 'Voice User',
          isPrimary: true,
        ),
      ),
    ];

    Widget app() => _onboardingTestApp(
      overrides: overrides(),
      child: _onboardingAnchorsScaffold(),
    );

    await tester.pumpWidget(app());
    await tester.pumpAndSettle();
    expect(getCount, 1);
    expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
    expect(find.text(l10n.onboardingChatsNavTitle), findsOneWidget);

    await tester.tap(find.widgetWithText(TextButton, l10n.onboardingSkip));
    await tester.pumpAndSettle();
    expect(postCount, 1);
    expect(persistedCompleted, isTrue);
    expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpWidget(app());
    await tester.pumpAndSettle();

    expect(getCount, 2);
    expect(postCount, 1);
    expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
  });

  testWidgets(
    'failed save-account step completion does not show profile modal',
    (tester) async {
      final l10n = AppLocalizationsEn();
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final failed = _FailedOnboardingController(completedSteps: const []);

      await tester.pumpWidget(
        _onboardingTestApp(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((_) async => http.Response('{}', 404)),
            ),
            activeProfileProvider.overrideWith(
              (ref) async => const VoiceProfile(
                id: 'prof-test',
                accountId: 'acc-test',
                username: 'voiceuser',
                discriminator: '4242',
                displayName: 'Voice User',
                isPrimary: true,
              ),
            ),
            onboardingControllerProvider.overrideWith(() => failed),
          ],
          child: const Scaffold(body: SizedBox.expand()),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      await tester.pumpAndSettle(const Duration(milliseconds: 100));
      expect(failed.completeCalls, 1);
      expect(find.text(l10n.onboardingSaveAccountTitle), findsNothing);
      expect(find.text(l10n.onboardingDismissFailed), findsNothing);
    },
  );

  testWidgets('spaces step opens search for a known space', (tester) async {
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
          onboardingControllerProvider.overrideWith(
            _OnboardingAtSpacesStep.new,
          ),
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
    await tester.tap(
      find.widgetWithText(TextButton, l10n.onboardingSpacesFind),
    );
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

    expect(
      find.widgetWithText(TextButton, l10n.onboardingSkip),
      findsOneWidget,
    );
    await tester.tap(find.widgetWithText(TextButton, l10n.onboardingSkip));
    await tester.pumpAndSettle(
      const Duration(milliseconds: 100),
      EnginePhase.sendSemanticsUpdate,
      const Duration(seconds: 2),
    );

    expect(recording.completedSteps, contains('dismiss'));
    expect(recording.state.completed, isTrue);
  });

  testWidgets(
    'coach-mark tour defers matchmaking until its navigation trigger',
    (tester) async {
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
      expect(
        container.read(navigationSectionProvider),
        NavigationSection.chats,
      );
      expect(find.text(l10n.onboardingMatchmakingTitle), findsNothing);

      expect(recording.completedSteps, ['save_account', 'chats_nav', 'spaces']);
      expect(recording.state.completed, isFalse);
      expect(recording.state.currentStep, OnboardingStep.matchmaking);
    },
  );

  testWidgets(
    'matchmaking coach-mark appears when social navigation is active',
    (tester) async {
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
    },
  );

  testWidgets(
    'coach-mark waits for delayed completion before showing next step',
    (tester) async {
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
    },
  );

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

  testWidgets('guest auto-skip does not retry a failed completion', (
    tester,
  ) async {
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

  testWidgets('onboarding coach marks use Russian l10n strings', (
    tester,
  ) async {
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
          onboardingControllerProvider.overrideWith(
            _OnboardingAtSpacesStep.new,
          ),
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
    expect(
      find.widgetWithText(TextButton, l10n.onboardingSkip),
      findsOneWidget,
    );
    expect(
      find.widgetWithText(FilledButton, l10n.onboardingLater),
      findsOneWidget,
    );
  });

  test('onboarding copy describes the invite-only space flow', () {
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
