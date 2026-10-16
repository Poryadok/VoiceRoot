import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/contact_profile_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  test(
    'scans all caller spaces and opaque roster pages, stopping on match',
    () async {
      final requests = <String>[];
      final client = MockClient((request) async {
        expect(request.headers['authorization'], 'Bearer test-access');
        expect(request.url.queryParameters['page_size'], '100');
        final path = request.url.path;
        final cursor = request.url.queryParameters['cursor'];
        requests.add('$path:${cursor ?? ""}');
        if (path == '/api/v1/spaces') {
          return _spaces(
            cursor == null ? ['s-first'] : ['s-second'],
            cursor: cursor == null ? 'spaces opaque +/=' : null,
          );
        }
        if (path == '/api/v1/spaces/s-first/members') {
          return _members(['other']);
        }
        if (path == '/api/v1/spaces/s-second/members') {
          return cursor == null
              ? _members(['high-uuid'], cursor: 'tuple opaque +/=')
              : _members(['target'], cursor: 'unneeded-after-match');
        }
        throw StateError('unexpected $request');
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      addTearDown(client.close);
      final subscription = container.listen(
        contactCommonSpacesProvider('target'),
        (_, _) {},
      );
      addTearDown(subscription.close);
      expect(
        (await container.read(
          contactCommonSpacesProvider('target').future,
        )).map((s) => s.id),
        ['s-second'],
      );
      expect(requests, [
        '/api/v1/spaces:',
        '/api/v1/spaces/s-first/members:',
        '/api/v1/spaces:spaces opaque +/=',
        '/api/v1/spaces/s-second/members:',
        '/api/v1/spaces/s-second/members:tuple opaque +/=',
      ]);
    },
  );

  test('large authorized roster reaches a target beyond fifty pages', () async {
    var memberPages = 0;
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/spaces') return _spaces(['large']);
      memberPages++;
      return memberPages == 51
          ? _members(['target'])
          : _members(['member-$memberPages'], cursor: 'page-$memberPages');
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);
    addTearDown(client.close);
    final subscription = container.listen(
      contactCommonSpacesProvider('target'),
      (_, _) {},
    );
    addTearDown(subscription.close);
    expect(
      (await container.read(
        contactCommonSpacesProvider('target').future,
      )).single.id,
      'large',
    );
    expect(memberPages, 51);
  });

  for (final failingPath in ['/api/v1/spaces', '/api/v1/spaces/one/members']) {
    test(
      'failed page $failingPath is an error, never a partial empty result',
      () async {
        final client = MockClient(
          (request) async => request.url.path == failingPath
              ? http.Response('{"code":7,"message":"permission denied"}', 403)
              : _spaces(['one']),
        );
        final container = ProviderContainer(
          overrides: voiceAppTestOverrides(client: client),
        );
        addTearDown(container.dispose);
        addTearDown(client.close);
        final subscription = container.listen(
          contactCommonSpacesProvider('target'),
          (_, _) {},
        );
        addTearDown(subscription.close);
        await expectLater(
          container.read(contactCommonSpacesProvider('target').future),
          throwsStateError,
        );
      },
    );
  }

  for (final cycleInSpaces in [false, true]) {
    test(
      'repeated ${cycleInSpaces ? "space" : "member"} cursor fails explicitly',
      () async {
        var calls = 0;
        final client = MockClient((request) async {
          calls++;
          if (request.url.path == '/api/v1/spaces') {
            return _spaces(['one'], cursor: cycleInSpaces ? 'repeat' : null);
          }
          return _members([], cursor: cycleInSpaces ? null : 'repeat');
        });
        final container = ProviderContainer(
          overrides: voiceAppTestOverrides(client: client),
        );
        addTearDown(container.dispose);
        addTearDown(client.close);
        final subscription = container.listen(
          contactCommonSpacesProvider('target'),
          (_, _) {},
        );
        addTearDown(subscription.close);
        await expectLater(
          container.read(contactCommonSpacesProvider('target').future),
          throwsStateError,
        );
        expect(calls, lessThanOrEqualTo(4));
      },
    );
  }

  test(
    'later failed space page does not publish an earlier confirmed match',
    () async {
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/members')) return _members(['target']);
        if (request.url.queryParameters.containsKey('cursor')) {
          return http.Response('{"code":14,"message":"unavailable"}', 503);
        }
        return _spaces(['one'], cursor: 'next-space-page');
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      addTearDown(client.close);
      final subscription = container.listen(
        contactCommonSpacesProvider('target'),
        (_, _) {},
      );
      addTearDown(subscription.close);
      await expectLater(
        container.read(contactCommonSpacesProvider('target').future),
        throwsStateError,
      );
      expect(
        container.read(contactCommonSpacesProvider('target')).hasError,
        isTrue,
      );
      expect(
        container.read(contactCommonSpacesProvider('target')).valueOrNull,
        isNull,
      );
    },
  );

  testWidgets(
    'a hung member page times out explicitly and makes no further requests',
    (tester) async {
      final pending = Completer<http.Response>();
      var calls = 0;
      final client = MockClient((request) async {
        calls++;
        return request.url.path == '/api/v1/spaces'
            ? _spaces(['one'])
            : pending.future;
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      addTearDown(client.close);
      final subscription = container.listen(
        contactCommonSpacesProvider('target'),
        (_, _) {},
      );
      addTearDown(subscription.close);
      final expectation = expectLater(
        container.read(contactCommonSpacesProvider('target').future),
        throwsA(isA<TimeoutException>()),
      );
      await tester.pump();
      await tester.pump(const Duration(seconds: 16));
      await expectation;
      expect(calls, 2);
      pending.complete(_members(['target']));
      await tester.pump();
      expect(
        container.read(contactCommonSpacesProvider('target')).hasError,
        isTrue,
      );
    },
  );

  test('disposal during page fetch prevents further roster requests', () async {
    final pending = Completer<http.Response>();
    var calls = 0;
    final client = MockClient((request) {
      calls++;
      return pending.future;
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    final subscription = container.listen(
      contactCommonSpacesProvider('target'),
      (_, _) {},
    );
    await Future<void>.delayed(Duration.zero);
    subscription.close();
    container.dispose();
    pending.complete(_spaces(['one']));
    await Future<void>.delayed(Duration.zero);
    expect(calls, 1);
    client.close();
  });

  test(
    'profile switch cancels old traversal before fetching any members',
    () async {
      final pending = Completer<http.Response>();
      var calls = 0;
      final client = MockClient((request) {
        calls++;
        return pending.future;
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      addTearDown(client.close);
      final subscription = container.listen(
        contactCommonSpacesProvider('target'),
        (_, _) {},
      );
      addTearDown(subscription.close);
      await Future<void>.delayed(Duration.zero);
      container.read(profileSwitchInProgressProvider.notifier).state = true;
      expect(
        await container.read(contactCommonSpacesProvider('target').future),
        isEmpty,
      );
      pending.complete(_spaces(['one']));
      await Future<void>.delayed(Duration.zero);
      expect(calls, 1);
      expect(
        container.read(contactCommonSpacesProvider('target')).requireValue,
        isEmpty,
      );
    },
  );
}

http.Response _spaces(List<String> ids, {String? cursor}) => http.Response(
  jsonEncode({
    'space_list': {
      'spaces': [
        for (final id in ids)
          {
            'id': id,
            'name': id,
            'visibility': 'private',
            'owner_profile_id': 'prof-test',
          },
      ],
      'next_cursor': cursor ?? '',
    },
  }),
  200,
  headers: {'content-type': 'application/json'},
);

http.Response _members(List<String> ids, {String? cursor}) => http.Response(
  jsonEncode({
    'space_member_list': {
      'members': [
        for (final id in ids)
          {
            'profile_id': id,
            'role_names': ['Member'],
            'joined_at': '2026-10-14T00:00:00Z',
          },
      ],
      'next_cursor': cursor ?? '',
    },
  }),
  200,
  headers: {'content-type': 'application/json'},
);
