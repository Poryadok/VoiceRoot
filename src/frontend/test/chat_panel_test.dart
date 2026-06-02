import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/social/presence_indicator.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget chatTestApp({required Widget home, required http.Client client}) {
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

  testWidgets('ChatListPanel shows chats from REST', (tester) async {
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-abc',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-a',
                      },
                      'last_message_preview': 'Preview text',
                      'unread_count': 3,
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

    expect(find.byKey(ChatListPanel.panelKey), findsOneWidget);
    expect(find.byKey(ChatListPanel.tileKey('chat-abc')), findsOneWidget);
    expect(find.text('Preview text'), findsOneWidget);
    expect(find.text('3'), findsOneWidget);
  });

  testWidgets('ChatListPanel resolves DM creator as peer after reload', (
    tester,
  ) async {
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-abc',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'peer-b',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/users/profiles/peer-b') {
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': 'peer-b',
                  'account_id': 'a-b',
                  'username': 'peer',
                  'discriminator': '0001',
                  'display_name': 'Peer User',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
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

    expect(find.text('Peer User'), findsOneWidget);
  });

  testWidgets('ChatListPanel shows peer online indicator when presence known', (
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
              if (req.url.path == '/api/v1/chats') {
                return http.Response(
                  jsonEncode({
                    'chat_list': {
                      'items': [
                        {
                          'chat': {
                            'id': 'chat-abc',
                            'type': 'CHAT_TYPE_DM',
                            'creator_profile_id': 'profile-a',
                          },
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/users/profiles/peer-b') {
                return http.Response(
                  jsonEncode({
                    'profile': {
                      'id': 'peer-b',
                      'account_id': 'a-b',
                      'username': 'peer',
                      'discriminator': '0001',
                      'display_name': 'Peer User',
                      'locale': 'en',
                      'theme': 'dark',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
                  }),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          dmPeerProfileByChatIdProvider.overrideWith(
            (ref) => {'chat-abc': 'peer-b'},
          ),
          presenceProvider.overrideWith((ref, id) {
            if (id == 'peer-b') {
              return const VoicePresence(profileId: 'peer-b', status: 'online');
            }
            return null;
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatListPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatListPanel.presenceIndicatorKey('chat-abc')),
      findsOneWidget,
    );
    final dot = tester.widget<PresenceIndicator>(
      find.byKey(ChatListPanel.presenceIndicatorKey('chat-abc')),
    );
    expect(dot.presence?.isOnline, isTrue);
  });

  testWidgets('selecting chat shows room and loads messages', (tester) async {
    var markReadCalls = 0;
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
              if (req.url.path == '/api/v1/messages') {
                return http.Response(
                  jsonEncode({
                    'message_list': {
                      'messages': [
                        {
                          'id': 'msg-2',
                          'chat': {'id': 'chat-abc'},
                          'sender_profile_id': 'profile-b',
                          'content': 'Newest first from API',
                          'created_at': '2024-01-01T00:00:01Z',
                        },
                        {
                          'id': 'msg-1',
                          'chat': {'id': 'chat-abc'},
                          'sender_profile_id': 'profile-b',
                          'content': 'Hello there',
                          'created_at': '2024-01-01T00:00:00Z',
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/messages/read') {
                markReadCalls++;
                expect(req.method, 'POST');
                final body = jsonDecode(req.body) as Map<String, dynamic>;
                expect(body['chat'], {'id': 'chat-abc'});
                expect(body['last_read_message_id'], 'msg-2');
                return http.Response('{}', 200);
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.panelKey), findsOneWidget);
    expect(find.text('Hello there'), findsOneWidget);
    expect(find.text('Newest first from API'), findsOneWidget);
    expect(markReadCalls, 1);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
