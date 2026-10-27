import 'dart:async';
import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/linked_oauth_client.dart';
import 'package:voice_frontend/state/linked_oauth_return.dart';

const attemptId = '11111111-1111-4111-8111-111111111111';
final handle = 'h' * 43;
final now = DateTime.utc(2026, 10, 26, 1);
const session = AuthSession(
  accessToken: 'original',
  refreshToken: 'refresh',
  accountId: 'account',
  activeProfileId: 'profile',
  expiresInSeconds: 300,
  accountType: 'regular',
);

void main() {
  test(
    'session change during completion notification cannot reread another actor',
    () async {
      final h = Harness();
      await h.controller.start('twitch');
      h.controller.addListener(() {
        if (h.controller.state.outcome == LinkedOAuthOutcome.success) {
          h.changeSession('logout');
        }
      });
      await h.controller.handleReturn(h.returnUrl);
      expect(h.events, isNot(contains('reread')));
    },
  );
  test(
    'duplicate OS return then cancel cannot discard in-flight consume authority',
    () async {
      final reply = Completer<http.Response>();
      final h = Harness(consumeReply: reply.future);
      await h.controller.start('twitch');
      final consume = h.controller.handleReturn(h.returnUrl);
      await Future<void>.delayed(Duration.zero);
      await h.controller.handleReturn(h.returnUrl);
      await h.controller.cancel();
      reply.complete(http.Response('{"outcome":"success"}', 200));
      await consume;
      expect(h.events.where((e) => e == 'consume'), hasLength(1));
      expect(h.controller.state.outcome, LinkedOAuthOutcome.success);
      expect(h.events.last, 'reread');
    },
  );
  test(
    'start while consuming cannot discard reconciliation of a committed source',
    () async {
      final reply = Completer<http.Response>();
      final h = Harness(consumeReply: reply.future);
      await h.controller.start('twitch');
      final consume = h.controller.handleReturn(h.returnUrl);
      await Future<void>.delayed(Duration.zero);
      await h.controller.start('twitch');
      expect(h.events.where((e) => e == 'start'), hasLength(1));
      reply.complete(http.Response('{"outcome":"success"}', 200));
      await consume;
      expect(h.events.last, 'reread');
    },
  );
  test(
    'commit then dropped consume response rereads without claiming success',
    () async {
      final h = Harness(dropConsumeResponse: true);
      await h.controller.start('twitch');
      await h.controller.handleReturn(h.returnUrl);
      expect(h.committed, isTrue);
      expect(h.events.last, 'reread');
      expect(h.controller.state.outcome, isNull);
      expect(h.controller.state.errorCode, 'network_error');
      expect(h.store.value, isNull);
    },
  );

  for (final response in [
    http.Response('broken', 200),
    http.Response('{"error":"internal_error"}', 500),
  ]) {
    test(
      'ambiguous consume ${response.statusCode} rereads without replay',
      () async {
        final h = Harness(consumeReply: Future.value(response));
        await h.controller.start('twitch');
        await h.controller.handleReturn(h.returnUrl);
        expect(h.events.last, 'reread');
        expect(h.events.where((e) => e == 'consume'), hasLength(1));
        expect(h.controller.state.outcome, isNull);
      },
    );
  }

  test(
    'cancel while consuming preserves eventual authoritative reconciliation',
    () async {
      final reply = Completer<http.Response>();
      final h = Harness(consumeReply: reply.future);
      await h.controller.start('twitch');
      final consume = h.controller.handleReturn(h.returnUrl);
      await Future<void>.delayed(Duration.zero);
      await h.controller.cancel();
      expect(h.controller.state.phase, LinkedOAuthPhase.consuming);
      reply.complete(http.Response('{"outcome":"success"}', 200));
      await consume;
      expect(h.controller.state.outcome, LinkedOAuthOutcome.success);
      expect(h.events.last, 'reread');
    },
  );
  test(
    'Windows direct URI strips secrets and retains explicit empty fragment',
    () {
      expect(
        stripLinkedOAuthReturnSecrets(
          Uri.parse(
            'voice://settings/oauth?attempt_id=private&result_handle=private#token=secret',
          ),
        ).toString(),
        'voice://settings/oauth#',
      );
    },
  );

  test('malformed percent encoding is scrubbed and fails neutrally', () async {
    final h = Harness();
    await h.controller.start('twitch');
    await h.controller.handleReturn(
      Uri.parse(
        'https://voice.test/settings/oauth?attempt_id=%FF&result_handle=$handle#',
      ),
    );
    expect(h.events, isNot(contains('consume')));
    expect(h.controller.state.errorCode, 'invalid_link_attempt');
  });
  test(
    'same-tab proof survives controller recreation; consume then reread',
    () async {
      final h = Harness();
      await h.controller.start('twitch');
      expect(h.events, ['start', 'navigate']);
      expect(h.store.value, isNot(contains('original')));
      expect(h.store.value, isNot(contains('refresh')));
      h.recreate();
      await h.controller.handleReturn(h.returnUrl);
      expect(h.events, ['start', 'navigate', 'scrub', 'consume', 'reread']);
      expect(h.controller.state.outcome, LinkedOAuthOutcome.success);
      expect(h.store.value, isNull);
      await h.controller.handleReturn(h.returnUrl);
      expect(h.events.where((v) => v == 'consume'), hasLength(1));
    },
  );

  test('nonempty fragment is scrubbed before any consume or reread', () async {
    final h = Harness();
    await h.controller.start('twitch');
    await h.controller.handleReturn(
      h.returnUrl.replace(fragment: 'access_token=secret'),
    );
    expect(h.events, ['start', 'navigate', 'scrub']);
    expect(h.scrubbed.toString(), 'https://voice.test/settings/oauth#');
    expect(h.controller.state.errorCode, 'invalid_link_attempt');
    expect(h.store.value, isNull);
  });

  test(
    'explicit empty fragment is accepted and URL handle is removed first',
    () async {
      final h = Harness();
      await h.controller.start('twitch');
      expect(h.returnUrl.toString(), endsWith('#'));
      await h.controller.handleReturn(h.returnUrl);
      expect(h.events.indexOf('scrub'), lessThan(h.events.indexOf('consume')));
      expect(h.scrubbed?.query, isEmpty);
    },
  );

  test('attempt expiry equality rejects without consuming', () async {
    final h = Harness();
    await h.controller.start('twitch');
    h.time = now.add(const Duration(minutes: 5));
    await h.controller.handleReturn(h.returnUrl);
    expect(h.controller.state.errorCode, 'result_expired');
    expect(h.events, isNot(contains('consume')));
  });

  for (final mutation in ['refresh', 'profile', 'logout', 'replacement']) {
    test('$mutation destroys proof and delayed start cannot launch', () async {
      final reply = Completer<http.Response>();
      final h = Harness(startReply: reply.future);
      final start = h.controller.start('twitch');
      await Future<void>.delayed(Duration.zero);
      h.changeSession(mutation);
      reply.complete(startResponse());
      await start;
      expect(h.store.value, isNull);
      expect(h.events, ['start']);
      await h.controller.handleReturn(h.returnUrl);
      expect(h.events, isNot(contains('consume')));
    });
    test('$mutation ignores delayed consume result', () async {
      final reply = Completer<http.Response>();
      final h = Harness(consumeReply: reply.future);
      await h.controller.start('twitch');
      final consume = h.controller.handleReturn(h.returnUrl);
      await Future<void>.delayed(Duration.zero);
      h.changeSession(mutation);
      reply.complete(http.Response('{"outcome":"success"}', 200));
      await consume;
      expect(h.events, isNot(contains('reread')));
      expect(h.controller.state.outcome, isNull);
    });
  }

  test(
    'other tab, wrong attempt, duplicate and unknown parameters reject',
    () async {
      for (final query in [
        'attempt_id=$attemptId&result_handle=$handle&code=secret',
        'attempt_id=$attemptId&attempt_id=$attemptId&result_handle=$handle',
        'attempt_id=22222222-2222-4222-8222-222222222222&result_handle=$handle',
        'attempt_id=$attemptId&result_handle=bad',
      ]) {
        final h = Harness();
        await h.controller.start('twitch');
        await h.controller.handleReturn(
          Uri.parse('https://voice.test/settings/oauth?$query#'),
        );
        expect(h.events, isNot(contains('consume')));
        expect(h.controller.state.errorCode, 'invalid_link_attempt');
      }
      final otherTab = Harness();
      await otherTab.controller.handleReturn(otherTab.returnUrl);
      expect(otherTab.events, ['scrub']);
    },
  );

  test(
    'a copied return cannot migrate proof to another credential after reload',
    () async {
      final h = Harness();
      await h.controller.start('twitch');
      h.recreate(
        current: const AuthSession(
          accessToken: 'new',
          refreshToken: 'refresh',
          accountId: 'account',
          activeProfileId: 'profile',
          expiresInSeconds: 300,
          accountType: 'regular',
        ),
      );
      await h.controller.handleReturn(h.returnUrl);
      expect(h.events, isNot(contains('consume')));
      expect(h.store.value, isNull);
    },
  );

  for (final outcome in [
    'cancelled',
    'denied',
    'provider_error',
    'unavailable',
  ]) {
    test('$outcome does not claim success or refresh verification', () async {
      final h = Harness(
        consumeReply: Future.value(
          http.Response(jsonEncode({'outcome': outcome}), 200),
        ),
      );
      await h.controller.start('twitch');
      await h.controller.handleReturn(h.returnUrl);
      expect(
        h.controller.state.outcome?.name,
        outcome == 'provider_error' ? 'providerError' : outcome,
      );
      expect(h.events, isNot(contains('reread')));
    });
  }

  test(
    'result_consumed rereads authority, expiry restarts without retry',
    () async {
      for (final code in ['result_consumed', 'result_expired']) {
        final h = Harness(
          consumeReply: Future.value(
            http.Response(
              jsonEncode({'error': code}),
              code == 'result_consumed' ? 409 : 410,
            ),
          ),
        );
        await h.controller.start('twitch');
        await h.controller.handleReturn(h.returnUrl);
        expect(h.controller.state.errorCode, code);
        expect(h.events.contains('reread'), code == 'result_consumed');
        expect(h.store.value, isNull);
      }
    },
  );

  test(
    'cancel clears proof before response and never undoes a source',
    () async {
      final h = Harness();
      await h.controller.start('twitch');
      await h.controller.cancel();
      expect(h.events, ['start', 'navigate', 'cancel']);
      expect(h.store.value, isNull);
      expect(h.controller.state.outcome, LinkedOAuthOutcome.cancelled);
    },
  );
}

http.Response startResponse() => http.Response(
  jsonEncode({
    'attempt_id': attemptId,
    'authorization_url': 'https://id.twitch.tv/oauth2/authorize?state=opaque',
    'expires_at': now.add(const Duration(minutes: 5)).toIso8601String(),
  }),
  200,
);

class MemoryStore implements LinkedOAuthTabStore {
  String? value;
  @override
  String? read() => value;
  @override
  void write(String value) => this.value = value;
  @override
  void clear() => value = null;
}

class Harness {
  Harness({
    this.startReply,
    this.consumeReply,
    this.dropConsumeResponse = false,
  }) {
    recreate();
  }
  final Future<http.Response>? startReply;
  final Future<http.Response>? consumeReply;
  final bool dropConsumeResponse;
  bool committed = false;
  final store = MemoryStore();
  final events = <String>[];
  DateTime time = now;
  Uri? scrubbed;
  String? challenge;
  late LinkedOAuthReturnController controller;
  Uri get returnUrl => Uri.parse(
    'https://voice.test/settings/oauth?attempt_id=$attemptId&result_handle=$handle#',
  );
  void recreate({AuthSession current = session}) {
    controller = LinkedOAuthReturnController(
      client: LinkedOAuthClient(
        httpClient: MockClient((request) async {
          expect(request.headers['authorization'], 'Bearer original');
          if (request.url.path.endsWith('/link')) {
            events.add('start');
            final body = jsonDecode(request.body) as Map<String, dynamic>;
            expect(
              body.keys,
              unorderedEquals([
                'client_surface',
                'return_challenge',
                'return_challenge_method',
              ]),
            );
            expect(
              body['return_challenge'],
              matches(RegExp(r'^[A-Za-z0-9_-]{43}$')),
            );
            challenge = body['return_challenge'] as String;
            return startReply ?? startResponse();
          }
          if (request.url.path.endsWith('/cancel')) {
            events.add('cancel');
            expect(request.body, isEmpty);
            return http.Response('', 204);
          }
          events.add('consume');
          if (dropConsumeResponse) {
            committed = true;
            throw http.ClientException('response lost');
          }
          expect(store.value, isNull);
          expect(scrubbed?.query, isEmpty);
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(
            body.keys,
            unorderedEquals(['attempt_id', 'result_handle', 'return_verifier']),
          );
          expect(
            body['return_verifier'],
            matches(RegExp(r'^[A-Za-z0-9_-]{43}$')),
          );
          expect(
            base64UrlEncode(
              sha256
                  .convert(ascii.encode(body['return_verifier'] as String))
                  .bytes,
            ).replaceAll('=', ''),
            challenge,
          );
          return consumeReply ?? http.Response('{"outcome":"success"}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'https://gateway.test'),
        authorizationEndpoints: {
          'twitch': Uri.parse('https://id.twitch.tv/oauth2/authorize'),
        },
      ),
      store: store,
      returnUri: Uri.parse('https://voice.test/settings/oauth'),
      surface: 'web',
      clock: () => time,
      navigateSameTab: (uri) async {
        events.add('navigate');
        return true;
      },
      scrubLocation: (uri) {
        events.add('scrub');
        scrubbed = uri;
      },
      rereadAuthoritativeState: () async {
        events.add('reread');
      },
    )..bindSession(current, sessionIdentity: 'install1');
  }

  void changeSession(String mutation) {
    controller.bindSession(
      mutation == 'logout'
          ? null
          : AuthSession(
              accessToken: mutation == 'refresh' ? 'new' : session.accessToken,
              refreshToken: session.refreshToken,
              accountId: session.accountId,
              activeProfileId: mutation == 'profile'
                  ? 'other'
                  : session.activeProfileId,
              expiresInSeconds: 300,
              accountType: 'regular',
            ),
      sessionIdentity: mutation == 'replacement' ? 'install2' : 'install1',
    );
  }
}
