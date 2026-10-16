import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/stories/story_create_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(Widget child, {required http.Client client}) {
    return ProviderScope(
      overrides: voiceAppTestOverrides(client: client),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: child,
      ),
    );
  }

  testWidgets('StoryCreateScreen shows mention picker for @username', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrap(
        const StoryCreateScreen(),
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('story_create_mention_picker')),
      findsOneWidget,
    );
  });

  testWidgets('StoryCreateScreen shows visibility audience selector', (
    tester,
  ) async {
    await tester.pumpWidget(
      wrap(
        const StoryCreateScreen(),
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_create_visibility')), findsOneWidget);
  });

  testWidgets('story create hides upstream failure details', (tester) async {
    const upstreamDetail = 'private story service diagnostic';
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/stories') {
        return http.Response(
          jsonEncode({
            'error_code': 'internal_error',
            'message': upstreamDetail,
          }),
          500,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(wrap(const StoryCreateScreen(), client: client));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(StoryCreateScreen.textFieldKey),
      'A short story',
    );
    await tester.drag(find.byType(ListView), const Offset(0, -700));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(StoryCreateScreen.submitKey));
    await tester.tap(find.byKey(StoryCreateScreen.submitKey));
    await tester.pumpAndSettle();

    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.text(upstreamDetail), findsNothing);
    expect(find.byKey(StoryCreateScreen.submitKey), findsOneWidget);
  });

  testWidgets('successful story creation returns to the previous screen', (
    tester,
  ) async {
    http.Request? createRequest;
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/stories') {
        createRequest = request;
        return http.Response(
          jsonEncode({
            'story': {
              'id': 'story-created',
              'author_profile_id': 'prof-test',
              'type': 'text',
              'text_content': 'A short story',
              'visibility': 'friends',
            },
          }),
          201,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(
      wrap(
        Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => const StoryCreateScreen(),
                ),
              ),
              child: const Text('Open story creator'),
            ),
          ),
        ),
        client: client,
      ),
    );
    await tester.tap(find.text('Open story creator'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(StoryCreateScreen.textFieldKey),
      'A short story',
    );
    await tester.drag(find.byType(ListView), const Offset(0, -700));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(StoryCreateScreen.submitKey));
    await tester.tap(find.byKey(StoryCreateScreen.submitKey));
    await tester.pumpAndSettle();

    expect(createRequest, isNotNull);
    expect(createRequest!.method, 'POST');
    expect(find.text('Open story creator'), findsOneWidget);
    expect(find.byType(StoryCreateScreen), findsNothing);
  });
}
