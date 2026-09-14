import 'dart:async';

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

import 'support/gateway_test_client.dart';

/// RED contract for the documented Realtime reconnect protocol.
///
/// `last_s` is retained by the client across the retired transport. The new
/// transport must send it only after its own `hello`; `resume` is not a REST
/// history or inbox replay trigger. REST reconciliation remains covered by
/// `t103_reconnect_history_delta_red_test.dart`.
void main() {
  test('reconnect resumes the prior last_s only after the new hello', () async {
    final harness = _ResumeReconnectHarness();
    addTearDown(harness.dispose);

    await harness.connectInitial();
    harness
        .connection(0)
        .addFrame(const RealtimeFrame(op: 'message_create', sequence: 42));
    await pumpEventQueue();

    await harness.connection(0).closeFrames();
    await harness.transport.waitForConnect(1);

    expect(
      harness.connection(1).resumeLastSequences,
      isEmpty,
      reason: 'resume waits for the newly accepted hello',
    );

    harness.connection(1).addHello();
    await pumpEventQueue();

    expect(harness.connection(1).resumeLastSequences, [42]);
  });

  test(
    'reconnect retains last_s when an intermediate transport closes pre-hello',
    () async {
      final harness = _ResumeReconnectHarness();
      addTearDown(harness.dispose);

      await harness.connectInitial();
      harness
          .connection(0)
          .addFrame(const RealtimeFrame(op: 'message_create', sequence: 42));
      await pumpEventQueue();

      await harness.connection(0).closeFrames();
      await harness.transport.waitForConnect(1);
      await harness.connection(1).closeFrames();
      await harness.transport.waitForConnect(2);

      expect(harness.connection(2).resumeLastSequences, isEmpty);

      harness.connection(2).addHello();
      await pumpEventQueue();

      expect(harness.connection(2).resumeLastSequences, [42]);
    },
  );
}

const _session = AuthSession(
  accessToken: 'access-a',
  refreshToken: 'refresh-a',
  accountId: 'account-a',
  activeProfileId: 'profile-a',
  expiresInSeconds: 900,
);

class _ResumeReconnectHarness {
  _ResumeReconnectHarness() {
    final client = MockClient((_) async => http.Response('{}', 404));
    auth = AuthController(
      authClient: VoiceAuthClient(gateway: gatewayHttpForTest(client)),
      storage: InMemoryAuthSessionStorage(),
      guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
    )..state = const AuthState(session: _session);
    container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith((_) => auth),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeTransportFactoryProvider.overrideWithValue(transport),
      ],
    );
    hub = container.read(realtimeHubProvider);
  }

  final transport = _ResumeTransportFactory();
  late final AuthController auth;
  late final ProviderContainer container;
  late final RealtimeHub hub;

  _ResumeConnection connection(int attempt) => transport.connections[attempt];

  Future<void> connectInitial() async {
    final connecting = hub.ensureConnected();
    await transport.waitForConnect(0);
    await connecting;
    connection(0).addHello();
    await pumpEventQueue();
    expect(
      container.read(realtimeLinkStatusProvider),
      RealtimeLinkStatus.connected,
    );
  }

  Future<void> dispose() async {
    container.dispose();
    await transport.dispose();
  }
}

class _ResumeTransportFactory implements RealtimeTransportFactory {
  final connections = <_ResumeConnection>[];

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    final connection = _ResumeConnection(connections.length);
    connections.add(connection);
    return connection;
  }

  Future<void> waitForConnect(int attempt) async {
    while (connections.length <= attempt) {
      await Future<void>.delayed(Duration.zero);
    }
    await connections[attempt].connectStarted.future;
  }

  Future<void> dispose() async {
    for (final connection in connections) {
      await connection.closeFrames();
    }
  }
}

class _ResumeConnection extends VoiceRealtimeConnection {
  _ResumeConnection(this.attempt)
    : super(uri: Uri.parse('ws://transport.test/ws'), headers: const {});

  final int attempt;
  final connectStarted = Completer<void>();
  final _frames = StreamController<RealtimeFrame>.broadcast(sync: true);
  final resumeLastSequences = <int>[];

  @override
  Stream<RealtimeFrame> get events => _frames.stream;

  @override
  Future<void> connect() async {
    if (!connectStarted.isCompleted) connectStarted.complete();
  }

  @override
  Future<void> dispose() async {}

  // Expected RealtimeTransport contract: resume data comes from the retired
  // connection and is supplied explicitly to this fresh transport.
  @override
  void sendResume({required int lastSequence}) {
    resumeLastSequences.add(lastSequence);
  }

  void addHello() => _frames.add(const RealtimeFrame(op: 'hello', sequence: 1));

  void addFrame(RealtimeFrame frame) => _frames.add(frame);

  Future<void> closeFrames() async {
    if (!_frames.isClosed) await _frames.close();
  }
}
