import 'dart:convert';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'GIF pagination retains results and retries the exact Chat cursor',
    (tester) async {
      final requests = <http.Request>[];
      var failPage = true;
      await _pumpPicker(
        tester,
        MockClient((request) async {
          requests.add(request);
          if (request.url.queryParameters['cursor'] == 'opaque/+==') {
            if (failPage) {
              return http.Response('{"error_code":"unavailable"}', 503);
            }
            return http.Response(
              '{"items":[{"provider":"fixture","provider_id":"one"},{"provider":"fixture","provider_id":"two"}]}',
              200,
            );
          }
          return http.Response(
            '{"items":[{"provider":"fixture","provider_id":"one"}],"next_cursor":"opaque/+=="}',
            200,
          );
        }),
      );
      await tester.tap(find.text('GIFs'));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<EditableText>(find.byType(EditableText))
            .focusNode
            .hasFocus,
        isTrue,
      );
      await tester.ensureVisible(find.text('Load more'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Load more'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('gif_result_fixture_one')), findsOneWidget);
      expect(find.text('GIF search unavailable'), findsOneWidget);
      failPage = false;
      await tester.tap(find.text('Retry'));
      await tester.pumpAndSettle();
      expect(requests.last.url, requests[requests.length - 2].url);
      expect(requests.last.url.queryParameters['cursor'], 'opaque/+==');
      expect(find.byKey(const Key('gif_result_fixture_one')), findsOneWidget);
      expect(find.byKey(const Key('gif_result_fixture_two')), findsOneWidget);
      expect(find.text('Load more'), findsNothing);
    },
  );

  testWidgets(
    'sticker search during loading filters the eventual installed catalog',
    (tester) async {
      final response = Completer<http.Response>();
      await _pumpPicker(tester, MockClient((_) => response.future));
      await tester.tap(find.text('Stickers'));
      await tester.pump();
      await tester.enterText(
        find.byKey(const Key('composer_media_search')),
        'beta',
      );
      response.complete(
        http.Response(
          '{"packs":[{"id":"beta","title":"Beta","stickers":[{"id":"b1","file_id":"file-b"}]}]}',
          200,
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.byKey(const Key('sticker_result_b1')), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    },
  );

  testWidgets(
    'installed sticker packs filter and switch without premium activation',
    (tester) async {
      final requests = <http.Request>[];
      await _pumpPicker(
        tester,
        MockClient((request) async {
          requests.add(request);
          return http.Response(
            jsonEncode({
              'packs': [
                {
                  'id': 'alpha',
                  'title': 'Alpha',
                  'is_system': true,
                  'stickers': [
                    {'id': 'a1', 'file_id': 'file-a', 'emoji': '👋'},
                  ],
                },
                {
                  'id': 'beta',
                  'title': 'Beta',
                  'stickers': [
                    {'id': 'b1', 'file_id': 'file-b', 'emoji': '🚀'},
                  ],
                },
                {
                  'id': 'premium',
                  'title': 'Premium pack',
                  'is_premium': true,
                  'stickers': [],
                },
              ],
            }),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }),
      );
      await tester.tap(find.text('Stickers'));
      await tester.pumpAndSettle();
      expect(requests.single.url.path, '/api/v1/sticker-packs');
      expect(find.text('Premium pack'), findsNothing);
      expect(find.byKey(const Key('sticker_result_a1')), findsOneWidget);
      await tester.enterText(
        find.byKey(const Key('composer_media_search')),
        'premium',
      );
      await tester.pumpAndSettle();
      expect(find.text('Premium pack'), findsNothing);
      await tester.enterText(
        find.byKey(const Key('composer_media_search')),
        '',
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('Beta'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('sticker_result_b1')), findsOneWidget);
      await tester.enterText(
        find.byKey(const Key('composer_media_search')),
        'alpha',
      );
      await tester.pumpAndSettle();
      expect(find.text('Beta'), findsNothing);
      expect(find.byKey(const Key('sticker_result_a1')), findsOneWidget);
      expect(
        requests,
        hasLength(1),
        reason: 'Sticker filtering is local to installed packs',
      );
    },
  );

  testWidgets(
    'late GIF query and profile responses cannot replace current catalog',
    (tester) async {
      final delayed = Completer<http.Response>();
      final container = await _pumpPicker(
        tester,
        MockClient((request) async {
          if (request.url.queryParameters['q'] == 'old') return delayed.future;
          if (request.url.queryParameters['q'] == 'new') {
            return http.Response(
              '{"items":[{"provider":"fixture","provider_id":"new"}]}',
              200,
            );
          }
          return http.Response('{"items":[]}', 200);
        }),
      );
      await tester.tap(find.text('GIFs'));
      await tester.pumpAndSettle();
      final search = find.byKey(const Key('composer_media_search'));
      await tester.enterText(search, 'old');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.enterText(search, 'new');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('gif_result_fixture_new')), findsOneWidget);
      delayed.complete(http.Response('{"error_code":"unavailable"}', 503));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('gif_result_fixture_new')), findsOneWidget);
      expect(find.text('GIF search unavailable'), findsNothing);
      container.read(authControllerProvider.notifier).state = const AuthState();
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('gif_result_fixture_new')), findsNothing);
      expect(tester.widget<TextField>(search).controller!.text, isEmpty);
    },
  );

  testWidgets(
    'real GIF catalog retries and searches without inventing send authority',
    (tester) async {
      final requests = <http.Request>[];
      var fail = true;
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              requests.add(request);
              expect(request.headers['authorization'], 'Bearer test-access');
              if (fail) {
                return http.Response(
                  '{"error_code":"unavailable","message":"GIF search unavailable"}',
                  503,
                );
              }
              return http.Response(
                jsonEncode({
                  'items': [
                    {
                      'provider': 'fixture',
                      'provider_id': 'wave',
                      'preview_url': 'https://cdn.test/wave.gif',
                      'file_id': 'some-file',
                      'width': 120,
                      'height': 80,
                    },
                  ],
                }),
                200,
              );
            }),
          ),
        ],
      );
      addTearDown(container.dispose);
      final selected = <String>[];
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Consumer(
              builder: (context, ref, _) => Scaffold(
                body: TextButton(
                  onPressed: () =>
                      openEmojiPanel(context, ref, onSelected: selected.add),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('GIFs'), findsOneWidget);
      await tester.tap(find.text('GIFs'));
      await tester.pumpAndSettle();
      expect(requests.single.url.path, '/api/v1/gifs/trending');
      expect(find.text('GIF search unavailable'), findsOneWidget);
      fail = false;
      await tester.tap(find.text('Retry'));
      await tester.pumpAndSettle();
      final result = find.byKey(const Key('gif_result_fixture_wave'));
      expect(result, findsOneWidget);
      expect(
        tester.widget<IconButton>(result).onPressed,
        isNull,
        reason:
            'Even a returned File ID is not authorized catalog-media send admission',
      );
      await tester.enterText(
        find.byKey(const Key('composer_media_search')),
        'wave',
      );
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      expect(requests.last.url.path, '/api/v1/gifs/search');
      expect(requests.last.url.queryParameters['q'], 'wave');
      expect(selected, isEmpty);
      expect(requests.every((request) => request.method == 'GET'), isTrue);
      expect(tester.takeException(), isNull);
    },
  );
}

Future<ProviderContainer> _pumpPicker(
  WidgetTester tester,
  http.Client client,
) async {
  final container = ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
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
        home: Consumer(
          builder: (context, ref, _) => Scaffold(
            body: TextButton(
              onPressed: () => openEmojiPanel(context, ref, onSelected: (_) {}),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
  return container;
}
