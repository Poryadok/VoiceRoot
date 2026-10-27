import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/ui/stories/story_archive_screen.dart';
import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _SessionController extends AuthController {
  _SessionController(Ref ref)
    : super(
        authClient: ref.read(voiceAuthClientProvider),
        storage: ref.read(authSessionStorageProvider),
        guestCredentialsStorage: ref.read(guestCredentialsStorageProvider),
      ) {
    replace('prof-test');
  }
  int _install = 0;
  int _token = 0;
  @override
  int get sessionInstallGeneration => _install;
  void replace(String profile, {bool reinstall = true}) {
    if (reinstall) _install++;
    _token++;
    state = AuthState(
      session: AuthSession(
        accessToken: 'access-$_token',
        refreshToken: 'refresh-$_token',
        accountId: 'account-test',
        activeProfileId: profile,
        expiresInSeconds: 900,
      ),
    );
  }
}

const _archiveJson =
    '{"stories":[{"id":"arch-1","author_profile_id":"prof-test","type":"text","text_content":"Private A story"}]}';

Future<(_SessionController, ProviderContainer)> _open(
  WidgetTester tester,
  http.Client client,
) async {
  final overrides = voiceAppTestOverrides(client: client);
  final container = ProviderContainer(
    overrides: [
      ...overrides,
      authControllerProvider.overrideWith(_SessionController.new),
    ],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const StoryArchiveScreen(),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return (
    container.read(authControllerProvider.notifier) as _SessionController,
    container,
  );
}

Future<void> _newHighlight(WidgetTester tester) async {
  await tester.tap(find.byKey(const Key('story_archive_add_highlight_arch-1')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('New highlight'));
  await tester.pumpAndSettle();
  await tester.enterText(find.byKey(const Key('highlight_edit_name')), 'Best');
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  testWidgets('real archive HTTP failure displays retry and recovers', (
    tester,
  ) async {
    var loads = 0;
    await _open(
      tester,
      MockClient(
        (request) async => http.Response(
          ++loads == 1 ? '{"message":"private HTTP diagnostic"}' : _archiveJson,
          loads == 1 ? 503 : 200,
        ),
      ),
    );
    expect(find.text('No archived stories'), findsNothing);
    expect(find.text('private HTTP diagnostic'), findsNothing);
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(loads, 2);
    expect(find.text('Private A story'), findsOneWidget);
  });

  testWidgets(
    'real highlights HTTP failure retries without showing empty success',
    (tester) async {
      var loads = 0;
      await _open(
        tester,
        MockClient((request) async {
          if (request.url.path.endsWith('/archive')) {
            return http.Response(_archiveJson, 200);
          }
          loads++;
          return http.Response(
            loads == 1
                ? '{"message":"private highlight diagnostic"}'
                : '{"highlights":[{"id":"hl-1","name":"Clips"}]}',
            loads == 1 ? 503 : 200,
          );
        }),
      );
      await tester.tap(
        find.byKey(const Key('story_archive_add_highlight_arch-1')),
      );
      await tester.pumpAndSettle();
      expect(find.text('private highlight diagnostic'), findsNothing);
      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(loads, 2);
      expect(find.text('Clips'), findsOneWidget);
    },
  );

  for (final sameProfile in [false, true]) {
    testWidgets(
      'seeded editor rejects ${sameProfile ? 'same-profile reinstall' : 'profile switch'} before save',
      (tester) async {
        var writes = 0;
        final (controller, _) = await _open(
          tester,
          MockClient((request) async {
            if (request.url.path.endsWith('/archive')) {
              return http.Response(_archiveJson, 200);
            }
            if (request.method == 'GET') {
              return http.Response('{"highlights":[]}', 200);
            }
            writes++;
            return http.Response(
              '{"highlight":{"id":"new-1","name":"Best"}}',
              200,
            );
          }),
        );
        await _newHighlight(tester);
        controller.replace(sameProfile ? 'prof-test' : 'prof-b');
        await tester.pumpAndSettle();
        if (find
            .byKey(const Key('highlight_edit_save'))
            .evaluate()
            .isNotEmpty) {
          await tester.tap(find.byKey(const Key('highlight_edit_save')));
          await tester.pumpAndSettle();
        }
        expect(writes, 0);
        expect(find.byKey(const Key('highlight_edit_name')), findsNothing);
      },
    );
  }

  testWidgets('profile switch while creating stops later membership writes', (
    tester,
  ) async {
    final response = Completer<http.Response>();
    var additions = 0;
    final (controller, _) = await _open(
      tester,
      MockClient((request) async {
        if (request.url.path.endsWith('/archive')) {
          return http.Response(_archiveJson, 200);
        }
        if (request.method == 'GET') {
          return http.Response('{"highlights":[]}', 200);
        }
        if (request.url.path.endsWith('/stories')) {
          additions++;
          return http.Response('{}', 200);
        }
        return response.future;
      }),
    );
    await _newHighlight(tester);
    await tester.tap(find.byKey(const Key('highlight_edit_save')));
    await tester.pump();
    controller.replace('prof-b');
    await tester.pump();
    response.complete(
      http.Response('{"highlight":{"id":"new-1","name":"Best"}}', 200),
    );
    await tester.pumpAndSettle();
    expect(additions, 0);
    expect(find.byKey(const Key('highlight_edit_name')), findsNothing);
  });

  testWidgets('open picker never offers B collections for A story', (
    tester,
  ) async {
    final (controller, _) = await _open(
      tester,
      MockClient((request) async {
        if (request.url.path.endsWith('/archive')) {
          return http.Response(_archiveJson, 200);
        }
        final name = request.url.queryParameters['profile_id'] == 'prof-b'
            ? 'B collection'
            : 'A collection';
        return http.Response(
          '{"highlights":[{"id":"hl-1","name":"$name"}]}',
          200,
        );
      }),
    );
    await tester.tap(
      find.byKey(const Key('story_archive_add_highlight_arch-1')),
    );
    await tester.pumpAndSettle();
    controller.replace('prof-b');
    await tester.pumpAndSettle();
    expect(find.text('B collection'), findsNothing);
    expect(find.text('A collection'), findsNothing);
  });

  testWidgets(
    'token rotation in the same installation allows membership retry',
    (tester) async {
      var creates = 0;
      var additions = 0;
      final (controller, _) = await _open(
        tester,
        MockClient((request) async {
          if (request.url.path.endsWith('/archive')) {
            return http.Response(_archiveJson, 200);
          }
          if (request.method == 'GET') {
            return http.Response('{"highlights":[]}', 200);
          }
          if (request.url.path.endsWith('/stories')) {
            additions++;
            return http.Response('{}', additions == 1 ? 503 : 200);
          }
          if (request.method == 'POST') creates++;
          return http.Response(
            '{"highlight":{"id":"new-1","name":"Best"}}',
            200,
          );
        }),
      );
      await _newHighlight(tester);
      await tester.tap(find.byKey(const Key('highlight_edit_save')));
      await tester.pumpAndSettle();
      controller.replace('prof-test', reinstall: false);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('highlight_edit_save')));
      await tester.pumpAndSettle();
      expect(creates, 1);
      expect(additions, 2);
      expect(find.byKey(const Key('highlight_edit_name')), findsNothing);
    },
  );

  testWidgets(
    'nested archive selection resets when the author session changes',
    (tester) async {
      var writes = 0;
      final (controller, _) = await _open(
        tester,
        MockClient((request) async {
          if (request.url.path.endsWith('/archive')) {
            return http.Response(_archiveJson, 200);
          }
          if (request.method == 'GET') {
            return http.Response('{"highlights":[]}', 200);
          }
          writes++;
          return http.Response('{}', 200);
        }),
      );
      await _newHighlight(tester);
      await tester.tap(find.byKey(const Key('highlight_edit_add_stories')));
      await tester.pumpAndSettle();
      expect(
        find.ancestor(
          of: find.text('Private A story'),
          matching: find.byType(ListTile),
        ),
        findsOneWidget,
      );
      controller.replace('prof-b');
      await tester.pumpAndSettle();
      expect(
        find.ancestor(
          of: find.text('Private A story'),
          matching: find.byType(ListTile),
        ),
        findsNothing,
      );
      expect(find.byKey(const Key('highlight_edit_name')), findsNothing);
      expect(writes, 0);
    },
  );
}
