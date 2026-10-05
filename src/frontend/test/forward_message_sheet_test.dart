import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
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
    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-target')),
      findsOneWidget,
    );
    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-source')),
      findsNothing,
    );

    await tester.tap(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-target')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.commentFieldKey), findsOneWidget);
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pumpAndSettle();

    expect(forwardCalled, isTrue);
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsNothing);
  });

  testWidgets('forward failure hides upstream details and keeps the sheet', (
    tester,
  ) async {
    const raw = 'internal forward trace=forward-secret';
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
                        'id': 'chat-target',
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'profile-test',
                        'name': 'Target group',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/forward') {
            return http.Response(
              jsonEncode({'error': 'internal_error', 'message': raw}),
              500,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('Open forward'));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-target')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.sheetKey), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining('forward-secret'), findsNothing);
  });

  testWidgets('ForwardMessageSheet forwards a multi-select batch (FW-05)', (
    tester,
  ) async {
    final forwardedIds = <String>[];
    final commentaries = <String?>[];
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => FilledButton(
            onPressed: () => ForwardMessageSheet.show(
              context,
              sourceMessages: const [
                VoiceMessage(
                  id: 'msg-src-a',
                  chatId: 'chat-source',
                  senderProfileId: 'profile-b',
                  content: 'fw-05-a',
                ),
                VoiceMessage(
                  id: 'msg-src-b',
                  chatId: 'chat-source',
                  senderProfileId: 'profile-b',
                  content: 'fw-05-b',
                ),
              ],
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
            final body = jsonDecode(req.body) as Map<String, dynamic>;
            forwardedIds.add(body['source_message_id'] as String);
            commentaries.add(body['commentary'] as String?);
            final srcId = body['source_message_id'] as String;
            return http.Response(
              jsonEncode({
                'message': {
                  'id': 'msg-fwd-$srcId',
                  'chat': {'id': 'chat-target'},
                  'sender_profile_id': 'profile-test',
                  'content': srcId == 'msg-src-a' ? 'fw-05-a' : 'fw-05-b',
                  'type': 'forward',
                  'message_kind': 'MESSAGE_KIND_FORWARD',
                  'forward_from_id': srcId,
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

    await tester.tap(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-target')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.commentFieldKey), findsOneWidget);
    await tester.enterText(
      find.byKey(ForwardMessageSheet.commentFieldKey),
      'batch note',
    );
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pumpAndSettle();

    expect(forwardedIds, ['msg-src-a', 'msg-src-b']);
    expect(commentaries, ['batch note', null]);
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsNothing);
  });

  testWidgets('ForwardMessageSheet filters chats by search', (tester) async {
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
    await tester.tap(find.text('Open forward'));
    await tester.pumpAndSettle();

    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-a')),
      findsOneWidget,
    );
    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-b')),
      findsOneWidget,
    );

    await tester.enterText(
      find.byKey(ForwardMessageSheet.searchFieldKey),
      'beta',
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ForwardMessageSheet.chatTileKey('chat-a')), findsNothing);
    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-b')),
      findsOneWidget,
    );
  });

  testWidgets('ForwardMessageSheet searches accepted friends on later pages', (
    tester,
  ) async {
    var friendPageReads = 0;
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
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            if (profileId == 'friend-first') {
              return const VoiceProfile(
                id: 'friend-first',
                accountId: 'acc-first',
                username: 'firstfriend',
                discriminator: '1010',
                displayName: 'First Friend',
              );
            }
            expect(profileId, 'friend-late');
            return const VoiceProfile(
              id: 'friend-late',
              accountId: 'acc-friend',
              username: 'latefriend',
              discriminator: '4242',
              displayName: 'Late Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            friendPageReads++;
            final cursor = req.url.queryParameters['cursor'];
            if (cursor == null) {
              return http.Response(
                jsonEncode({
                  'friend_list': {
                    'friends': [
                      {'profile_id': 'friend-first'},
                    ],
                    'next_cursor': 'page-2',
                  },
                }),
                200,
              );
            }
            expect(cursor, 'page-2');
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-late'},
                  ],
                  'next_cursor': '',
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

    expect(friendPageReads, 2);
    await tester.enterText(
      find.byKey(ForwardMessageSheet.searchFieldKey),
      'lAtE fRiEnD',
    );
    await tester.pumpAndSettle();

    expect(find.text('Late Friend'), findsOneWidget);
  });

  testWidgets('contact is created and forwarded only after explicit submit', (
    tester,
  ) async {
    final events = <String>[];
    final forwardBodies = <Map<String, dynamic>>[];
    final createDmResponse = Completer<http.Response>();
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
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            expect(profileId, 'friend-target');
            return const VoiceProfile(
              id: 'friend-target',
              accountId: 'acc-friend',
              username: 'targetfriend',
              discriminator: '0242',
              displayName: 'Target Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-target'},
                  ],
                  'next_cursor': '',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/dm') {
            events.add('create-dm');
            expect(jsonDecode(req.body)['other_profile_id'], 'friend-target');
            return createDmResponse.future;
          }
          if (req.url.path == '/api/v1/messages/forward') {
            events.add('forward');
            forwardBodies.add(jsonDecode(req.body) as Map<String, dynamic>);
            return http.Response(
              jsonEncode({
                'message': {
                  'id': 'msg-forwarded',
                  'chat': {'id': 'chat-friend'},
                  'sender_profile_id': 'profile-test',
                  'content': 'Forward me',
                  'type': 'forward',
                  'message_kind': 'MESSAGE_KIND_FORWARD',
                  'forward_from_id': 'msg-src',
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

    final contactKey = ForwardMessageSheet.contactTileKey('friend-target');
    expect(find.byKey(contactKey), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.closeButtonKey), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.cancelButtonKey), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.submitButtonKey), findsNothing);
    await tester.tap(find.byKey(contactKey));
    await tester.pumpAndSettle();
    expect(events, isEmpty);
    expect(find.byType(AlertDialog), findsNothing);
    expect(find.byKey(ForwardMessageSheet.commentFieldKey), findsOneWidget);
    expect(find.byKey(ForwardMessageSheet.submitButtonKey), findsOneWidget);
    await tester.tap(find.byKey(ForwardMessageSheet.cancelButtonKey));
    await tester.pumpAndSettle();
    expect(events, isEmpty);

    await tester.tap(find.text('Open forward'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(contactKey));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(ForwardMessageSheet.commentFieldKey),
      '  hello friend  ',
    );
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pump();
    expect(events, ['create-dm']);
    expect(
      tester
          .widget<FilledButton>(find.byKey(ForwardMessageSheet.submitButtonKey))
          .onPressed,
      isNull,
    );
    await tester.tap(
      find.byKey(ForwardMessageSheet.submitButtonKey),
      warnIfMissed: false,
    );
    expect(events, ['create-dm']);
    createDmResponse.complete(
      http.Response(
        jsonEncode({
          'chat': {
            'id': 'chat-friend',
            'type': 'CHAT_TYPE_DM',
            'creator_profile_id': 'profile-test',
          },
        }),
        200,
      ),
    );
    await tester.pumpAndSettle();

    expect(events, ['create-dm', 'forward']);
    expect(forwardBodies.single['target_chat'], {'id': 'chat-friend'});
    expect(forwardBodies.single['commentary'], 'hello friend');
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsNothing);
  });

  testWidgets('profile switch while confirming contact prevents creation', (
    tester,
  ) async {
    final events = <String>[];
    await tester.pumpWidget(
      testApp(
        home: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceChatId: 'chat-source',
        ),
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            return VoiceProfile(
              id: profileId,
              accountId: 'acc-friend',
              username: 'friend',
              discriminator: '1010',
              displayName: 'Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-target'},
                  ],
                  'next_cursor': '',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/dm' ||
              req.url.path == '/api/v1/messages/forward') {
            events.add(req.url.path);
          }
          return http.Response('{}', 500);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
    );
    await tester.pumpAndSettle();
    final container = ProviderScope.containerOf(
      tester.element(find.byKey(ForwardMessageSheet.sheetKey)),
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'different-token',
        refreshToken: 'different-refresh',
        accountId: 'acc-test',
        activeProfileId: 'different-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pumpAndSettle();

    expect(events, isEmpty);
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsOneWidget);
  });

  testWidgets('profile switch during a pending batch forward stops the batch', (
    tester,
  ) async {
    final forwardResponse = Completer<http.Response>();
    final forwardRequests = <String>[];
    var chatListRequests = 0;
    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => FilledButton(
            onPressed: () => ForwardMessageSheet.show(
              context,
              sourceMessages: const [
                sourceMessage,
                VoiceMessage(
                  id: 'msg-src-second',
                  chatId: 'chat-source',
                  senderProfileId: 'profile-b',
                  content: 'Forward me second',
                ),
              ],
              sourceChatId: 'chat-source',
            ),
            child: const Text('Open forward'),
          ),
        ),
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            return const VoiceProfile(
              id: 'friend-target',
              accountId: 'acc-friend',
              username: 'friend',
              discriminator: '1010',
              displayName: 'Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            chatListRequests++;
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-target'},
                  ],
                  'next_cursor': '',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/dm') {
            return http.Response(
              jsonEncode({
                'chat': {
                  'id': 'chat-friend',
                  'type': 'CHAT_TYPE_DM',
                  'creator_profile_id': 'profile-test',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/messages/forward') {
            forwardRequests.add(req.headers['authorization'] ?? 'missing-auth');
            if (forwardRequests.length == 1) return forwardResponse.future;
            return http.Response(
              jsonEncode({
                'message': {
                  'id': 'msg-forwarded-second',
                  'chat': {'id': 'chat-friend'},
                  'sender_profile_id': 'different-profile',
                  'content': 'Forward me second',
                  'type': 'forward',
                  'message_kind': 'MESSAGE_KIND_FORWARD',
                  'forward_from_id': 'msg-src-second',
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
    await tester.tap(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pump();
    expect(forwardRequests, hasLength(1));

    final container = ProviderScope.containerOf(
      tester.element(find.byKey(ForwardMessageSheet.sheetKey)),
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'different-token',
        refreshToken: 'different-refresh',
        accountId: 'acc-test',
        activeProfileId: 'different-profile',
        expiresInSeconds: 900,
      ),
    );
    container.read(selectedChatIdProvider.notifier).state = 'new-profile-chat';
    await tester.pump(const Duration(milliseconds: 250));
    final chatListRequestsBeforeCompletion = chatListRequests;
    forwardResponse.complete(
      http.Response(
        jsonEncode({
          'message': {
            'id': 'msg-forwarded-first',
            'chat': {'id': 'chat-friend'},
            'sender_profile_id': 'profile-test',
            'content': 'Forward me',
            'type': 'forward',
            'message_kind': 'MESSAGE_KIND_FORWARD',
            'forward_from_id': 'msg-src',
          },
        }),
        200,
      ),
    );
    await tester.pumpAndSettle();

    expect(forwardRequests, hasLength(1));
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsOneWidget);
    expect(container.read(selectedChatIdProvider), 'new-profile-chat');
    expect(chatListRequests, chatListRequestsBeforeCompletion);
  });

  testWidgets('existing DM is shown once and searchable by friend profile', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        home: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceChatId: 'chat-source',
        ),
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            return VoiceProfile(
              id: profileId,
              accountId: 'acc-friend',
              username: 'targetfriend',
              discriminator: '0242',
              displayName: 'Target Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'chat-friend',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-test',
                      },
                      'dm_peer_profile_id': 'friend-target',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-target'},
                  ],
                  'next_cursor': '',
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

    expect(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
      findsNothing,
    );
    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-friend')),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(ForwardMessageSheet.searchFieldKey),
      'TARGETFRIEND',
    );
    await tester.pumpAndSettle();
    expect(
      find.byKey(ForwardMessageSheet.chatTileKey('chat-friend')),
      findsOneWidget,
    );
    expect(find.text('Target Friend'), findsOneWidget);
  });

  testWidgets('DM creation failure never attempts a forward', (tester) async {
    final events = <String>[];
    await tester.pumpWidget(
      testApp(
        home: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceChatId: 'chat-source',
        ),
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            return VoiceProfile(
              id: profileId,
              accountId: 'acc-friend',
              username: 'friend',
              discriminator: '1010',
              displayName: 'Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-target'},
                  ],
                  'next_cursor': '',
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/dm') {
            events.add('create-dm');
            return http.Response(jsonEncode({'error': 'unavailable'}), 503);
          }
          if (req.url.path == '/api/v1/messages/forward') {
            events.add('forward');
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ForwardMessageSheet.submitButtonKey));
    await tester.pumpAndSettle();

    expect(events, ['create-dm']);
    expect(find.byKey(ForwardMessageSheet.sheetKey), findsOneWidget);
    expect(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
      findsOneWidget,
    );
  });

  testWidgets('failed accepted-friend load can be retried', (tester) async {
    var friendPageReads = 0;
    await tester.pumpWidget(
      testApp(
        home: ForwardMessageSheet(
          sourceMessage: sourceMessage,
          sourceChatId: 'chat-source',
        ),
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            return const VoiceProfile(
              id: 'friend-target',
              accountId: 'acc-friend',
              username: 'targetfriend',
              discriminator: '0242',
              displayName: 'Target Friend',
            );
          }),
        ],
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/friends') {
            friendPageReads++;
            if (friendPageReads == 1) {
              return http.Response(jsonEncode({'error': 'unavailable'}), 503);
            }
            return http.Response(
              jsonEncode({
                'friend_list': {
                  'friends': [
                    {'profile_id': 'friend-target'},
                  ],
                  'next_cursor': '',
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

    expect(find.text('Could not load friends'), findsOneWidget);
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(friendPageReads, 2);
    expect(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
      findsOneWidget,
    );
  });

  testWidgets('captures H and V recipient-list layouts without overflow', (
    tester,
  ) async {
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/chats') {
        return http.Response(
          jsonEncode({
            'chat_list': {'items': []},
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/friends') {
        return http.Response(
          jsonEncode({
            'friend_list': {
              'friends': [
                {'profile_id': 'friend-target'},
              ],
              'next_cursor': '',
            },
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
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
        extraOverrides: [
          profileProvider.overrideWith((ref, profileId) async {
            return VoiceProfile(
              id: profileId,
              accountId: 'acc-friend',
              username: 'targetfriend',
              discriminator: '0242',
              displayName: 'Target Friend',
            );
          }),
        ],
        client: client,
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Open forward'));
    await tester.pump();
    for (
      var attempt = 0;
      attempt < 20 &&
          find
              .byKey(ForwardMessageSheet.contactTileKey('friend-target'))
              .evaluate()
              .isEmpty;
      attempt++
    ) {
      await tester.pump(const Duration(milliseconds: 100));
    }
    expect(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);

    tester.view.physicalSize = const Size(390, 844);
    tester.binding.handleMetricsChanged();
    await tester.pump(const Duration(milliseconds: 500));
    expect(tester.takeException(), isNull);
    expect(
      find.byKey(ForwardMessageSheet.contactTileKey('friend-target')),
      findsOneWidget,
    );
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
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
