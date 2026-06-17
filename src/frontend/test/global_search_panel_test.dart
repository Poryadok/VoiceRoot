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
import 'package:voice_frontend/ui/search/global_search_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
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

    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'raid');
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

    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'raid');
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

    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'raid');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    final contacts = tester.getTopLeft(find.byKey(GlobalSearchPanel.contactsSectionKey));
    final spaces = tester.getTopLeft(find.byKey(GlobalSearchPanel.spacesSectionKey));
    final messages = tester.getTopLeft(find.byKey(GlobalSearchPanel.messagesSectionKey));

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
              jsonEncode({'error_code': 'internal', 'message': 'search unavailable'}),
              500,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'raid');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('global_search_error')), findsOneWidget);
    expect(find.textContaining('search unavailable'), findsOneWidget);
  });

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

    await tester.enterText(find.byKey(GlobalSearchPanel.searchFieldKey), 'raid');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(GlobalSearchPanel.contactsSectionKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
