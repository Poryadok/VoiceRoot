import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/group_members_management_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';

void main() {
  test('loads every roster page with the documented page size', () async {
    final calls = <({String? cursor, int? pageSize, String authorization})>[];
    final client = _ScriptedChatsClient((
      authorization,
      cursor,
      pageSize,
    ) async {
      calls.add((
        cursor: cursor,
        pageSize: pageSize,
        authorization: authorization,
      ));
      if (cursor == null) {
        return const ChatsApiOk(
          MemberListData(
            members: [ChatMember(profileId: 'member-1', role: 'member')],
            nextCursor: 'opaque-next',
          ),
        );
      }
      return const ChatsApiOk(
        MemberListData(
          members: [
            ChatMember(profileId: 'member-2', role: 'admin'),
            ChatMember(profileId: 'member-3', role: 'owner'),
          ],
        ),
      );
    });
    final container = _container(client);
    addTearDown(container.dispose);
    final provider = groupMembersManagementProvider('chat-1');
    final subscription = container.listen(provider, (_, _) {});
    addTearDown(subscription.close);
    container.read(provider);

    await _waitFor(() => !container.read(provider).isLoading);

    expect(calls.map((call) => call.cursor), [null, 'opaque-next']);
    expect(calls.map((call) => call.pageSize), [500, 500]);
    expect(container.read(provider).members.map((member) => member.profileId), [
      'member-1',
      'member-2',
      'member-3',
    ]);
    expect(container.read(provider).members[1].role, kChatRoleAdmin);
  });

  test('drops a delayed page after the active profile changes', () async {
    final firstPage = Completer<ChatsApiResult<MemberListData>>();
    final profileBPage = Completer<void>();
    final calls = <({String authorization, String? cursor})>[];
    final client = _ScriptedChatsClient((authorization, cursor, _) async {
      calls.add((authorization: authorization, cursor: cursor));
      if (authorization == 'Bearer test-access') return firstPage.future;
      profileBPage.complete();
      return const ChatsApiOk(
        MemberListData(
          members: [ChatMember(profileId: 'profile-b-member', role: 'member')],
        ),
      );
    });
    final container = _container(client);
    addTearDown(container.dispose);
    final provider = groupMembersManagementProvider('chat-2');
    final subscription = container.listen(provider, (_, _) {});
    addTearDown(subscription.close);
    container.read(provider);

    await _waitFor(() => calls.length == 1);
    final controller = container.read(authControllerProvider.notifier);
    controller.state = controller.state.copyWith(
      session: const AuthSession(
        accessToken: 'profile-b-access',
        refreshToken: 'profile-b-refresh',
        accountId: 'acc-test',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    await profileBPage.future;
    await _waitFor(() => !container.read(provider).isLoading);
    firstPage.complete(
      const ChatsApiOk(
        MemberListData(
          members: [
            ChatMember(profileId: 'stale-profile-a-member', role: 'member'),
          ],
        ),
      ),
    );
    await Future<void>.delayed(Duration.zero);

    expect(calls.map((call) => call.authorization), [
      'Bearer test-access',
      'Bearer profile-b-access',
    ]);
    expect(container.read(provider).members.map((member) => member.profileId), [
      'profile-b-member',
    ]);
  });

  test('failed add can retry once under the same captured session', () async {
    final addCalls = <({String authorization, List<String> profileIds})>[];
    final client = _ScriptedChatsClient(
      (_, _, _) async => const ChatsApiOk(
        MemberListData(
          members: [ChatMember(profileId: 'prof-test', role: 'member')],
        ),
      ),
      onAdd: (authorization, chatId, profileIds) async {
        addCalls.add((
          authorization: authorization,
          profileIds: List<String>.of(profileIds),
        ));
        if (addCalls.length == 1) {
          return const ChatsApiFailure(message: 'permission_denied');
        }
        return const ChatsApiOk<void>(null);
      },
    );
    final container = _container(client);
    addTearDown(container.dispose);
    final provider = groupMembersManagementProvider('chat-retry');
    final subscription = container.listen(provider, (_, _) {});
    addTearDown(subscription.close);
    container.read(provider);
    await _waitFor(() => !container.read(provider).isLoading);

    final controller = container.read(provider.notifier);
    final context = controller.captureContext()!;
    expect(
      await controller.addMembers(['candidate'], expectedContext: context),
      'permission_denied',
    );
    expect(container.read(provider).isMutating, isFalse);
    expect(
      await controller.addMembers(['candidate'], expectedContext: context),
      isNull,
    );

    expect(addCalls, hasLength(2));
    expect(addCalls.map((call) => call.authorization), [
      'Bearer test-access',
      'Bearer test-access',
    ]);
    expect(addCalls.map((call) => call.profileIds.toList()), [
      ['candidate'],
      ['candidate'],
    ]);
  });

  test(
    'delayed add success after a profile switch has no cache effects',
    () async {
      final delayedAdd = Completer<ChatsApiResult<void>>();
      final addCalls = <String>[];
      final listCalls = <String>[];
      final client = _ScriptedChatsClient(
        (authorization, _, _) async {
          listCalls.add(authorization);
          return ChatsApiOk(
            MemberListData(
              members: [
                ChatMember(
                  profileId: authorization == 'Bearer test-access'
                      ? 'prof-test'
                      : 'profile-b',
                  role: 'member',
                ),
              ],
            ),
          );
        },
        onAdd: (authorization, _, _) {
          addCalls.add(authorization);
          return delayedAdd.future;
        },
      );
      final container = _container(client);
      addTearDown(container.dispose);
      final provider = groupMembersManagementProvider('chat-stale-add');
      final subscription = container.listen(provider, (_, _) {});
      addTearDown(subscription.close);
      container.read(provider);
      await _waitFor(() => !container.read(provider).isLoading);

      final controller = container.read(provider.notifier);
      final expectedContext = controller.captureContext()!;
      final pending = controller.addMembers([
        'candidate',
      ], expectedContext: expectedContext);
      await _waitFor(() => addCalls.length == 1);
      container.read(authControllerProvider.notifier).state = container
          .read(authControllerProvider)
          .copyWith(
            session: const AuthSession(
              accessToken: 'profile-b-access',
              refreshToken: 'profile-b-refresh',
              accountId: 'acc-test',
              activeProfileId: 'profile-b',
              expiresInSeconds: 900,
            ),
          );
      await _waitFor(
        () => listCalls.length == 2 && !container.read(provider).isLoading,
      );
      delayedAdd.complete(const ChatsApiOk<void>(null));

      expect(await pending, kGroupMembersStaleContext);
      expect(addCalls, ['Bearer test-access']);
      expect(listCalls, ['Bearer test-access', 'Bearer profile-b-access']);
      expect(container.read(provider).members.single.profileId, 'profile-b');
    },
  );
}

ProviderContainer _container(VoiceChatsClient client) => ProviderContainer(
  overrides: [
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://api.test'),
    ),
    httpClientProvider.overrideWithValue(
      MockClient((_) async => http.Response('{}', 500)),
    ),
    authControllerProvider.overrideWith(authenticatedAuthController),
    voiceChatsClientProvider.overrideWithValue(client),
  ],
);

Future<void> _waitFor(bool Function() condition) async {
  for (var attempt = 0; attempt < 100; attempt++) {
    if (condition()) return;
    await Future<void>.delayed(const Duration(milliseconds: 1));
  }
  fail('The provider did not settle within the bounded test window.');
}

class _ScriptedChatsClient extends VoiceChatsClient {
  _ScriptedChatsClient(this.onList, {this.onAdd})
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final Future<ChatsApiResult<MemberListData>> Function(
    String authorization,
    String? cursor,
    int? pageSize,
  )
  onList;
  final Future<ChatsApiResult<void>> Function(
    String authorization,
    String chatId,
    List<String> profileIds,
  )?
  onAdd;

  @override
  Future<ChatsApiResult<MemberListData>> listGroupMembers({
    required String authorization,
    required String chatId,
    String? cursor,
    int? pageSize,
  }) => onList(authorization, cursor, pageSize);

  @override
  Future<ChatsApiResult<void>> addGroupMembers({
    required String authorization,
    required String chatId,
    required List<String> profileIds,
  }) async =>
      onAdd?.call(authorization, chatId, profileIds) ??
      const ChatsApiOk<void>(null);
}
