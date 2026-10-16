import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';

import 'support/auth_test_overrides.dart';

final gameLink = StateProvider<RealtimeLinkStatus>(
  (ref) => RealtimeLinkStatus.connected,
);

ProviderContainer gameContainer(
  http.Client client,
  Stream<RealtimeFrame> events,
) => ProviderContainer(
  overrides: [
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    authControllerProvider.overrideWith(authenticatedAuthController),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://api.test'),
    ),
    httpClientProvider.overrideWithValue(client),
    realtimeLinkStatusProvider.overrideWith((ref) => ref.watch(gameLink)),
    realtimeEventProvider.overrideWith((ref) => events),
  ],
);

http.Response gameResponse(String title) => http.Response(
  jsonEncode({
    'presenceStatus': {
      'profileId': 'peer',
      'status': 'online',
      'gameTitle': title,
    },
  }),
  200,
);

void main() {
  for (final oldStatus in [200, 503]) {
    test('new WS observation wins over old bulk REST $oldStatus', () async {
      final delayed = Completer<http.Response>();
      final events = StreamController<RealtimeFrame>();
      final client = MockClient(
        (req) async => req.url.path.endsWith('/bulk')
            ? delayed.future
            : gameResponse('Initial game'),
      );
      final container = gameContainer(client, events.stream);
      addTearDown(() async {
        container.dispose();
        await events.close();
        client.close();
      });
      container.read(presenceProvider('peer'));
      await pumpEventQueue();
      final pending = container.read(presenceMapProvider.notifier).refreshBulk([
        'peer',
      ]);
      await pumpEventQueue();
      events.add(
        RealtimeFrame(
          op: 'presence_update',
          data: {
            'profile_id': 'peer',
            'status': 'online',
            if (oldStatus == 503) 'game_title': 'New approved game',
          },
        ),
      );
      await pumpEventQueue();
      delayed.complete(
        oldStatus == 200
            ? http.Response(
                '{"byProfileId":{"peer":{"profileId":"peer","status":"online","gameTitle":"Stale bulk game"}}}',
                200,
              )
            : http.Response('{}', 503),
      );
      await pending;
      expect(
        container.read(presenceProvider('peer'))?.gameTitle,
        oldStatus == 503 ? 'New approved game' : isNull,
      );
    });
  }

  test('reconnect replaces the game through current bulk REST', () async {
    var title = 'Before reconnect';
    final client = MockClient(
      (req) async => req.url.path.endsWith('/bulk')
          ? http.Response(
              jsonEncode({
                'byProfileId': {
                  'peer': {
                    'profileId': 'peer',
                    'status': 'online',
                    'gameTitle': title,
                  },
                },
              }),
              200,
            )
          : gameResponse(title),
    );
    final container = gameContainer(client, const Stream.empty());
    addTearDown(() {
      container.dispose();
      client.close();
    });
    container.read(presenceProvider('peer'));
    await pumpEventQueue();
    expect(container.read(presenceProvider('peer'))?.gameTitle, title);
    container.read(gameLink.notifier).state = RealtimeLinkStatus.reconnecting;
    await pumpEventQueue();
    title = 'After reconnect';
    container.read(gameLink.notifier).state = RealtimeLinkStatus.connected;
    await pumpEventQueue();
    expect(container.read(presenceProvider('peer'))?.gameTitle, title);
  });

  test(
    'failed profile switch clears cached title and permits a fresh lookup',
    () async {
      var reads = 0;
      final client = MockClient((req) async {
        if (req.url.path == '/api/v1/auth/switch-profile') {
          return http.Response('{}', 503);
        }
        reads++;
        return gameResponse(reads == 1 ? 'Old game' : 'Fresh game');
      });
      final container = gameContainer(client, const Stream.empty());
      addTearDown(() {
        container.dispose();
        client.close();
      });
      container.read(presenceProvider('peer'));
      await pumpEventQueue();
      expect(container.read(presenceProvider('peer'))?.gameTitle, 'Old game');
      container.read(profileSwitchInProgressProvider.notifier).state = true;
      expect(container.read(presenceProvider('peer'))?.gameTitle, isNull);
      await container
          .read(authControllerProvider.notifier)
          .switchActiveProfile('other-profile');
      container.read(profileSwitchInProgressProvider.notifier).state = false;
      container.read(presenceProvider('peer'));
      await pumpEventQueue();
      expect(reads, 2);
      expect(container.read(presenceProvider('peer'))?.gameTitle, 'Fresh game');
    },
  );

  test('disposed controller ignores pending REST completion', () async {
    final delayed = Completer<http.Response>();
    final client = MockClient((_) => delayed.future);
    final container = gameContainer(client, const Stream.empty());
    container.read(presenceProvider('peer'));
    await pumpEventQueue();
    container.dispose();
    delayed.complete(gameResponse('Disposed game'));
    await pumpEventQueue();
    client.close();
  });

  for (final oldStatus in [200, 503]) {
    test('new WS observation wins over old single REST $oldStatus', () async {
      final delayed = Completer<http.Response>();
      final events = StreamController<RealtimeFrame>();
      final client = MockClient((_) => delayed.future);
      final container = gameContainer(client, events.stream);
      addTearDown(() async {
        container.dispose();
        await events.close();
        client.close();
      });
      container.read(presenceProvider('peer'));
      await pumpEventQueue();
      events.add(
        RealtimeFrame(
          op: 'presence_update',
          data: {
            'profile_id': 'peer',
            'status': 'online',
            if (oldStatus == 503) 'game_title': 'New approved game',
          },
        ),
      );
      await pumpEventQueue();
      delayed.complete(
        oldStatus == 200
            ? gameResponse('Stale game')
            : http.Response('{}', 503),
      );
      await pumpEventQueue();
      expect(
        container.read(presenceProvider('peer'))?.gameTitle,
        oldStatus == 503 ? 'New approved game' : isNull,
      );
    });
  }

  test(
    'new bulk omission wins over old single result and rejects extra IDs',
    () async {
      final delayed = Completer<http.Response>();
      final client = MockClient((req) async {
        if (req.url.path.endsWith('/bulk')) {
          return http.Response(
            '{"byProfileId":{"unrequested":{"profileId":"unrequested","status":"online","gameTitle":"Unexpected"}}}',
            200,
          );
        }
        return delayed.future;
      });
      final container = gameContainer(client, const Stream.empty());
      addTearDown(() {
        container.dispose();
        client.close();
      });
      container.read(presenceProvider('peer'));
      await pumpEventQueue();
      await container.read(presenceMapProvider.notifier).refreshBulk(['peer']);
      delayed.complete(gameResponse('Old single game'));
      await pumpEventQueue();
      expect(
        container.read(presenceMapProvider).containsKey('unrequested'),
        isFalse,
      );
      expect(container.read(presenceProvider('peer'))?.gameTitle, isNull);
    },
  );

  test('empty bulk snapshot revokes a previously visible title', () async {
    final client = MockClient(
      (req) async => req.url.path.endsWith('/bulk')
          ? http.Response('{"byProfileId":{}}', 200)
          : gameResponse('Old game'),
    );
    final container = gameContainer(client, const Stream.empty());
    addTearDown(() {
      container.dispose();
      client.close();
    });
    container.read(presenceProvider('peer'));
    await pumpEventQueue();
    expect(container.read(presenceProvider('peer'))?.gameTitle, 'Old game');
    await container.read(presenceMapProvider.notifier).refreshBulk(['peer']);
    expect(container.read(presenceProvider('peer'))?.gameTitle, isNull);
  });

  for (final revoked in [
    <String, dynamic>{'profile_id': 'peer'},
    <String, dynamic>{'profile_id': 'peer', 'status': 'online'},
    <String, dynamic>{
      'profile_id': 'peer',
      'status': 'invisible',
      'game_title': 'Hidden game',
    },
  ]) {
    test(
      'filtered REST title is replaced by sparse/invisible WS $revoked',
      () async {
        final events = StreamController<RealtimeFrame>();
        final client = MockClient((_) async => gameResponse('Allowed game'));
        final container = gameContainer(client, events.stream);
        addTearDown(() async {
          container.dispose();
          await events.close();
          client.close();
        });
        container.read(presenceProvider('peer'));
        await pumpEventQueue();
        expect(
          container.read(presenceProvider('peer'))?.gameTitle,
          'Allowed game',
        );
        events.add(RealtimeFrame(op: 'presence_update', data: revoked));
        await pumpEventQueue();
        expect(container.read(presenceProvider('peer'))?.gameTitle, isNull);
      },
    );
  }

  test('current REST failure removes previously visible game activity', () async {
    var fail = false;
    final client = MockClient((req) async {
      if (fail) return http.Response('{}', 503);
      if (req.url.path.endsWith('/bulk')) {
        return http.Response(
          '{"byProfileId":{"peer":{"profileId":"peer","status":"online","gameTitle":"Allowed game"}}}',
          200,
        );
      }
      return gameResponse('Allowed game');
    });
    final container = gameContainer(client, const Stream.empty());
    addTearDown(() {
      container.dispose();
      client.close();
    });
    container.read(presenceProvider('peer'));
    await pumpEventQueue();
    expect(container.read(presenceProvider('peer'))?.gameTitle, 'Allowed game');
    fail = true;
    await container.read(presenceMapProvider.notifier).refreshBulk(['peer']);
    expect(container.read(presenceProvider('peer'))?.gameTitle, isNull);
  });

  for (final failedSwitch in [false, true]) {
    test(
      'late old-viewer REST cannot restore game failedSwitch=$failedSwitch',
      () async {
        final delayed = Completer<http.Response>();
        var requests = 0;
        final client = MockClient((req) async {
          if (req.url.path == '/api/v1/auth/switch-profile') {
            return http.Response('{}', 503);
          }
          requests++;
          if (requests == 1) return delayed.future;
          return http.Response('{}', 404);
        });
        final container = gameContainer(client, const Stream.empty());
        addTearDown(() {
          container.dispose();
          client.close();
        });
        container.read(presenceProvider('peer'));
        await pumpEventQueue();
        expect(requests, 1);
        final auth = container.read(authControllerProvider.notifier);
        if (failedSwitch) {
          final before = auth.state.session;
          final generation = auth.sessionInstallGeneration;
          container.read(profileSwitchInProgressProvider.notifier).state = true;
          await auth.switchActiveProfile('other-profile');
          expect(auth.state.session, same(before));
          expect(auth.sessionInstallGeneration, greaterThan(generation));
          container.read(profileSwitchInProgressProvider.notifier).state =
              false;
        } else {
          auth.state = const AuthState(
            session: AuthSession(
              accessToken: 'other-access',
              refreshToken: 'other-refresh',
              accountId: 'other-account',
              activeProfileId: 'other-profile',
              expiresInSeconds: 900,
            ),
          );
        }
        await pumpEventQueue();
        delayed.complete(gameResponse('Old private game'));
        await pumpEventQueue();
        expect(container.read(presenceProvider('peer'))?.gameTitle, isNull);
      },
    );
  }
}
