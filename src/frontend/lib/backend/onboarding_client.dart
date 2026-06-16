import 'api_result.dart';
import 'gateway_http.dart';

sealed class OnboardingApiResult<T> {
  const OnboardingApiResult();
}

final class OnboardingApiOk<T> extends OnboardingApiResult<T> {
  const OnboardingApiOk(this.data);
  final T data;
}

final class OnboardingApiFailure extends OnboardingApiResult<Never> {
  const OnboardingApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class OnboardingState {
  const OnboardingState({
    required this.profileId,
    required this.completedSteps,
    required this.completed,
  });

  final String profileId;
  final List<String> completedSteps;
  final bool completed;

  factory OnboardingState.fromJson(Map<String, dynamic> json) {
    final state = json['onboarding_state'] as Map<String, dynamic>? ?? json;
    return OnboardingState(
      profileId: state['profile_id'] as String? ?? '',
      completedSteps: (state['completed_steps'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const [],
      completed: state['completed'] as bool? ?? false,
    );
  }
}

class VoiceOnboardingClient {
  VoiceOnboardingClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<OnboardingApiResult<OnboardingState>> getState({
    required String authorization,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/users/me/onboarding'),
      authorization: authorization,
    );
    return _map(result, OnboardingState.fromJson);
  }

  Future<OnboardingApiResult<OnboardingState>> completeStep({
    required String authorization,
    required String stepId,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/users/me/onboarding/steps'),
      authorization: authorization,
      body: {'step_id': stepId},
    );
    return _map(result, OnboardingState.fromJson);
  }

  OnboardingApiResult<T> _map<T>(
    GatewayHttpResult<Map<String, dynamic>> result,
    T Function(Map<String, dynamic>) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => OnboardingApiOk(parse(data)),
      GatewayHttpFailure(:final error) => OnboardingApiFailure(
          message: GatewayApiResultMapper.failureMessage(error),
          errorCode: GatewayApiResultMapper.failureCode(error),
          statusCode: GatewayApiResultMapper.failureStatus(error),
        ),
    };
  }
}
