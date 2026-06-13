import 'api_result.dart';
import 'gateway_http.dart';

const String kRolesMissingBaseUrlDetail = 'missing base URL';

/// System role names returned by Role Service (Phase 5).
const String kSpaceRoleOwner = 'Owner';
const String kSpaceRoleAdmin = 'Admin';

sealed class RolesApiResult<T> {
  const RolesApiResult();
}

final class RolesApiOk<T> extends RolesApiResult<T> {
  const RolesApiOk(this.data);
  final T data;
}

final class RolesApiFailure extends RolesApiResult<Never> {
  const RolesApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class SpaceRole {
  const SpaceRole({
    required this.id,
    required this.spaceId,
    required this.name,
    this.position = 0,
    this.permissionsMask = 0,
    this.managed = false,
  });

  final String id;
  final String spaceId;
  final String name;
  final int position;
  final int permissionsMask;
  final bool managed;

  factory SpaceRole.fromJson(Map<String, dynamic> json) {
    return SpaceRole(
      id: json['id'] as String? ?? '',
      spaceId: json['space_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      position: _jsonInt(json['position']),
      permissionsMask: _jsonInt(json['permissions_mask']),
      managed: json['managed'] as bool? ?? false,
    );
  }
}

int _jsonInt(Object? value) {
  if (value is num) {
    return value.toInt();
  }
  if (value is String) {
    return int.tryParse(value) ?? 0;
  }
  return 0;
}

/// HTTP client for Role routes (`/api/v1/roles/**`).
class VoiceRolesClient {
  VoiceRolesClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<RolesApiResult<List<SpaceRole>>> listRoles({
    required String authorization,
    required String spaceId,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/roles',
      queryParameters: {'space_id': spaceId},
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _map(result, _rolesFromListPayload);
  }

  Future<RolesApiResult<List<SpaceRole>>> getMemberRoles({
    required String authorization,
    required String spaceId,
    required String profileId,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/roles/members',
      queryParameters: {
        'space_id': spaceId,
        'profile_id': profileId,
      },
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _map(result, _rolesFromListPayload);
  }

  Future<RolesApiResult<void>> assignRole({
    required String authorization,
    required String spaceId,
    required String profileId,
    required String roleId,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/roles/assign'),
      authorization: authorization,
      jsonBody: {
        'space_id': spaceId,
        'profile_id': profileId,
        'role_id': roleId,
      },
    );
    return _mapEmpty(result);
  }

  Future<RolesApiResult<void>> revokeRole({
    required String authorization,
    required String spaceId,
    required String profileId,
    required String roleId,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/roles/revoke'),
      authorization: authorization,
      jsonBody: {
        'space_id': spaceId,
        'profile_id': profileId,
        'role_id': roleId,
      },
    );
    return _mapEmpty(result);
  }

  Future<RolesApiResult<bool>> checkPermission({
    required String authorization,
    required String spaceId,
    required String profileId,
    required String permissionName,
    String? chatId,
    String? voiceRoomId,
  }) async {
    final params = <String, String>{
      'space_id': spaceId,
      'profile_id': profileId,
      'permission_name': permissionName,
    };
    if (chatId != null && chatId.isNotEmpty) {
      params['chat_id'] = chatId;
    }
    if (voiceRoomId != null && voiceRoomId.isNotEmpty) {
      params['voice_room_id'] = voiceRoomId;
    }
    final uri = _gateway.replace(
      path: '/api/v1/roles/check',
      queryParameters: params,
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _map(result, (data) => data['allowed'] as bool? ?? false);
  }

  Future<RolesApiResult<SpaceRole>> createRole({
    required String authorization,
    required String spaceId,
    required String name,
    int permissionsMask = 0,
    int position = 1,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/roles'),
      authorization: authorization,
      body: {
        'space_id': spaceId,
        'name': name,
        'permissions_mask': permissionsMask,
        'position': position,
      },
    );
    return _map(result, (data) {
      final role = data['role'];
      if (role is Map<String, dynamic>) return SpaceRole.fromJson(role);
      return SpaceRole.fromJson(data);
    });
  }

  Future<RolesApiResult<SpaceRole>> updateRole({
    required String authorization,
    required String roleId,
    String? name,
    int? permissionsMask,
    int? position,
  }) async {
    final body = <String, dynamic>{};
    if (name != null) body['name'] = name;
    if (permissionsMask != null) body['permissions_mask'] = permissionsMask;
    if (position != null) body['position'] = position;
    final result = await _gateway.patchJson(
      uri: _gateway.resolve('/api/v1/roles/$roleId'),
      authorization: authorization,
      body: body,
    );
    return _map(result, (data) {
      final role = data['role'];
      if (role is Map<String, dynamic>) return SpaceRole.fromJson(role);
      return SpaceRole.fromJson(data);
    });
  }

  Future<RolesApiResult<void>> deleteRole({
    required String authorization,
    required String roleId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/roles/$roleId'),
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  Future<RolesApiResult<void>> reorderRoles({
    required String authorization,
    required String spaceId,
    required List<String> orderedRoleIds,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/roles/reorder'),
      authorization: authorization,
      jsonBody: {
        'space_id': spaceId,
        'ordered_role_ids': orderedRoleIds,
      },
    );
    return _mapEmpty(result);
  }

  Future<RolesApiResult<int>> getEffectivePermissions({
    required String authorization,
    required String spaceId,
    required String profileId,
    String? chatId,
    String? voiceRoomId,
  }) async {
    final params = <String, String>{
      'space_id': spaceId,
      'profile_id': profileId,
    };
    if (chatId != null && chatId.isNotEmpty) params['chat_id'] = chatId;
    if (voiceRoomId != null && voiceRoomId.isNotEmpty) {
      params['voice_room_id'] = voiceRoomId;
    }
    final uri = _gateway.replace(
      path: '/api/v1/roles/effective',
      queryParameters: params,
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _map(result, (data) {
      final set = data['permission_set'] ?? data['permissionSet'];
      if (set is Map<String, dynamic>) {
        return _jsonInt(set['effective_mask'] ?? set['effectiveMask']);
      }
      return 0;
    });
  }

  Future<RolesApiResult<SpaceRole>> getDefaultJoinRole({
    required String authorization,
    required String spaceId,
  }) async {
    final uri = _gateway.replace(
      path: '/api/v1/roles/default-join',
      queryParameters: {'space_id': spaceId},
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _map(result, (data) {
      final role = data['role'];
      if (role is Map<String, dynamic>) return SpaceRole.fromJson(role);
      return SpaceRole.fromJson(data);
    });
  }

  Future<RolesApiResult<void>> setDefaultJoinRole({
    required String authorization,
    required String spaceId,
    required String roleId,
  }) async {
    final result = await _gateway.putJson(
      uri: _gateway.resolve('/api/v1/roles/default-join'),
      authorization: authorization,
      body: {'space_id': spaceId, 'role_id': roleId},
      allowNoContent: true,
    );
    return switch (result) {
      GatewayHttpOk() => const RolesApiOk(null),
      GatewayHttpFailure(:final error) => RolesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<RolesApiResult<void>> setChatOverride({
    required String authorization,
    required String spaceId,
    required String chatId,
    required String roleId,
    int allowMask = 0,
    int denyMask = 0,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/roles/chat-overrides'),
      authorization: authorization,
      jsonBody: {
        'space_id': spaceId,
        'chat': {'id': chatId},
        'role_id': roleId,
        'allow_mask': allowMask,
        'deny_mask': denyMask,
      },
    );
    return _mapEmpty(result);
  }

  Future<RolesApiResult<void>> setVoiceRoomOverride({
    required String authorization,
    required String spaceId,
    required String voiceRoomId,
    required String roleId,
    int allowMask = 0,
    int denyMask = 0,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/roles/voice-overrides'),
      authorization: authorization,
      jsonBody: {
        'space_id': spaceId,
        'voice_room_id': voiceRoomId,
        'role_id': roleId,
        'allow_mask': allowMask,
        'deny_mask': denyMask,
      },
    );
    return _mapEmpty(result);
  }

  List<SpaceRole> _rolesFromListPayload(Map<String, dynamic> data) {
    final roleList = data['role_list'] ?? data['roleList'];
    if (roleList is! Map<String, dynamic>) return const [];
    final roles = roleList['roles'];
    if (roles is! List) return const [];
    return roles
        .whereType<Map<String, dynamic>>()
        .map(SpaceRole.fromJson)
        .toList(growable: false);
  }

  RolesApiResult<T> _map<T>(
    GatewayHttpResult<Map<String, dynamic>> result,
    T Function(Map<String, dynamic> data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => RolesApiOk(parse(data)),
      GatewayHttpFailure(:final error) => RolesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  RolesApiResult<void> _mapEmpty(GatewayHttpResult<void> result) {
    return switch (result) {
      GatewayHttpOk() => const RolesApiOk(null),
      GatewayHttpFailure(:final error) => RolesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
