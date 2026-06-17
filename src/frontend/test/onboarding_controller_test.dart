import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/onboarding_controller.dart';

void main() {
  ProviderContainer buildContainer(MockClient mock) {
    return ProviderContainer(
      overrides: [
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(mock),
        authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        authControllerProvider.overrideWith((ref) {
          final c = AuthController(
            authClient: ref.watch(voiceAuthClientProvider),
            storage: ref.watch(authSessionStorageProvider),
            guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
          );
          c.state = const AuthState(
            session: AuthSession(
              accessToken: 'access',
              refreshToken: 'refresh',
              accountId: 'acc-1',
              activeProfileId: 'prof-1',
              expiresInSeconds: 900,
            ),
          );
          return c;
        }),
      ],
    );
  }

  Map<String, dynamic> onboardingJson({
    required List<String> completedSteps,
    required bool completed,
  }) =>
      {
        'onboarding_state': {
          'profile_id': 'prof-1',
          'completed_steps': completedSteps,
          'completed': completed,
        },
      };

  test('onboarding progresses through steps in order', () async {
    var stepIndex = 0;
    const steps = ['save_account', 'chats_nav', 'spaces', 'matchmaking', 'wrap_up'];
    final mock = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/users/me/onboarding') {
        return http.Response(
          jsonEncode(onboardingJson(completedSteps: steps.take(stepIndex).toList(), completed: false)),
          200,
        );
      }
      if (req.method == 'POST' && req.url.path == '/api/v1/users/me/onboarding/steps') {
        final body = jsonDecode(utf8.decode(req.bodyBytes)) as Map<String, dynamic>;
        final step = body['step_id'] as String;
        expect(step, steps[stepIndex]);
        stepIndex++;
        return http.Response(
          jsonEncode(onboardingJson(completedSteps: steps.take(stepIndex).toList(), completed: stepIndex == steps.length)),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final container = buildContainer(mock);
    addTearDown(container.dispose);
    final controller = container.read(onboardingControllerProvider.notifier);

    await controller.load();
    expect(controller.state.currentStep, OnboardingStep.saveAccount);

    for (final step in steps) {
      await controller.completeStep(step);
    }
    expect(controller.state.completed, isTrue);
    expect(controller.state.currentStep, isNull);
  });

  test('dismiss marks onboarding completed', () async {
    final mock = MockClient((req) async {
      if (req.method == 'POST' && req.url.path == '/api/v1/users/me/onboarding/steps') {
        return http.Response(
          jsonEncode(onboardingJson(completedSteps: ['dismiss'], completed: true)),
          200,
        );
      }
      if (req.method == 'GET' && req.url.path == '/api/v1/users/me/onboarding') {
        return http.Response(jsonEncode(onboardingJson(completedSteps: [], completed: false)), 200);
      }
      return http.Response('not found', 404);
    });

    final container = buildContainer(mock);
    addTearDown(container.dispose);
    final controller = container.read(onboardingControllerProvider.notifier);
    await controller.load();
    await controller.dismiss();
    expect(controller.state.completed, isTrue);
    expect(controller.state.shouldShowHints, isFalse);
  });

  test('completed onboarding does not re-show hints', () async {
    final mock = MockClient((req) async {
      if (req.method == 'GET' && req.url.path == '/api/v1/users/me/onboarding') {
        return http.Response(
          jsonEncode(onboardingJson(completedSteps: ['wrap_up'], completed: true)),
          200,
        );
      }
      return http.Response('not found', 404);
    });

    final container = buildContainer(mock);
    addTearDown(container.dispose);
    final controller = container.read(onboardingControllerProvider.notifier);
    await controller.load();
    expect(controller.state.completed, isTrue);
    expect(controller.state.shouldShowHints, isFalse);
    expect(controller.state.currentStep, isNull);
  });
}
