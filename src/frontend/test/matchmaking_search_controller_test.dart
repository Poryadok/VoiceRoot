import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/state/matchmaking_search_controller.dart';

void main() {
  test('onPushNotificationData search_nudge sets nudgeVisible', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final notifier = container.read(matchmakingSearchControllerProvider.notifier);
    notifier.onPushNotificationData(const {
      'type': 'search_nudge',
      'session_id': 'sess-1',
    });

    final state = container.read(matchmakingSearchControllerProvider);
    expect(state.nudgeVisible, isTrue);
    expect(state.timedOut, isFalse);
  });

  test('onPushNotificationData search_timeout clears session', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    container.read(activeSearchSessionProvider.notifier).state = SearchSessionData(
      id: 'sess-1',
      profileId: 'p1',
      gameId: 'g1',
      mode: 'Duo',
      criteriaJson: '{}',
      status: 'searching',
    );

    container.read(matchmakingSearchControllerProvider.notifier).onPushNotificationData(const {
      'type': 'search_timeout',
      'session_id': 'sess-1',
    });

    expect(container.read(activeSearchSessionProvider), isNull);
    final state = container.read(matchmakingSearchControllerProvider);
    expect(state.timedOut, isTrue);
    expect(state.recoveryReason, SearchRecoveryReason.timeout);
    expect(state.nudgeVisible, isFalse);
  });
}
