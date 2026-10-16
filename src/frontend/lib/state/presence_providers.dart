import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/realtime_client.dart';
import '../backend/users_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'social_providers.dart';

/// REST snapshot + live `presence_update` over WS; REST polling when WS is down.
///
/// See [docs/PLAN.md] (app stack presence), [docs/ARCHITECTURE_REQUIREMENTS.md]
/// (ephemeral presence — no WS catch-up; refresh via User API after reconnect).
class PresenceController extends StateNotifier<Map<String, VoicePresence>> {
  PresenceController(this._ref) : super({}) {
    _linkSub = _ref.listen<RealtimeLinkStatus>(
      realtimeLinkStatusProvider,
      (prev, next) => _onLinkStatus(prev, next),
    );
    _eventSub = _ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) => next.whenData(_onRealtimeFrame),
    );
  }

  static const Duration pollInterval = Duration(seconds: 60);

  final Ref _ref;
  final Set<String> _watched = {};
  final Map<String, int> _versions = {};
  Timer? _pollTimer;
  ProviderSubscription<RealtimeLinkStatus>? _linkSub;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _eventSub;

  void ensureWatched(String profileId) {
    if (_ref.read(profileSwitchInProgressProvider)) return;
    if (profileId.isEmpty) return;
    final added = _watched.add(profileId);
    if (added && !state.containsKey(profileId)) {
      unawaited(_refreshOne(profileId));
    }
    _syncPolling();
  }

  void watchMany(Iterable<String> profileIds) {
    final ids = profileIds.where((id) => id.isNotEmpty).toList();
    if (ids.isEmpty) return;
    for (final id in ids) {
      _watched.add(id);
    }
    unawaited(refreshBulk(ids));
    _syncPolling();
  }

  void _onRealtimeFrame(RealtimeFrame frame) {
    if (_ref.read(profileSwitchInProgressProvider)) return;
    if (frame.op != 'presence_update') return;
    final data = frame.data;
    if (data == null) return;
    final profileId = data['profile_id'] as String?;
    final status = data['status'] as String?;
    if (profileId == null || profileId.isEmpty) return;
    if (!_watched.contains(profileId)) return;
    _nextVersion(profileId);
    // Realtime sends a viewer-filtered sparse snapshot. Missing fields revoke
    // previous visibility; retaining them would leak stale presence data.
    final statusIsInvisible = status == 'invisible';
    final visibleStatus = status == null || status.isEmpty || statusIsInvisible
        ? 'offline'
        : status;
    final customStatus = data['custom_status'] as String?;
    final gameTitle = data['game_title'] as String?;
    final lastSeenRaw = data['last_seen'] as String?;
    state = {
      ...state,
      profileId: VoicePresence(
        profileId: profileId,
        status: visibleStatus,
        gameTitle:
            statusIsInvisible ||
                visibleStatus == 'offline' ||
                gameTitle == null ||
                gameTitle.isEmpty
            ? null
            : gameTitle,
        customStatus:
            statusIsInvisible || customStatus == null || customStatus.isEmpty
            ? null
            : customStatus,
        lastSeen: statusIsInvisible || lastSeenRaw == null
            ? null
            : DateTime.tryParse(lastSeenRaw)?.toUtc(),
      ),
    };
  }

  void _onLinkStatus(RealtimeLinkStatus? prev, RealtimeLinkStatus next) {
    if (next == RealtimeLinkStatus.connected) {
      _stopPolling();
      if (prev == RealtimeLinkStatus.reconnecting && _watched.isNotEmpty) {
        unawaited(refreshBulk(_watched));
      }
      return;
    }
    if (next == RealtimeLinkStatus.disconnected ||
        next == RealtimeLinkStatus.reconnecting) {
      _startPolling();
    }
  }

  void _syncPolling() {
    final link = _ref.read(realtimeLinkStatusProvider);
    if (link == RealtimeLinkStatus.disconnected ||
        link == RealtimeLinkStatus.reconnecting) {
      _startPolling();
    }
  }

  void _startPolling() {
    if (_watched.isEmpty) return;
    if (_pollTimer != null) return;
    unawaited(refreshBulk(_watched));
    _pollTimer = Timer.periodic(pollInterval, (_) {
      unawaited(refreshBulk(_watched));
    });
  }

  void _stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }

  Future<void> refreshBulk(Iterable<String> profileIds) async {
    final ids = profileIds.where((id) => id.isNotEmpty).toSet().toList();
    if (ids.isEmpty || _ref.read(profileSwitchInProgressProvider)) return;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final controller = _ref.read(authControllerProvider.notifier);
    final generation = controller.sessionInstallGeneration;
    final versions = {for (final id in ids) id: _nextVersion(id)};
    final result = await _ref
        .read(voiceUsersClientProvider)
        .getBulkPresence(authorization: auth, profileIds: ids);
    if (!_sameViewer(controller, generation, auth)) return;
    final next = {...state};
    for (final id in ids) {
      if (_versions[id] != versions[id]) continue;
      if (result case UsersApiOk(
        :final data,
      ) when data[id] != null && data[id]!.profileId == id) {
        next[id] = data[id]!;
      } else if (next[id] case final previous?) {
        next[id] = _withoutGame(previous);
      }
    }
    state = next;
  }

  Future<void> _refreshOne(String profileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final controller = _ref.read(authControllerProvider.notifier);
    final generation = controller.sessionInstallGeneration;
    final version = _nextVersion(profileId);
    final result = await _ref
        .read(voiceUsersClientProvider)
        .getPresence(authorization: auth, profileId: profileId);
    if (!_sameViewer(controller, generation, auth) ||
        _versions[profileId] != version) {
      return;
    }
    if (result case UsersApiOk(:final data) when data.profileId == profileId) {
      state = {...state, profileId: data};
    } else if (state[profileId] case final previous?) {
      state = {...state, profileId: _withoutGame(previous)};
    }
  }

  int _nextVersion(String id) => _versions[id] = (_versions[id] ?? 0) + 1;

  bool _sameViewer(AuthController controller, int generation, String auth) =>
      mounted &&
      identical(_ref.read(authControllerProvider.notifier), controller) &&
      controller.sessionInstallGeneration == generation &&
      _ref.read(authorizationHeaderProvider) == auth &&
      !_ref.read(profileSwitchInProgressProvider);

  VoicePresence _withoutGame(VoicePresence previous) => VoicePresence(
    profileId: previous.profileId,
    status: previous.status,
    customStatus: previous.customStatus,
    lastSeen: previous.lastSeen,
  );

  @override
  void dispose() {
    _stopPolling();
    _linkSub?.close();
    _eventSub?.close();
    super.dispose();
  }
}

final presenceMapProvider =
    StateNotifierProvider<PresenceController, Map<String, VoicePresence>>((
      ref,
    ) {
      // Viewer-filtered snapshots cannot survive login/profile replacement or a
      // switch attempt that returns the same session after failure.
      ref.watch(authControllerProvider.select((state) => state.session));
      ref.watch(profileSwitchInProgressProvider);
      return PresenceController(ref);
    });

/// Presence for a profile: initial REST, live WS, polling fallback when WS down.
final presenceProvider = Provider.family<VoicePresence?, String>((
  ref,
  profileId,
) {
  ref.watch(presenceMapProvider);
  ref.read(presenceMapProvider.notifier).ensureWatched(profileId);
  return ref.watch(presenceMapProvider)[profileId];
});

/// Bulk-load presence for friends when the list is available.
final friendsPresenceSyncProvider = Provider<void>((ref) {
  final friends = ref.watch(friendsListProvider).valueOrNull;
  if (friends == null) return;
  ref.read(presenceMapProvider.notifier).watchMany(friends.friends);
});
