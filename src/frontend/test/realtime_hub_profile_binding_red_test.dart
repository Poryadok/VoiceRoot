import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';

import 'support/gateway_test_client.dart';

/// T-053 Cycle 3 RED contract.
///
/// The only intentionally missing production API is the controlled external
/// transport seam below: [RealtimeTransportFactory] and
/// [realtimeTransportFactoryProvider]. Its production adapter must preserve
/// the existing web ticket/native-header rules while receiving the immutable
/// session for one connection attempt. This test does not replace the real
/// [RealtimeHub], [ProfileSwitchCoordinator], [AuthController], or container.
///
/// The profile-switch coordinator owns its generation. The hub must bind every
/// asynchronous transport continuation to the corresponding immutable session:
/// late B ticket/connect/hello/frame/error/done signals must never displace an
/// already connected C binding. Reconciliation belongs to T-052/Cycle 4, so a
/// Cycle 3 transport test must never call `/chats`.
void main() {
  for (final lateSignal in _LateBSignal.values) {
    test(
      'late B ${lateSignal.label} after C keeps C transport binding authoritative',
      () async {
        final harness = _RealtimeHubHarness(lateSignal);
        addTearDown(harness.dispose);

        await harness.connectA();
        final b = harness.coordinator.switchTo('profile-b');
        await harness.transport.waitForOpen('profile-b');

        if (lateSignal == _LateBSignal.connect) {
          harness.transport.releaseOpen('profile-b');
          await harness.transport.waitForConnect('profile-b');
        } else if (lateSignal != _LateBSignal.ticket) {
          harness.transport.releaseOpen('profile-b');
          await b;
        }

        final c = harness.coordinator.switchTo('profile-c');
        await harness.transport.waitForOpen('profile-c');
        harness.transport.releaseOpen('profile-c');
        await c;
        harness.connection('profile-c').addHello();
        await pumpEventQueue();

        harness.hub.ensureSubscribed('chat-c');
        await pumpEventQueue();
        final eventCountBeforeLateB = harness.events.length;

        await harness.releaseLateB(lateSignal, b);
        await pumpEventQueue();

        expect(harness.controller.state.session, _session('profile-c'));
        expect(
          harness.container.read(authorizationHeaderProvider),
          'Bearer access-profile-c',
        );
        expect(await harness.storage.read(), _session('profile-c'));
        expect(
          harness.transport.requestFor('profile-c').session,
          _session('profile-c'),
          reason:
              'the C transport must bind the C session, never B credentials',
        );
        expect(
          harness.container.read(realtimeLinkStatusProvider),
          RealtimeLinkStatus.connected,
        );
        expect(harness.hub.subscribedChatIds, {'chat-c'});
        expect(harness.connection('profile-c').subscribedChatIds, ['chat-c']);
        expect(harness.connection('profile-c').disposed, isFalse);
        expect(
          harness.connection('profile-b').subscribedChatIds,
          isNot(contains('chat-c')),
          reason: 'a late B continuation must not replay C subscriptions',
        );
        expect(
          harness.connection('profile-b').disposed,
          isTrue,
          reason: 'a superseded B transport must be retired, not made active',
        );
        expect(
          harness.events.length,
          eventCountBeforeLateB,
          reason: 'late B frames are not events for profile C',
        );
        expect(
          harness.gatewayPaths.where((path) => path.contains('/chats')),
          isEmpty,
          reason: 'Cycle 3 must not start T-052 reconciliation',
        );
      },
    );
  }
}

enum _LateBSignal {
  ticket('ticket'),
  connect('connect'),
  hello('hello'),
  frame('frame'),
  error('error'),
  done('done');

  const _LateBSignal(this.label);

  final String label;
}

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'access-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-1',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

class _RealtimeHubHarness {
  _RealtimeHubHarness(this.lateSignal)
    : storage = _RecordingAuthSessionStorage(_session('profile-a')) {
    final client = MockClient(_respond);
    transport = _ControlledRealtimeTransportFactory(lateSignal);
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
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeTransportFactoryProvider.overrideWithValue(transport),
      ],
    );
    hub = container.read(realtimeHubProvider);
    coordinator = container.read(profileSwitchCoordinatorProvider);
    _eventSubscription = hub.events.listen(events.add);
  }

  final _LateBSignal lateSignal;
  final _RecordingAuthSessionStorage storage;
  final List<String> gatewayPaths = [];
  final List<RealtimeFrame> events = [];
  late final _ControlledRealtimeTransportFactory transport;
  late final AuthController controller;
  late final ProviderContainer container;
  late final RealtimeHub hub;
  late final ProfileSwitchCoordinator coordinator;
  late final StreamSubscription<RealtimeFrame> _eventSubscription;

  Future<void> connectA() async {
    final connecting = hub.ensureConnected();
    await transport.waitForOpen('profile-a');
    transport.releaseOpen('profile-a');
    await connecting;
    connection('profile-a').addHello();
    await pumpEventQueue();
    expect(
      container.read(realtimeLinkStatusProvider),
      RealtimeLinkStatus.connected,
    );
  }

  _ControlledVoiceRealtimeConnection connection(String profileId) =>
      transport.connection(profileId);

  Future<void> releaseLateB(
    _LateBSignal signal,
    Future<ProfileSwitchResult> b,
  ) async {
    switch (signal) {
      case _LateBSignal.ticket:
        transport.releaseOpen('profile-b');
        await b;
      case _LateBSignal.connect:
        transport.completeConnect('profile-b');
        await b;
      case _LateBSignal.hello:
        connection('profile-b').addHello();
      case _LateBSignal.frame:
        connection(
          'profile-b',
        ).add(const RealtimeFrame(op: 'message_create', sequence: 99));
      case _LateBSignal.error:
        connection('profile-b').addError(StateError('late-b'));
      case _LateBSignal.done:
        await connection('profile-b').closeEvents();
    }
  }

  Future<http.Response> _respond(http.Request request) async {
    gatewayPaths.add(request.url.path);
    if (request.url.path != '/api/v1/auth/switch-profile') {
      return http.Response('not found', 404);
    }
    final body = jsonDecode(request.body) as Map<String, dynamic>;
    final profileId = body['profile_id'] as String?;
    if (profileId != 'profile-b' && profileId != 'profile-c') {
      return http.Response('unknown profile', 400);
    }
    return utf8JsonResponse(jsonEncode(_session(profileId!).toJson()));
  }

  Future<void> dispose() async {
    await _eventSubscription.cancel();
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

/// Expected minimal production transport seam. It intentionally controls ticket
/// issuance and connect completion as one external operation, so a web ticket
/// response cannot re-enter a newer profile binding.
class _ControlledRealtimeTransportFactory implements RealtimeTransportFactory {
  _ControlledRealtimeTransportFactory(this._lateSignal) {
    for (final profileId in const ['profile-a', 'profile-b', 'profile-c']) {
      _slots[profileId] = _TransportSlot(
        profileId,
        holdConnect:
            profileId == 'profile-b' && _lateSignal == _LateBSignal.connect,
      );
    }
  }

  final _LateBSignal _lateSignal;
  final Map<String, _TransportSlot> _slots = {};
  final List<_TransportRequest> requests = [];

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    final slot = _slots[session.activeProfileId]!;
    requests.add(_TransportRequest(uri: uri, session: session));
    slot.openStarted.complete();
    await slot.openGate.future;
    return slot.connection;
  }

  Future<void> waitForOpen(String profileId) =>
      _slot(profileId).openStarted.future;

  Future<void> waitForConnect(String profileId) =>
      _slot(profileId).connection.connectStarted.future;

  void releaseOpen(String profileId) {
    final gate = _slot(profileId).openGate;
    if (!gate.isCompleted) gate.complete();
  }

  void completeConnect(String profileId) =>
      _slot(profileId).connection.completeConnect();

  _ControlledVoiceRealtimeConnection connection(String profileId) =>
      _slot(profileId).connection;

  _TransportRequest requestFor(String profileId) => requests.singleWhere(
    (request) => request.session.activeProfileId == profileId,
  );

  _TransportSlot _slot(String profileId) => _slots[profileId]!;

  Future<void> dispose() async {
    for (final slot in _slots.values) {
      await slot.connection.closeEvents();
    }
  }
}

class _TransportSlot {
  _TransportSlot(this.profileId, {required bool holdConnect})
    : connection = _ControlledVoiceRealtimeConnection(
        profileId,
        holdConnect: holdConnect,
      );

  final String profileId;
  final Completer<void> openStarted = Completer<void>();
  final Completer<void> openGate = Completer<void>();
  final _ControlledVoiceRealtimeConnection connection;
}

class _TransportRequest {
  const _TransportRequest({required this.uri, required this.session});

  final Uri uri;
  final AuthSession session;
}

class _ControlledVoiceRealtimeConnection extends VoiceRealtimeConnection {
  _ControlledVoiceRealtimeConnection(
    this.profileId, {
    required bool holdConnect,
  }) : _connectGate = holdConnect ? Completer<void>() : null,
       super(uri: Uri.parse('ws://transport.test/ws'), headers: const {});

  final String profileId;
  final Completer<void>? _connectGate;
  final Completer<void> connectStarted = Completer<void>();
  final StreamController<RealtimeFrame> _frames =
      StreamController<RealtimeFrame>.broadcast(sync: true);
  final List<String> subscribedChatIds = [];
  var disposed = false;

  @override
  Stream<RealtimeFrame> get events => _frames.stream;

  @override
  Future<void> connect() async {
    if (!connectStarted.isCompleted) connectStarted.complete();
    await _connectGate?.future;
  }

  @override
  void sendSubscribe(String chatId) {
    subscribedChatIds.add(chatId);
  }

  @override
  Future<void> dispose() async {
    disposed = true;
  }

  void completeConnect() {
    final gate = _connectGate;
    if (gate != null && !gate.isCompleted) gate.complete();
  }

  void addHello() => add(const RealtimeFrame(op: 'hello', sequence: 1));

  void add(RealtimeFrame frame) {
    if (!_frames.isClosed) _frames.add(frame);
  }

  void addError(Object error) {
    if (!_frames.isClosed) _frames.addError(error);
  }

  Future<void> closeEvents() async {
    if (!_frames.isClosed) await _frames.close();
  }
}
