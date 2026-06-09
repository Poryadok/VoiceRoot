import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/forward_message_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget testApp({
    required Widget home,
    required http.Client client,
    List<Override> extraOverrides = const [],
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
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ...extraOverrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  const sourceMessage = VoiceMessage(
    id: 'msg-src',
    chatId: 'chat-source',
    senderProfileId: 'profile-b',
    content: 'Forward me',
  );

  testWidgets('ForwardMessageSheet forwards to selected chat', (tester) async {
    var forwardCalled = false;
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => FilledButton(
            onPressed: () => ForwardMessageSheet.show(
              context,
              sourceMessage: sourceMessage,
              sourceChatId: 'chat-source',
            ),
            child: const Text('Open forward'),
          ),
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-source',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-test',
                      },
                    },
                    {
                      'chat': {
                        'id': 'chat-target',
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'profile-test',
                        'name': 'Friday squad',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/forward') {
            forwardCalled = true;
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            expect(body['source_message_id'], 'msg-src');
            expect(body['target_chat'], {'id': 'chat-target'});
            return http.Response(
              jsonEncode({
                'message': {
                  'id': 'msg-fwd',
                  'chat': {'id': 'chat-target'},
                  'sender_profile_id': 'profile-test',
                  'content': 'Forward me',
                  'type': 'forward',
                  'message_kind': 'MESSAGE_KIND_FORWARD',
                  'forward_from_id': 'msg-src',
                  'forward_from_sender': 'Alice',
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Open forward'));
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.sheetKey), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-target')), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-source')), findsNothing);

    await tester.tap(find.byKey(ForwardMessageSheet.chatTileKey('chat-target')));
    await tester.pumpAndSettle();

    expect(find.text('Add a comment'), findsOneWidget);
    await tester.tap(find.text('Forward'));
    await tester.pumpAndSettle();

    expect(forwardCalled, isTrue);
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsNothing);
  });

  testWidgets('ForwardMessageSheet filters chats by search', (tester) async {
    await tester.pumpWidget(
      testApp(
        home: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceChatId: 'chat-source',
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-source',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-test',
                      },
                    },
                    {
                      'chat': {
                        'id': 'chat-a',
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'profile-test',
                        'name': 'Alpha team',
                      },
                    },
                    {
                      'chat': {
                        'id': 'chat-b',
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'profile-test',
                        'name': 'Beta crew',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-a')), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-b')), findsOneWidget);

    await tester.enterText(find.byKey(ForwardMessageSheet.searchFieldKey), 'beta');
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-a')), findsNothing);
    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-b')), findsOneWidget);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  Future<void> disconnect() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  void typingStart(String chatId) {}

  @override
  void typingStop(String chatId) {}

  @override
  Future<void> markRead(String chatId, String messageId) async {}

  @override
  Future<void> deliveryAck({
    required String chatId,
    required String messageId,
    required String senderProfileId,
  }) async {}
}
