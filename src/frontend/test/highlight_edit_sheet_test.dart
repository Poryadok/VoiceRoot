import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart' show LogicalKeyboardKey;
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/highlight_edit_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _highlight = HighlightData(
  id: 'hl-1',
  name: 'Clips',
  storyIds: ['story-1'],
);

Widget _wrap({
  required http.Client client,
  required bool editing,
  List<String> initialStoryIds = const [],
}) {
  return ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(client: client),
      storyArchiveProvider.overrideWith(
        (ref) async => const [
          StoryData(
            id: 'story-1',
            authorProfileId: 'prof-test',
            type: 'text',
            textContent: 'First story',
          ),
          StoryData(
            id: 'archived-1',
            authorProfileId: 'prof-test',
            type: 'text',
            textContent: 'Archived story',
          ),
        ],
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: Builder(
          builder: (context) => Center(
            child: TextButton(
              key: const Key('open_highlight_editor'),
              onPressed: () => editing
                  ? HighlightEditSheet.showEdit(context, highlight: _highlight)
                  : HighlightEditSheet.showCreate(
                      context,
                      initialStoryIds: initialStoryIds,
                    ),
              child: const Text('Open'),
            ),
          ),
        ),
      ),
    ),
  );
}

Future<void> _openEditor(WidgetTester tester, Widget widget) async {
  await tester.pumpWidget(widget);
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const Key('open_highlight_editor')));
  await tester.pumpAndSettle();
}

http.Response _failure(int status, String diagnostic) => http.Response(
  '{"error":"internal","message":"$diagnostic"}',
  status,
  headers: const {'content-type': 'application/json'},
);

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('Highlight editor keeps blank-name validation local', (
    tester,
  ) async {
    var requests = 0;
    final mock = MockClient((_) async {
      requests++;
      return _failure(500, 'private create diagnostic');
    });
    await _openEditor(
      tester,
      _wrap(client: mock, editing: false, initialStoryIds: const ['story-1']),
    );

    await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(requests, 0);
    expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);
  });

  testWidgets('Highlight editor hides upstream create error and stays open', (
    tester,
  ) async {
    http.Request? request;
    final mock = MockClient((req) async {
      request = req;
      return _failure(500, 'private create diagnostic');
    });
    await _openEditor(
      tester,
      _wrap(client: mock, editing: false, initialStoryIds: const ['story-1']),
    );
    await tester.enterText(
      find.byKey(HighlightEditSheet.nameFieldKey),
      'Summer',
    );
    await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(request?.method, 'POST');
    expect(request?.url.path, '/api/v1/stories/highlights');
    expect(find.text('private create diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);
  });

  testWidgets(
    'Checklist toggle unselects locally and save commits PATCH then DELETE',
    (tester) async {
      final requests = <http.Request>[];
      final mock = MockClient((req) async {
        requests.add(req);
        return http.Response(
          '{}',
          200,
          headers: const {'content-type': 'application/json'},
        );
      });
      await _openEditor(tester, _wrap(client: mock, editing: true));

      await tester.tap(
        find.byKey(
          const Key(
            '${HighlightEditSheet.storyRowKeyPrefix}archived-1'
            '${HighlightEditSheet.storyToggleKeySuffix}',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(requests, isEmpty);

      await tester.tap(
        find.byKey(
          const Key(
            '${HighlightEditSheet.storyRowKeyPrefix}story-1'
            '${HighlightEditSheet.storyToggleKeySuffix}',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(requests, isEmpty);

      await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
      await tester.pumpAndSettle();

      expect(requests.map((r) => r.method).toList(), const [
        'PATCH',
        'POST',
        'DELETE',
      ]);
      expect(requests.first.url.path, '/api/v1/stories/highlights/hl-1');
      expect(requests[1].url.path, '/api/v1/stories/highlights/hl-1/stories');
      expect(
        requests.last.url.path,
        '/api/v1/stories/highlights/hl-1/stories/story-1',
      );
      expect(find.byKey(HighlightEditSheet.sheetKey), findsNothing);
    },
  );

  testWidgets(
    'Highlight editor hides upstream save failure and retries pending writes',
    (tester) async {
      final requests = <http.Request>[];
      final mock = MockClient((req) async {
        requests.add(req);
        if (req.method == 'PATCH') {
          return http.Response(
            '{"highlight":{"id":"hl-1","name":"Clips"}}',
            200,
            headers: const {'content-type': 'application/json'},
          );
        }
        return _failure(500, 'private membership diagnostic');
      });
      await _openEditor(tester, _wrap(client: mock, editing: true));

      await tester.tap(
        find.byKey(
          const Key(
            '${HighlightEditSheet.storyRowKeyPrefix}archived-1'
            '${HighlightEditSheet.storyToggleKeySuffix}',
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      expect(
        requests
            .where((r) => r.url.path.endsWith('/highlights/hl-1/stories'))
            .length,
        1,
      );
      expect(find.text('private membership diagnostic'), findsNothing);
      expect(find.text('Could not complete this action.'), findsOneWidget);
      expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);

      await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      expect(
        requests
            .where((r) => r.url.path.endsWith('/highlights/hl-1/stories'))
            .length,
        2,
      );
      // A failed retry keeps the sheet open so pending writes stay staged.
      expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);
    },
  );

  testWidgets('Empty selection blocks save and prompts the user with a hint', (
    tester,
  ) async {
    var requests = 0;
    final mock = MockClient((req) async {
      requests++;
      return http.Response(
        '{}',
        200,
        headers: const {'content-type': 'application/json'},
      );
    });
    await _openEditor(tester, _wrap(client: mock, editing: true));
    await tester.tap(
      find.byKey(
        const Key(
          '${HighlightEditSheet.storyRowKeyPrefix}story-1'
          '${HighlightEditSheet.storyToggleKeySuffix}',
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
    await tester.pumpAndSettle();

    expect(requests, 0);
    expect(find.text('Select at least one story to save'), findsOneWidget);
    expect(find.byKey(HighlightEditSheet.coverPreviewKey), findsNothing);
    expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);
  });

  testWidgets(
    'Cover picker is display-only and cycles without touching the API',
    (tester) async {
      var requests = 0;
      final mock = MockClient((req) async {
        requests++;
        return http.Response(
          '{}',
          200,
          headers: const {'content-type': 'application/json'},
        );
      });
      await _openEditor(
        tester,
        _wrap(
          client: mock,
          editing: true,
          initialStoryIds: const ['archived-1'],
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byKey(HighlightEditSheet.coverPreviewKey), findsOneWidget);

      await tester.tap(find.byKey(HighlightEditSheet.coverButtonKey));
      await tester.pumpAndSettle();
      expect(requests, 0);

      await tester.enterText(
        find.byKey(HighlightEditSheet.nameFieldKey),
        'Clips two',
      );
      await tester.tap(find.byKey(HighlightEditSheet.saveButtonKey));
      await tester.pumpAndSettle();
      expect(find.byKey(HighlightEditSheet.sheetKey), findsNothing);
    },
  );

  testWidgets(
    'Highlight editor delete cancels and confirms without leaking diagnostics',
    (tester) async {
      final requests = <http.Request>[];
      final mock = MockClient((req) async {
        requests.add(req);
        return _failure(503, 'private delete diagnostic');
      });
      await _openEditor(tester, _wrap(client: mock, editing: true));

      await tester.tap(find.byKey(HighlightEditSheet.deleteButtonKey));
      await tester.pumpAndSettle();
      expect(find.text('Cancel'), findsOneWidget);
      expect(find.text('Delete'), findsOneWidget);
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(requests, isEmpty);
      expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);

      await tester.tap(find.byKey(HighlightEditSheet.deleteButtonKey));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Delete'));
      await tester.pumpAndSettle();
      expect(requests.length, 1);
      expect(requests.first.method, 'DELETE');
      expect(requests.first.url.path, '/api/v1/stories/highlights/hl-1');
      expect(find.text('private delete diagnostic'), findsNothing);
      expect(
        find.text('The service is temporarily unavailable. Please try again.'),
        findsOneWidget,
      );
      expect(find.byKey(HighlightEditSheet.sheetKey), findsOneWidget);
    },
  );

  testWidgets('Highlight editor cancels on Escape', (tester) async {
    final mock = MockClient((_) async => http.Response('{}', 200));
    await _openEditor(
      tester,
      _wrap(client: mock, editing: false, initialStoryIds: const ['story-1']),
    );

    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();

    expect(find.byKey(HighlightEditSheet.sheetKey), findsNothing);
  });

  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    testWidgets(
      'Highlight editor lays out without overflow at ${size.width}x${size.height}',
      (tester) async {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);

        final mock = MockClient((_) async => http.Response('{}', 200));
        await _openEditor(tester, _wrap(client: mock, editing: true));

        expect(find.byKey(HighlightEditSheet.nameFieldKey), findsOneWidget);
        expect(find.byKey(HighlightEditSheet.saveButtonKey), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  }
}
