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
import 'package:voice_frontend/ui/search/in_chat_search.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget inChatSearchTestApp({
    required http.Client client,
    required String chatId,
  }) {
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
        home: Scaffold(
          body: InChatSearch(chatId: chatId),
        ),
      ),
    );
  }

  testWidgets('InChatSearch shows field and navigation controls', (tester) async {
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-1',
        client: MockClient((_) async => http.Response('{}', 200)),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.panelKey), findsOneWidget);
    expect(find.byKey(InChatSearch.searchFieldKey), findsOneWidget);
    expect(find.byKey(InChatSearch.prevHitKey), findsOneWidget);
    expect(find.byKey(InChatSearch.nextHitKey), findsOneWidget);
  });

  testWidgets('in-chat search queries scoped chat_id', (tester) async {
    String? chatIdParam;
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-scoped',
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            chatIdParam = req.url.queryParameters['chat_id'];
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {
                      'messageId': 'msg-1',
                      'snippet': 'alpha match',
                      'score': 1.0,
                    },
                    {
                      'messageId': 'msg-2',
                      'snippet': 'beta match',
                      'score': 0.9,
                    },
                  ],
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

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'match');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(chatIdParam, 'chat-scoped');
    expect(find.textContaining('alpha match'), findsOneWidget);
  });

  testWidgets('in-chat search up/down navigates between hits', (tester) async {
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-1',
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {
                      'messageId': 'msg-1',
                      'snippet': 'first hit',
                      'score': 1.0,
                    },
                    {
                      'messageId': 'msg-2',
                      'snippet': 'second hit',
                      'score': 0.9,
                    },
                  ],
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

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'hit');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.activeHitKey('msg-1')), findsOneWidget);

    await tester.tap(find.byKey(InChatSearch.nextHitKey));
    await tester.pumpAndSettle();
    expect(find.byKey(InChatSearch.activeHitKey('msg-2')), findsOneWidget);

    await tester.tap(find.byKey(InChatSearch.prevHitKey));
    await tester.pumpAndSettle();
    expect(find.byKey(InChatSearch.activeHitKey('msg-1')), findsOneWidget);
  });

  testWidgets('in-chat search highlights snippet match', (tester) async {
    await tester.pumpWidget(
      inChatSearchTestApp(
        chatId: 'chat-1',
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/search/in-chat') {
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {
                      'messageId': 'msg-1',
                      'snippet': 'find the <mark>needle</mark> here',
                      'score': 1.0,
                    },
                  ],
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

    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();

    expect(find.byKey(InChatSearch.highlightKey), findsOneWidget);
  });
}
