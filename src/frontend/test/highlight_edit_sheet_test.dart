import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
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

Widget _wrap({required http.Client client, required bool editing}) {
  return ProviderScope(
    overrides: [
      ...voiceAppTestOverrides(client: client),
      storyArchiveProvider.overrideWith(
        (ref) async => const [
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
                  : HighlightEditSheet.showCreate(context),
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
  testWidgets('Highlight editor keeps blank-name validation local', (
    tester,
  ) async {
    var requests = 0;
    final mock = MockClient((_) async {
      requests++;
      return _failure(500, 'private create diagnostic');
    });
    await _openEditor(tester, _wrap(client: mock, editing: false));

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
    await _openEditor(tester, _wrap(client: mock, editing: false));
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

  testWidgets('Highlight editor hides upstream remove error without removing', (
    tester,
  ) async {
    http.Request? request;
    final mock = MockClient((req) async {
      request = req;
      return _failure(500, 'private remove diagnostic');
    });
    await _openEditor(tester, _wrap(client: mock, editing: true));

    await tester.tap(
      find.descendant(
        of: find.byKey(const Key('highlight_story_story-1')),
        matching: find.byIcon(Icons.remove_circle_outline),
      ),
    );
    await tester.pumpAndSettle();

    expect(request?.method, 'DELETE');
    expect(
      request?.url.path,
      '/api/v1/stories/highlights/hl-1/stories/story-1',
    );
    expect(find.text('private remove diagnostic'), findsNothing);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.byKey(const Key('highlight_story_story-1')), findsOneWidget);
  });

  testWidgets('Highlight editor hides upstream add error without adding story', (
    tester,
  ) async {
    http.Request? request;
    final mock = MockClient((req) async {
      request = req;
      return _failure(503, 'private add diagnostic');
    });
    await _openEditor(tester, _wrap(client: mock, editing: true));

    await tester.tap(find.byKey(const Key('highlight_edit_add_stories')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Archived story'));
    await tester.pumpAndSettle();

    expect(request?.method, 'POST');
    expect(request?.url.path, '/api/v1/stories/highlights/hl-1/stories');
    expect(find.text('private add diagnostic'), findsNothing);
    expect(
      find.text(
        'Social and chat features are unavailable. Start the full API stack (docker compose --profile app).',
      ),
      findsOneWidget,
    );
    expect(find.byKey(const Key('highlight_story_archived-1')), findsNothing);
  });
}
