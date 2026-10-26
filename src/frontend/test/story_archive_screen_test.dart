import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:go_router/go_router.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/story_archive_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(
    Widget child, {
    http.Client? client,
    Future<List<HighlightData>> Function()? highlights,
  }) {
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, _) => child),
        GoRoute(
          path: '/stories/create',
          builder: (_, _) => const Scaffold(body: Text('Create destination')),
        ),
        GoRoute(
          path: '/stories/viewer',
          builder: (_, state) => Scaffold(
            body: Text('Viewer ${state.uri.queryParameters['ids']}'),
          ),
        ),
      ],
    );
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: client ?? MockClient((_) async => throw UnimplementedError()),
        ),
        storyArchiveProvider.overrideWith(
          (ref) async => const [
            StoryData(
              id: 'arch-1',
              authorProfileId: 'prof-test',
              type: 'text',
              textContent: 'Expired story',
            ),
          ],
        ),
        profileHighlightsProvider('prof-test').overrideWith(
          (ref) async => highlights != null
              ? await highlights()
              : const [HighlightData(id: 'hl-1', name: 'Clips')],
        ),
      ],
      child: MaterialApp.router(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
      ),
    );
  }

  testWidgets(
    'archive opens story viewer and create through real route helpers',
    (tester) async {
      await tester.pumpWidget(wrap(const StoryArchiveScreen()));
      await tester.pumpAndSettle();
      expect(find.byType(GridView), findsOneWidget);
      await tester.tap(find.byKey(const Key('story_archive_tile_arch-1')));
      await tester.pumpAndSettle();
      expect(find.text('Viewer arch-1'), findsOneWidget);
      GoRouter.of(tester.element(find.text('Viewer arch-1'))).pop();
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('story_archive_create')));
      await tester.pumpAndSettle();
      expect(find.text('Create destination'), findsOneWidget);
    },
  );

  testWidgets('highlight lookup failure offers retry and new highlight', (
    tester,
  ) async {
    var calls = 0;
    await tester.pumpWidget(
      wrap(
        const StoryArchiveScreen(),
        highlights: () async {
          if (++calls == 1) throw StateError('private lookup diagnostic');
          return const [];
        },
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(const Key('story_archive_add_highlight_arch-1')),
    );
    await tester.pumpAndSettle();
    expect(find.text('private lookup diagnostic'), findsNothing);
    final retry = AppLocalizations.of(
      tester.element(find.byKey(const Key('story_archive_screen'))),
    )!.commonRetry;
    expect(find.text(retry), findsOneWidget);
    await tester.tap(find.text(retry));
    await tester.pumpAndSettle();
    expect(calls, 2);
    await tester.tap(find.text('New highlight'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('highlight_edit_sheet')), findsOneWidget);
    expect(find.byKey(const Key('highlight_story_arch-1')), findsOneWidget);
  });

  testWidgets('StoryArchiveScreen shows archived story and add action', (
    tester,
  ) async {
    await tester.pumpWidget(wrap(const StoryArchiveScreen()));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_archive_screen')), findsOneWidget);
    expect(find.text('Expired story'), findsOneWidget);
    expect(
      find.byKey(const Key('story_archive_add_highlight_arch-1')),
      findsOneWidget,
    );
  });

  testWidgets('create is reachable and activated with keyboard', (
    tester,
  ) async {
    await tester.pumpWidget(wrap(const StoryArchiveScreen()));
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(find.text('Create destination'), findsOneWidget);
  });

  testWidgets(
    'story preview text keeps AA contrast on an accent in dark theme',
    (tester) async {
      await tester.pumpWidget(wrap(const StoryArchiveScreen()));
      await tester.pumpAndSettle();
      final text = tester.widget<Text>(find.text('Expired story'));
      final material = tester.widget<Material>(
        find
            .ancestor(
              of: find.text('Expired story'),
              matching: find.byType(Material),
            )
            .first,
      );
      final foreground = text.style!.color!.computeLuminance();
      final background = material.color!.computeLuminance();
      final ratio = foreground > background
          ? (foreground + .05) / (background + .05)
          : (background + .05) / (foreground + .05);
      expect(ratio, greaterThanOrEqualTo(4.5));
    },
  );

  testWidgets(
    'new highlight membership failure stays open and retries the same collection',
    (tester) async {
      var created = 0;
      var added = 0;
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/stories/highlights')) {
          created++;
          return http.Response(
            '{"highlight":{"id":"created-1","name":"Best"}}',
            200,
          );
        }
        if (request.url.path.endsWith('/highlights/created-1/stories')) {
          added++;
          return http.Response(
            added == 1 ? '{"message":"private save diagnostic"}' : '{}',
            added == 1 ? 503 : 200,
          );
        }
        if (request.method == 'PATCH' &&
            request.url.path.endsWith('/highlights/created-1')) {
          return http.Response(
            '{"highlight":{"id":"created-1","name":"Best"}}',
            200,
          );
        }
        throw StateError('Unexpected request ${request.url}');
      });
      await tester.pumpWidget(
        wrap(
          const StoryArchiveScreen(),
          client: client,
          highlights: () async => [],
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(
        find.byKey(const Key('story_archive_add_highlight_arch-1')),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('New highlight'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const Key('highlight_edit_name')),
        'Best',
      );
      await tester.tap(find.byKey(const Key('highlight_edit_save')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('highlight_edit_sheet')), findsOneWidget);
      expect(find.text('private save diagnostic'), findsNothing);
      await tester.tap(find.byKey(const Key('highlight_edit_save')));
      await tester.pumpAndSettle();
      expect(created, 1);
      expect(added, 2);
      expect(find.byKey(const Key('highlight_edit_sheet')), findsNothing);
    },
  );

  testWidgets('StoryArchiveScreen hides upstream add-to-highlight error', (
    tester,
  ) async {
    final mock = MockClient(
      (_) async => http.Response(
        '{"error":"internal","message":"private archive diagnostic"}',
        503,
        headers: const {'content-type': 'application/json'},
      ),
    );
    await tester.pumpWidget(wrap(const StoryArchiveScreen(), client: mock));
    await tester.pumpAndSettle();

    await tester.tap(
      find.byKey(const Key('story_archive_add_highlight_arch-1')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Clips'));
    await tester.pumpAndSettle();

    expect(find.text('private archive diagnostic'), findsNothing);
    expect(
      find.text(
        'Social and chat features are unavailable. Start the full API stack (docker compose --profile app).',
      ),
      findsOneWidget,
    );
  });
}
