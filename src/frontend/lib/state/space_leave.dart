import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/spaces_client.dart';
import '../backend/gateway_http.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'call_providers.dart';
import 'space_providers.dart';

enum SpaceLeaveResult { success, failed, uncertain, stale, busy }

final spaceLeaveMediaCleanupProvider =
    Provider<Future<void> Function(String, String)>((ref) {
      return (spaceId, profileId) => ref
          .read(callControllerProvider.notifier)
          .disconnectDepartedSpace(spaceId, profileId: profileId);
    });

/// Reads the generation synchronously rather than relying on lazy UI rebuilds.
final spaceLeaveIdentityProvider = Provider<GatewayRequestIdentity? Function()>(
  (ref) {
    return () =>
        ref.read(authControllerProvider.notifier).gatewayRequestIdentity;
  },
);

/// Retires responses across every actor transition, including switch-away/back.
final spaceLeaveActionsProvider = Provider<SpaceLeaveActions>((ref) {
  final actions = SpaceLeaveActions(ref);
  ref.onDispose(actions.dispose);
  return actions;
});

class SpaceLeaveActions {
  SpaceLeaveActions(this._ref);
  final Ref _ref;
  final Set<String> _pending = {};
  bool _disposed = false;

  void dispose() => _disposed = true;

  Future<SpaceLeaveResult> leave(String spaceId) async {
    if (_disposed) return SpaceLeaveResult.stale;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return SpaceLeaveResult.failed;
    if (!_pending.add(spaceId)) return SpaceLeaveResult.busy;
    final identityReader = _ref.read(spaceLeaveIdentityProvider);
    final identity = identityReader();
    if (identity == null) {
      _pending.remove(spaceId);
      return SpaceLeaveResult.failed;
    }
    try {
      final result = await _ref
          .read(voiceSpacesClientProvider)
          .leaveSpace(authorization: auth, spaceId: spaceId);
      if (_disposed || identity != identityReader()) {
        return SpaceLeaveResult.stale;
      }
      if (result case SpacesApiFailure(:final statusCode, :final errorCode)) {
        if (errorCode == 'network_error' || (statusCode ?? 0) >= 500) {
          _ref.invalidate(mySpacesProvider);
          return SpaceLeaveResult.uncertain;
        }
        return SpaceLeaveResult.failed;
      }
      await _ref.read(spaceLeaveMediaCleanupProvider)(
        spaceId,
        identity.profileId,
      );
      if (_disposed || identity != identityReader()) {
        return SpaceLeaveResult.stale;
      }
      _ref.invalidate(mySpacesProvider);
      _ref.invalidate(spaceProvider(spaceId));
      _ref.invalidate(spaceTreeProvider(spaceId));
      _ref.invalidate(spaceMembersProvider(spaceId));
      if (_ref.read(selectedSpaceIdProvider) == spaceId) {
        _ref.read(selectedSpaceIdProvider.notifier).state = null;
        _ref.read(selectedChatIdProvider.notifier).state = null;
      }
      return SpaceLeaveResult.success;
    } finally {
      _pending.remove(spaceId);
    }
  }
}
