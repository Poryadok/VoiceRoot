import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';

import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

/// T-053 Cycle 4 RED contract.
///
/// The real profile switch coordinator, generation-bound RealtimeHub, and
/// T-052 InboxReconciler meet here. A global Chat List snapshot is not a
/// token-change side effect: only an active B transport `hello` may accept the
/// one B `main`/`requests`/`archive` triplet. The real ChatListController is
/// deliberately mounted so that its legacy auth-triggered ListChats bypass is
/// observable rather than hidden by a test override.
void main() {
  group('T053 Cycle4 matching Realtime hello to T052 inbox reconciliation', () {
    test(
      'does not list B inboxes before matching B hello or for stale A hello',
      () async {
        final harness = _Cycle4Harness();
        addTearDown(harness.dispose);

        await harness.switchToBWithoutHello();

        expect(
          harness.bCalls,
          isEmpty,
          reason:
              'changing the persisted B session and completing its WS connect '
              'must not itself call ListChats',
        );

        harness.connection('profile-a').addHello();
        harness.connection('profile-a').addHello();
        await pumpEventQueue();

        expect(
          harness.bCalls,
          isEmpty,
          reason: 'a retired A hello must not accept the pending B snapshot',
        );
        expect(harness.messages.getCalls, isEmpty);
      },
    );

    test(
      'matching B hello starts exactly one B three-scope REST snapshot',
      () async {
        final harness = _Cycle4Harness();
        addTearDown(harness.dispose);

        await harness.switchToBWithoutHello();
        harness.connection('profile-b').addHello();
        await pumpEventQueue();

        expect(harness.bCalls, hasLength(3));
        expect(harness.bCalls.map((call) => call.inbox).toSet(), {
          'main',
          'requests',
          'archive',
        });
        expect(
          harness.bCalls.every(
            (call) =>
                call.authorization == 'Bearer access-profile-b' &&
                call.profileId == 'profile-b' &&
                call.cursor == null &&
                call.pageSize == null &&
                call.folderId == null,
          ),
          isTrue,
        );
        expect(harness.chats.unmatchedCalls, isEmpty);
        expect(harness.messages.getCalls, isEmpty);
      },
    );

    test(
      'repeated B hello and bare status changes cannot duplicate B triplet',
      () async {
        final harness = _Cycle4Harness();
        addTearDown(harness.dispose);

        await harness.switchToBWithoutHello();
        harness.connection('profile-b').addHello();
        await pumpEventQueue();
        expect(harness.bCalls, hasLength(3));

        harness.connection('profile-b').addHello();
        harness.container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        harness.container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        await pumpEventQueue();

        expect(
          harness.bCalls,
          hasLength(3),
          reason:
              'one accepted B hello, not a repeated hello or mutable link status, '
              'owns this generation-correlated snapshot',
        );
        expect(harness.messages.getCalls, isEmpty);
      },
    );
  });
}

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'access-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-1',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

class _Cycle4Harness {
  _Cycle4Harness()
    : chats = InboxReconcilerChatsFake(
        profileByAuthorization: const {
          'Bearer access-profile-a': 'profile-a',
          'Bearer access-profile-b': 'profile-b',
        },
      ),
      messages = InboxReconcilerMessagesFake(
        profileByAuthorization: const {
          'Bearer access-profile-a': 'profile-a',
          'Bearer access-profile-b': 'profile-b',
        },
      ),
      storage = _RecordingAuthSessionStorage(_session('profile-a')) {
    // The mounted legacy ChatListController loads A once. B has two main
    // scripts so an early legacy request and the required reconciler triplet
    // are both observable without turning the fake into the assertion.
    chats.enqueue(
      const InboxChatPageScript(
        inbox: 'main',
        cursor: null,
        profileId: 'profile-a',
        authorization: 'Bearer access-profile-a',
        result: ChatsApiOk(ChatListData(items: [])),
      ),
    );
    for (final inbox in ['main', 'requests', 'archive', 'main']) {
      chats.enqueue(
        InboxChatPageScript(
          inbox: inbox,
          cursor: null,
          profileId: 'profile-b',
          authorization: 'Bearer access-profile-b',
          result: const ChatsApiOk(ChatListData(items: [])),
        ),
      );
    }
    final client = MockClient(_respond);
    transport = _ControlledRealtimeTransportFactory();
    controller = AuthController(
      authClient: VoiceAuthClient(gateway: gatewayHttpForTest(client)),
      storage: storage,
      guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
    )..state = AuthState(session: _session('profile-a'));
    container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith((_) => controller),
        authSessionStorageProvider.overrideWithValue(storage),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
        voiceChatsClientProvider.overrideWithValue(chats),
        voiceMessagesClientProvider.overrideWithValue(messages),
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeTransportFactoryProvider.overrideWithValue(transport),
      ],
    );
    hub = container.read(realtimeHubProvider);
    coordinator = container.read(profileSwitchCoordinatorProvider);
    container.read(inboxReconcilerProvider);
    // Do not override this production controller: its current auth listener is
    // the duplicate-path regression this Cycle4 contract must expose.
    container.read(chatListControllerProvider);
  }

  final InboxReconcilerChatsFake chats;
  final InboxReconcilerMessagesFake messages;
  final _RecordingAuthSessionStorage storage;
  late final _ControlledRealtimeTransportFactory transport;
  late final AuthController controller;
  late final ProviderContainer container;
  late final RealtimeHub hub;
  late final ProfileSwitchCoordinator coordinator;

  List<InboxChatCall> get bCalls => chats.calls
      .where(
        (call) =>
            call.profileId == 'profile-b' &&
            call.authorization == 'Bearer access-profile-b',
      )
      .toList(growable: false);

  Future<void> switchToBWithoutHello() async {
    await pumpEventQueue();
    expect(
      chats.calls.where((call) => call.profileId == 'profile-a'),
      hasLength(1),
    );

    final a = hub.ensureConnected();
    await transport.waitForOpen('profile-a');
    transport.releaseOpen('profile-a');
    await a;

    final b = coordinator.switchTo('profile-b');
    await transport.waitForOpen('profile-b');
    transport.releaseOpen('profile-b');
    expect(await b, isA<ProfileSwitchApplied>());
    await pumpEventQueue();
  }

  _ControlledVoiceRealtimeConnection connection(String profileId) =>
      transport.connection(profileId);

  Future<http.Response> _respond(http.Request request) async {
    if (request.url.path != '/api/v1/auth/switch-profile') {
      return http.Response('not found', 404);
    }
    final body = jsonDecode(request.body) as Map<String, dynamic>;
    if (body['profile_id'] != 'profile-b') {
      return http.Response('unknown profile', 400);
    }
    return utf8JsonResponse(jsonEncode(_session('profile-b').toJson()));
  }

  Future<void> dispose() async {
    container.dispose();
    await transport.dispose();
  }
}

class _RecordingAuthSessionStorage implements AuthSessionStorage {
  _RecordingAuthSessionStorage(this._session);

  AuthSession? _session;

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async {
    _session = session;
  }
}

class _ControlledRealtimeTransportFactory implements RealtimeTransportFactory {
  _ControlledRealtimeTransportFactory() {
    for (final profileId in const ['profile-a', 'profile-b']) {
      _slots[profileId] = _TransportSlot(profileId);
    }
  }

  final Map<String, _TransportSlot> _slots = {};

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    final slot = _slots[session.activeProfileId]!;
    slot.openStarted.complete();
    await slot.openGate.future;
    return slot.connection;
  }

  Future<void> waitForOpen(String profileId) =>
      _slot(profileId).openStarted.future;

  void releaseOpen(String profileId) {
    final gate = _slot(profileId).openGate;
    if (!gate.isCompleted) gate.complete();
  }

  _ControlledVoiceRealtimeConnection connection(String profileId) =>
      _slot(profileId).connection;

  _TransportSlot _slot(String profileId) => _slots[profileId]!;

  Future<void> dispose() async {
    for (final slot in _slots.values) {
      await slot.connection.closeEvents();
    }
  }
}

class _TransportSlot {
  _TransportSlot(String profileId)
    : connection = _ControlledVoiceRealtimeConnection(profileId);

  final Completer<void> openStarted = Completer<void>();
  final Completer<void> openGate = Completer<void>();
  final _ControlledVoiceRealtimeConnection connection;
}

class _ControlledVoiceRealtimeConnection extends VoiceRealtimeConnection {
  _ControlledVoiceRealtimeConnection(this.profileId)
    : super(uri: Uri.parse('ws://transport.test/ws'), headers: const {});

  final String profileId;
  final StreamController<RealtimeFrame> _frames =
      StreamController<RealtimeFrame>.broadcast(sync: true);

  @override
  Stream<RealtimeFrame> get events => _frames.stream;

  @override
  Future<void> connect() async {}

  @override
  Future<void> dispose() async {}

  void addHello() {
    if (!_frames.isClosed) {
      _frames.add(const RealtimeFrame(op: 'hello', sequence: 1));
    }
  }

  Future<void> closeEvents() async {
    if (!_frames.isClosed) await _frames.close();
  }
}
