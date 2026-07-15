import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// System text scale covered by docs/features/accessibility.md (×1.5 smoke).
const _textScale = 1.5;

void main() {
  testWidgets('ChatListPanel has no overflow at text scale 1.5', (tester) async {
    await tester.pumpWidget(
      _scaledChatTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-long',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-a',
                      },
                      'last_message_preview':
                          'A longer preview line to stress layout at larger text sizes',
                      'unread_count': 12,
                    },
                    {
                      'chat': {
                        'id': 'chat-b',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-b',
                      },
                      'last_message_preview': 'Second chat preview',
                      'unread_count': 0,
                    },
                  ],
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        home: const ChatListPanel(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListPanel.panelKey), findsOneWidget);
    expect(find.byKey(ChatListPanel.tileKey('chat-long')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('ChatRoomPanel composer has no overflow at text scale 1.5', (
    tester,
  ) async {
    await tester.pumpWidget(
      _scaledChatTestApp(
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({'message_list': {'messages': []}}),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
        overrides: [
          chatRoomControllerProvider('chat-abc').overrideWith(
            (ref) => _EmptyRoomController(ref, 'chat-abc'),
          ),
        ],
        home: const ChatRoomPanel(chatId: 'chat-abc'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.inputKey), findsOneWidget);
    expect(find.byKey(ChatRoomPanel.sendKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}

Widget _scaledChatTestApp({
  required http.Client client,
  required Widget home,
  List<Override> overrides = const [],
}) {
  return ProviderScope(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
      realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
      ...overrides,
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      builder: (context, child) {
        final media = MediaQuery.of(context);
        return MediaQuery(
          data: media.copyWith(
            textScaler: const TextScaler.linear(_textScale),
            size: const Size(390, 844),
          ),
          child: child!,
        );
      },
      home: Scaffold(
        body: SizedBox(
          width: 390,
          height: 844,
          child: home,
        ),
      ),
    ),
  );
}

class _EmptyRoomController extends ChatRoomController {
  _EmptyRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(messages: []);
  }
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
