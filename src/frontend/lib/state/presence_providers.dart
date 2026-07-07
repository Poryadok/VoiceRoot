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
  Timer? _pollTimer;
  ProviderSubscription<RealtimeLinkStatus>? _linkSub;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _eventSub;

  void ensureWatched(String profileId) {
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
    if (frame.op != 'presence_update') return;
    final data = frame.data;
    if (data == null) return;
    final profileId = data['profile_id'] as String?;
    final status = data['status'] as String?;
    if (profileId == null || profileId.isEmpty || status == null) return;
    if (!_watched.contains(profileId)) return;
    final lastSeenRaw = data['last_seen'] as String?;
    state = {
      ...state,
      profileId: VoicePresence(
        profileId: profileId,
        status: status,
        lastSeen: lastSeenRaw == null ? null : DateTime.tryParse(lastSeenRaw)?.toUtc(),
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
    if (ids.isEmpty) return;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await _ref
        .read(voiceUsersClientProvider)
        .getBulkPresence(authorization: auth, profileIds: ids);
    if (!mounted) return;
    if (result case UsersApiOk(:final data)) {
      if (data.isEmpty) return;
      state = {...state, ...data};
    }
  }

  Future<void> _refreshOne(String profileId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await _ref
        .read(voiceUsersClientProvider)
        .getPresence(authorization: auth, profileId: profileId);
    if (!mounted) return;
    if (result case UsersApiOk(:final data)) {
      state = {...state, profileId: data};
    }
  }

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
