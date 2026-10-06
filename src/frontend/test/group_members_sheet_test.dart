import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/create_group_friends_provider.dart';
import 'package:voice_frontend/state/group_members_management_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/group_members_sheet.dart';
import 'package:voice_frontend/ui/chat/group_member_picker_sheet.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/core/voice_state_panel.dart';

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

  testWidgets(
    'group members sheet uses the Voice list skeleton while loading',
    (tester) async {
      const chatId = 'group-members-loading';
      final members = Completer<MemberListData>();

      await tester.pumpWidget(
        testApp(
          home: const GroupMembersSheet(chatId: chatId),
          client: MockClient((_) async => http.Response('{}', 404)),
          extraOverrides: [
            groupMembersManagementProvider(chatId).overrideWith((ref) {
              final controller = _TestGroupMembersController(
                ref,
                chatId,
                loading: true,
              );
              members.future.then(
                (data) => controller.setMembers(data.members),
              );
              return controller;
            }),
          ],
        ),
      );
      await tester.pump();

      expect(find.byType(VoiceListSkeleton), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsNothing);

      members.complete(const MemberListData(members: []));
      await tester.pumpAndSettle();
    },
  );

  testWidgets('admin can kick an ordinary member but not owner or admin', (
    tester,
  ) async {
    const chatId = 'group-members-admin-kick';
    const adminId = 'profile-admin';
    const ordinaryId = 'profile-ordinary';
    const otherAdminId = 'profile-other-admin';
    const ownerId = 'profile-owner';

    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((_) async => http.Response('{}', 404)),
        extraOverrides: [
          authControllerProvider.overrideWith((ref) {
            final controller = authenticatedAuthController(ref);
            controller.state = controller.state.copyWith(
              session: const AuthSession(
                accessToken: 'test-access',
                refreshToken: 'test-refresh',
                accountId: 'acc-test',
                activeProfileId: adminId,
                expiresInSeconds: 900,
              ),
            );
            return controller;
          }),
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: const [
                ChatMember(profileId: adminId, role: 'admin'),
                ChatMember(profileId: ordinaryId, role: 'member'),
                ChatMember(profileId: otherAdminId, role: 'admin'),
                ChatMember(profileId: ownerId, role: 'owner'),
              ],
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(GroupMembersSheet.kickMemberKey(ordinaryId)),
      findsOneWidget,
    );
    expect(find.byKey(GroupMembersSheet.kickMemberKey(adminId)), findsNothing);
    expect(
      find.byKey(GroupMembersSheet.kickMemberKey(otherAdminId)),
      findsNothing,
    );
    expect(find.byKey(GroupMembersSheet.kickMemberKey(ownerId)), findsNothing);
  });

  testWidgets('member selection stays local until explicit add', (
    tester,
  ) async {
    const chatId = 'group-add-members';
    final addBodies = <Map<String, dynamic>>[];
    final requestPaths = <String>[];
    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((request) async {
          requestPaths.add('${request.method} ${request.url.path}');
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/chats/$chatId/members') {
            addBodies.add(jsonDecode(request.body) as Map<String, dynamic>);
            return http.Response('', 204);
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: const [
                ChatMember(profileId: 'prof-test', role: 'member'),
                ChatMember(profileId: 'already-added', role: 'member'),
              ],
            ),
          ),
          createGroupFriendsProvider.overrideWith(
            (_) async => const ['already-added', 'candidate-profile'],
          ),
          profileProvider('candidate-profile').overrideWith(
            (_) async => const VoiceProfile(
              id: 'candidate-profile',
              accountId: 'candidate-account',
              username: 'candidate',
              discriminator: '0001',
              displayName: 'Candidate',
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(GroupMembersSheet.addMembersKey));
    await tester.pumpAndSettle();
    expect(
      find.byKey(GroupMemberPickerSheet.memberKey('already-added')),
      findsNothing,
    );
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(addBodies, isEmpty);

    await tester.tap(find.byKey(GroupMembersSheet.addMembersKey));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(GroupMemberPickerSheet.memberKey('candidate-profile')),
    );
    await tester.pump();
    expect(addBodies, isEmpty);
    expect(
      tester
          .widget<FilledButton>(find.byKey(GroupMemberPickerSheet.submitKey))
          .onPressed,
      isNotNull,
    );
    await tester.tap(find.byKey(GroupMemberPickerSheet.submitKey));
    await tester.pumpAndSettle();

    expect(requestPaths, contains('POST /api/v1/chats/$chatId/members'));
    expect(addBodies, hasLength(1), reason: 'Recorded requests: $requestPaths');
    expect(addBodies.single['profile_ids'], ['candidate-profile']);
  });

  testWidgets('500 members disables Add with the documented tooltip', (
    tester,
  ) async {
    const chatId = 'group-at-limit';
    var addRequests = 0;
    var transferRequests = 0;
    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((request) async {
          if (request.method == 'POST') addRequests++;
          if (request.url.path == '/api/v1/chats/$chatId/transfer-ownership') {
            transferRequests++;
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: List<ChatMember>.generate(
                500,
                (index) => ChatMember(
                  profileId: index == 0 ? 'prof-test' : 'profile-$index',
                  role: index == 0 ? kChatRoleOwner : kChatRoleMember,
                ),
              ),
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(
      tester.takeException(),
      isNull,
      reason: 'the maximum owner roster must remain bounded and scrollable',
    );
    expect(
      find.byKey(GroupMembersSheet.transferOwnerKey('profile-1')),
      findsOneWidget,
    );
    expect(find.byKey(GroupMembersSheet.addMembersKey), findsOneWidget);
    expect(find.byKey(GroupMembersSheet.leaveKey), findsOneWidget);

    final addButton = tester.widget<IconButton>(
      find.byKey(GroupMembersSheet.addMembersKey),
    );
    expect(addButton.onPressed, isNull);
    expect(
      addButton.tooltip,
      'This group has reached its 500-member limit. Create a Space for a larger community.',
    );
    await tester.tap(find.byKey(GroupMembersSheet.addMembersKey));
    await tester.pump();
    expect(addRequests, 0);

    final lastTransfer = find.byKey(
      GroupMembersSheet.transferOwnerKey('profile-499'),
    );
    final roster = find.byType(ListView).first;
    for (
      var attempt = 0;
      attempt < 80 && lastTransfer.evaluate().isEmpty;
      attempt++
    ) {
      await tester.drag(roster, const Offset(0, -1000));
      await tester.pumpAndSettle();
    }
    expect(lastTransfer, findsOneWidget);
    await tester.tap(lastTransfer);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(transferRequests, 0);
    expect(find.byKey(GroupMembersSheet.addMembersKey), findsOneWidget);
    expect(find.byKey(GroupMembersSheet.leaveKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('picker submit is discarded after the active profile changes', (
    tester,
  ) async {
    const chatId = 'group-add-stale-profile';
    var addRequests = 0;
    late AuthController authController;
    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((request) async {
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/chats/$chatId/members') {
            addRequests++;
            return http.Response('', 204);
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [
          authControllerProvider.overrideWith((ref) {
            authController = authenticatedAuthController(ref);
            authController.state = authController.state.copyWith(
              session: const AuthSession(
                accessToken: 'profile-a-access',
                refreshToken: 'profile-a-refresh',
                accountId: 'acc-test',
                activeProfileId: 'profile-a',
                expiresInSeconds: 900,
              ),
            );
            return authController;
          }),
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: const [
                ChatMember(profileId: 'profile-a', role: 'owner'),
              ],
            ),
          ),
          createGroupFriendsProvider.overrideWith(
            (_) async => const ['candidate-profile'],
          ),
          profileProvider('candidate-profile').overrideWith(
            (_) async => const VoiceProfile(
              id: 'candidate-profile',
              accountId: 'candidate-account',
              username: 'candidate',
              discriminator: '0001',
              displayName: 'Candidate',
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(GroupMembersSheet.addMembersKey));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(GroupMemberPickerSheet.memberKey('candidate-profile')),
    );
    await tester.pump();
    authController.state = authController.state.copyWith(
      session: const AuthSession(
        accessToken: 'profile-b-access',
        refreshToken: 'profile-b-refresh',
        accountId: 'acc-test',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    await tester.tap(find.byKey(GroupMemberPickerSheet.submitKey));
    await tester.pumpAndSettle();

    expect(addRequests, 0);
    expect(find.byType(GroupMemberPickerSheet), findsNothing);
  });

  testWidgets('member row opens that profile detail', (tester) async {
    const chatId = 'group-member-profile';
    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((_) async => http.Response('{}', 404)),
        extraOverrides: [
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: const [
                ChatMember(profileId: 'prof-test', role: 'owner'),
                ChatMember(profileId: 'candidate-profile', role: 'member'),
              ],
            ),
          ),
          profileProvider('candidate-profile').overrideWith(
            (_) async => const VoiceProfile(
              id: 'candidate-profile',
              accountId: 'candidate-account',
              username: 'candidate',
              discriminator: '0001',
              displayName: 'Candidate',
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(
      find.descendant(
        of: find.byKey(GroupMembersSheet.memberTileKey('candidate-profile')),
        matching: find.byType(ListTile),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(ProfileDetailSheet), findsOneWidget);
  });

  testWidgets('ownership transfer asks for confirmation and sends one action', (
    tester,
  ) async {
    const chatId = 'group-transfer-owner';
    final transferBodies = <Map<String, dynamic>>[];
    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((request) async {
          if (request.method == 'POST' &&
              request.url.path == '/api/v1/chats/$chatId/transfer-ownership') {
            transferBodies.add(
              jsonDecode(request.body) as Map<String, dynamic>,
            );
            return http.Response('', 204);
          }
          return http.Response('{}', 404);
        }),
        extraOverrides: [
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: const [
                ChatMember(profileId: 'prof-test', role: 'owner'),
                ChatMember(profileId: 'candidate-profile', role: 'member'),
              ],
            ),
          ),
          profileProvider('candidate-profile').overrideWith(
            (_) async => const VoiceProfile(
              id: 'candidate-profile',
              accountId: 'candidate-account',
              username: 'candidate',
              discriminator: '0001',
              displayName: 'Candidate',
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    final transferButton = find.byKey(
      GroupMembersSheet.transferOwnerKey('candidate-profile'),
    );
    expect(transferButton, findsOneWidget);
    await tester.tap(transferButton);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(transferBodies, isEmpty);

    await tester.tap(transferButton);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Transfer'));
    await tester.pumpAndSettle();

    expect(transferBodies, hasLength(1));
    expect(transferBodies.single['new_owner_profile_id'], 'candidate-profile');
  });

  testWidgets('group leave failure hides upstream details', (tester) async {
    const chatId = 'group-leave-error';
    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((_) async => http.Response('{}', 404)),
        extraOverrides: [
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              members: const [
                ChatMember(profileId: 'prof-test', role: 'owner'),
              ],
            ),
          ),
          chatActionsProvider.overrideWith(_FailingLeaveChatActions.new),
        ],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(GroupMembersSheet.leaveKey));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Leave group'));
    await tester.pumpAndSettle();

    expect(find.text('Could not complete this action.'), findsOneWidget);
    expect(find.textContaining('group-leave-secret'), findsNothing);
  });

  testWidgets('group members error is localized in the bottom sheet', (
    tester,
  ) async {
    const chatId = 'group-members-error-sheet';

    await tester.pumpWidget(
      testApp(
        home: const GroupMembersSheet(chatId: chatId),
        client: MockClient((_) async => http.Response('{}', 503)),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(VoiceStatePanel), findsOneWidget);
    expect(find.text(AppLocalizationsEn().backendUnavailable), findsOneWidget);
    expect(find.textContaining('BackendUnavailableException'), findsNothing);
  });

  testWidgets('ChatInfo embeds the same localized group members state', (
    tester,
  ) async {
    const chatId = 'group-members-error-chat-info';

    await tester.pumpWidget(
      testApp(
        home: const SizedBox(
          width: 400,
          height: 600,
          child: ChatInfoPanel(chatId: chatId, isGroup: true),
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
        extraOverrides: [
          groupMembersManagementProvider(chatId).overrideWith(
            (ref) => _TestGroupMembersController(
              ref,
              chatId,
              error: 'raw upstream failure',
            ),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    final groupMembers = find.byType(GroupMembersContent);
    expect(groupMembers, findsOneWidget);
    expect(
      find.descendant(of: groupMembers, matching: find.byType(VoiceStatePanel)),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: groupMembers,
        matching: find.text('Could not load members'),
      ),
      findsNWidgets(2),
    );
    expect(find.textContaining('raw upstream failure'), findsNothing);
  });

  testWidgets('group info modal fits member and settings controls on 400x800', (
    tester,
  ) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(400, 800));
    const chatId = 'group-roles-1';
    const ownerId = 'prof-test';
    const memberId = 'profile-member';

    await tester.pumpWidget(
      testApp(
        home: MediaQuery(
          data: const MediaQueryData(size: Size(400, 800)),
          child: const SizedBox(
            width: 400,
            height: 800,
            child: ChatRoomPanel(chatId: chatId),
          ),
        ),
        client: MockClient((req) async {
          final path = req.url.path;
          if (path == '/api/v1/chats') {
            return http.Response(
              jsonEncode({
                'chat_list': {
                  'items': [
                    {
                      'chat': {
                        'id': chatId,
                        'type': 'CHAT_TYPE_GROUP',
                        'creator_profile_id': ownerId,
                        'name': 'Roles squad',
                      },
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (path == '/api/v1/chats/$chatId/messages') {
            return http.Response(
              jsonEncode({
                'message_list': {'messages': []},
              }),
              200,
            );
          }
          if (path == '/api/v1/chats/$chatId/members') {
            return http.Response(
              jsonEncode({
                'member_list': {
                  'members': [
                    {'profile_id': ownerId, 'role': 'owner'},
                    {'profile_id': memberId, 'role': 'member'},
                  ],
                },
              }),
              200,
            );
          }
          if (path.startsWith('/api/v1/chats/$chatId/shared-media')) {
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
          if (path == '/api/v1/users/profiles' && req.method == 'GET') {
            return http.Response(
              jsonEncode({
                'profile_list': {
                  'profiles': [
                    {
                      'id': ownerId,
                      'account_id': 'acc-owner',
                      'username': 'owner',
                      'discriminator': '0001',
                      'display_name': 'Owner',
                      'is_primary': true,
                      'verification_type': 'none',
                    },
                  ],
                },
              }),
              200,
            );
          }
          if (path.startsWith('/api/v1/users/profiles/')) {
            final segments = path.split('/');
            final id = segments.length >= 5
                ? segments[4]
                : path.split('/').last;
            return http.Response(
              jsonEncode({
                'profile': {
                  'id': id,
                  'account_id': 'acc-$id',
                  'username': id,
                  'discriminator': '0001',
                  'display_name': id == ownerId ? 'Owner' : 'Member',
                  'is_primary': id == ownerId,
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

    expect(find.byKey(ChatRoomPanel.groupMembersKey), findsOneWidget);
    await tester.tap(find.byKey(ChatRoomPanel.groupMembersKey));
    await tester.pumpAndSettle();

    expect(find.byKey(ChatInfoPanel.panelKey), findsOneWidget);
    expect(
      find.byKey(StandaloneChatGuestSettingsSection.sectionKey),
      findsOneWidget,
    );
    expect(
      find.byKey(ChatNotificationOverridesSection.sectionKey),
      findsOneWidget,
    );
    expect(find.byKey(ChatInfoPanel.mediaTabKey), findsOneWidget);
    expect(find.text('Owner'), findsOneWidget);
    await tester.scrollUntilVisible(
      find.byKey(GroupMembersSheet.kickMemberKey(memberId)),
      48,
      scrollable: find
          .descendant(
            of: find.byKey(ChatInfoPanel.panelKey),
            matching: find.byType(Scrollable),
          )
          .first,
    );
    expect(
      find.byKey(GroupMembersSheet.kickMemberKey(memberId)),
      findsOneWidget,
    );
    expect(find.byKey(GroupMembersSheet.kickMemberKey(ownerId)), findsNothing);
    expect(find.byKey(GroupMembersSheet.leaveKey), findsOneWidget);
    expect(
      tester.takeException(),
      isNull,
      reason: 'the narrow group info modal must not report a layout exception',
    );
  });
}

class _TestGroupMembersController extends GroupMembersManagementController {
  _TestGroupMembersController(
    super.ref,
    super.chatId, {
    List<ChatMember> members = const <ChatMember>[],
    bool loading = false,
    Object? error,
  }) : super() {
    state = GroupMembersManagementState(
      members: members,
      isLoading: loading,
      error: error,
    );
  }

  void setMembers(List<ChatMember> members) {
    state = GroupMembersManagementState(members: members);
  }

  @override
  Future<void> load({GroupMembersAuthContext? expectedContext}) async {}
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> disconnect() async {}
}

class _FailingLeaveChatActions extends ChatActions {
  _FailingLeaveChatActions(super.ref);

  @override
  Future<String?> leaveGroup(String chatId) async =>
      'internal failure trace=group-leave-secret';
}
