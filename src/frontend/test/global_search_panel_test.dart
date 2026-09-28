import 'dart:async';
import 'dart:convert';

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
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/search/global_search_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  http.Response searchResponse(
    String snippet, {
    List<String> profileIds = const [],
  }) => http.Response(
    jsonEncode({
      'global_search_results': {
        'messages': [
          {'message_id': 'message-$snippet', 'snippet': snippet, 'score': 1.0},
        ],
        'profile_ids': profileIds,
        'matched_chats': [],
        'space_ids': [],
      },
    }),
    200,
  );

  Future<void> enterDebouncedQuery(WidgetTester tester, String query) async {
    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), query);
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump();
  }

  Future<void> flushHttpResponse(WidgetTester tester) async {
    for (var i = 0; i < 5; i++) {
      await tester.pump(const Duration(milliseconds: 1));
    }
  }

  Widget globalSearchTestApp({required http.Client client}) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: GlobalSearchPanel()),
      ),
    );
  }

  testWidgets('GlobalSearchPanel shows search field and section headers', (
    tester,
  ) async {
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((_) async => http.Response('{}', 200)),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(GlobalSearchPanel.panelKey), findsOneWidget);
    expect(find.byKey(GlobalSearchPanel.searchFieldKey), findsOneWidget);

    await tester.enterText(
      find.byKey(GlobalSearchPanel.searchFieldKey),
      'raid',
    );
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsOneWidget);
    expect(find.byKey(GlobalSearchPanel.spacesSectionKey), findsOneWidget);
    expect(find.byKey(GlobalSearchPanel.messagesSectionKey), findsOneWidget);
  });

  testWidgets('global search debounces input for 300ms', (tester) async {
    var globalCalls = 0;
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/global') {
            globalCalls++;
          }
          return http.Response(
            jsonEncode({
              'global_search_results': {
                'messages': [],
                'profile_ids': [],
                'matched_chats': [],
                'space_ids': [],
              },
            }),
            200,
          );
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GlobalSearchPanel.searchFieldKey),
      'raid',
    );
    await tester.pump();
    expect(globalCalls, 0);

    await tester.pump(const Duration(milliseconds: 300));
    await tester.pumpAndSettle();
    expect(globalCalls, 1);
  });

  testWidgets('global search renders contacts before spaces and messages', (
    tester,
  ) async {
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/global') {
            return http.Response(
              jsonEncode({
                'global_search_results': {
                  'messages': [
                    {
                      'message_id': 'msg-1',
                      'snippet': 'raid tonight',
                      'score': 1.0,
                    },
                  ],
                  'profile_ids': ['profile-carol'],
                  'matched_chats': [
                    {'id': 'chat-dm-1'},
                  ],
                  'space_ids': ['space-raid'],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/users/profiles/profile-carol') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'profile-carol',
                  'account_id': 'a-1',
                  'username': 'carol',
                  'discriminator': '0001',
                  'display_name': 'Carol',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              }),
              200,
            );
          }
          return http.Response('not found', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GlobalSearchPanel.searchFieldKey),
      'raid',
    );
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    final contacts = tester.getTopLeft(
      find.byKey(GlobalSearchPanel.contactsSectionKey),
    );
    final spaces = tester.getTopLeft(
      find.byKey(GlobalSearchPanel.spacesSectionKey),
    );
    final messages = tester.getTopLeft(
      find.byKey(GlobalSearchPanel.messagesSectionKey),
    );

    expect(contacts.dy, lessThan(spaces.dy));
    expect(spaces.dy, lessThan(messages.dy));
    expect(find.text('Carol'), findsOneWidget);
    expect(find.textContaining('raid tonight'), findsOneWidget);
  });

  testWidgets('global search API error shows message to user', (tester) async {
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/global') {
            return http.Response(
              jsonEncode({
                'error_code': 'internal',
                'message': 'search unavailable',
              }),
              500,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GlobalSearchPanel.searchFieldKey),
      'raid',
    );
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('global_search_error')), findsOneWidget);
    expect(find.textContaining('Could not complete search'), findsOneWidget);
  });

  testWidgets(
    'clearing a completed search restores idle state without a blank request',
    (tester) async {
      final queries = <String>[];
      await tester.pumpWidget(
        globalSearchTestApp(
          client: MockClient((req) async {
            if (req.url.path == '/api/v1/search/global') {
              queries.add(req.url.queryParameters['q'] ?? '');
              return searchResponse('raid result');
            }
            return http.Response('{}', 200);
          }),
        ),
      );
      await tester.pumpAndSettle();

      await enterDebouncedQuery(tester, 'raid');
      expect(find.text('raid result'), findsOneWidget);
      expect(queries, ['raid']);

      await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), '');
      await tester.pump();
      expect(find.text('raid result'), findsNothing);
      expect(find.byKey(const Key('global_search_error')), findsNothing);
      expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsNothing);

      await tester.pump(const Duration(milliseconds: 350));
      expect(queries, ['raid']);
      expect(find.byKey(const Key('global_search_error')), findsNothing);
      expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsNothing);
    },
  );

  testWidgets('later query results survive an older out-of-order success', (
    tester,
  ) async {
    final first = Completer<http.Response>();
    final second = Completer<http.Response>();
    final queries = <String>[];
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((req) {
          final query = req.url.queryParameters['q']!;
          queries.add(query);
          return query == 'alpha' ? first.future : second.future;
        }),
      ),
    );
    await tester.pumpAndSettle();

    await enterDebouncedQuery(tester, 'alpha');
    await enterDebouncedQuery(tester, 'bravo');
    expect(queries, ['alpha', 'bravo']);

    second.complete(searchResponse('bravo result'));
    await flushHttpResponse(tester);
    expect(find.text('bravo result'), findsOneWidget);

    first.complete(searchResponse('alpha result'));
    await flushHttpResponse(tester);
    expect(find.text('bravo result'), findsOneWidget);
    expect(find.text('alpha result'), findsNothing);
    expect(find.byKey(const Key('global_search_error')), findsNothing);
  });

  testWidgets('clearing an in-flight search ignores its late result', (
    tester,
  ) async {
    final pending = Completer<http.Response>();
    final queries = <String>[];
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((req) {
          queries.add(req.url.queryParameters['q'] ?? '');
          return pending.future;
        }),
      ),
    );
    await tester.pumpAndSettle();

    await enterDebouncedQuery(tester, 'alpha');
    expect(queries, ['alpha']);
    expect(find.byType(VoiceListSkeleton), findsOneWidget);

    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), '');
    await tester.pump();
    expect(find.byType(VoiceListSkeleton), findsNothing);

    pending.complete(searchResponse('late alpha result'));
    await flushHttpResponse(tester);
    await tester.pump(const Duration(milliseconds: 350));
    expect(queries, ['alpha']);
    expect(find.text('late alpha result'), findsNothing);
    expect(find.byKey(const Key('global_search_error')), findsNothing);
    expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsNothing);
  });

  testWidgets('older profile hydration cannot replace newer search results', (
    tester,
  ) async {
    final oldProfile = Completer<http.Response>();
    var profileRequests = 0;
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient((req) {
          if (req.url.path == '/api/v1/users/profiles/profile-alpha') {
            profileRequests++;
            return oldProfile.future;
          }
          final query = req.url.queryParameters['q'];
          if (query == 'alpha') {
            return Future.value(
              searchResponse('alpha result', profileIds: ['profile-alpha']),
            );
          }
          if (query == 'bravo') {
            return Future.value(searchResponse('bravo result'));
          }
          return Future.value(http.Response('not found', 404));
        }),
      ),
    );
    await tester.pumpAndSettle();

    await enterDebouncedQuery(tester, 'alpha');
    expect(profileRequests, 1);
    await enterDebouncedQuery(tester, 'bravo');
    await flushHttpResponse(tester);
    expect(find.text('bravo result'), findsOneWidget);

    oldProfile.complete(
      http.Response(
        jsonEncode({
          'profile': {
            'id': 'profile-alpha',
            'account_id': 'account-alpha',
            'username': 'alpha',
            'discriminator': '0001',
            'display_name': 'Alpha',
            'verification_type': 'none',
          },
        }),
        200,
      ),
    );
    await flushHttpResponse(tester);
    expect(find.text('bravo result'), findsOneWidget);
    expect(find.text('alpha result'), findsNothing);
    expect(find.text('Alpha'), findsNothing);
    expect(find.byKey(const Key('global_search_error')), findsNothing);
  });

  testWidgets('older completion cannot stop a newer pending search loading', (
    tester,
  ) async {
    final first = Completer<http.Response>();
    final second = Completer<http.Response>();
    await tester.pumpWidget(
      globalSearchTestApp(
        client: MockClient(
          (req) => req.url.queryParameters['q'] == 'alpha'
              ? first.future
              : second.future,
        ),
      ),
    );
    await tester.pumpAndSettle();

    await enterDebouncedQuery(tester, 'alpha');
    await enterDebouncedQuery(tester, 'bravo');
    expect(find.byType(VoiceListSkeleton), findsOneWidget);

    first.complete(searchResponse('alpha result'));
    await flushHttpResponse(tester);
    expect(find.byType(VoiceListSkeleton), findsOneWidget);
    expect(find.text('alpha result'), findsNothing);

    second.complete(searchResponse('bravo result'));
    await flushHttpResponse(tester);
    expect(find.byType(VoiceListSkeleton), findsNothing);
    expect(find.text('bravo result'), findsOneWidget);
  });

  testWidgets(
    'new valid query clears an error and ignores an older late failure',
    (tester) async {
      final oldFailure = Completer<http.Response>();
      final queries = <String>[];
      await tester.pumpWidget(
        globalSearchTestApp(
          client: MockClient((req) {
            final query = req.url.queryParameters['q']!;
            queries.add(query);
            if (query == 'old') {
              return Future.value(
                http.Response(
                  jsonEncode({
                    'error_code': 'internal',
                    'message': 'old failure',
                  }),
                  500,
                ),
              );
            }
            if (query == 'pending') return oldFailure.future;
            return Future.value(searchResponse('fresh result'));
          }),
        ),
      );
      await tester.pumpAndSettle();

      await enterDebouncedQuery(tester, 'old');
      expect(find.byKey(const Key('global_search_error')), findsOneWidget);

      await tester.enterText(
        find.byKey(GlobalSearchPanel.searchFieldKey),
        'pending',
      );
      await tester.pump();
      expect(find.byKey(const Key('global_search_error')), findsNothing);
      await tester.pump(const Duration(milliseconds: 300));
      await tester.pump();
      expect(queries, ['old', 'pending']);

      await enterDebouncedQuery(tester, 'fresh');
      expect(find.text('fresh result'), findsOneWidget);
      expect(find.byKey(const Key('global_search_error')), findsNothing);

      oldFailure.complete(
        http.Response(
          jsonEncode({'error_code': 'internal', 'message': 'late failure'}),
          500,
        ),
      );
      await flushHttpResponse(tester);
      expect(find.text('fresh result'), findsOneWidget);
      expect(find.byKey(const Key('global_search_error')), findsNothing);
    },
  );

  testWidgets('compact mode search results do not require Expanded parent', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/search/global') {
                return http.Response(
                  jsonEncode({
                    'global_search_results': {
                      'messages': [],
                      'profile_ids': ['profile-carol'],
                      'matched_chats': [],
                      'space_ids': ['space-raid'],
                    },
                  }),
                  200,
                );
              }
              return http.Response('{}', 200);
            }),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: Column(
              children: [
                GlobalSearchPanel(compact: true),
                Expanded(child: SizedBox()),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(GlobalSearchPanel.searchFieldKey),
      'raid',
    );
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
