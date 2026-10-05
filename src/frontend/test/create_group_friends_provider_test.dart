import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/create_group_friends_provider.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

void main() {
  ProviderContainer containerFor(http.Client client) => ProviderContainer(
    overrides: [
      authorizationHeaderProvider.overrideWithValue('Bearer test'),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
    ],
  );

  test('loads all cursor pages and keeps first-seen profile order', () async {
    final requests = <http.Request>[];
    final container = containerFor(
      MockClient((request) async {
        requests.add(request);
        expect(request.headers['authorization'], 'Bearer test');
        if (request.url.queryParameters['cursor'] == null) {
          return http.Response(
            jsonEncode({
              'friends': [
                {'profile_id': 'friend-a'},
                {'profile_id': 'friend-b'},
              ],
              'next_cursor': 'page-2',
            }),
            200,
          );
        }
        expect(request.url.queryParameters['cursor'], 'page-2');
        return http.Response(
          jsonEncode({
            'friends': [
              {'profile_id': 'friend-b'},
              {'profile_id': 'friend-c'},
            ],
          }),
          200,
        );
      }),
    );
    addTearDown(container.dispose);

    expect(await container.read(createGroupFriendsProvider.future), [
      'friend-a',
      'friend-b',
      'friend-c',
    ]);
    expect(requests, hasLength(2));
  });

  test(
    'a changed authorization session does not continue the stale cursor',
    () async {
      final authorization = StateProvider<String?>((ref) => 'Bearer first');
      final firstPage = Completer<http.Response>();
      final requests = <http.Request>[];
      final container = ProviderContainer(
        overrides: [
          authorizationHeaderProvider.overrideWith(
            (ref) => ref.watch(authorization),
          ),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              requests.add(request);
              if (request.headers['authorization'] == 'Bearer first') {
                return firstPage.future;
              }
              return http.Response(
                jsonEncode({
                  'friends': [
                    {'profile_id': 'new-session-friend'},
                  ],
                }),
                200,
              );
            }),
          ),
        ],
      );
      final subscription = container.listen(
        createGroupFriendsProvider,
        (_, _) {},
      );
      addTearDown(() {
        subscription.close();
        container.dispose();
      });

      final staleLoad = container.read(createGroupFriendsProvider.future);
      await Future<void>.delayed(Duration.zero);
      expect(requests, hasLength(1));
      expect(requests.single.headers['authorization'], 'Bearer first');

      container.read(authorization.notifier).state = 'Bearer second';
      final currentLoad = container.read(createGroupFriendsProvider.future);
      expect(await currentLoad, ['new-session-friend']);

      firstPage.complete(
        http.Response(
          jsonEncode({
            'friends': [
              {'profile_id': 'stale-friend'},
            ],
            'next_cursor': 'stale-cursor',
          }),
          200,
        ),
      );
      expect(await staleLoad, ['new-session-friend']);
      expect(requests, hasLength(2));
      expect(requests.last.headers['authorization'], 'Bearer second');
      expect(requests.last.url.queryParameters['cursor'], isNull);
    },
  );

  test(
    'fails instead of returning a partial list for a repeated cursor',
    () async {
      var calls = 0;
      final container = containerFor(
        MockClient((request) async {
          calls++;
          return http.Response(
            jsonEncode({
              'friends': [
                {'profile_id': calls == 1 ? 'friend-a' : 'friend-b'},
              ],
              'next_cursor': 'same-cursor',
            }),
            200,
          );
        }),
      );
      addTearDown(container.dispose);

      await expectLater(
        container.read(createGroupFriendsProvider.future),
        throwsA(isA<StateError>()),
      );
      expect(calls, 2);
    },
  );

  test(
    'does not schedule another page after the provider is disposed',
    () async {
      final firstPage = Completer<http.Response>();
      var calls = 0;
      final container = containerFor(
        MockClient((request) {
          calls++;
          return firstPage.future;
        }),
      );
      final subscription = container.listen(
        createGroupFriendsProvider,
        (_, _) {},
      );
      final result = container.read(createGroupFriendsProvider.future);
      await Future<void>.delayed(Duration.zero);
      expect(calls, 1);

      subscription.close();
      await Future<void>.delayed(Duration.zero);
      firstPage.complete(
        http.Response(
          jsonEncode({
            'friends': [
              {'profile_id': 'friend-a'},
            ],
            'next_cursor': 'page-2',
          }),
          200,
        ),
      );

      expect(await result, isEmpty);
      expect(calls, 1);
      container.dispose();
    },
  );
}
