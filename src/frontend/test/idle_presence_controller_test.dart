import 'package:fake_async/fake_async.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/idle_presence_controller.dart';
import 'package:voice_frontend/state/social_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';

class _RecordingUsersClient extends VoiceUsersClient {
  _RecordingUsersClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 404)),
        ),
      );

  final List<String> statuses = [];

  @override
  Future<UsersApiResult<void>> updatePresence({
    required String authorization,
    required String status,
    String? customStatus,
  }) async {
    statuses.add(status);
    return const UsersApiOk(null);
  }
}

void main() {
  group('IdlePresenceController', () {
    test('sends UpdatePresence idle after 5 minutes without activity', () {
      fakeAsync((async) {
        final users = _RecordingUsersClient();
        final container = ProviderContainer(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            voiceUsersClientProvider.overrideWithValue(users),
          ],
        );
        addTearDown(container.dispose);

        container.read(idlePresenceLifecycleProvider);
        expect(users.statuses, isEmpty);

        async.elapse(const Duration(minutes: 4, seconds: 59));
        expect(users.statuses, isEmpty);

        async.elapse(const Duration(seconds: 1));
        async.flushMicrotasks();
        expect(users.statuses, ['idle']);
        expect(container.read(idlePresenceControllerProvider).isIdle, isTrue);
      });
    });

    test('activity before timeout resets the idle timer', () {
      fakeAsync((async) {
        final users = _RecordingUsersClient();
        final container = ProviderContainer(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            voiceUsersClientProvider.overrideWithValue(users),
          ],
        );
        addTearDown(container.dispose);

        final tracker = container.read(idlePresenceControllerProvider);
        container.read(idlePresenceLifecycleProvider);

        async.elapse(const Duration(minutes: 4));
        tracker.onUserActivity();
        async.elapse(const Duration(minutes: 4));
        expect(users.statuses, isEmpty);

        async.elapse(const Duration(minutes: 1));
        async.flushMicrotasks();
        expect(users.statuses, ['idle']);
      });
    });

    test('activity after auto-idle restores online', () {
      fakeAsync((async) {
        final users = _RecordingUsersClient();
        final container = ProviderContainer(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            voiceUsersClientProvider.overrideWithValue(users),
          ],
        );
        addTearDown(container.dispose);

        final tracker = container.read(idlePresenceControllerProvider);
        container.read(idlePresenceLifecycleProvider);

        async.elapse(kIdlePresenceTimeout);
        async.flushMicrotasks();
        expect(users.statuses, ['idle']);

        tracker.onUserActivity();
        async.flushMicrotasks();
        expect(users.statuses, ['idle', 'online']);
        expect(tracker.isIdle, isFalse);
      });
    });

    test('manual DND is not overridden by idle timeout', () {
      fakeAsync((async) {
        final users = _RecordingUsersClient();
        final container = ProviderContainer(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            voiceUsersClientProvider.overrideWithValue(users),
          ],
        );
        addTearDown(container.dispose);

        final tracker = container.read(idlePresenceControllerProvider);
        container.read(idlePresenceLifecycleProvider);
        tracker.onManualStatus('dnd');

        async.elapse(kIdlePresenceTimeout);
        async.flushMicrotasks();
        expect(users.statuses, isEmpty);
        expect(tracker.isManualLocked, isTrue);
      });
    });

    test('stop cancels pending idle timer', () {
      fakeAsync((async) {
        final users = _RecordingUsersClient();
        final container = ProviderContainer(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            voiceUsersClientProvider.overrideWithValue(users),
          ],
        );
        addTearDown(container.dispose);

        final tracker = container.read(idlePresenceControllerProvider);
        container.read(idlePresenceLifecycleProvider);
        expect(async.pendingTimers.length, 1);

        tracker.stop();
        expect(async.pendingTimers, isEmpty);

        async.elapse(kIdlePresenceTimeout);
        async.flushMicrotasks();
        expect(users.statuses, isEmpty);
      });
    });

    test('container dispose cancels pending idle timer', () {
      fakeAsync((async) {
        final users = _RecordingUsersClient();
        final container = ProviderContainer(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            voiceUsersClientProvider.overrideWithValue(users),
          ],
        );

        container.read(idlePresenceLifecycleProvider);
        expect(async.pendingTimers.length, 1);

        container.dispose();
        expect(async.pendingTimers, isEmpty);
      });
    });
  });
}
