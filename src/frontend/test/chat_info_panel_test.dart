import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/chat/channel_settings_panel.dart';
import 'package:voice_frontend/ui/chat/create_group_sheet.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('chat bot list uses the canonical skeleton while loading', (
    tester,
  ) async {
    final pending = Completer<List<ChatBotSettings>>();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spacePermissionProvider.overrideWith((ref, query) async => true),
          botsInChatProvider.overrideWith((ref, key) => pending.future),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: ChatBotsSettingsSection(
              chatId: 'chat-loading',
              spaceId: 'space-loading',
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();

    expect(find.byType(VoiceListSkeleton), findsOneWidget);

    pending.complete(const []);
  });

  Widget testApp({
    required Widget home,
    required http.Client client,
    List<Override> extraOverrides = const [],
    void Function(ProviderContainer container)? onContainer,
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
        ...extraOverrides,
      ],
      child: Builder(
        builder: (context) {
          onContainer?.call(ProviderScope.containerOf(context));
          return MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(body: home),
          );
        },
      ),
    );
  }

  Map<String, Object?> channelListResponse(String chatId, {String? spaceId}) =>
      {
        'chat_list': {
          'items': [
            {
              'chat': {
                'id': chatId,
                'type': 'CHAT_TYPE_CHANNEL',
                'creator_profile_id': 'profile-owner',
                'space_id': ?spaceId,
              },
            },
          ],
        },
      };

  testWidgets('DM Chat Info offers the documented create-group action', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        home: const ChatInfoPanel(chatId: 'dm-chat'),
        client: MockClient((request) async {
          if (request.method == 'GET' && request.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': 'dm-chat',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-me',
                      },
                      'dm_peer_profile_id': 'dm-peer',
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

    expect(find.byKey(const Key('chat_info_create_group')), findsOneWidget);
    await tester.tap(find.byKey(ChatInfoPanel.createGroupKey));
    await tester.pumpAndSettle();
    expect(find.byKey(CreateGroupSheet.sheetKey), findsOneWidget);
    expect(
      find.byKey(CreateGroupSheet.memberTileKey('dm-peer')),
      findsOneWidget,
    );
  });

  testWidgets('DM Chat Info ignores a peer remembered for another session', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        home: const ChatInfoPanel(chatId: 'dm-chat'),
        client: MockClient((request) async {
          if (request.method == 'GET' && request.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {'items': []},
              }),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [
          dmPeerProfileByChatIdProvider.overrideWith(
            (ref) => const {'dm-chat': 'peer-from-another-profile'},
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.createGroupKey), findsNothing);
  });

  testWidgets(
    'standalone channel Chat Info confirms leave and handles failure',
    (tester) async {
      const chatId = 'standalone-channel';
      var leaveCalls = 0;
      await tester.pumpWidget(
        testApp(
          home: const ChatInfoPanel(chatId: chatId),
          client: MockClient((request) async {
            if (request.method == 'GET' &&
                request.url.path == '/api/v1/chats') {
              return http.Response(
                jsonEncode(channelListResponse(chatId)),
                200,
              );
            }
            if (request.url.path == '/api/v1/chats/$chatId/leave') {
              leaveCalls++;
              return http.Response(
                '{"message":"private upstream detail"}',
                412,
              );
            }
            return http.Response('{}', 404);
          }),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(ChatInfoPanel.leaveChannelKey), findsOneWidget);
      await tester.tap(find.byKey(ChatInfoPanel.leaveChannelKey));
      await tester.pumpAndSettle();
      expect(find.text('Leave channel?'), findsOneWidget);
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(leaveCalls, 0);

      await tester.tap(find.byKey(ChatInfoPanel.leaveChannelKey));
      await tester.pumpAndSettle();
      await tester.tap(
        find.byKey(const Key('chat_info_leave_channel_confirm')),
      );
      await tester.pumpAndSettle();

      expect(leaveCalls, 1);
      expect(find.text('Could not complete this action.'), findsOneWidget);
      expect(find.textContaining('private upstream detail'), findsNothing);
      expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    },
  );

  testWidgets('channel leave completion cannot change another active profile', (
    tester,
  ) async {
    const chatId = 'channel-profile-switch';
    final pendingLeave = Completer<http.Response>();
    var leaveCalls = 0;
    ProviderContainer? container;
    await tester.pumpWidget(
      testApp(
        home: const ChatInfoPanel(chatId: chatId),
        client: MockClient((request) async {
          if (request.method == 'GET' && request.url.path == '/api/v1/chats') {
            return http.Response(jsonEncode(channelListResponse(chatId)), 200);
          }
          if (request.url.path == '/api/v1/chats/$chatId/leave') {
            leaveCalls++;
            return pendingLeave.future;
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [selectedChatIdProvider.overrideWith((ref) => chatId)],
        onContainer: (value) => container = value,
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ChatInfoPanel.leaveChannelKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat_info_leave_channel_confirm')));
    await tester.pump();
    expect(leaveCalls, 1);

    container!.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'other-access',
        refreshToken: 'other-refresh',
        accountId: 'other-account',
        activeProfileId: 'other-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    expect(find.byKey(ChatInfoPanel.leaveChannelKey), findsNothing);
    pendingLeave.complete(http.Response('{}', 204));
    await tester.pumpAndSettle();

    expect(container!.read(selectedChatIdProvider), chatId);
    expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    expect(find.text('Could not complete this action.'), findsNothing);
  });

  testWidgets('channel leave success clears only the selected channel', (
    tester,
  ) async {
    const chatId = 'channel-leave-success';
    var chatReads = 0;
    var leaveCalls = 0;
    ProviderContainer? container;
    await tester.pumpWidget(
      testApp(
        home: const ChatInfoPanel(chatId: chatId),
        client: MockClient((request) async {
          if (request.method == 'GET' && request.url.path == '/api/v1/chats') {
            chatReads++;
            return http.Response(
              jsonEncode(
                chatReads == 1
                    ? channelListResponse(chatId)
                    : {
                        'chat_list': {'items': []},
                      },
              ),
              200,
            );
          }
          if (request.url.path == '/api/v1/chats/$chatId/leave') {
            leaveCalls++;
            return http.Response('', 204);
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [selectedChatIdProvider.overrideWith((ref) => chatId)],
        onContainer: (value) => container = value,
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(ChatInfoPanel.leaveChannelKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat_info_leave_channel_confirm')));
    await tester.pumpAndSettle();

    expect(leaveCalls, 1);
    expect(container!.read(selectedChatIdProvider), isNull);
    expect(chatReads, greaterThanOrEqualTo(2));
  });

  testWidgets('space-owned channel does not expose standalone leave', (
    tester,
  ) async {
    const chatId = 'space-channel';
    await tester.pumpWidget(
      testApp(
        home: const ChatInfoPanel(chatId: chatId),
        client: MockClient((request) async {
          if (request.method == 'GET' && request.url.path == '/api/v1/chats') {
            return http.Response(
              jsonEncode(channelListResponse(chatId, spaceId: 'space-1')),
              200,
            );
          }
          return http.Response('{}', 404);
        }),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.leaveChannelKey), findsNothing);
  });

  Future<void> verifyWideSettingsCanReturn(
    WidgetTester tester,
    String scenario,
  ) async {
    final pendingReload = Completer<http.Response>();
    var chatReads = 0;
    var patchCalls = 0;
    final client = MockClient((req) async {
      if (req.url.path == '/api/v1/chats') {
        chatReads++;
        if (chatReads > 1 && scenario == 'loading') {
          return pendingReload.future;
        }
        if (chatReads > 1 && scenario == 'load error') {
          return http.Response('{}', 503);
        }
        final type = chatReads > 1 && scenario == 'invalid chat'
            ? 'CHAT_TYPE_GROUP'
            : 'CHAT_TYPE_CHANNEL';
        return http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': 'channel-settings',
                    'type': type,
                    'creator_profile_id': 'prof-test',
                    'threads_enabled': false,
                    'allow_user_main_feed': false,
                  },
                },
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/chats/channel-settings/members') {
        final role = scenario == 'authorization loss' && chatReads > 1
            ? 'member'
            : 'owner';
        return http.Response(
          jsonEncode({
            'member_list': {
              'members': [
                {'profile_id': 'prof-test', 'role': role},
              ],
            },
          }),
          200,
        );
      }
      if (req.method == 'PATCH') patchCalls++;
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

    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    await tester.pumpWidget(
      testApp(
        home: const SizedBox(
          height: 800,
          width: 300,
          child: ChatInfoPanel(chatId: 'channel-settings'),
        ),
        client: client,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(StandaloneChannelSettingsEntry.entryKey), findsOneWidget);
    final settingsEntry = find.byKey(StandaloneChannelSettingsEntry.entryKey);
    await tester.ensureVisible(settingsEntry);
    await tester.tap(settingsEntry);
    await tester.pumpAndSettle();

    expect(find.byKey(ChannelSettingsPanel.closeKey), findsOneWidget);
    await tester.tap(find.byKey(ChannelSettingsPanel.closeKey));
    await tester.pumpAndSettle();

    expect(find.byType(ChatInfoPanel), findsOneWidget);
    expect(find.byKey(ChannelSettingsPanel.panelKey), findsNothing);
    expect(patchCalls, 0);
    if (scenario == 'loading') {
      pendingReload.complete(
        http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': 'channel-settings',
                    'type': 'CHAT_TYPE_CHANNEL',
                    'creator_profile_id': 'prof-test',
                    'threads_enabled': false,
                    'allow_user_main_feed': false,
                  },
                },
              ],
            },
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();
    }
    await tester.pumpWidget(const SizedBox.shrink());
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  }

  for (final scenario in [
    'loading',
    'load error',
    'authorization loss',
    'invalid chat',
  ]) {
    testWidgets('wide channel settings can return during $scenario', (
      tester,
    ) async {
      await verifyWideSettingsCanReturn(tester, scenario);
    });
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

  testWidgets(
    'standalone channel owner opens channel settings from Chat Info',
    (tester) async {
      final client = MockClient((req) async {
        if (req.url.path == '/api/v1/chats') {
          return http.Response(
            jsonEncode({
              'chat_list': {
                'items': [
                  {
                    'chat': {
                      'id': 'channel-settings',
                      'type': 'CHAT_TYPE_CHANNEL',
                      'creator_profile_id': 'prof-test',
                      'threads_enabled': false,
                      'allow_user_main_feed': false,
                    },
                  },
                ],
              },
            }),
            200,
          );
        }
        if (req.url.path == '/api/v1/chats/channel-settings/members') {
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

      final semantics = tester.ensureSemantics();
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.pumpWidget(
          testApp(
            home: SizedBox(
              height: size.width < 600 ? size.height * 0.75 : size.height,
              width: size.width < 600 ? size.width : 300,
              child: ChatInfoPanel(chatId: 'channel-settings'),
            ),
            client: client,
          ),
        );
        await tester.pumpAndSettle();

        expect(
          find.byKey(StandaloneChannelSettingsEntry.entryKey),
          findsOneWidget,
        );
        await tester.ensureVisible(
          find.byKey(StandaloneChannelSettingsEntry.entryKey),
        );
        await tester.tap(find.byKey(StandaloneChannelSettingsEntry.entryKey));
        await tester.pumpAndSettle();

        expect(find.byKey(ChannelSettingsPanel.panelKey), findsOneWidget);
        expect(
          find.byKey(ChannelSettingsPanel.threadsToggleKey),
          findsOneWidget,
        );
        expect(
          find.byKey(ChannelSettingsPanel.memberPostsToggleKey),
          findsOneWidget,
        );
        expect(
          find.byType(BottomSheet),
          size.width < 600 ? findsOneWidget : findsNothing,
        );
        final threadsSemantics = tester.getSemantics(
          find.descendant(
            of: find.byKey(ChannelSettingsPanel.threadsToggleKey),
            matching: find.byType(Switch),
          ),
        );
        expect(threadsSemantics.label, 'Enable threads');
        expect(
          tester
              .widget<SwitchListTile>(
                find.byKey(ChannelSettingsPanel.threadsToggleKey),
              )
              .value,
          isFalse,
        );
        expect(
          tester
              .widget<SwitchListTile>(
                find.byKey(ChannelSettingsPanel.memberPostsToggleKey),
              )
              .value,
          isFalse,
        );
        await tester.pumpWidget(const SizedBox.shrink());
      }
      semantics.dispose();
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    },
  );

  testWidgets(
    'guest admission stays disabled until its update and authoritative reload finish',
    (tester) async {
      const chatId = 'serialized-guest-admission-group';
      final pendingPatch = Completer<http.Response>();
      final pendingReload = Completer<http.Response>();
      var listCalls = 0;
      var updateCalls = 0;
      final client = MockClient((req) async {
        if (req.url.path == '/api/v1/chats') {
          listCalls++;
          if (listCalls > 1) return pendingReload.future;
          return http.Response(
            jsonEncode({
              'chat_list': {
                'items': [
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
        if (req.url.path == '/api/v1/chats/$chatId/members') {
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
        if (req.url.path == '/api/v1/chats/$chatId' && req.method == 'PATCH') {
          updateCalls++;
          return pendingPatch.future;
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
          home: SizedBox(
            height: 700,
            width: 400,
            child: ChatInfoPanel(chatId: chatId),
          ),
          client: client,
        ),
      );
      await tester.pumpAndSettle();

      final toggle = find.byKey(StandaloneChatGuestSettingsSection.toggleKey);
      await tester.tap(toggle);
      await tester.pump();
      expect(updateCalls, 1);
      expect(tester.widget<SwitchListTile>(toggle).onChanged, isNull);

      await tester.tap(toggle);
      await tester.pump();
      expect(updateCalls, 1);

      pendingPatch.complete(
        http.Response(
          jsonEncode({
            'chat': {
              'id': chatId,
              'type': 'CHAT_TYPE_GROUP',
              'creator_profile_id': 'prof-test',
              'allow_guests': true,
            },
          }),
          200,
        ),
      );
      await tester.pump();
      expect(tester.widget<SwitchListTile>(toggle).onChanged, isNull);

      await tester.tap(toggle);
      await tester.pump();
      expect(updateCalls, 1);

      pendingReload.complete(
        http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': chatId,
                    'type': 'CHAT_TYPE_GROUP',
                    'creator_profile_id': 'prof-test',
                    'allow_guests': true,
                  },
                },
              ],
            },
          }),
          200,
        ),
      );
      await tester.pumpAndSettle();

      expect(updateCalls, 1);
      expect(tester.widget<SwitchListTile>(toggle).onChanged, isNotNull);
      expect(tester.widget<SwitchListTile>(toggle).value, isTrue);
    },
  );

  testWidgets('chat info fits the compact 320 by 400 panel', (tester) async {
    tester.view
      ..physicalSize = const Size(320, 400)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      testApp(
        home: Builder(
          builder: (context) => Consumer(
            builder: (context, ref, _) => Center(
              child: ElevatedButton(
                onPressed: () => openChatInfoPanel(
                  context,
                  ref,
                  chatId: 'compact-panel-group',
                  isGroup: true,
                ),
                child: const Text('Open'),
              ),
            ),
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
                        'id': 'compact-panel-group',
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
    await tester.tap(find.text('Open'));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

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

  testWidgets(
    'a failed guest admission update keeps the value and hides server details',
    (tester) async {
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
                  'error': 'internal_error',
                  'message': 'db-password=guest-action-secret',
                }),
                500,
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

      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(StandaloneChatGuestSettingsSection.toggleKey),
            )
            .value,
        isTrue,
      );
      expect(
        find.descendant(
          of: find.byType(SnackBar),
          matching: find.text('Could not complete this action.'),
        ),
        findsOneWidget,
      );
      expect(find.textContaining('guest-action-secret'), findsNothing);
    },
  );

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
