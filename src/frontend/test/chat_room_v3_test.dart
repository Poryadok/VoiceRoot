import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/search/in_chat_search.dart';

import 'in_chat_search_test.dart' show chatRoomInChatSearchTestApp;

void main() {
  for (final viewport in [
    const Size(390, 844),
    const Size(1280, 800),
    const Size(1280, 320),
  ]) {
    testWidgets(
      'room search header navigates real hits at width ${viewport.width} height ${viewport.height}',
      (tester) async {
        await tester.binding.setSurfaceSize(viewport);
        addTearDown(() => tester.binding.setSurfaceSize(null));
        var queries = 0;
        await tester.pumpWidget(
          chatRoomInChatSearchTestApp(
            client: MockClient((request) async {
              if (request.url.path == '/api/v1/search/in-chat') {
                queries++;
                return http.Response(
                  jsonEncode({
                    'searchResults': {
                      'hits': [
                        {
                          'messageId': 'msg-1',
                          'snippet': 'first needle',
                          'score': 1,
                        },
                        {
                          'messageId': 'msg-2',
                          'snippet': 'second needle',
                          'score': .9,
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              return http.Response('{}', 200);
            }),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
        await tester.pumpAndSettle();
        final header = find.byKey(const Key('in_chat_search_inline_header'));
        expect(
          find.descendant(
            of: header,
            matching: find.byKey(InChatSearch.prevHitKey),
          ),
          findsOneWidget,
        );
        await tester.enterText(
          find.byKey(InChatSearch.searchFieldKey),
          'needle',
        );
        await tester.pump(const Duration(milliseconds: 350));
        await tester.pumpAndSettle();
        expect(queries, 1);
        expect(find.text('1 / 2'), findsOneWidget);
        await tester.tap(find.byKey(InChatSearch.nextHitKey));
        await tester.pumpAndSettle();
        expect(find.text('2 / 2'), findsOneWidget);
        await tester.tap(find.byKey(InChatSearch.prevHitKey));
        await tester.pumpAndSettle();
        expect(find.text('1 / 2'), findsOneWidget);
        if (viewport.width < 600) {
          expect(find.byKey(InChatSearch.resultsOverlayKey), findsNothing);
        }
        expect(tester.takeException(), isNull);
        await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
        await tester.pumpAndSettle();
        expect(header, findsNothing);
      },
    );
  }

  testWidgets(
    'room search failure retries the same query without losing input',
    (tester) async {
      var attempts = 0;
      await tester.pumpWidget(
        chatRoomInChatSearchTestApp(
          client: MockClient((request) async {
            if (request.url.path == '/api/v1/search/in-chat') {
              expect(request.url.queryParameters['q'], 'needle');
              attempts++;
              if (attempts == 1) return http.Response('{}', 503);
              return http.Response(
                jsonEncode({
                  'searchResults': {
                    'hits': [
                      {'messageId': 'msg-1', 'snippet': 'needle', 'score': 1},
                    ],
                  },
                }),
                200,
              );
            }
            return http.Response('{}', 200);
          }),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'needle');
      await tester.pump(const Duration(milliseconds: 350));
      await tester.pumpAndSettle();
      expect(find.byKey(InChatSearch.errorKey), findsOneWidget);
      await tester.tap(
        find.descendant(
          of: find.byKey(InChatSearch.panelKey),
          matching: find.byType(TextButton),
        ),
      );
      await tester.pumpAndSettle();
      expect(attempts, 2);
      expect(find.text('1 / 1'), findsOneWidget);
      expect(find.byKey(InChatSearch.errorKey), findsNothing);
      expect(
        tester
            .widget<TextField>(find.byKey(InChatSearch.searchFieldKey))
            .controller!
            .text,
        'needle',
      );
    },
  );

  testWidgets('late search response cannot replace the newer query results', (
    tester,
  ) async {
    final oldResponse = Completer<http.Response>();
    await tester.pumpWidget(
      chatRoomInChatSearchTestApp(
        client: MockClient((request) async {
          if (request.url.path == '/api/v1/search/in-chat') {
            if (request.url.queryParameters['q'] == 'old') {
              return oldResponse.future;
            }
            return http.Response(
              jsonEncode({
                'searchResults': {
                  'hits': [
                    {'messageId': 'msg-1', 'snippet': 'new first', 'score': 1},
                    {
                      'messageId': 'msg-2',
                      'snippet': 'new second',
                      'score': .9,
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 200);
        }),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.inChatSearchKey));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'old');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.enterText(find.byKey(InChatSearch.searchFieldKey), 'new');
    await tester.pump(const Duration(milliseconds: 350));
    await tester.pumpAndSettle();
    expect(find.text('1 / 2'), findsOneWidget);
    oldResponse.complete(
      http.Response(
        jsonEncode({
          'searchResults': {
            'hits': [
              {'messageId': 'old-message', 'snippet': 'old', 'score': 1},
            ],
          },
        }),
        200,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('1 / 2'), findsOneWidget);
    expect(find.text('old'), findsNothing);
  });
}
