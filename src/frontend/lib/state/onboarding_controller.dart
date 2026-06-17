import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/onboarding_client.dart';
import 'auth_providers.dart';

enum OnboardingStep {
  saveAccount,
  chatsNav,
  spaces,
  matchmaking,
  wrapUp,
}

const _stepOrder = [
  OnboardingStep.saveAccount,
  OnboardingStep.chatsNav,
  OnboardingStep.spaces,
  OnboardingStep.matchmaking,
  OnboardingStep.wrapUp,
];

const _stepIds = {
  OnboardingStep.saveAccount: 'save_account',
  OnboardingStep.chatsNav: 'chats_nav',
  OnboardingStep.spaces: 'spaces',
  OnboardingStep.matchmaking: 'matchmaking',
  OnboardingStep.wrapUp: 'wrap_up',
};

class OnboardingUiState {
  const OnboardingUiState({
    this.completed = false,
    this.completedSteps = const [],
    this.loading = false,
  });

  final bool completed;
  final List<String> completedSteps;
  final bool loading;

  bool get shouldShowHints => !completed;

  OnboardingStep? get currentStep {
    if (completed) return null;
    for (final step in _stepOrder) {
      final id = _stepIds[step]!;
      if (!completedSteps.contains(id)) return step;
    }
    return null;
  }

  OnboardingUiState copyWith({
    bool? completed,
    List<String>? completedSteps,
    bool? loading,
  }) {
    return OnboardingUiState(
      completed: completed ?? this.completed,
      completedSteps: completedSteps ?? this.completedSteps,
      loading: loading ?? this.loading,
    );
  }
}

final voiceOnboardingClientProvider = Provider<VoiceOnboardingClient>((ref) {
  return VoiceOnboardingClient(gateway: ref.watch(gatewayHttpClientProvider));
});

class OnboardingController extends Notifier<OnboardingUiState> {
  @override
  OnboardingUiState build() => const OnboardingUiState();

  VoiceOnboardingClient get _client => ref.read(voiceOnboardingClientProvider);

  Future<void> load() async {
    final auth = ref.read(authControllerProvider);
    if (!auth.isAuthenticated || auth.session == null) return;
    state = state.copyWith(loading: true);
    final result = await _client.getState(
      authorization: 'Bearer ${auth.session!.accessToken}',
    );
    switch (result) {
      case OnboardingApiOk(:final data):
        state = OnboardingUiState(
          completed: data.completed,
          completedSteps: data.completedSteps,
        );
      case OnboardingApiFailure():
        state = state.copyWith(loading: false);
    }
  }

  Future<void> completeStep(String stepId) async {
    final auth = ref.read(authControllerProvider);
    if (auth.session == null) return;
    final result = await _client.completeStep(
      authorization: 'Bearer ${auth.session!.accessToken}',
      stepId: stepId,
    );
    switch (result) {
      case OnboardingApiOk(:final data):
        state = OnboardingUiState(
          completed: data.completed,
          completedSteps: data.completedSteps,
        );
      case OnboardingApiFailure():
        break;
    }
  }

  Future<void> completeCurrentStep() async {
    final step = state.currentStep;
    if (step == null) return;
    await completeStep(_stepIds[step]!);
  }

  Future<void> dismiss() async {
    await completeStep('dismiss');
  }
}

final onboardingControllerProvider =
    NotifierProvider<OnboardingController, OnboardingUiState>(
  OnboardingController.new,
);
