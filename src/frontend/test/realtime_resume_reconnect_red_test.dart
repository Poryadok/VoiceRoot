import 'dart:async';

import 'package:fake_async/fake_async.dart';
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
  test(
    'refreshed access token replaces the Realtime connection binding',
    () async {
      final harness = _ResumeReconnectHarness(autoConnect: true);
      addTearDown(harness.dispose);

      await harness.connectInitial();
      harness.auth.state = const AuthState(session: _rotatedSession);

      await harness.transport
          .waitForConnect(1)
          .timeout(const Duration(seconds: 2));
      expect(harness.transport.sessions[1], _rotatedSession);
      harness.connection(1).addHello();
      await pumpEventQueue();
      expect(
        harness.container.read(realtimeLinkStatusProvider),
        RealtimeLinkStatus.connected,
      );
    },
  );

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
    'pre-hello failures retain exponential backoff until accepted hello',
    () {
      late _ResumeReconnectHarness harness;
      fakeAsync((clock) {
        final timeline = <String>[];
        void record(String event) =>
            timeline.add('t=${clock.elapsed.inMilliseconds}ms $event');
        void snapshot(String label) => record(
          '$label connections=${harness.transport.connections.length} '
          'status=${harness.container.read(realtimeLinkStatusProvider)}',
        );

        harness = _ResumeReconnectHarness(trace: record);
        try {
          unawaited(harness.hub.ensureConnected());
          clock.flushMicrotasks();
          snapshot('initial');
          expect(harness.transport.connections, hasLength(1));

          const delays = [1, 2, 4, 8, 16, 30, 30];
          var elapsedSeconds = 0;
          for (var attempt = 0; attempt < delays.length; attempt++) {
            harness
                .connection(attempt)
                .events
                .listen(
                  (_) {},
                  onDone: () => record('observer onDone connection=$attempt'),
                );
            record('request close connection=$attempt');
            unawaited(harness.connection(attempt).closeFrames());
            clock.flushMicrotasks();
            snapshot('after close connection=$attempt');

            final delay = delays[attempt];
            record(
              'expected pending timer attempt=${attempt + 1} '
              'deadline=${clock.elapsed.inMilliseconds + delay * 1000}ms',
            );
            clock.elapse(Duration(seconds: delay - 1));
            clock.flushMicrotasks();
            snapshot(
              'before retry deadline connection=$attempt delay=${delay}s',
            );
            expect(
              harness.transport.connections,
              hasLength(attempt + 1),
              reason:
                  'attempt ${attempt + 1} must wait ${delay}s; '
                  'timeline: ${timeline.join(' | ')}',
            );

            clock.elapse(const Duration(seconds: 1));
            clock.flushMicrotasks();
            elapsedSeconds += delay;
            snapshot('at retry deadline expectedAttempt=${attempt + 1}');
            expect(
              harness.transport.connections,
              hasLength(attempt + 2),
              reason:
                  'attempt ${attempt + 2} starts at t=$elapsedSeconds; '
                  'timeline: ${timeline.join(' | ')}',
            );
          }

          final recovered = harness.connection(delays.length);
          recovered.addHello();
          clock.flushMicrotasks();
          unawaited(recovered.closeFrames());
          clock.flushMicrotasks();

          clock.elapse(const Duration(milliseconds: 999));
          clock.flushMicrotasks();
          expect(harness.transport.connections, hasLength(delays.length + 1));
          clock.elapse(const Duration(milliseconds: 1));
          clock.flushMicrotasks();
          expect(
            harness.transport.connections,
            hasLength(delays.length + 2),
            reason: 'the first disconnect after accepted hello starts at 1s',
          );
        } finally {
          harness.container.dispose();
          for (final connection in harness.transport.connections) {
            unawaited(connection.closeFrames());
          }
          clock.flushMicrotasks();
        }
      });
    },
  );

  test('manual retry is single-flight and uses the current session', () async {
    final harness = _ResumeReconnectHarness();
    addTearDown(harness.dispose);

    await harness.connectInitial();
    await harness.connection(0).closeFrames();
    await pumpEventQueue();

    final first = harness.hub.retryCurrentSession();
    final duplicate = harness.hub.retryCurrentSession();
    await harness.transport.waitForConnect(1);
    await Future.wait([first, duplicate]);

    expect(harness.transport.sessions, [_session, _session]);
    expect(harness.transport.connections, hasLength(2));
    expect(harness.hub.canRetryCurrentSession, isFalse);
    harness.connection(1).addHello();
    await pumpEventQueue();
    expect(
      harness.container.read(realtimeLinkStatusProvider),
      RealtimeLinkStatus.connected,
    );
  });

  test('a session change invalidates a pending manual retry', () async {
    final harness = _ResumeReconnectHarness();
    addTearDown(harness.dispose);

    await harness.connectInitial();
    await harness.connection(0).closeFrames();
    await pumpEventQueue();

    final staleRetry = harness.hub.retryCurrentSession();
    harness.auth.state = const AuthState(session: _rotatedSession);
    await staleRetry;
    expect(harness.transport.connections, hasLength(1));

    await harness.hub.ensureConnected();
    await harness.transport.waitForConnect(1);
    expect(harness.transport.sessions[1], _rotatedSession);
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

const _rotatedSession = AuthSession(
  accessToken: 'access-b',
  refreshToken: 'refresh-b',
  accountId: 'account-a',
  activeProfileId: 'profile-a',
  expiresInSeconds: 900,
);

class _ResumeReconnectHarness {
  _ResumeReconnectHarness({
    bool autoConnect = false,
    void Function(String event)? trace,
  }) : transport = _ResumeTransportFactory(trace) {
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
        realtimeAutoConnectProvider.overrideWithValue(autoConnect),
        realtimeTransportFactoryProvider.overrideWithValue(transport),
      ],
    );
    hub = container.read(realtimeHubProvider);
  }

  final _ResumeTransportFactory transport;
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
  _ResumeTransportFactory(this._trace);

  final void Function(String event)? _trace;
  final connections = <_ResumeConnection>[];
  final sessions = <AuthSession>[];

  @override
  Future<VoiceRealtimeConnection> open({
    required Uri uri,
    required AuthSession session,
  }) async {
    _trace?.call(
      'transport open attempt=${connections.length} '
      'session=${session.accountId}/${session.activeProfileId}',
    );
    sessions.add(session);
    final connection = _ResumeConnection(connections.length, _trace);
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
  _ResumeConnection(this.attempt, this._trace)
    : super(uri: Uri.parse('ws://transport.test/ws'), headers: const {});

  final int attempt;
  final void Function(String event)? _trace;
  final connectStarted = Completer<void>();
  final _frames = StreamController<RealtimeFrame>.broadcast(sync: true);
  final resumeLastSequences = <int>[];

  @override
  Stream<RealtimeFrame> get events => _frames.stream;

  @override
  Future<void> connect() async {
    _trace?.call('connect start attempt=$attempt');
    if (!connectStarted.isCompleted) connectStarted.complete();
    _trace?.call('connect complete attempt=$attempt');
  }

  @override
  Future<void> dispose() async {
    _trace?.call('dispose attempt=$attempt');
  }

  // Expected RealtimeTransport contract: resume data comes from the retired
  // connection and is supplied explicitly to this fresh transport.
  @override
  void sendResume({required int lastSequence}) {
    resumeLastSequences.add(lastSequence);
  }

  void addHello() {
    _trace?.call('hello attempt=$attempt');
    _frames.add(const RealtimeFrame(op: 'hello', sequence: 1));
  }

  void addFrame(RealtimeFrame frame) => _frames.add(frame);

  Future<void> closeFrames() async {
    if (!_frames.isClosed) {
      _trace?.call('stream close start attempt=$attempt');
      await _frames.close();
      _trace?.call('stream close complete attempt=$attempt');
    }
  }
}
