import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'row cancel failure retries exact authenticated API then removes row',
    (tester) async {
      var cancelCalls = 0;
      var updateCalls = 0;
      final cancelStarted = Completer<void>();
      final cancelResponse = Completer<http.Response>();
      final staleResponse = Completer<http.Response>();
      final requests = <http.Request>[];
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/messages/scheduled' &&
            request.method == 'GET') {
          return http.Response(
            jsonEncode({
              'scheduled_messages': [
                for (final id in ['one', 'two'])
                  {
                    'id': id,
                    'chat': {'id': 'chat'},
                    'sender_profile_id': 'prof-test',
                    'send_when_online': true,
                    'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
                    'payload': {'content': 'Preview $id'},
                  },
              ],
              'page': {'has_more': false},
            }),
            200,
          );
        }
        if (request.method == 'DELETE') {
          if (request.url.path.endsWith('/two')) return staleResponse.future;
          requests.add(request);
          cancelCalls++;
          if (cancelCalls == 1) {
            cancelStarted.complete();
            return cancelResponse.future;
          }
          if (cancelCalls == 2) return http.Response('{}', 503);
          return http.Response('', 204);
        }
        if (request.method == 'PATCH') {
          updateCalls++;
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(body['payload']['content'], 'Edited scheduled draft');
          expect(body['send_when_online'], isTrue);
          if (updateCalls == 1) return http.Response('{}', 503);
          return http.Response(
            jsonEncode({
              'scheduled_message': {
                'id': 'one',
                'chat': {'id': 'chat'},
                'sender_profile_id': 'prof-test',
                'send_when_online': true,
                'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
                'payload': body['payload'],
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(client: client),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(
              baseUrl: 'http://api.test',
              scheduledMessagesEnabled: true,
            ),
          ),
          voiceMessagesClientProvider.overrideWith(
            (ref) => VoiceMessagesClient(
              gateway: GatewayHttpClient(
                config: ref.watch(gatewayConfigProvider),
                httpClient: client,
              ),
            ),
          ),
          selectedChatIdProvider.overrideWith((ref) => 'chat'),
          presenceProvider.overrideWith((ref, id) => null),
          chatRoomControllerProvider(
            'chat',
          ).overrideWith((ref) => _Room(ref, 'chat')),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat')),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(ChatRoomPanel.inputKey),
        'Ordinary unsent draft',
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_edit_one')));
      await tester.pumpAndSettle();
      TextEditingController composer() => tester
          .widget<TextField>(
            find.descendant(
              of: find.byKey(ChatRoomPanel.inputKey),
              matching: find.byType(TextField),
            ),
          )
          .controller!;
      expect(composer().text, 'Preview one');
      expect(find.byType(AlertDialog), findsNothing);
      await tester.enterText(
        find.byKey(ChatRoomPanel.inputKey),
        'Edited scheduled draft',
      );
      await tester.tap(find.byKey(const Key('scheduled_edit_cancel')));
      await tester.pumpAndSettle();
      expect(composer().text, 'Ordinary unsent draft');
      await tester.tap(find.byKey(const ValueKey('scheduled_edit_one')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(ChatRoomPanel.inputKey), '');
      await tester.tap(find.byKey(ChatRoomPanel.sendKey));
      await tester.pumpAndSettle();
      expect(
        find.text('Enter a message of 1–4000 characters.'),
        findsOneWidget,
      );
      expect(updateCalls, 0);
      await tester.enterText(
        find.byKey(ChatRoomPanel.inputKey),
        'Edited scheduled draft',
      );
      await tester.tap(find.byKey(ChatRoomPanel.sendKey));
      await tester.pumpAndSettle();
      expect(composer().text, 'Edited scheduled draft');
      expect(updateCalls, 1);
      await tester.ensureVisible(
        find.byKey(const ValueKey('scheduled_retry_one')),
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_retry_one')));
      await tester.pumpAndSettle();
      expect(updateCalls, 2);
      expect(composer().text, 'Ordinary unsent draft');
      expect(find.text('Edited scheduled draft'), findsOneWidget);
      await tester.ensureVisible(
        find.byKey(const ValueKey('scheduled_cancel_one')),
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_cancel_one')));
      await tester.pump();
      expect(cancelStarted.isCompleted, isTrue);
      expect(find.byKey(const ValueKey('scheduled_retry_one')), findsNothing);
      expect(
        tester
            .widget<TextButton>(
              find.byKey(const ValueKey('scheduled_cancel_one')),
            )
            .onPressed,
        isNull,
      );
      cancelResponse.complete(http.Response('{}', 503));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('scheduled_retry_one')), findsOneWidget);
      expect(find.byKey(const ValueKey('scheduled_retry_two')), findsNothing);
      final obsoleteCancelRetry = tester
          .widget<TextButton>(find.byKey(const ValueKey('scheduled_retry_one')))
          .onPressed!;
      await tester.ensureVisible(
        find.byKey(const ValueKey('scheduled_edit_one')),
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_edit_one')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('scheduled_retry_one')), findsNothing);
      obsoleteCancelRetry();
      await tester.pumpAndSettle();
      expect(cancelCalls, 1);
      expect(composer().text, 'Edited scheduled draft');
      await tester.tap(find.byKey(const Key('scheduled_edit_cancel')));
      await tester.pumpAndSettle();
      await tester.ensureVisible(
        find.byKey(const ValueKey('scheduled_cancel_one')),
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_cancel_one')));
      await tester.pumpAndSettle();
      await tester.ensureVisible(
        find.byKey(const ValueKey('scheduled_retry_one')),
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_retry_one')));
      await tester.pumpAndSettle();
      expect(cancelCalls, 3);
      expect(
        requests.map((r) => r.url.path),
        everyElement('/api/v1/messages/scheduled/one'),
      );
      expect(
        requests.map((r) => r.headers['authorization']),
        everyElement('Bearer test-access'),
      );
      expect(find.text('Preview one'), findsNothing);
      expect(find.text('Preview two'), findsOneWidget);
      await tester.ensureVisible(
        find.byKey(const ValueKey('scheduled_cancel_two')),
      );
      await tester.tap(find.byKey(const ValueKey('scheduled_cancel_two')));
      await tester.pump();
      container.read(selectedChatIdProvider.notifier).state = 'other-chat';
      await tester.pump();
      container.read(selectedChatIdProvider.notifier).state = 'chat';
      await tester.pump();
      staleResponse.complete(http.Response('{}', 503));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('scheduled_retry_two')), findsNothing);
      expect(
        tester
            .widget<TextButton>(
              find.byKey(const ValueKey('scheduled_cancel_two')),
            )
            .onPressed,
        isNotNull,
      );
      expect(tester.takeException(), isNull);
    },
  );
}

class _Room extends ChatRoomController {
  _Room(super.ref, super.chatId) : super() {
    state = const ChatRoomState(historyProfileId: 'prof-test');
  }
  @override
  Future<void> loadInitial() async {}
}
