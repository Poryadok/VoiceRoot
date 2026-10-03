import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/in_app_notifications.dart';
import 'package:voice_frontend/state/space_providers.dart';

import 'support/gateway_test_client.dart';
import 'support/auth_test_overrides.dart';

final _activeProfileProvider = StateProvider<String>((ref) => 'profile-a');

void main() {
  for (final subject in const [
    'role.chat_override_set',
    'role.chat_override_removed',
  ]) {
    test('$subject refreshes active permissions for only its chat', () async {
      final hub = _FakeRealtimeHub();
      final roles = _RecordingRolesClient();
      final container = _container(hub, roles);
      addTearDown(container.dispose);

      const affectedA = (
        spaceId: 'space-a',
        permission: 'TEXT_CHAT_SEND_MESSAGES',
        chatId: 'chat-a',
        voiceRoomId: null,
      );
      const affectedB = (
        spaceId: 'space-a',
        permission: 'TEXT_CHAT_ATTACH_FILES',
        chatId: 'chat-a',
        voiceRoomId: null,
      );
      const otherChat = (
        spaceId: 'space-a',
        permission: 'TEXT_CHAT_SEND_MESSAGES',
        chatId: 'chat-b',
        voiceRoomId: null,
      );
      const otherSpace = (
        spaceId: 'space-b',
        permission: 'TEXT_CHAT_SEND_MESSAGES',
        chatId: 'chat-a',
        voiceRoomId: null,
      );
      const voiceRoom = (
        spaceId: 'space-a',
        permission: 'VOICE_SPEAK',
        chatId: null,
        voiceRoomId: 'room-a',
      );
      final queries = [affectedA, affectedB, otherChat, otherSpace, voiceRoom];
      final subscriptions = queries
          .map(
            (query) =>
                container.listen(spacePermissionProvider(query), (_, _) {}),
          )
          .toList();
      addTearDown(() {
        for (final subscription in subscriptions) {
          subscription.close();
        }
      });

      for (final query in queries) {
        expect(
          await container.read(spacePermissionProvider(query).future),
          isTrue,
        );
      }
      expect(roles.calls, hasLength(5));

      container.read(_activeProfileProvider.notifier).state = 'profile-b';
      await pumpEventQueue();
      expect(
        roles.calls.where((call) => call.profileId == 'profile-b'),
        hasLength(5),
      );

      roles.calls.clear();
      roles.allowed = false;
      container.read(inAppNotificationControllerProvider);
      hub.emit(
        RealtimeFrame(
          op: 'role_update',
          data: {
            'subject': subject,
            'space_id': 'space-a',
            'chat_id': 'chat-a',
            // role_id describes the changed policy; it does not grant access.
            'role_id': 'role-changed',
          },
        ),
      );
      await pumpEventQueue();
      expect(
        await container.read(spacePermissionProvider(affectedA).future),
        isFalse,
      );
      expect(
        await container.read(spacePermissionProvider(affectedB).future),
        isFalse,
      );

      expect(roles.calls, hasLength(2));
      expect(roles.calls.map((call) => call.permission).toSet(), {
        'TEXT_CHAT_SEND_MESSAGES',
        'TEXT_CHAT_ATTACH_FILES',
      });
      expect(roles.calls.every((call) => call.spaceId == 'space-a'), isTrue);
      expect(roles.calls.every((call) => call.chatId == 'chat-a'), isTrue);
      expect(
        roles.calls.every((call) => call.profileId == 'profile-b'),
        isTrue,
      );
    });
  }

  test(
    'unknown, malformed, and unrelated role updates do not refetch',
    () async {
      final hub = _FakeRealtimeHub();
      final roles = _RecordingRolesClient();
      final container = _container(hub, roles);
      addTearDown(container.dispose);
      const query = (
        spaceId: 'space-a',
        permission: 'TEXT_CHAT_SEND_MESSAGES',
        chatId: 'chat-a',
        voiceRoomId: null,
      );
      final subscription = container.listen(
        spacePermissionProvider(query),
        (_, _) {},
      );
      addTearDown(subscription.close);
      await container.read(spacePermissionProvider(query).future);
      final initialCalls = roles.calls.length;
      container.read(inAppNotificationControllerProvider);

      for (final data in const <Map<String, dynamic>>[
        {'subject': 'role.updated', 'space_id': 'space-a', 'chat_id': 'chat-a'},
        {
          'subject': 'role.chat_override_set',
          'space_id': 'space-a',
          'chat_id': 'chat-a',
        },
        {
          'subject': 'role.chat_override_removed',
          'space_id': '',
          'chat_id': 'chat-a',
          'role_id': 'role-1',
        },
        {
          'subject': 'role.chat_override_set',
          'space_id': 'space-a',
          'chat_id': 42,
          'role_id': 'role-1',
        },
        {
          'subject': 'role.chat_override_removed',
          'space_id': 'space-a',
          'chat_id': 'another-chat',
          'role_id': 'role-1',
        },
      ]) {
        hub.emit(RealtimeFrame(op: 'role_update', data: data));
        await pumpEventQueue();
      }

      expect(roles.calls, hasLength(initialCalls));
    },
  );
}

ProviderContainer _container(
  _FakeRealtimeHub hub,
  _RecordingRolesClient roles,
) => ProviderContainer(
  overrides: [
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    authControllerProvider.overrideWith(authenticatedAuthController),
    authorizationHeaderProvider.overrideWithValue('Bearer test'),
    spaceViewerProfileIdProvider.overrideWith(
      (ref) => ref.watch(_activeProfileProvider),
    ),
    voiceRolesClientProvider.overrideWithValue(roles),
    realtimeHubProvider.overrideWithValue(hub),
  ],
);

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub() : super(_UnwiredRef());

  final _events = StreamController<RealtimeFrame>.broadcast();

  @override
  Stream<RealtimeFrame> get events => _events.stream;

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  void emit(RealtimeFrame frame) => _events.add(frame);

  @override
  Future<void> dispose() => _events.close();
}

class _RecordingRolesClient extends VoiceRolesClient {
  _RecordingRolesClient()
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('{}', 500)),
        ),
      );

  final calls =
      <
        ({
          String spaceId,
          String profileId,
          String permission,
          String? chatId,
          String? voiceRoomId,
        })
      >[];
  bool allowed = true;

  @override
  Future<RolesApiResult<bool>> checkPermission({
    required String authorization,
    required String spaceId,
    required String profileId,
    required String permissionName,
    String? chatId,
    String? voiceRoomId,
  }) async {
    calls.add((
      spaceId: spaceId,
      profileId: profileId,
      permission: permissionName,
      chatId: chatId,
      voiceRoomId: voiceRoomId,
    ));
    return RolesApiOk(allowed);
  }
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
