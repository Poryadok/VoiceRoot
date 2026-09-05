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

    test(
      'late reconciler mount consumes one already accepted current B hello',
      () async {
        final harness = _Cycle4Harness(createReconciler: false);
        addTearDown(harness.dispose);

        await harness.switchToBWithoutHello();
        harness.connection('profile-b').addHello();
        await pumpEventQueue();

        expect(
          harness.bCalls,
          isEmpty,
          reason:
              'the B hello is accepted and stored by the real hub before '
              'this test creates InboxReconciler',
        );

        harness.mountReconciler();
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
                call.cursor == null,
          ),
          isTrue,
        );
        expect(harness.messages.getCalls, isEmpty);

        final subscription = harness.container.listen<InboxReconcilerState>(
          inboxReconcilerProvider,
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        harness.mountReconciler();
        await pumpEventQueue();

        expect(
          harness.bCalls,
          hasLength(3),
          reason:
              'repeated reads/listeners must not replay the stored hello or '
              'duplicate its one current-generation triplet',
        );
        expect(harness.messages.getCalls, isEmpty);
        expect(harness.chats.unmatchedCalls, isEmpty);
      },
    );

    test(
      'late mount ignores a hello published by a transport torn down for reconnect',
      () async {
        final harness = _Cycle4Harness(createReconciler: false);
        addTearDown(harness.dispose);

        await harness.connectAWithoutHello();
        harness.connection('profile-a').addHello();
        await pumpEventQueue();
        expect(
          harness.container.read(realtimeHelloBindingProvider),
          isNotNull,
          reason: 'the first current A transport accepted its hello',
        );

        // Supply enough external responses to make an erroneous replay before
        // replacement hello observable separately from the one allowed after.
        for (var snapshot = 0; snapshot < 2; snapshot++) {
          for (final inbox in ['main', 'requests', 'archive']) {
            harness.chats.enqueue(
              InboxChatPageScript(
                inbox: inbox,
                cursor: null,
                profileId: 'profile-a',
                authorization: 'Bearer access-profile-a',
                result: ChatsApiOk(ChatListData(items: [])),
              ),
            );
          }
        }

        final reconnect = harness.hub.reconnectWithNewSession();
        await harness.transport.waitForOpen('profile-a', attempt: 1);
        harness.transport.releaseOpen('profile-a', attempt: 1);
        await reconnect;
        expect(
          harness.connection('profile-a', attempt: 1),
          isNot(same(harness.connection('profile-a'))),
          reason: 'the replacement hello must belong to a new transport',
        );

        expect(
          harness.container.read(realtimeHelloBindingProvider),
          isNull,
          reason:
              'tearing down the accepted transport must retire its published '
              'hello before a replacement transport proves the same session',
        );
        final callsBeforeLateMount = harness.chats.calls.length;
        harness.mountReconciler();
        await pumpEventQueue();
        expect(harness.chats.calls, hasLength(callsBeforeLateMount));
        expect(harness.messages.getCalls, isEmpty);

        harness.connection('profile-a', attempt: 1).addHello();
        harness.connection('profile-a', attempt: 1).addHello();
        await pumpEventQueue();

        final replacementCalls = harness.chats.calls
            .skip(callsBeforeLateMount)
            .toList(growable: false);
        expect(replacementCalls, hasLength(3));
        expect(replacementCalls.map((call) => call.inbox).toSet(), {
          'main',
          'requests',
          'archive',
        });
        expect(
          replacementCalls.every(
            (call) =>
                call.authorization == 'Bearer access-profile-a' &&
                call.profileId == 'profile-a' &&
                call.cursor == null,
          ),
          isTrue,
        );
        expect(harness.messages.getCalls, isEmpty);
        expect(harness.chats.unmatchedCalls, isEmpty);
      },
    );

    test(
      'a delayed retired A1 teardown cannot clobber accepted replacement A2',
      () async {
        final harness = _Cycle4Harness(createReconciler: false);
        addTearDown(harness.dispose);
        final frames = <RealtimeFrame>[];
        final frameSubscription = harness.hub.events.listen(frames.add);
        addTearDown(frameSubscription.cancel);
        final providerFrames = <RealtimeFrame>[];
        final providerSubscription = harness.container
            .listen<AsyncValue<RealtimeFrame>>(
              realtimeEventProvider,
              (_, next) => next.whenData(providerFrames.add),
              fireImmediately: true,
            );
        addTearDown(providerSubscription.close);

        await harness.connectAWithoutHello();
        final a1 = harness.connection('profile-a');
        a1.addHello();
        harness.hub.ensureSubscribed('retained-chat');
        await pumpEventQueue();
        expect(a1.subscribedChatIds, ['retained-chat']);

        for (final inbox in ['main', 'requests', 'archive']) {
          harness.chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-profile-a',
              result: const ChatsApiOk(ChatListData(items: [])),
            ),
          );
        }

        a1.blockFirstDispose();
        addTearDown(a1.releaseFirstDispose);
        final firstReconnect = harness.hub.reconnectWithNewSession();
        await a1.waitForFirstDispose();

        final replacementReconnect = harness.hub.reconnectWithNewSession();
        await harness.transport.waitForOpen('profile-a', attempt: 1);
        harness.transport.releaseOpen('profile-a', attempt: 1);
        await replacementReconnect;
        final a2 = harness.connection('profile-a', attempt: 1);
        expect(a2, isNot(same(a1)));
        expect(a2.subscribedChatIds, ['retained-chat']);

        a2.addHello();
        await pumpEventQueue();
        harness.mountReconciler();
        await pumpEventQueue();
        expect(harness.chats.calls.skip(1), hasLength(3));
        expect(harness.messages.getCalls, isEmpty);
        final framesBeforeA1Release = frames.length;
        final providerFramesBeforeA1Release = providerFrames.length;
        final a2HelloBinding = harness.container.read(
          realtimeHelloBindingProvider,
        );
        expect(a2HelloBinding, isNotNull);

        a1.releaseFirstDispose();
        await firstReconnect;
        a2.addFrame(const RealtimeFrame(op: 'message_create', sequence: 2));
        await pumpEventQueue();

        expect(harness.hub.status, RealtimeLinkStatus.connected);
        expect(harness.hub.subscribedChatIds, {'retained-chat'});
        expect(a2.subscribedChatIds, ['retained-chat']);
        expect(
          frames,
          hasLength(framesBeforeA1Release + 1),
          reason:
              'A2 must remain the active event source after the retired A1 '
              'dispose completes',
        );
        expect(
          providerFrames,
          hasLength(providerFramesBeforeA1Release + 1),
          reason:
              'the provider-facing realtime event stream must remain bound to '
              'accepted A2 after retired A1 completes',
        );
        expect(
          harness.container.read(realtimeHelloBindingProvider),
          same(a2HelloBinding),
          reason:
              'a delayed retired teardown must not clear or replace A2 hello '
              'proof published to providers',
        );
        expect(
          harness.chats.calls.skip(1),
          hasLength(3),
          reason: 'a delayed A1 teardown must not replay the A2 snapshot',
        );
        expect(harness.messages.getCalls, isEmpty);
        expect(harness.chats.unmatchedCalls, isEmpty);
      },
    );

    test(
      'concurrent reconnect teardown owns retired A1 cleanup exactly once',
      () async {
        final harness = _Cycle4Harness(createReconciler: false);
        addTearDown(harness.dispose);
        final hubFrames = <RealtimeFrame>[];
        final hubSubscription = harness.hub.events.listen(hubFrames.add);
        addTearDown(hubSubscription.cancel);
        final providerFrames = <RealtimeFrame>[];
        final providerSubscription = harness.container
            .listen<AsyncValue<RealtimeFrame>>(
              realtimeEventProvider,
              (_, next) => next.whenData(providerFrames.add),
              fireImmediately: true,
            );
        addTearDown(providerSubscription.close);

        await harness.connectAWithoutHello();
        final a1 = harness.connection('profile-a');
        a1.addHello();
        harness.hub.ensureSubscribed('retained-chat');
        await pumpEventQueue();

        for (final inbox in ['main', 'requests', 'archive']) {
          harness.chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-profile-a',
              result: const ChatsApiOk(ChatListData(items: [])),
            ),
          );
        }

        a1.blockFirstDispose();
        addTearDown(a1.releaseFirstDispose);
        final firstReconnect = harness.hub.reconnectWithNewSession();
        await a1.waitForFirstDispose();

        final replacementReconnect = harness.hub.reconnectWithNewSession();
        await harness.transport.waitForOpen('profile-a', attempt: 1);
        harness.transport.releaseOpen('profile-a', attempt: 1);
        await replacementReconnect;
        final a2 = harness.connection('profile-a', attempt: 1);
        expect(a2, isNot(same(a1)));
        expect(harness.transport.openCount('profile-a'), 2);
        expect(a2.subscribedChatIds, ['retained-chat']);

        a2.addHello();
        await pumpEventQueue();
        harness.mountReconciler();
        await pumpEventQueue();
        expect(harness.chats.calls.skip(1), hasLength(3));
        final hubFramesBeforeA1Release = hubFrames.length;
        final providerFramesBeforeA1Release = providerFrames.length;
        final a2HelloBinding = harness.container.read(
          realtimeHelloBindingProvider,
        );
        expect(a2HelloBinding, isNotNull);

        a1.releaseFirstDispose();
        await firstReconnect;
        a2.addFrame(const RealtimeFrame(op: 'message_create', sequence: 3));
        await pumpEventQueue();

        expect(
          [a1.frameSubscriptionCancelCount, a1.disposeCount],
          [1, 1],
          reason:
              'only the teardown that captured A1 may cancel its stream and '
              'dispose its transport',
        );
        expect(harness.transport.openCount('profile-a'), 2);
        expect(harness.hub.status, RealtimeLinkStatus.connected);
        expect(harness.hub.subscribedChatIds, {'retained-chat'});
        expect(a2.subscribedChatIds, ['retained-chat']);
        expect(hubFrames, hasLength(hubFramesBeforeA1Release + 1));
        expect(providerFrames, hasLength(providerFramesBeforeA1Release + 1));
        expect(
          harness.container.read(realtimeHelloBindingProvider),
          same(a2HelloBinding),
        );
        expect(harness.chats.calls.skip(1), hasLength(3));
        expect(harness.messages.getCalls, isEmpty);
        expect(harness.chats.unmatchedCalls, isEmpty);
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
  _Cycle4Harness({bool createReconciler = true})
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
    if (createReconciler) mountReconciler();
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
    await connectAWithoutHello();

    final b = coordinator.switchTo('profile-b');
    await transport.waitForOpen('profile-b');
    transport.releaseOpen('profile-b');
    expect(await b, isA<ProfileSwitchApplied>());
    await pumpEventQueue();
  }

  _ControlledVoiceRealtimeConnection connection(
    String profileId, {
    int attempt = 0,
  }) => transport.connection(profileId, attempt: attempt);

  Future<void> connectAWithoutHello() async {
    await pumpEventQueue();
    expect(
      chats.calls.where((call) => call.profileId == 'profile-a'),
      hasLength(1),
    );

    final a = hub.ensureConnected();
    await transport.waitForOpen('profile-a');
    transport.releaseOpen('profile-a');
    await a;
  }

  void mountReconciler() {
    container.read(inboxReconcilerProvider);
  }

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
    final attempt = slot.nextOpen();
    attempt.openStarted.complete();
    await attempt.openGate.future;
    return attempt.connection;
  }

  Future<void> waitForOpen(String profileId, {int attempt = 0}) =>
      _slot(profileId).attempt(attempt).openStarted.future;

  void releaseOpen(String profileId, {int attempt = 0}) {
    final gate = _slot(profileId).attempt(attempt).openGate;
    if (!gate.isCompleted) gate.complete();
  }

  _ControlledVoiceRealtimeConnection connection(
    String profileId, {
    int attempt = 0,
  }) => _slot(profileId).attempt(attempt).connection;

  int openCount(String profileId) => _slot(profileId).openCount;

  _TransportSlot _slot(String profileId) => _slots[profileId]!;

  Future<void> dispose() async {
    for (final slot in _slots.values) {
      for (final attempt in slot.attempts) {
        await attempt.connection.closeEvents();
      }
    }
  }
}

class _TransportSlot {
  _TransportSlot(this.profileId);

  final String profileId;
  final List<_TransportAttempt> _attempts = [];
  var _nextOpenAttempt = 0;

  Iterable<_TransportAttempt> get attempts => _attempts;
  int get openCount => _nextOpenAttempt;

  _TransportAttempt nextOpen() {
    final attempt = this.attempt(_nextOpenAttempt);
    _nextOpenAttempt++;
    return attempt;
  }

  _TransportAttempt attempt(int index) {
    while (_attempts.length <= index) {
      _attempts.add(_TransportAttempt(profileId));
    }
    return _attempts[index];
  }
}

class _TransportAttempt {
  _TransportAttempt(String profileId)
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
  final List<String> subscribedChatIds = [];
  final Completer<void> _firstDisposeStarted = Completer<void>();
  Completer<void>? _firstDisposeGate;
  var _disposeCount = 0;
  var _frameSubscriptionCancelCount = 0;

  int get disposeCount => _disposeCount;
  int get frameSubscriptionCancelCount => _frameSubscriptionCancelCount;

  @override
  Stream<RealtimeFrame> get events => _CancelCountingStream(
    _frames.stream,
    onCancel: () => _frameSubscriptionCancelCount++,
  );

  @override
  Future<void> connect() async {}

  @override
  Future<void> dispose() async {
    _disposeCount++;
    if (_disposeCount != 1) return;
    if (!_firstDisposeStarted.isCompleted) _firstDisposeStarted.complete();
    final gate = _firstDisposeGate;
    if (gate != null) await gate.future;
  }

  @override
  void sendSubscribe(String chatId) {
    subscribedChatIds.add(chatId);
  }

  void addHello() {
    if (!_frames.isClosed) {
      _frames.add(const RealtimeFrame(op: 'hello', sequence: 1));
    }
  }

  void addFrame(RealtimeFrame frame) {
    if (!_frames.isClosed) _frames.add(frame);
  }

  void blockFirstDispose() {
    _firstDisposeGate ??= Completer<void>();
  }

  Future<void> waitForFirstDispose() => _firstDisposeStarted.future;

  void releaseFirstDispose() {
    final gate = _firstDisposeGate;
    if (gate != null && !gate.isCompleted) gate.complete();
  }

  Future<void> closeEvents() async {
    if (!_frames.isClosed) await _frames.close();
  }
}

class _CancelCountingStream<T> extends Stream<T> {
  _CancelCountingStream(this._delegate, {required this.onCancel});

  final Stream<T> _delegate;
  final void Function() onCancel;

  @override
  bool get isBroadcast => _delegate.isBroadcast;

  @override
  StreamSubscription<T> listen(
    void Function(T event)? onData, {
    Function? onError,
    void Function()? onDone,
    bool? cancelOnError,
  }) {
    return _CancelCountingSubscription<T>(
      _delegate.listen(
        onData,
        onError: onError,
        onDone: onDone,
        cancelOnError: cancelOnError,
      ),
      onCancel,
    );
  }
}

class _CancelCountingSubscription<T> implements StreamSubscription<T> {
  _CancelCountingSubscription(this._delegate, this._onCancel);

  final StreamSubscription<T> _delegate;
  final void Function() _onCancel;

  @override
  Future<E> asFuture<E>([E? futureValue]) => _delegate.asFuture(futureValue);

  @override
  Future<void> cancel() {
    _onCancel();
    return _delegate.cancel();
  }

  @override
  bool get isPaused => _delegate.isPaused;

  @override
  void onData(void Function(T data)? handleData) =>
      _delegate.onData(handleData);

  @override
  void onDone(void Function()? handleDone) => _delegate.onDone(handleDone);

  @override
  void onError(Function? handleError) => _delegate.onError(handleError);

  @override
  void pause([Future<void>? resumeSignal]) => _delegate.pause(resumeSignal);

  @override
  void resume() => _delegate.resume();
}
