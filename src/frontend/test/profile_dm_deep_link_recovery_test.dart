import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/deep_link_navigation.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/profile_deep_link_resolver.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';

import 'support/auth_test_overrides.dart';

final _linkCallerVisibleProvider = StateProvider<bool>((ref) => true);

void main() {
  for (final path in ['/u/alice', '/dm/target-profile']) {
    for (final status in [403, 404]) {
      testWidgets(
        '$path stale $status cannot overlay a newer pending resolve',
        (tester) async {
          final olderDomain = Completer<http.Response>();
          final newerResolve = Completer<http.Response>();
          var resolveCalls = 0;
          var domainCalls = 0;
          final container = await _pumpLink(
            tester,
            path,
            (_) {
              domainCalls++;
              return olderDomain.future;
            },
            nextPath: '/ch/newer-chat',
            resolveResponse: (_) async {
              resolveCalls++;
              if (resolveCalls == 1) {
                return _json({
                  'kind': path.startsWith('/u/') ? 'profile' : 'dm',
                });
              }
              return newerResolve.future;
            },
          );
          await tester.tap(find.text('open link'));
          await tester.pump();
          expect(domainCalls, 1);
          await tester.tap(find.text('open newer'));
          await tester.pump();
          expect(resolveCalls, 2);
          olderDomain.complete(
            _json({'message': 'private domain detail'}, status: status),
          );
          await tester.pumpAndSettle();

          expect(find.text('$status'), findsNothing);
          expect(find.text('private domain detail'), findsNothing);
          newerResolve.complete(_json({'kind': 'chat'}));
          await tester.pumpAndSettle();
          expect(container.read(selectedChatIdProvider), 'newer-chat');
          expect(find.text('$status'), findsNothing);
        },
      );
    }

    testWidgets('$path domain error is ignored after caller unmounts', (
      tester,
    ) async {
      final domain = Completer<http.Response>();
      final container = await _pumpLink(tester, path, (_) => domain.future);
      await tester.tap(find.text('open link'));
      await tester.pump();
      container.read(_linkCallerVisibleProvider.notifier).state = false;
      await tester.pump();
      expect(find.text('open link'), findsNothing);
      domain.complete(_json({'message': 'private domain detail'}, status: 403));
      await tester.pumpAndSettle();
      expect(find.text('home shell'), findsOneWidget);
      expect(find.text('403'), findsNothing);
      expect(tester.takeException(), isNull);
    });
  }

  for (final exact in [false, true]) {
    testWidgets('scanned profile URL requires exact handle (exact: $exact)', (
      tester,
    ) async {
      String? resolved;
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: MockClient((request) async {
            expect(request.url.path, '/api/v1/users/search');
            return _json({
              'profile_list': {
                'profiles': [
                  _profile('unrelated', 'alice-neighbor'),
                  if (exact) _profile('exact', 'ALIce'),
                ],
              },
            });
          }),
        ),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            home: Consumer(
              builder: (context, ref, _) => TextButton(
                onPressed: () async {
                  resolved = await resolveProfileIdFromDeepLinkText(
                    ref,
                    'https://voice.gg/u/alice',
                  );
                },
                child: const Text('scan profile'),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('scan profile'));
      await tester.pumpAndSettle();
      expect(resolved, exact ? 'exact' : isNull);
    });
  }

  for (final profiles in [
    <Map<String, String>>[],
    [_profile('unrelated', 'alice-neighbor')],
  ]) {
    testWidgets('missing exact username shows 404 for $profiles', (
      tester,
    ) async {
      final container = await _pumpLink(tester, '/u/alice', (request) async {
        if (request.url.path == '/api/v1/users/search') {
          expect(request.url.queryParameters['q'], 'alice');
          return _json({
            'profile_list': {'profiles': profiles},
          });
        }
        return _json({}, status: 404);
      });

      await tester.tap(find.text('open link'));
      await tester.pumpAndSettle();

      expect(find.byType(ProfileDetailSheet), findsNothing);
      expect(
        container.read(navigationSectionProvider),
        NavigationSection.chats,
      );
      await _expectRecoveryAndHome(tester, 404);
    });
  }

  testWidgets('case-insensitive exact handle wins over first fuzzy result', (
    tester,
  ) async {
    await _pumpLink(tester, '/u/ALIce', (request) async {
      if (request.url.path == '/api/v1/users/search') {
        return _json({
          'profile_list': {
            'profiles': [
              _profile('unrelated', 'alice-neighbor'),
              _profile('exact', 'alice'),
            ],
          },
        });
      }
      return _json({}, status: 404);
    });
    await tester.tap(find.text('open link'));
    await tester.pumpAndSettle();

    expect(find.byType(ProfileDetailSheet), findsOneWidget);
    expect(
      tester
          .widget<ProfileDetailSheet>(find.byType(ProfileDetailSheet))
          .profileId,
      'exact',
    );
    expect(find.text('404'), findsNothing);
  });

  for (final status in [403, 404]) {
    for (final path in ['/u/alice', '/dm/target-profile']) {
      testWidgets('$path domain $status after resolve uses safe recovery', (
        tester,
      ) async {
        var domainCalls = 0;
        final container = await _pumpLink(tester, path, (request) async {
          expect(
            request.url.path,
            path.startsWith('/u/')
                ? '/api/v1/users/search'
                : '/api/v1/chats/dm',
          );
          domainCalls++;
          if (path.startsWith('/dm/')) {
            expect(request.method, 'POST');
            expect(
              jsonDecode(request.body)['other_profile_id'],
              'target-profile',
            );
          }
          return _json({
            'code': 'PRIVATE_DETAIL',
            'message': 'private domain detail',
          }, status: status);
        });

        await tester.tap(find.text('open link'));
        await tester.pumpAndSettle();

        expect(domainCalls, 1);
        expect(container.read(selectedChatIdProvider), isNull);
        expect(find.byType(ProfileDetailSheet), findsNothing);
        expect(find.text('private domain detail'), findsNothing);
        await _expectRecoveryAndHome(tester, status);
      });
    }
  }

  for (final path in ['/u/alice', '/dm/target-profile']) {
    testWidgets('$path ignores domain denial after logout', (tester) async {
      final pending = Completer<http.Response>();
      final container = await _pumpLink(tester, path, (_) => pending.future);
      await tester.tap(find.text('open link'));
      await tester.pump();
      container.read(authControllerProvider.notifier).state = const AuthState();
      pending.complete(
        _json({'message': 'private domain detail'}, status: 403),
      );
      await tester.pumpAndSettle();
      expect(find.text('403'), findsNothing);
      expect(find.text('private domain detail'), findsNothing);
      expect(find.byType(ProfileDetailSheet), findsNothing);
      expect(container.read(selectedChatIdProvider), isNull);
    });
  }
}

Map<String, String> _profile(String id, String username) => {
  'id': id,
  'account_id': 'account-$id',
  'username': username,
  'display_name': username,
  'discriminator': '1234',
};

http.Response _json(Object body, {int status = 200}) => http.Response(
  jsonEncode(body),
  status,
  headers: const {'content-type': 'application/json'},
);

Future<ProviderContainer> _pumpLink(
  WidgetTester tester,
  String path,
  Future<http.Response> Function(http.Request) domainResponse, {
  String? nextPath,
  Future<http.Response> Function(http.Request)? resolveResponse,
}) async {
  final container = ProviderContainer(
    overrides: [
      ...voiceAppTestOverrides(
        client: MockClient((request) async {
          if (request.url.path == '/api/v1/links/resolve') {
            if (resolveResponse != null) return resolveResponse(request);
            return _json({'kind': path.startsWith('/u/') ? 'profile' : 'dm'});
          }
          return domainResponse(request);
        }),
      ),
      // Exercise the actual createDm client/status mapping, not the shared fake.
      voiceChatsClientProvider.overrideWith(
        (ref) =>
            VoiceChatsClient(gateway: ref.watch(gatewayHttpClientProvider)),
      ),
      presenceProvider.overrideWith((ref, profileId) => null),
    ],
  );
  addTearDown(container.dispose);
  final router = createVoiceGoRouter(
    shellBuilder: (context, state) => Consumer(
      builder: (context, ref, _) => Scaffold(
        body: Column(
          children: [
            const Text('home shell'),
            if (ref.watch(_linkCallerVisibleProvider))
              Consumer(
                builder: (context, ref, _) => TextButton(
                  onPressed: () => resolveAndNavigateDeepLink(
                    ref,
                    parseDeepLinkUrl('https://voice.gg$path'),
                  ),
                  child: const Text('open link'),
                ),
              ),
            if (nextPath != null)
              TextButton(
                onPressed: () => resolveAndNavigateDeepLink(
                  ref,
                  parseDeepLinkUrl('https://voice.gg$nextPath'),
                ),
                child: const Text('open newer'),
              ),
          ],
        ),
      ),
    ),
  );
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp.router(
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
      ),
    ),
  );
  return container;
}

Future<void> _expectRecoveryAndHome(WidgetTester tester, int status) async {
  expect(find.text('$status'), findsOneWidget);
  expect(
    find.text(status == 403 ? 'Access denied' : 'Not found'),
    findsOneWidget,
  );
  expect(
    find.text(
      status == 403
          ? 'You do not have permission to open this link. It may be intended for another profile.'
          : 'This link is unavailable or no longer valid.',
    ),
    findsOneWidget,
  );
  await tester.tap(find.text('Go to Home'));
  await tester.pumpAndSettle();
  expect(find.text('home shell'), findsOneWidget);
  expect(find.text('$status'), findsNothing);
}
