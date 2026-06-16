import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_list_panel.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/markdown_message_content.dart';
import 'package:voice_frontend/ui/social/presence_indicator.dart';

import 'support/auth_test_overrides.dart';
import 'support/markdown_test_helpers.dart';
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

  testWidgets(
    'ChatListPanel bumps unread badge when notification arrives for another chat',
    (tester) async {
      late _CapturingRealtimeHub hub;
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
                              'id': 'chat-quiet',
                              'type': 'CHAT_TYPE_DM',
                              'creator_profile_id': 'profile-a',
                            },
                            'last_message_preview': 'Quiet chat',
                            'unread_count': 0,
                          },
                          {
                            'chat': {
                              'id': 'chat-open',
                              'type': 'CHAT_TYPE_DM',
                              'creator_profile_id': 'profile-b',
                            },
                            'last_message_preview': 'Open chat',
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
            ),
            realtimeAutoConnectProvider.overrideWithValue(false),
            realtimeHubProvider.overrideWith((ref) => hub = _CapturingRealtimeHub(ref)),
            selectedChatIdProvider.overrideWith((ref) => 'chat-open'),
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

      expect(find.text('0'), findsNothing);

      hub.emit(
        const RealtimeFrame(
          op: 'notification',
          data: {
            'type': 'new_message',
            'chat_id': 'chat-quiet',
            'message_id': 'msg-live',
            'sender_profile_id': 'profile-a',
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(ChatListPanel.tileKey('chat-quiet')), findsOneWidget);
      expect(find.text('1'), findsOneWidget);
    },
  );

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

  testWidgets('ChatListPanel retries after a recoverable load error', (
    tester,
  ) async {
    var calls = 0;
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            calls++;
            if (calls == 1) {
              return http.Response('temporary failure', 500);
            }
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-after-retry',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-a',
                      },
                      'last_message_preview': 'Loaded after retry',
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

    expect(find.text('Try again'), findsOneWidget);
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ChatListPanel.tileKey('chat-after-retry')),
      findsOneWidget,
    );
    expect(find.text('Loaded after retry'), findsOneWidget);
  });

  testWidgets('ChatListPanel uses a labeled load more control', (tester) async {
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatListPanel(),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats' &&
              req.url.queryParameters['cursor'] == 'cursor-2') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-2',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-a',
                      },
                      'last_message_preview': 'Older preview',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'next_cursor': 'cursor-2',
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-1',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-a',
                      },
                      'last_message_preview': 'First preview',
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

    expect(find.text('Load more chats'), findsOneWidget);
    await tester.tap(find.text('Load more chats'));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListPanel.tileKey('chat-2')), findsOneWidget);
  });

  testWidgets('ChatListPanel resolves DM peer when caller is creator', (
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
                        'creator_profile_id': 'profile-a',
                      },
                      'dm_peer_profile_id': 'peer-b',
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
                          'attachments_json': jsonEncode([
                            {
                              'file_id': 'file-image',
                              'type': 'image',
                              'preview_url': 'https://cdn.example/thumb.webp',
                              'name': 'cat.png',
                            },
                            {
                              'file_id': 'file-doc',
                              'type': 'document',
                              'name': 'report.pdf',
                              'size_bytes': 2048,
                            },
                          ]),
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
    expectMessagePlainText(tester, 'Hello there');
    expectMessagePlainText(tester, 'Newest first from API');
    expect(
      find.byKey(ChatRoomPanel.attachmentPreviewKey('file-image')),
      findsOneWidget,
    );
    expect(find.text('report.pdf'), findsOneWidget);
    expect(find.text('2.0 KB'), findsOneWidget);
    expect(markReadCalls, 1);
  });

  testWidgets('ChatRoomPanel refocuses composer after sending a message', (
    tester,
  ) async {
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatRoomPanel(chatId: 'chat-abc'),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {'messages': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/send') {
            return http.Response(
              jsonEncode({
                'message': {
                  'id': 'msg-new',
                  'chat': {'id': 'chat-abc'},
                  'sender_profile_id': 'profile-test',
                  'content': 'Hello',
                  'created_at': '2024-01-01T00:00:00Z',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(ChatRoomPanel.inputKey), 'Hello');
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();

    final input = tester.widget<TextField>(find.byKey(ChatRoomPanel.inputKey));
    expect(input.focusNode?.hasFocus, isTrue);
    expect(input.controller?.text, isEmpty);
  });

  testWidgets('ChatRoomPanel shows a mobile back action when provided', (
    tester,
  ) async {
    var backTapped = false;
    await tester.pumpWidget(
      chatTestApp(
        home: ChatRoomPanel(
          chatId: 'chat-abc',
          onBack: () => backTapped = true,
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {'messages': []},
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byTooltip('Back to chats'), findsOneWidget);
    await tester.tap(find.byTooltip('Back to chats'));

    expect(backTapped, isTrue);
  });

  testWidgets('ChatRoomPanel uploads and sends an attachment', (tester) async {
    var sentAttachment = false;
    await tester.pumpWidget(
      chatTestApp(
        home: ChatRoomPanel(
          chatId: 'chat-abc',
          attachmentPicker: () async => ChatAttachmentFile(
            bytes: Uint8List.fromList([1, 2, 3]),
            contentType: 'application/pdf',
            name: 'report.pdf',
          ),
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {'messages': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/files/upload') {
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            expect(body['context_chat'], {
              'id': 'chat-abc',
              'type': 'CHAT_TYPE_DM',
            });
            return http.Response(
              jsonEncode({
                'upload_response': {
                  'file_id': 'file-doc',
                  'presigned_put_url': 'https://r2.example/upload',
                  'r2_key': 'attachments/file-doc/report.pdf',
                },
              }),
              200,
            );
          }
          if (req.method == 'PUT' &&
              req.url.toString() == 'https://r2.example/upload') {
            expect(req.bodyBytes, [1, 2, 3]);
            return http.Response('', 200);
          }
          if (req.url.path == '/api/v1/files/file-doc/confirm') {
            return http.Response(
              jsonEncode({
                'file_metadata': {
                  'id': 'file-doc',
                  'file_type': 'document',
                  'status': 'ready',
                  'original_name': 'report.pdf',
                  'r2_key': 'attachments/file-doc/report.pdf',
                  'size_bytes': 3,
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/send') {
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            final attachments =
                jsonDecode(body['attachments_json'] as String) as List<dynamic>;
            expect(attachments.single, containsPair('file_id', 'file-doc'));
            sentAttachment = true;
            return http.Response(
              jsonEncode({
                'message': {
                  'id': 'msg-file',
                  'chat': {'id': 'chat-abc'},
                  'sender_profile_id': 'profile-test',
                  'content': '',
                  'attachments_json': body['attachments_json'],
                  'created_at': '2024-01-01T00:00:00Z',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ChatRoomPanel.attachKey));
    await tester.pumpAndSettle();

    expect(sentAttachment, isTrue);
    expect(find.text('report.pdf'), findsOneWidget);
    expect(
      find.byKey(ChatRoomPanel.attachmentPreviewKey('file-doc')),
      findsOneWidget,
    );
  });

  testWidgets('ChatRoomPanel shows call actions from chat list peer metadata', (
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
            const GatewayConfig(
              baseUrl: 'http://api.test',
              livekitUrl: 'wss://livekit.test',
            ),
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
                          'dm_peer_profile_id': 'peer-b',
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/messages') {
                return http.Response(
                  jsonEncode({
                    'message_list': {'messages': []},
                  }),
                  200,
                );
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

    expect(find.byKey(ChatRoomPanel.audioCallKey), findsOneWidget);
    expect(find.byKey(ChatRoomPanel.videoCallKey), findsOneWidget);
  });

  testWidgets(
    'ChatRoomPanel infers call peer from messages when list omits peer id',
    (tester) async {
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
              const GatewayConfig(
                baseUrl: 'http://api.test',
                livekitUrl: 'wss://livekit.test',
              ),
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
                              'creator_profile_id': 'prof-test',
                            },
                          },
                        ],
                      },
                    }),
                    200,
                  );
                }
                if (req.url.path == '/api/v1/messages') {
                  return http.Response(
                    jsonEncode({
                      'message_list': {
                        'messages': [
                          {
                            'id': 'msg-1',
                            'chat': {'id': 'chat-abc'},
                            'sender_profile_id': 'peer-b',
                            'content': 'Hi',
                            'created_at': '2024-01-01T00:00:00Z',
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

      expect(find.byKey(ChatRoomPanel.audioCallKey), findsOneWidget);
      expect(find.byKey(ChatRoomPanel.videoCallKey), findsOneWidget);
    },
  );

  testWidgets('ChatRoomPanel shows forward attribution on forwarded messages', (
    tester,
  ) async {
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatRoomPanel(chatId: 'chat-abc'),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-fwd',
                      'chat': {'id': 'chat-abc'},
                      'sender_profile_id': 'profile-test',
                      'content': 'Original text',
                      'type': 'forward',
                      'message_kind': 'MESSAGE_KIND_FORWARD',
                      'forward_from_id': 'msg-src',
                      'forward_from_sender': 'Alice',
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Forwarded from Alice'), findsOneWidget);
    expectMessagePlainText(tester, 'Original text');
  });

  testWidgets('ChatRoomPanel appends message after WS message_create', (
    tester,
  ) async {
    late _CapturingRealtimeHub hub;
    var messageFetches = 0;
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
                messageFetches++;
                final messages = messageFetches == 1
                    ? [
                        {
                          'id': 'msg-1',
                          'chat': {'id': 'chat-abc'},
                          'sender_profile_id': 'profile-b',
                          'content': 'Hello there',
                          'created_at': '2024-01-01T00:00:00Z',
                        },
                      ]
                    : [
                        {
                          'id': 'msg-1',
                          'chat': {'id': 'chat-abc'},
                          'sender_profile_id': 'profile-b',
                          'content': 'Hello there',
                          'created_at': '2024-01-01T00:00:00Z',
                        },
                        {
                          'id': 'msg-2',
                          'chat': {'id': 'chat-abc'},
                          'sender_profile_id': 'profile-b',
                          'content': 'Live from WS',
                          'created_at': '2024-01-01T00:00:01Z',
                        },
                      ];
                return http.Response(
                  jsonEncode({'message_list': {'messages': messages}}),
                  200,
                );
              }
              if (req.url.path == '/api/v1/messages/read') {
                return http.Response('{}', 200);
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeHubProvider.overrideWith((ref) => hub = _CapturingRealtimeHub(ref)),
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
    expectMessagePlainText(tester, 'Hello there');
    expect(find.text('Live from WS'), findsNothing);

    hub.emit(
      const RealtimeFrame(
        op: 'message_create',
        data: {
          'chat_id': 'chat-abc',
          'message_id': 'msg-2',
          'sender_profile_id': 'profile-b',
        },
        sequence: 2,
      ),
    );
    await tester.pumpAndSettle();

    expectMessagePlainText(tester, 'Live from WS');
    expect(messageFetches, greaterThan(1));
  });

  testWidgets('ChatRoomPanel shows typing label on typing WS frame', (
    tester,
  ) async {
    late _CapturingRealtimeHub hub;
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
                  jsonEncode({'message_list': {'messages': []}}),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeHubProvider.overrideWith((ref) => hub = _CapturingRealtimeHub(ref)),
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
    expect(find.text('Typing…'), findsNothing);

    hub.emit(
      const RealtimeFrame(
        op: 'typing',
        data: {
          'chat_id': 'chat-abc',
          'profile_id': 'profile-b',
          'kind': 'start',
        },
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Typing…'), findsOneWidget);
  });

  testWidgets('ChatListPanel shows accept and decline in requests inbox', (
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
                final inbox = req.url.queryParameters['inbox'] ?? 'main';
                if (inbox == 'requests') {
                  return http.Response(
                    jsonEncode({
                      'chat_list': {
                        'items': [
                          {
                            'chat': {
                              'id': 'chat-req',
                              'type': 'CHAT_TYPE_DM',
                              'creator_profile_id': 'profile-stranger',
                            },
                            'last_message_preview': 'Hi',
                            'unread_count': 0,
                          },
                        ],
                      },
                    }),
                    200,
                  );
                }
                return http.Response(
                  jsonEncode({'chat_list': {'items': []}}),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatInboxProvider.overrideWith((ref) => 'requests'),
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

    expect(find.text('Accept'), findsOneWidget);
    expect(find.text('Decline'), findsOneWidget);
  });

  testWidgets('ChatRoomPanel renders markdown message content', (tester) async {
    await tester.pumpWidget(
      chatTestApp(
        home: const ChatRoomPanel(chatId: 'chat-abc'),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {
                  'messages': [
                    {
                      'id': 'msg-md',
                      'chat': {'id': 'chat-abc'},
                      'sender_profile_id': 'profile-b',
                      'content': '**bold** text',
                      'created_at': '2024-01-01T00:00:00Z',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/read') {
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(MarkdownMessageContent), findsOneWidget);
    final rich = tester.widgetList<RichText>(find.byType(RichText));
    final plain = rich.map((r) => r.text.toPlainText()).join();
    expect(plain, contains('bold'));
    expect(plain, isNot(contains('**')));
  });

  testWidgets('ChatRoomPanel shows group name without DM call actions', (
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
            const GatewayConfig(
              baseUrl: 'http://api.test',
              livekitUrl: 'wss://livekit.test',
            ),
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
                            'id': 'group-1',
                            'type': 'CHAT_TYPE_GROUP',
                            'creator_profile_id': 'owner',
                            'name': 'Squad Chat',
                          },
                        },
                      ],
                    },
                  }),
                  200,
                );
              }
              if (req.url.path == '/api/v1/messages') {
                return http.Response(
                  jsonEncode({
                    'message_list': {
                      'messages': [
                        {
                          'id': 'm1',
                          'chat': {'id': 'group-1'},
                          'sender_profile_id': 'peer-b',
                          'content': 'hello team',
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
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'group-1'),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'group-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Squad Chat'), findsOneWidget);
    expect(find.text('peer-b'), findsOneWidget);
    expect(find.byKey(ChatRoomPanel.audioCallKey), findsNothing);
    expect(find.byKey(ChatRoomPanel.videoCallKey), findsNothing);
    expect(find.byKey(ChatRoomPanel.groupMembersKey), findsOneWidget);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

class _CapturingRealtimeHub extends RealtimeHub {
  _CapturingRealtimeHub(super.ref);

  final _events = StreamController<RealtimeFrame>.broadcast();

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  void emit(RealtimeFrame frame) => _events.add(frame);

  @override
  Future<void> dispose() async {
    await _events.close();
  }
}
