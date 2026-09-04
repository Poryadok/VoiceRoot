import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session.dart';

/// T-053 RED contract. The reference fixture is deliberately test-only: the
/// first test remains RED until production exposes the same adapter.
void main() {
  group('Atomic profile-switch orchestrator', () {
    test('production adapter is available to every profile UI entry point', () {
      final harness = _Harness.withSession(_session('profile-a'));

      // RED: replace this test-only placeholder with the production adapter.
      expect(
        () => _productionCoordinatorUnderTest(harness.dependencies),
        returnsNormally,
      );
    });

    test('all profile UI entry points use the common coordinator provider', () {
      const entryPoints = [
        'lib/ui/profile/profile_switcher.dart',
        'lib/ui/profile/profile_avatar_menu.dart',
        'lib/ui/profile/profile_avatar_switcher.dart',
        'lib/ui/profile/create_profile_sheet.dart',
      ];

      for (final path in entryPoints) {
        expect(
          File(path).readAsStringSync(),
          contains('profileSwitchCoordinatorProvider'),
          reason: '$path must not bypass the T-053 switch coordinator',
        );
      }
    });

    test(
      'one success persists, commits, reconnects and snapshots exactly once',
      () async {
        final harness = _Harness.withSession(_session('profile-a'));
        harness.auth.succeed('profile-b', _session('profile-b'));
        final coordinator = _ContractCoordinator(harness.dependencies);

        final result = await coordinator.switchTo('profile-b');

        expect(result, const AtomicProfileSwitchSuccess(generation: 1));
        expect(harness.storage.writes, [_session('profile-b')]);
        expect(harness.context.sessionTransitions, [_session('profile-b')]);
        expect(harness.realtime.reconnects, [_session('profile-b')]);
        expect(harness.inbox.requests, const [
          InboxReconcileRequest(profileId: 'profile-b', generation: 1),
        ]);
        expect(harness.trace, [
          'retire',
          'persist:profile-b',
          'commit:profile-b',
          'reconnect:profile-b',
        ]);
      },
    );

    test(
      'canonical Auth failure preserves all A state and starts no handoff',
      () async {
        final oldSession = _session('profile-a');
        final harness = _Harness.withSession(
          oldSession,
          selectedChatId: 'chat-a',
          subscribedChatIds: const {'chat-a', 'other-a'},
        );
        harness.auth.fail('profile-b', 'profile_not_switchable');
        final coordinator = _ContractCoordinator(harness.dependencies);

        final result = await coordinator.switchTo('profile-b');

        expect(
          result,
          const AtomicProfileSwitchFailure('profile_not_switchable'),
        );
        expect(harness.storage.writes, isEmpty);
        expect(harness.context.sessionTransitions, isEmpty);
        expect(harness.context.session, oldSession);
        expect(harness.storage.persisted, oldSession);
        expect(harness.context.selectedChatId, 'chat-a');
        expect(harness.context.subscribedChatIds, {'chat-a', 'other-a'});
        expect(harness.realtime.reconnects, isEmpty);
        expect(harness.inbox.requests, isEmpty);
        expect(harness.trace, isEmpty);
      },
    );

    test('late B Auth cannot overwrite C', () async {
      final harness = _Harness.withSession(_session('profile-a'));
      final bAuth = harness.auth.pause('profile-b');
      harness.auth.succeed('profile-c', _session('profile-c'));
      final coordinator = _ContractCoordinator(harness.dependencies);

      final b = coordinator.switchTo('profile-b');
      final c = coordinator.switchTo('profile-c');
      await c;
      bAuth.complete(_SwitchAuthSuccess(_session('profile-b')));
      expect(await b, const AtomicProfileSwitchSuperseded());

      harness.inbox
          .completer('profile-c', 2)
          .complete(
            const InboxReconciliation(profileId: 'profile-c', generation: 2),
          );
      await _drainMicrotasks();

      expect(harness.context.session, _session('profile-c'));
      expect(harness.inbox.requests, const [
        InboxReconcileRequest(profileId: 'profile-c', generation: 2),
      ]);
      expect(harness.context.inboxCommits, const [
        InboxReconciliation(profileId: 'profile-c', generation: 2),
      ]);
    });

    test('late B snapshot is ignored after a committed C transition', () async {
      final harness = _Harness.withSession(_session('profile-a'));
      harness.auth.succeed('profile-b', _session('profile-b'));
      harness.auth.succeed('profile-c', _session('profile-c'));
      final coordinator = _ContractCoordinator(harness.dependencies);

      await coordinator.switchTo('profile-b');
      await coordinator.switchTo('profile-c');
      harness.inbox
          .completer('profile-c', 2)
          .complete(
            const InboxReconciliation(profileId: 'profile-c', generation: 2),
          );
      await _drainMicrotasks();
      harness.inbox
          .completer('profile-b', 1)
          .complete(
            const InboxReconciliation(profileId: 'profile-b', generation: 1),
          );
      await _drainMicrotasks();

      expect(harness.context.inboxCommits, const [
        InboxReconciliation(profileId: 'profile-c', generation: 2),
      ]);
    });

    test('retires A selection and subscriptions before successor WS', () async {
      final harness = _Harness.withSession(
        _session('profile-a'),
        selectedChatId: 'chat-owned-by-a',
        subscribedChatIds: const {'chat-owned-by-a', 'also-owned-by-a'},
      );
      harness.auth.succeed('profile-b', _session('profile-b'));
      final coordinator = _ContractCoordinator(harness.dependencies);

      await coordinator.switchTo('profile-b');

      expect(harness.context.selectedChatId, isNull);
      expect(harness.context.subscribedChatIds, isEmpty);
      expect(harness.realtime.handoffs, [
        RealtimeProfileHandoff(
          retiredSelectedChatId: 'chat-owned-by-a',
          retiredSubscriptionIds: {'chat-owned-by-a', 'also-owned-by-a'},
        ),
      ]);
      expect(
        harness.trace.indexOf('retire'),
        lessThan(harness.trace.indexOf('reconnect:profile-b')),
      );
    });
  });
}

Future<void> _drainMicrotasks() => Future<void>.delayed(Duration.zero);

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'access-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-1',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

AtomicProfileSwitchCoordinator _productionCoordinatorUnderTest(
  _AtomicProfileSwitchDependencies dependencies,
) => throw UnsupportedError(
  'T-053 production AtomicProfileSwitchCoordinator adapter is not implemented',
);

abstract interface class AtomicProfileSwitchCoordinator {
  Future<AtomicProfileSwitchResult> switchTo(String profileId);
}

sealed class AtomicProfileSwitchResult {
  const AtomicProfileSwitchResult();
}

final class AtomicProfileSwitchSuccess extends AtomicProfileSwitchResult {
  const AtomicProfileSwitchSuccess({required this.generation});

  final int generation;

  @override
  bool operator ==(Object other) =>
      other is AtomicProfileSwitchSuccess && other.generation == generation;

  @override
  int get hashCode => generation.hashCode;
}

final class AtomicProfileSwitchFailure extends AtomicProfileSwitchResult {
  const AtomicProfileSwitchFailure(this.errorCode);

  final String errorCode;

  @override
  bool operator ==(Object other) =>
      other is AtomicProfileSwitchFailure && other.errorCode == errorCode;

  @override
  int get hashCode => errorCode.hashCode;
}

final class AtomicProfileSwitchSuperseded extends AtomicProfileSwitchResult {
  const AtomicProfileSwitchSuperseded();

  @override
  bool operator ==(Object other) => other is AtomicProfileSwitchSuperseded;

  @override
  int get hashCode => 0;
}

class _ContractCoordinator implements AtomicProfileSwitchCoordinator {
  _ContractCoordinator(this._dependencies);

  final _AtomicProfileSwitchDependencies _dependencies;
  int _generation = 0;
  final Set<String> _committedSnapshots = {};

  @override
  Future<AtomicProfileSwitchResult> switchTo(String profileId) async {
    final generation = ++_generation;
    final result = await _dependencies.auth.switchProfile(
      current: _dependencies.context.session,
      profileId: profileId,
    );
    if (generation != _generation) return const AtomicProfileSwitchSuperseded();
    if (result case _SwitchAuthFailure(:final errorCode)) {
      return AtomicProfileSwitchFailure(errorCode);
    }
    final session = (result as _SwitchAuthSuccess).session;
    final handoff = _dependencies.context.retireTextContext();
    await _dependencies.storage.write(session);
    _dependencies.context.replaceSession(session);
    await _dependencies.realtime.reconnect(
      successorSession: session,
      handoff: handoff,
    );
    unawaited(_acceptSnapshot(profileId: profileId, generation: generation));
    return AtomicProfileSwitchSuccess(generation: generation);
  }

  Future<void> _acceptSnapshot({
    required String profileId,
    required int generation,
  }) async {
    final reconciliation = await _dependencies.inbox.reconcile(
      profileId: profileId,
      generation: generation,
    );
    final key = '$profileId:$generation';
    if (generation != _generation ||
        _dependencies.context.session.activeProfileId != profileId ||
        !_committedSnapshots.add(key)) {
      return;
    }
    _dependencies.context.commitInbox(reconciliation);
  }
}

class _Harness {
  _Harness.withSession(
    AuthSession session, {
    String? selectedChatId,
    Set<String> subscribedChatIds = const {},
  }) : trace = [],
       storage = _RecordingSessionStorage(session, []),
       context = _RecordingProfileContext(
         session,
         [],
         selectedChatId: selectedChatId,
         subscribedChatIds: subscribedChatIds,
       ),
       auth = _FakeSwitchAuth(),
       inbox = _ManualInboxReconciler() {
    realtime = _RecordingRealtime(trace);
    storage._trace = trace;
    context._trace = trace;
  }

  final List<String> trace;
  final _RecordingSessionStorage storage;
  final _RecordingProfileContext context;
  final _FakeSwitchAuth auth;
  late final _RecordingRealtime realtime;
  final _ManualInboxReconciler inbox;

  _AtomicProfileSwitchDependencies get dependencies =>
      _AtomicProfileSwitchDependencies(
        auth: auth,
        storage: storage,
        context: context,
        realtime: realtime,
        inbox: inbox,
      );
}

class _AtomicProfileSwitchDependencies {
  const _AtomicProfileSwitchDependencies({
    required this.auth,
    required this.storage,
    required this.context,
    required this.realtime,
    required this.inbox,
  });

  final _SwitchAuth auth;
  final _SessionStorage storage;
  final _ProfileContext context;
  final _ProfileRealtime realtime;
  final _InboxReconciler inbox;
}

abstract interface class _SwitchAuth {
  Future<_SwitchAuthResult> switchProfile({
    required AuthSession current,
    required String profileId,
  });
}

sealed class _SwitchAuthResult {
  const _SwitchAuthResult();
}

final class _SwitchAuthSuccess extends _SwitchAuthResult {
  const _SwitchAuthSuccess(this.session);

  final AuthSession session;
}

final class _SwitchAuthFailure extends _SwitchAuthResult {
  const _SwitchAuthFailure(this.errorCode);

  final String errorCode;
}

class _FakeSwitchAuth implements _SwitchAuth {
  final Map<String, Future<_SwitchAuthResult>> _responses = {};

  void succeed(String profileId, AuthSession session) =>
      _responses[profileId] = Future.value(_SwitchAuthSuccess(session));

  void fail(String profileId, String errorCode) =>
      _responses[profileId] = Future.value(_SwitchAuthFailure(errorCode));

  Completer<_SwitchAuthResult> pause(String profileId) {
    final completer = Completer<_SwitchAuthResult>();
    _responses[profileId] = completer.future;
    return completer;
  }

  @override
  Future<_SwitchAuthResult> switchProfile({
    required AuthSession current,
    required String profileId,
  }) =>
      _responses[profileId] ??
      Future.error(StateError('No Auth response for $profileId'));
}

abstract interface class _SessionStorage {
  Future<void> write(AuthSession session);
}

class _RecordingSessionStorage implements _SessionStorage {
  _RecordingSessionStorage(this.persisted, this._trace);

  AuthSession? persisted;
  List<String> _trace;
  final List<AuthSession> writes = [];

  @override
  Future<void> write(AuthSession session) async {
    writes.add(session);
    persisted = session;
    _trace.add('persist:${session.activeProfileId}');
  }
}

abstract interface class _ProfileContext {
  AuthSession get session;
  String? get selectedChatId;
  Set<String> get subscribedChatIds;
  void replaceSession(AuthSession session);
  RealtimeProfileHandoff retireTextContext();
  void commitInbox(InboxReconciliation reconciliation);
}

class _RecordingProfileContext implements _ProfileContext {
  _RecordingProfileContext(
    this._session,
    this._trace, {
    String? selectedChatId,
    Set<String> subscribedChatIds = const {},
  }) : _selectedChatId = selectedChatId,
       _subscribedChatIds = {...subscribedChatIds};

  AuthSession _session;
  List<String> _trace;
  String? _selectedChatId;
  final Set<String> _subscribedChatIds;
  final List<AuthSession> sessionTransitions = [];
  final List<InboxReconciliation> inboxCommits = [];

  @override
  AuthSession get session => _session;

  @override
  String? get selectedChatId => _selectedChatId;

  @override
  Set<String> get subscribedChatIds => Set.unmodifiable(_subscribedChatIds);

  @override
  void replaceSession(AuthSession session) {
    sessionTransitions.add(session);
    _session = session;
    _trace.add('commit:${session.activeProfileId}');
  }

  @override
  RealtimeProfileHandoff retireTextContext() {
    final handoff = RealtimeProfileHandoff(
      retiredSelectedChatId: _selectedChatId,
      retiredSubscriptionIds: _subscribedChatIds,
    );
    _selectedChatId = null;
    _subscribedChatIds.clear();
    _trace.add('retire');
    return handoff;
  }

  @override
  void commitInbox(InboxReconciliation reconciliation) {
    inboxCommits.add(reconciliation);
  }
}

class RealtimeProfileHandoff {
  RealtimeProfileHandoff({
    required this.retiredSelectedChatId,
    required Set<String> retiredSubscriptionIds,
  }) : retiredSubscriptionIds = Set.unmodifiable(retiredSubscriptionIds);

  final String? retiredSelectedChatId;
  final Set<String> retiredSubscriptionIds;

  @override
  bool operator ==(Object other) =>
      other is RealtimeProfileHandoff &&
      other.retiredSelectedChatId == retiredSelectedChatId &&
      _sameSet(other.retiredSubscriptionIds, retiredSubscriptionIds);

  @override
  int get hashCode => Object.hash(
    retiredSelectedChatId,
    Object.hashAll(retiredSubscriptionIds.toList()..sort()),
  );
}

bool _sameSet(Set<String> a, Set<String> b) =>
    a.length == b.length && a.containsAll(b);

abstract interface class _ProfileRealtime {
  Future<void> reconnect({
    required AuthSession successorSession,
    required RealtimeProfileHandoff handoff,
  });
}

class _RecordingRealtime implements _ProfileRealtime {
  _RecordingRealtime(this._trace);

  final List<String> _trace;
  final List<AuthSession> reconnects = [];
  final List<RealtimeProfileHandoff> handoffs = [];

  @override
  Future<void> reconnect({
    required AuthSession successorSession,
    required RealtimeProfileHandoff handoff,
  }) async {
    reconnects.add(successorSession);
    handoffs.add(handoff);
    _trace.add('reconnect:${successorSession.activeProfileId}');
  }
}

class InboxReconcileRequest {
  const InboxReconcileRequest({
    required this.profileId,
    required this.generation,
  });

  final String profileId;
  final int generation;

  @override
  bool operator ==(Object other) =>
      other is InboxReconcileRequest &&
      other.profileId == profileId &&
      other.generation == generation;

  @override
  int get hashCode => Object.hash(profileId, generation);
}

class InboxReconciliation extends InboxReconcileRequest {
  const InboxReconciliation({
    required super.profileId,
    required super.generation,
  });
}

abstract interface class _InboxReconciler {
  /// T-052 owns snapshot contents, pagination, retry and message history.
  Future<InboxReconciliation> reconcile({
    required String profileId,
    required int generation,
  });
}

class _ManualInboxReconciler implements _InboxReconciler {
  final List<InboxReconcileRequest> requests = [];
  final Map<String, Completer<InboxReconciliation>> _completers = {};

  @override
  Future<InboxReconciliation> reconcile({
    required String profileId,
    required int generation,
  }) {
    final key = '$profileId:$generation';
    requests.add(
      InboxReconcileRequest(profileId: profileId, generation: generation),
    );
    return (_completers[key] ??= Completer<InboxReconciliation>()).future;
  }

  Completer<InboxReconciliation> completer(String profileId, int generation) =>
      _completers['$profileId:$generation'] ??
      (throw StateError('No snapshot requested for $profileId:$generation'));
}
