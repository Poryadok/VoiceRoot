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
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';

import 'support/gateway_test_client.dart';

/// T-053 Cycle 2a RED contract.
///
/// This deliberately imports the missing production coordinator seam. The
/// coordinator must own only the Auth-to-Realtime handoff: T-052 owns the
/// later `hello`-gated inbox triplet, and Cycle 3 owns transport binding.
/// No test-only coordinator or source-file inspection is used here.
void main() {
  group('ProfileSwitchCoordinator', () {
    test(
      'accepted A to B persists and retires before one immutable reconnect handoff',
      () async {
        final harness = _CoordinatorHarness();
        addTearDown(harness.dispose);
        harness.replyWithSession('profile-b', _session('profile-b'));
        harness.realtime.seedSubscriptions({'chat-a', 'chat-a-background'});
        harness.container.read(selectedChatIdProvider.notifier).state =
            'chat-a';

        final outcome = await harness.coordinator.switchTo('profile-b');

        expect(outcome, isA<ProfileSwitchApplied>());
        final applied = outcome as ProfileSwitchApplied;
        expect(applied.handoff.generation, 1);
        expect(applied.handoff.previousSession, _session('profile-a'));
        expect(applied.handoff.nextSession, _session('profile-b'));
        expect(applied.handoff.nextAuthorization, 'Bearer access-profile-b');
        expect(applied.handoff.retiredSubscriptionIds, {
          'chat-a',
          'chat-a-background',
        });
        expect(harness.realtime.handoffs, [applied.handoff]);
        expect(harness.realtime.selectedChatAtHandoff, [null]);
        expect(harness.realtime.retiredSubscriptions, [
          {'chat-a', 'chat-a-background'},
        ]);
        expect(harness.realtime.activeSubscriptions, isEmpty);
        expect(harness.realtime.persistedAtHandoff, [_session('profile-b')]);
        expect(harness.realtime.headerAtHandoff, ['Bearer access-profile-b']);

        expect(harness.controller.state.session, _session('profile-b'));
        expect(
          harness.container.read(authorizationHeaderProvider),
          'Bearer access-profile-b',
        );
        expect(await harness.storage.read(), _session('profile-b'));
        expect(harness.storage.writes, [_session('profile-b')]);
        expect(
          harness.gatewayPaths.where((path) => path.contains('/chats')),
          isEmpty,
          reason:
              'the core handoff must not start T-052 ListChats before a '
              'matching Realtime hello',
        );
      },
    );

    test(
      'late B handoff cannot overwrite committed C state or C selection',
      () async {
        final harness = _CoordinatorHarness();
        addTearDown(harness.dispose);
        harness.replyWithSession('profile-b', _session('profile-b'));
        harness.replyWithSession('profile-c', _session('profile-c'));
        harness.realtime.pauseGeneration(1);

        final b = harness.coordinator.switchTo('profile-b');
        await harness.realtime.waitForStart(1);
        expect(harness.controller.state.session, _session('profile-b'));
        expect(
          harness.realtime.handoffs.single.nextAuthorization,
          'Bearer access-profile-b',
        );

        final c = harness.coordinator.switchTo('profile-c');
        await harness.realtime.waitForStart(2);
        harness.realtime.completeGeneration(2);
        expect(await c, isA<ProfileSwitchApplied>());

        // A newer UI choice is observable state; a late B continuation must
        // not clear or otherwise apply its stale transition to it.
        harness.container.read(selectedChatIdProvider.notifier).state =
            'chat-c';
        harness.realtime.completeGeneration(1);
        expect(await b, isA<ProfileSwitchSuperseded>());

        expect(harness.controller.state.session, _session('profile-c'));
        expect(
          harness.container.read(authorizationHeaderProvider),
          'Bearer access-profile-c',
        );
        expect(await harness.storage.read(), _session('profile-c'));
        expect(harness.container.read(selectedChatIdProvider), 'chat-c');
        expect(harness.realtime.handoffs.map((handoff) => handoff.generation), [
          1,
          2,
        ]);
        expect(
          harness.realtime.handoffs.map((handoff) => handoff.nextAuthorization),
          ['Bearer access-profile-b', 'Bearer access-profile-c'],
        );
        expect(
          harness.gatewayPaths.where((path) => path.contains('/chats')),
          isEmpty,
        );
      },
    );

    test('rejected B leaves every A-owned handoff input untouched', () async {
      final harness = _CoordinatorHarness();
      addTearDown(harness.dispose);
      harness.reject('profile-b');
      harness.realtime.seedSubscriptions({'chat-a', 'chat-a-background'});
      harness.container.read(selectedChatIdProvider.notifier).state = 'chat-a';

      final outcome = await harness.coordinator.switchTo('profile-b');

      expect(outcome, isA<ProfileSwitchRejected>());
      expect(harness.controller.state.session, _session('profile-a'));
      expect(
        harness.container.read(authorizationHeaderProvider),
        'Bearer access-profile-a',
      );
      expect(await harness.storage.read(), _session('profile-a'));
      expect(harness.storage.writes, isEmpty);
      expect(harness.container.read(selectedChatIdProvider), 'chat-a');
      expect(harness.realtime.handoffs, isEmpty);
      expect(harness.realtime.activeSubscriptions, {
        'chat-a',
        'chat-a-background',
      });
      expect(
        harness.gatewayPaths.where((path) => path.contains('/chats')),
        isEmpty,
      );
    });
  });
}

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'access-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-1',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

class _CoordinatorHarness {
  _CoordinatorHarness()
    : storage = _RecordingAuthSessionStorage(_session('profile-a')) {
    final client = MockClient(_respond);
    controller = AuthController(
      authClient: VoiceAuthClient(gateway: gatewayHttpForTest(client)),
      storage: storage,
      guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
    )..state = AuthState(session: _session('profile-a'));
    realtime = _RecordingProfileSwitchRealtimeBoundary(
      () => container.read(selectedChatIdProvider),
      () => storage._session,
      () => container.read(authorizationHeaderProvider),
    );
    container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith((_) => controller),
        authSessionStorageProvider.overrideWithValue(storage),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        httpClientProvider.overrideWithValue(client),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        profileSwitchRealtimeBoundaryProvider.overrideWithValue(realtime),
      ],
    );
    coordinator = container.read(profileSwitchCoordinatorProvider);
  }

  late final ProviderContainer container;
  late final AuthController controller;
  late final ProfileSwitchCoordinator coordinator;
  late final _RecordingProfileSwitchRealtimeBoundary realtime;
  final _RecordingAuthSessionStorage storage;
  final Map<String, AuthSession> _sessions = {};
  final Set<String> _rejectedProfiles = {};
  final List<String> gatewayPaths = [];

  void replyWithSession(String profileId, AuthSession session) {
    _sessions[profileId] = session;
  }

  void reject(String profileId) => _rejectedProfiles.add(profileId);

  Future<http.Response> _respond(http.Request request) async {
    gatewayPaths.add(request.url.path);
    if (request.url.path != '/api/v1/auth/switch-profile') {
      return http.Response('not found', 404);
    }
    final body = jsonDecode(request.body) as Map<String, dynamic>;
    final profileId = body['profile_id'] as String?;
    if (profileId == null || _rejectedProfiles.contains(profileId)) {
      return utf8JsonResponse(
        jsonEncode({'error': 'profile_not_switchable'}),
        status: 403,
      );
    }
    final session = _sessions[profileId];
    if (session == null) return http.Response('unknown profile', 400);
    return utf8JsonResponse(jsonEncode(session.toJson()));
  }

  Future<void> dispose() async {
    container.dispose();
  }
}

class _RecordingAuthSessionStorage implements AuthSessionStorage {
  _RecordingAuthSessionStorage(this._session);

  AuthSession? _session;
  final List<AuthSession> writes = [];

  @override
  Future<void> clear() async => _session = null;

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async {
    writes.add(session);
    _session = session;
  }
}

class _RecordingProfileSwitchRealtimeBoundary
    implements ProfileSwitchRealtimeBoundary {
  _RecordingProfileSwitchRealtimeBoundary(
    this._selectedChat,
    this._persistedSession,
    this._authorization,
  );

  final String? Function() _selectedChat;
  final AuthSession? Function() _persistedSession;
  final String? Function() _authorization;
  final List<ProfileSwitchHandoff> handoffs = [];
  final List<String?> selectedChatAtHandoff = [];
  final List<AuthSession?> persistedAtHandoff = [];
  final List<String?> headerAtHandoff = [];
  final List<Set<String>> retiredSubscriptions = [];
  @override
  final Set<String> activeSubscriptions = {};
  final Map<int, Completer<void>> _paused = {};
  final Map<int, Completer<void>> _started = {};

  void pauseGeneration(int generation) {
    _paused[generation] = Completer<void>();
  }

  void seedSubscriptions(Set<String> chatIds) {
    activeSubscriptions
      ..clear()
      ..addAll(chatIds);
  }

  Future<void> waitForStart(int generation) =>
      (_started[generation] ??= Completer<void>()).future;

  void completeGeneration(int generation) {
    final pending = _paused[generation];
    if (pending != null && !pending.isCompleted) pending.complete();
  }

  @override
  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff) async {
    handoffs.add(handoff);
    selectedChatAtHandoff.add(_selectedChat());
    persistedAtHandoff.add(_persistedSession());
    headerAtHandoff.add(_authorization());
    retiredSubscriptions.add(handoff.retiredSubscriptionIds);
    activeSubscriptions.removeAll(handoff.retiredSubscriptionIds);
    (_started[handoff.generation] ??= Completer<void>()).complete();
    await _paused[handoff.generation]?.future;
  }
}
