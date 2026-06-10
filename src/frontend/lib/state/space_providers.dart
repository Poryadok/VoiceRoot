import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/api_errors.dart';
import '../backend/roles_client.dart';
import '../backend/spaces_client.dart';
import 'auth_providers.dart';

final voiceSpacesClientProvider = Provider<VoiceSpacesClient>((ref) {
  return VoiceSpacesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final voiceRolesClientProvider = Provider<VoiceRolesClient>((ref) {
  return VoiceRolesClient(gateway: ref.watch(gatewayHttpClientProvider));
});

/// Active profile for space member UI; override in tests. Wired from auth in [VoiceApp].
final spaceViewerProfileIdProvider = Provider<String?>((ref) => null);

final _mySpacesRefreshTokenProvider = StateProvider<int>((ref) => 0);

void _invalidateMySpaces(Ref ref) {
  ref.invalidate(mySpacesProvider);
  ref.read(_mySpacesRefreshTokenProvider.notifier).state++;
}

final mySpacesProvider = FutureProvider<SpaceListData>((ref) async {
  ref.watch(_mySpacesRefreshTokenProvider);
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .watch(voiceSpacesClientProvider)
      .listMySpaces(authorization: auth);
  return switch (result) {
    SpacesApiOk(:final data) => data,
    SpacesApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    SpacesApiFailure(:final message) => throw Exception(message),
  };
});

final spaceProvider = FutureProvider.family<VoiceSpace, String>((
  ref,
  spaceId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .read(voiceSpacesClientProvider)
      .getSpace(authorization: auth, spaceId: spaceId);
  return switch (result) {
    SpacesApiOk(:final data) => data,
    SpacesApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    SpacesApiFailure(:final message) => throw Exception(message),
  };
});

class SpaceActions {
  SpaceActions(this._ref);

  final Ref _ref;

  /// Creates a private space; PATCHes icon when [iconUrl] is non-empty.
  Future<String?> createSpace({
    required String name,
    String? description,
    String? iconUrl,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final trimmedName = name.trim();
    if (trimmedName.isEmpty) return 'name_required';

    final trimmedDescription = description?.trim();
    final createResult = await _ref.read(voiceSpacesClientProvider).createSpace(
      authorization: auth,
      name: trimmedName,
      description:
          trimmedDescription == null || trimmedDescription.isEmpty
              ? null
              : trimmedDescription,
    );

    return switch (createResult) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk(:final data) => _patchIconIfNeeded(
        auth: auth,
        space: data,
        iconUrl: iconUrl?.trim(),
      ),
    };
  }

  Future<String?> _patchIconIfNeeded({
    required String auth,
    required VoiceSpace space,
    String? iconUrl,
  }) async {
    if (iconUrl == null || iconUrl.isEmpty) {
      _invalidateMySpaces(_ref);
      return null;
    }
    final updateResult = await _ref.read(voiceSpacesClientProvider).updateSpace(
      authorization: auth,
      spaceId: space.id,
      iconUrl: iconUrl,
    );
    return switch (updateResult) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk() => () {
        _invalidateMySpaces(_ref);
        return null;
      }(),
    };
  }
}

final spaceActionsProvider = Provider<SpaceActions>((ref) {
  return SpaceActions(ref);
});

final selectedSpaceIdProvider = StateProvider<String?>((ref) => null);

final spaceTreeProvider = FutureProvider.family<SpaceTreeData, String>((
  ref,
  spaceId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .read(voiceSpacesClientProvider)
      .listSpaceTree(authorization: auth, spaceId: spaceId);
  return switch (result) {
    SpacesApiOk(:final data) => data,
    SpacesApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    SpacesApiFailure(:final message) => throw Exception(message),
  };
});

class SpaceTreeActions {
  SpaceTreeActions(this._ref);

  final Ref _ref;

  Future<void> refreshTree(String spaceId) async {
    _ref.invalidate(spaceTreeProvider(spaceId));
    await _ref.read(spaceTreeProvider(spaceId).future);
  }
}

final spaceTreeActionsProvider = Provider<SpaceTreeActions>((ref) {
  return SpaceTreeActions(ref);
});

final spaceInvitesProvider = FutureProvider.family<List<SpaceInvite>, String>((
  ref,
  spaceId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .read(voiceSpacesClientProvider)
      .listInvites(authorization: auth, spaceId: spaceId);
  return switch (result) {
    SpacesApiOk(:final data) => data,
    SpacesApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    SpacesApiFailure(:final message) => throw Exception(message),
  };
});

class SpaceInviteActions {
  SpaceInviteActions(this._ref);

  final Ref _ref;

  Future<String?> createInvite({
    required String spaceId,
    int? maxUses,
    DateTime? expiresAt,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceSpacesClientProvider).createInvite(
      authorization: auth,
      spaceId: spaceId,
      maxUses: maxUses,
      expiresAt: expiresAt,
    );
    return switch (result) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk() => () {
        _ref.invalidate(spaceInvitesProvider(spaceId));
        return null;
      }(),
    };
  }

  Future<String?> revokeInvite({
    required String spaceId,
    required String inviteId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceSpacesClientProvider).revokeInvite(
      authorization: auth,
      spaceId: spaceId,
      inviteId: inviteId,
    );
    return switch (result) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk() => () {
        _ref.invalidate(spaceInvitesProvider(spaceId));
        return null;
      }(),
    };
  }

  Future<({String? error, String? spaceId})> joinByInvite({
    required String code,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return (error: 'not_authenticated', spaceId: null);

    final trimmed = code.trim();
    if (trimmed.isEmpty) return (error: 'code_required', spaceId: null);

    final result = await _ref
        .read(voiceSpacesClientProvider)
        .joinByInvite(authorization: auth, code: trimmed);
    return switch (result) {
      SpacesApiFailure(:final message) => (error: message, spaceId: null),
      SpacesApiOk(:final data) => () {
        _invalidateMySpaces(_ref);
        _ref.invalidate(spaceProvider(data.spaceId));
        return (error: null, spaceId: data.spaceId);
      }(),
    };
  }
}

final spaceInviteActionsProvider = Provider<SpaceInviteActions>((ref) {
  return SpaceInviteActions(ref);
});

/// Space member roster with [SpaceMemberRosterEntry.roleNames] from `GET /api/v1/spaces/{id}/members`.
final spaceMembersProvider =
    FutureProvider.family<List<SpaceMemberRosterEntry>, String>((
  ref,
  spaceId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .read(voiceSpacesClientProvider)
      .listMembers(authorization: auth, spaceId: spaceId);
  return switch (result) {
    SpacesApiOk(:final data) => data.members,
    SpacesApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    SpacesApiFailure(:final message) => throw Exception(message),
  };
});

/// Role hierarchy for a space from `GET /api/v1/roles?space_id=`.
final spaceRolesProvider = FutureProvider.family<List<SpaceRole>, String>((
  ref,
  spaceId,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref.read(voiceRolesClientProvider).listRoles(
    authorization: auth,
    spaceId: spaceId,
  );
  return switch (result) {
    RolesApiOk(:final data) => data,
    RolesApiFailure(:final statusCode)
        when isBackendUnavailable(statusCode) =>
      throw const BackendUnavailableException(),
    RolesApiFailure(:final message) => throw Exception(message),
  };
});

class SpaceMemberActions {
  SpaceMemberActions(this._ref);

  final Ref _ref;

  Future<String?> banMember({
    required String spaceId,
    required String accountId,
    String? profileId,
    String? reason,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceSpacesClientProvider).banMember(
      authorization: auth,
      spaceId: spaceId,
      accountId: accountId,
      profileId: profileId,
      reason: reason,
    );
    return switch (result) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk() => () {
        _ref.invalidate(spaceMembersProvider(spaceId));
        return null;
      }(),
    };
  }

  Future<String?> timeoutMember({
    required String spaceId,
    required String profileId,
    required int durationSeconds,
    String? reason,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceSpacesClientProvider).timeoutMember(
      authorization: auth,
      spaceId: spaceId,
      profileId: profileId,
      durationSeconds: durationSeconds,
      reason: reason,
    );
    return switch (result) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk() => () {
        _ref.invalidate(spaceMembersProvider(spaceId));
        return null;
      }(),
    };
  }

  Future<String?> kickMember({
    required String spaceId,
    required String profileId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceSpacesClientProvider).kickMember(
      authorization: auth,
      spaceId: spaceId,
      profileId: profileId,
    );
    return switch (result) {
      SpacesApiFailure(:final message) => message,
      SpacesApiOk() => () {
        _ref.invalidate(spaceMembersProvider(spaceId));
        return null;
      }(),
    };
  }

  Future<String?> assignRole({
    required String spaceId,
    required String profileId,
    required String roleId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceRolesClientProvider).assignRole(
      authorization: auth,
      spaceId: spaceId,
      profileId: profileId,
      roleId: roleId,
    );
    return switch (result) {
      RolesApiFailure(:final message) => message,
      RolesApiOk() => () {
        _ref.invalidate(spaceMembersProvider(spaceId));
        return null;
      }(),
    };
  }

  Future<String?> revokeRole({
    required String spaceId,
    required String profileId,
    required String roleId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';

    final result = await _ref.read(voiceRolesClientProvider).revokeRole(
      authorization: auth,
      spaceId: spaceId,
      profileId: profileId,
      roleId: roleId,
    );
    return switch (result) {
      RolesApiFailure(:final message) => message,
      RolesApiOk() => () {
        _ref.invalidate(spaceMembersProvider(spaceId));
        return null;
      }(),
    };
  }
}

final spaceMemberActionsProvider = Provider<SpaceMemberActions>((ref) {
  return SpaceMemberActions(ref);
});

bool viewerCanManageSpaceMembers(
  List<SpaceMemberRosterEntry> members,
  String? activeProfileId,
) {
  if (activeProfileId == null) return true;
  for (final member in members) {
    if (member.profileId != activeProfileId) continue;
    return member.roleNames.any(
      (role) => role == kSpaceRoleOwner || role == kSpaceRoleAdmin,
    );
  }
  return false;
}

typedef SpacePermissionQuery = ({String spaceId, String permission});

final spacePermissionProvider = FutureProvider.family<bool, SpacePermissionQuery>(
  (ref, query) async {
    final auth = ref.watch(authorizationHeaderProvider);
    final profileId = ref.watch(spaceViewerProfileIdProvider);
    if (auth == null || profileId == null) return false;
    final result = await ref.read(voiceRolesClientProvider).checkPermission(
      authorization: auth,
      spaceId: query.spaceId,
      profileId: profileId,
      permissionName: query.permission,
    );
    return switch (result) {
      RolesApiOk(:final data) => data,
      RolesApiFailure() => false,
    };
  },
);
