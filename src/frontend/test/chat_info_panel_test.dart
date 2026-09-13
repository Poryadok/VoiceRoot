import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget testApp({required Widget home, required http.Client client}) {
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
        home: Scaffold(body: home),
      ),
    );
  }

  testWidgets('chat info panel shows shared media tabs and empty state', (
    tester,
  ) async {
    const chatId = 'chat-shared-media-1';

    await tester.pumpWidget(
      testApp(
        home: SizedBox(
          height: 500,
          width: 400,
          child: ChatInfoPanel(chatId: chatId),
        ),
        client: MockClient((req) async {
          if (req.url.path.contains('/shared-media')) {
            return http.Response(
              jsonEncode({
                'shared_media_list': {
                  'items': [],
                  'next_cursor': '',
                  'has_more': false,
                },
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );

    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.mediaTabKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.filesTabKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.linksTabKey), findsOneWidget);
    expect(find.byKey(ChatInfoPanel.voiceTabKey), findsOneWidget);
    expect(find.text('Nothing here yet'), findsOneWidget);
  });

  testWidgets('shared media loading uses the canonical skeleton', (
    tester,
  ) async {
    final pending = Completer<http.Response>();

    await tester.pumpWidget(
      testApp(
        home: SizedBox(
          height: 500,
          width: 400,
          child: ChatInfoPanel(chatId: 'chat-shared-media-loading'),
        ),
        client: MockClient((_) => pending.future),
      ),
    );
    await tester.pump();

    expect(find.byType(VoiceListSkeleton), findsWidgets);
    expect(find.byType(CircularProgressIndicator), findsNothing);

    pending.complete(http.Response('{}', 200));
  });

  testWidgets(
    'standalone group owner can update guest admission and refreshes',
    (tester) async {
      var updateCalls = 0;
      var listCalls = 0;
      final client = MockClient((req) async {
        if (req.url.path == '/api/v1/chats') {
          listCalls++;
          return http.Response(
            jsonEncode({
              'chat_list': {
                'items': [
                  {
                    'chat': {
                      'id': 'standalone-group',
                      'type': 'CHAT_TYPE_GROUP',
                      'creator_profile_id': 'prof-test',
                      // The reload is authoritative: another admin may have
                      // changed the setting while this update was in flight.
                      'allow_guests': false,
                    },
                  },
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path == '/api/v1/chats/standalone-group/members') {
          return http.Response(
            jsonEncode({
              'member_list': {
                'members': [
                  {'profile_id': 'prof-test', 'role': 'owner'},
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path == '/api/v1/chats/standalone-group' &&
            req.method == 'PATCH') {
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['allow_guests'], true);
          updateCalls++;
          return http.Response(
            jsonEncode({
              'chat': {
                'id': 'standalone-group',
                'type': 'CHAT_TYPE_GROUP',
                'creator_profile_id': 'prof-test',
                'allow_guests': true,
              },
            }),
            200,
          );
        }
        if (req.url.path.contains('/shared-media')) {
          return http.Response(
            jsonEncode({
              'shared_media_list': {'items': []},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        testApp(
          home: const SizedBox(
            height: 700,
            width: 400,
            child: ChatInfoPanel(chatId: 'standalone-group'),
          ),
          client: client,
        ),
      );
      await tester.pumpAndSettle();

      expect(
        find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
        findsOneWidget,
      );
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isFalse,
      );

      await tester.tap(
        find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      );
      await tester.pumpAndSettle();

      expect(updateCalls, 1);
      expect(listCalls, greaterThanOrEqualTo(2));
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isFalse,
      );
    },
  );

  testWidgets('guest admission control stays hidden for a regular member', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        home: const SizedBox(
          height: 700,
          width: 400,
          child: ChatInfoPanel(chatId: 'member-group'),
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'member-group',
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'other-profile',
                        'allow_guests': false,
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/member-group/members') {
            return http.Response(
              jsonEncode({
                'member_list': {
                  'members': [
                    {'profile_id': 'prof-test', 'role': 'member'},
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path.contains('/shared-media')) {
            return http.Response(
              jsonEncode({
                'shared_media_list': {'items': []},
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
      find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      findsNothing,
    );
  });

  testWidgets('a failed guest admission update keeps the authoritative value', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        home: const SizedBox(
          height: 700,
          width: 400,
          child: ChatInfoPanel(chatId: 'failed-update-group'),
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'failed-update-group',
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'prof-test',
                        'allow_guests': true,
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/failed-update-group/members') {
            return http.Response(
              jsonEncode({
                'member_list': {
                  'members': [
                    {'profile_id': 'prof-test', 'role': 'owner'},
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path == '/api/v1/chats/failed-update-group') {
            return http.Response(
              jsonEncode({
                'error': 'permission_denied',
                'message': 'forbidden',
              }),
              403,
            );
          }
          if (req.url.path.contains('/shared-media')) {
            return http.Response(
              jsonEncode({
                'shared_media_list': {'items': []},
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(StandaloneChatGuestSettingsSection.toggleKey));
    await tester.pumpAndSettle();

    expect(
      tester
          .widget<SwitchListTile>(
            find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
          )
          .value,
      isTrue,
    );
    expect(find.text('forbidden'), findsOneWidget);
  });

  testWidgets(
    'a failed refresh retains the successful guest admission update',
    (tester) async {
      var listCalls = 0;
      await tester.pumpWidget(
        testApp(
          home: const SizedBox(
            height: 700,
            width: 400,
            child: ChatInfoPanel(chatId: 'refresh-failure-group'),
          ),
          client: MockClient((req) async {
            if (req.url.path == '/api/v1/chats') {
              listCalls++;
              if (listCalls > 1) {
                return http.Response(
                  jsonEncode({
                    'error': 'unavailable',
                    'message': 'reload failed',
                  }),
                  503,
                );
              }
              return http.Response(
                jsonEncode({
                  'chat_list': {
                    'items': [
                      {
                        'chat': {
                          'id': 'refresh-failure-group',
                          'type': 'CHAT_TYPE_GROUP',
                          'creator_profile_id': 'prof-test',
                          'allow_guests': false,
                        },
                      },
                    ],
                  },
                }),
                200,
              );
            }
            if (req.url.path == '/api/v1/chats/refresh-failure-group/members') {
              return http.Response(
                jsonEncode({
                  'member_list': {
                    'members': [
                      {'profile_id': 'prof-test', 'role': 'owner'},
                    ],
                  },
                }),
                200,
              );
            }
            if (req.url.path == '/api/v1/chats/refresh-failure-group') {
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': 'refresh-failure-group',
                    'type': 'CHAT_TYPE_GROUP',
                    'creator_profile_id': 'prof-test',
                    'allow_guests': true,
                  },
                }),
                200,
              );
            }
            if (req.url.path.contains('/shared-media')) {
              return http.Response(
                jsonEncode({
                  'shared_media_list': {'items': []},
                }),
                200,
              );
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(
        find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      );
      await tester.pumpAndSettle();

      expect(listCalls, greaterThanOrEqualTo(2));
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isTrue,
      );
    },
  );

  testWidgets('channel admins see the control but Space channels do not', (
    tester,
  ) async {
    const standaloneChannel = 'standalone-channel';
    const spaceChannel = 'space-channel';
    var selectedChatId = standaloneChannel;
    late StateSetter selectChat;
    await tester.pumpWidget(
      testApp(
        home: StatefulBuilder(
          builder: (context, setState) {
            selectChat = setState;
            return SizedBox(
              height: 700,
              width: 400,
              child: ChatInfoPanel(chatId: selectedChatId),
            );
          },
        ),
        client: MockClient((req) async {
          if (req.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': standaloneChannel,
                        'type': 'CHAT_TYPE_CHANNEL',
                        'creator_profile_id': 'other-profile',
                      },
                    },
                    {
                      'chat': {
                        'id': spaceChannel,
                        'type': 'CHAT_TYPE_CHANNEL',
                        'space_id': 'space-1',
                        'creator_profile_id': 'other-profile',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path.endsWith('/members')) {
            return http.Response(
              jsonEncode({
                'member_list': {
                  'members': [
                    {'profile_id': 'prof-test', 'role': 'admin'},
                  ],
                },
              }),
              200,
            );
          }
          if (req.url.path.contains('/shared-media')) {
            return http.Response(
              jsonEncode({
                'shared_media_list': {'items': []},
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
      find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      findsOneWidget,
    );

    selectChat(() => selectedChatId = spaceChannel);
    await tester.pumpAndSettle();
    expect(
      find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      findsNothing,
    );
  });

  testWidgets(
    'a pending update for the previous chat does not change the next chat',
    (tester) async {
      const firstChatId = 'standalone-group-a';
      const secondChatId = 'standalone-group-b';
      final pendingUpdate = Completer<http.Response>();
      var selectedChatId = firstChatId;
      late StateSetter selectChat;
      final client = MockClient((req) async {
        if (req.url.path == '/api/v1/chats') {
          return http.Response(
            jsonEncode({
              'chat_list': {
                'items': [
                  for (final chatId in [firstChatId, secondChatId])
                    {
                      'chat': {
                        'id': chatId,
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'prof-test',
                        'allow_guests': false,
                      },
                    },
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path == '/api/v1/chats/$firstChatId/members' ||
            req.url.path == '/api/v1/chats/$secondChatId/members') {
          return http.Response(
            jsonEncode({
              'member_list': {
                'members': [
                  {'profile_id': 'prof-test', 'role': 'owner'},
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path == '/api/v1/chats/$firstChatId' &&
            req.method == 'PATCH') {
          return pendingUpdate.future;
        }
        if (req.url.path.contains('/shared-media')) {
          return http.Response(
            jsonEncode({
              'shared_media_list': {'items': []},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        testApp(
          home: StatefulBuilder(
            builder: (context, setState) {
              selectChat = setState;
              return SizedBox(
                height: 700,
                width: 400,
                child: ChatInfoPanel(chatId: selectedChatId),
              );
            },
          ),
          client: client,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(
        find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      );
      await tester.pump();

      selectChat(() => selectedChatId = secondChatId);
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isFalse,
      );

      pendingUpdate.complete(
        http.Response(
          jsonEncode({
            'chat': {
              'id': firstChatId,
              'type': 'CHAT_TYPE_GROUP',
              'creator_profile_id': 'prof-test',
              'allow_guests': true,
            },
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();

      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isFalse,
      );
    },
  );

  testWidgets(
    'a completed update cannot use a disposed chat while its refresh is pending',
    (tester) async {
      const firstChatId = 'refresh-pending-group-a';
      const secondChatId = 'refresh-pending-group-b';
      final pendingReload = Completer<http.Response>();
      var listCalls = 0;
      var selectedChatId = firstChatId;
      late StateSetter selectChat;
      final client = MockClient((req) async {
        if (req.url.path == '/api/v1/chats') {
          listCalls++;
          if (listCalls > 1) return pendingReload.future;
          return http.Response(
            jsonEncode({
              'chat_list': {
                'items': [
                  for (final chatId in [firstChatId, secondChatId])
                    {
                      'chat': {
                        'id': chatId,
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': 'prof-test',
                        'allow_guests': false,
                      },
                    },
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path.endsWith('/members')) {
          return http.Response(
            jsonEncode({
              'member_list': {
                'members': [
                  {'profile_id': 'prof-test', 'role': 'owner'},
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path == '/api/v1/chats/$firstChatId' &&
            req.method == 'PATCH') {
          return http.Response(
            jsonEncode({
              'chat': {
                'id': firstChatId,
                'type': 'CHAT_TYPE_GROUP',
                'creator_profile_id': 'prof-test',
                'allow_guests': true,
              },
            }),
            200,
          );
        }
        if (req.url.path.contains('/shared-media')) {
          return http.Response(
            jsonEncode({
              'shared_media_list': {'items': []},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        testApp(
          home: StatefulBuilder(
            builder: (context, setState) {
              selectChat = setState;
              return SizedBox(
                height: 700,
                width: 400,
                child: ChatInfoPanel(chatId: selectedChatId),
              );
            },
          ),
          client: client,
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(
        find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
      );
      await tester.pump();
      expect(listCalls, 2);

      selectChat(() => selectedChatId = secondChatId);
      await tester.pumpAndSettle();
      pendingReload.complete(
        http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                for (final chatId in [firstChatId, secondChatId])
                  {
                    'chat': {
                      'id': chatId,
                      'type': 'CHAT_TYPE_GROUP',
                      'creator_profile_id': 'prof-test',
                      'allow_guests': chatId == firstChatId,
                    },
                  },
              ],
            },
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isFalse,
      );
    },
  );

  testWidgets('shared media backend failures use localized copy', (
    tester,
  ) async {
    const rawPayload = 'internal gateway details';
    tester.view
      ..physicalSize = const Size(800, 1000)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      testApp(
        home: SizedBox(
          height: 900,
          width: 400,
          child: ChatInfoPanel(chatId: 'chat-shared-media-backend-error'),
        ),
        client: MockClient((_) async {
          return http.Response(
            jsonEncode({'error': 'backend_down', 'message': rawPayload}),
            503,
          );
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.text(
        'Social and chat features are unavailable. Start the full API stack (docker compose --profile app).',
      ),
      findsWidgets,
    );
    expect(find.text(rawPayload), findsNothing);
  });

  testWidgets('shared media failures hide raw API details', (tester) async {
    const rawPayload = 'database connection string';

    await tester.pumpWidget(
      testApp(
        home: SizedBox(
          height: 900,
          width: 400,
          child: ChatInfoPanel(chatId: 'chat-shared-media-error'),
        ),
        client: MockClient((_) async {
          return http.Response(
            jsonEncode({'error': 'internal_error', 'message': rawPayload}),
            500,
          );
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Could not load shared media'), findsWidgets);
    expect(find.text(rawPayload), findsNothing);
  });
}
