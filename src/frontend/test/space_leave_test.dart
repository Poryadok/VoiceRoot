import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/space_leave.dart';
import 'package:voice_frontend/state/space_providers.dart';

final _actor = StateProvider<int>((ref) => 0);

void main() {
  test(
    'leave uses the existing empty-body route and preserves owner denial',
    () async {
      final client = VoiceSpacesClient(
        gateway: GatewayHttpClient(
          config: const GatewayConfig(baseUrl: 'https://api.test'),
          httpClient: MockClient((request) async {
            expect(request.method, 'POST');
            expect(request.url.path, '/api/v1/spaces/space-1/leave');
            expect(request.body, isEmpty);
            expect(request.headers['Authorization'], 'Bearer a');
            return http.Response(
              jsonEncode({
                'error': 'failed_precondition',
                'message': 'owner must transfer',
              }),
              409,
            );
          }),
        ),
      );
      final result = await client.leaveSpace(
        authorization: 'Bearer a',
        spaceId: 'space-1',
      );
      expect(result, isA<SpacesApiFailure>());
      expect((result as SpacesApiFailure).statusCode, 409);
    },
  );

  for (final status in [204, 403, 503]) {
    test('leave status $status updates selection only on success', () async {
      final container = _container((_) async => http.Response('', status));
      addTearDown(container.dispose);
      container.read(selectedSpaceIdProvider.notifier).state = 'space-1';
      final result = await container
          .read(spaceLeaveActionsProvider)
          .leave('space-1');
      expect(
        result,
        status == 204
            ? SpaceLeaveResult.success
            : status >= 500
            ? SpaceLeaveResult.uncertain
            : SpaceLeaveResult.failed,
      );
      expect(
        container.read(selectedSpaceIdProvider),
        status == 204 ? null : 'space-1',
      );
    });
  }

  test('late leave cannot clear a newer selection', () async {
    final pending = Completer<http.Response>();
    final container = _container((_) => pending.future);
    addTearDown(container.dispose);
    container.read(selectedSpaceIdProvider.notifier).state = 'space-1';
    final leave = container.read(spaceLeaveActionsProvider).leave('space-1');
    container.read(selectedSpaceIdProvider.notifier).state = 'space-2';
    pending.complete(http.Response('', 204));
    expect(await leave, SpaceLeaveResult.success);
    expect(container.read(selectedSpaceIdProvider), 'space-2');
  });

  test(
    'profile switch away and back retires the response; duplicate submit is fenced',
    () async {
      final pending = Completer<http.Response>();
      var requests = 0;
      final container = _container((_) {
        requests++;
        return pending.future;
      });
      addTearDown(container.dispose);
      container.read(selectedSpaceIdProvider.notifier).state = 'space-1';
      final actions = container.read(spaceLeaveActionsProvider);
      final leave = actions.leave('space-1');
      expect(await actions.leave('space-1'), SpaceLeaveResult.busy);
      container.read(_actor.notifier).state = 1;
      container.read(_actor.notifier).state = 2;
      pending.complete(http.Response('', 204));
      expect(await leave, SpaceLeaveResult.stale);
      expect(requests, 1);
      expect(container.read(selectedSpaceIdProvider), 'space-1');
    },
  );
}

ProviderContainer _container(
  Future<http.Response> Function(http.Request) respond,
) => ProviderContainer(
  overrides: [
    authorizationHeaderProvider.overrideWith((ref) => 'Bearer a'),
    spaceLeaveMediaCleanupProvider.overrideWith((ref) => (_, _) async {}),
    spaceLeaveIdentityProvider.overrideWith(
      (ref) =>
          () => GatewayRequestIdentity(
            generation: ref.read(_actor),
            accountId: 'account-a',
            profileId: 'profile-a',
          ),
    ),
    voiceSpacesClientProvider.overrideWith(
      (ref) => VoiceSpacesClient(
        gateway: GatewayHttpClient(
          config: const GatewayConfig(baseUrl: 'https://api.test'),
          httpClient: MockClient(respond),
        ),
      ),
    ),
  ],
);
