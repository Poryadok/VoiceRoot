import '../gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import '../gen/voice/chat/v1/chat.pbenum.dart';
import '../gen/voice/space/v1/space.pb.dart' as space_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

const String kSpacesMissingBaseUrlDetail = 'missing base URL';

sealed class SpacesApiResult<T> {
  const SpacesApiResult();
}

final class SpacesApiOk<T> extends SpacesApiResult<T> {
  const SpacesApiOk(this.data);
  final T data;
}

final class SpacesApiFailure extends SpacesApiResult<Never> {
  const SpacesApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class VoiceSpace {
  const VoiceSpace({
    required this.id,
    required this.name,
    this.description,
    this.iconUrl,
    required this.visibility,
    required this.ownerProfileId,
    this.memberCount = 0,
    this.createdAt,
    this.updatedAt,
  });

  final String id;
  final String name;
  final String? description;
  final String? iconUrl;
  final String visibility;
  final String ownerProfileId;
  final int memberCount;
  final DateTime? createdAt;
  final DateTime? updatedAt;
}

class SpaceListData {
  const SpaceListData({required this.spaces, this.nextCursor});

  final List<VoiceSpace> spaces;
  final String? nextCursor;
}

class SpaceCategory {
  const SpaceCategory({
    required this.id,
    required this.spaceId,
    required this.name,
    required this.sortOrder,
  });

  final String id;
  final String spaceId;
  final String name;
  final int sortOrder;
}

class VoiceRoomData {
  const VoiceRoomData({
    required this.id,
    required this.spaceId,
    required this.name,
  });

  final String id;
  final String spaceId;
  final String name;
}

class SpaceTreeNodeData {
  const SpaceTreeNodeData({
    required this.id,
    required this.spaceId,
    this.categoryId,
    required this.kind,
    this.linkedChatId,
    this.voiceRoomId,
    required this.sortOrder,
    this.isSystem = false,
    required this.displayName,
  });

  final String id;
  final String spaceId;
  final String? categoryId;
  final String kind;
  final String? linkedChatId;
  final String? voiceRoomId;
  final int sortOrder;
  final bool isSystem;
  final String displayName;

  bool get isTextChat => kind == 'text_chat';
  bool get isVoiceRoom => kind == 'voice_room';
}

class SpaceInvite {
  const SpaceInvite({
    required this.id,
    required this.spaceId,
    required this.code,
    required this.creatorProfileId,
    this.maxUses,
    required this.useCount,
    this.expiresAt,
    required this.createdAt,
    this.revokedAt,
  });

  final String id;
  final String spaceId;
  final String code;
  final String creatorProfileId;
  final int? maxUses;
  final int useCount;
  final DateTime? expiresAt;
  final DateTime createdAt;
  final DateTime? revokedAt;

  String get inviteLink => 'https://voice.gg/invite/$code';
}

class SpaceMembershipData {
  const SpaceMembershipData({
    required this.spaceId,
    required this.profileId,
    required this.joinedAt,
    this.nickname,
  });

  final String spaceId;
  final String profileId;
  final DateTime joinedAt;
  final String? nickname;
}

class SpaceMemberRosterEntry {
  const SpaceMemberRosterEntry({
    required this.profileId,
    required this.roleNames,
    required this.joinedAt,
    this.nickname,
  });

  final String profileId;
  final List<String> roleNames;
  final DateTime joinedAt;
  final String? nickname;

  bool get isOwner => roleNames.contains('Owner');
}

class SpaceMemberListData {
  const SpaceMemberListData({required this.members, this.nextCursor});

  final List<SpaceMemberRosterEntry> members;
  final String? nextCursor;
}

class SpaceTreeData {
  const SpaceTreeData({
    required this.categories,
    required this.nodes,
    required this.voiceRooms,
  });

  final List<SpaceCategory> categories;
  final List<SpaceTreeNodeData> nodes;
  final List<VoiceRoomData> voiceRooms;
}

/// HTTP client for Space routes (`/api/v1/spaces/**`).
class VoiceSpacesClient {
  VoiceSpacesClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<SpacesApiResult<SpaceListData>> listMySpaces({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/spaces',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: space_pb.ListMySpacesResponse.create,
    );
    return _map(
      result,
      (data) => spaceListFromProto(
        data.hasSpaceList() ? data.spaceList : space_pb.SpaceList(),
      ),
    );
  }

  Future<SpacesApiResult<VoiceSpace>> getSpace({
    required String authorization,
    required String spaceId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/spaces/$spaceId'),
      authorization: authorization,
      createEmpty: space_pb.GetSpaceResponse.create,
    );
    return _map(result, (data) => voiceSpaceFromProto(data.space));
  }

  Future<SpacesApiResult<VoiceSpace>> createSpace({
    required String authorization,
    required String name,
    String? description,
    String visibility = 'private',
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/spaces'),
      authorization: authorization,
      body: createSpaceRequestToProto(
        name: name,
        description: description,
        visibility: visibility,
      ),
      createEmpty: space_pb.CreateSpaceResponse.create,
    );
    return _map(result, (data) => voiceSpaceFromProto(data.space));
  }

  Future<SpacesApiResult<VoiceSpace>> updateSpace({
    required String authorization,
    required String spaceId,
    String? iconUrl,
    String? description,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId'),
      authorization: authorization,
      body: updateSpaceRequestToProto(iconUrl: iconUrl, description: description),
      createEmpty: space_pb.UpdateSpaceResponse.create,
    );
    return _map(result, (data) => voiceSpaceFromProto(data.space));
  }

  Future<SpacesApiResult<SpaceTreeData>> listSpaceTree({
    required String authorization,
    required String spaceId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/spaces/$spaceId/tree'),
      authorization: authorization,
      createEmpty: space_pb.ListSpaceTreeResponse.create,
    );
    return _map(
      result,
      (data) => spaceTreeFromProto(data as space_pb.ListSpaceTreeResponse),
    );
  }

  Future<SpacesApiResult<SpaceCategory>> createCategory({
    required String authorization,
    required String spaceId,
    required String name,
    int sortOrder = 0,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/categories'),
      authorization: authorization,
      body: space_pb.CreateCategoryRequest(
        spaceId: spaceId,
        name: name,
        sortOrder: sortOrder,
      ),
      createEmpty: space_pb.CreateCategoryResponse.create,
    );
    return _map(result, (data) {
      final c = data.category;
      return SpaceCategory(
        id: c.id,
        spaceId: c.spaceId,
        name: c.name,
        sortOrder: c.sortOrder,
      );
    });
  }

  Future<SpacesApiResult<VoiceRoomData>> createVoiceRoom({
    required String authorization,
    required String spaceId,
    required String name,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/voice-rooms'),
      authorization: authorization,
      body: space_pb.CreateVoiceRoomRequest(spaceId: spaceId, name: name),
      createEmpty: space_pb.CreateVoiceRoomResponse.create,
    );
    return _map(result, (data) {
      final vr = data.voiceRoom;
      return VoiceRoomData(id: vr.id, spaceId: vr.spaceId, name: vr.name);
    });
  }

  Future<SpacesApiResult<SpaceInvite>> createInvite({
    required String authorization,
    required String spaceId,
    int? maxUses,
    DateTime? expiresAt,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/invites'),
      authorization: authorization,
      body: createInviteRequestToProto(
        spaceId: spaceId,
        maxUses: maxUses,
        expiresAt: expiresAt,
      ),
      createEmpty: space_pb.CreateInviteResponse.create,
    );
    return _map(result, (data) => spaceInviteFromProto(data.invite));
  }

  Future<SpacesApiResult<List<SpaceInvite>>> listInvites({
    required String authorization,
    required String spaceId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/spaces/$spaceId/invites'),
      authorization: authorization,
      createEmpty: space_pb.ListInvitesResponse.create,
    );
    return _map<List<SpaceInvite>>(result, (data) {
      final list = data.hasInviteList() ? data.inviteList : space_pb.InviteList();
      return <SpaceInvite>[
        for (final invite in list.invites) spaceInviteFromProto(invite),
      ];
    });
  }

  Future<SpacesApiResult<void>> revokeInvite({
    required String authorization,
    required String spaceId,
    required String inviteId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/invites/$inviteId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const SpacesApiOk(null),
      GatewayHttpFailure(:final error) => SpacesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<SpacesApiResult<SpaceInvite>> getInvite({
    required String authorization,
    required String code,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/invites/$code'),
      authorization: authorization,
      createEmpty: space_pb.GetInviteResponse.create,
    );
    return _map(result, (data) => spaceInviteFromProto(data.invite));
  }

  Future<SpacesApiResult<SpaceMembershipData>> joinByInvite({
    required String authorization,
    required String code,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/invites/$code/join'),
      authorization: authorization,
      body: space_pb.JoinByInviteRequest(code: code),
      createEmpty: space_pb.JoinByInviteResponse.create,
    );
    return _map(
      result,
      (data) => spaceMembershipFromProto(data.spaceMembership),
    );
  }

  Future<SpacesApiResult<SpaceMemberListData>> listMembers({
    required String authorization,
    required String spaceId,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/spaces/$spaceId/members',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return _mapJson(result, spaceMemberListFromJson);
  }

  Future<SpacesApiResult<void>> banMember({
    required String authorization,
    required String spaceId,
    required String accountId,
    String? profileId,
    String? reason,
  }) async {
    final body = <String, dynamic>{'account_id': accountId};
    if (profileId != null && profileId.isNotEmpty) {
      body['profile_id'] = profileId;
    }
    if (reason != null && reason.isNotEmpty) {
      body['reason'] = reason;
    }
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/bans'),
      authorization: authorization,
      body: body,
    );
    return _mapVoidJson(result);
  }

  Future<SpacesApiResult<void>> unbanMember({
    required String authorization,
    required String spaceId,
    required String accountId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/bans/$accountId'),
      authorization: authorization,
    );
    return _mapVoidHttp(result);
  }

  Future<SpacesApiResult<void>> timeoutMember({
    required String authorization,
    required String spaceId,
    required String profileId,
    required int durationSeconds,
    String? reason,
  }) async {
    final body = <String, dynamic>{'duration_seconds': durationSeconds};
    if (reason != null && reason.isNotEmpty) {
      body['reason'] = reason;
    }
    final result = await _gateway.postJson(
      uri: _gateway.resolve(
        '/api/v1/spaces/$spaceId/members/$profileId/timeout',
      ),
      authorization: authorization,
      body: body,
    );
    return _mapVoidJson(result);
  }

  Future<SpacesApiResult<void>> removeMemberTimeout({
    required String authorization,
    required String spaceId,
    required String profileId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve(
        '/api/v1/spaces/$spaceId/members/$profileId/timeout',
      ),
      authorization: authorization,
    );
    return _mapVoidHttp(result);
  }

  Future<SpacesApiResult<void>> kickMember({
    required String authorization,
    required String spaceId,
    required String profileId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/members/$profileId'),
      authorization: authorization,
    );
    return _mapVoidHttp(result);
  }

  Future<SpacesApiResult<SpaceTreeNodeData>> createSpaceChat({
    required String authorization,
    required String spaceId,
    required String name,
    ChatType chatType = ChatType.CHAT_TYPE_GROUP,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/spaces/$spaceId/chats'),
      authorization: authorization,
      body: chat_pb.CreateChatRequest(type: chatType, name: name),
      createEmpty: space_pb.UpsertTreeNodeResponse.create,
    );
    return _map(result, (data) {
      final voiceById = <String, VoiceRoomData>{};
      return spaceTreeNodeFromProto(data.spaceTreeNode, voiceById);
    });
  }

  SpacesApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => SpacesApiOk(parse(data)),
      GatewayHttpFailure(:final error) => SpacesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  SpacesApiResult<void> _mapVoidHttp(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const SpacesApiOk(null),
      GatewayHttpFailure(:final error) => SpacesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  SpacesApiResult<void> _mapVoidJson(
    GatewayHttpResult<Map<String, dynamic>> result,
  ) {
    return switch (result) {
      GatewayHttpOk() => const SpacesApiOk(null),
      GatewayHttpFailure(:final error) => SpacesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  SpacesApiResult<T> _mapJson<T>(
    GatewayHttpResult<Map<String, dynamic>> result,
    T Function(Map<String, dynamic> data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => SpacesApiOk(parse(data)),
      GatewayHttpFailure(:final error) => SpacesApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}

SpaceMemberListData spaceMemberListFromJson(Map<String, dynamic> data) {
  final list = data['space_member_list'];
  if (list is! Map<String, dynamic>) {
    return const SpaceMemberListData(members: []);
  }
  final rawMembers = list['members'];
  final members = <SpaceMemberRosterEntry>[];
  if (rawMembers is List) {
    for (final item in rawMembers) {
      if (item is! Map<String, dynamic>) continue;
      final roleNames = item['role_names'];
      members.add(
        SpaceMemberRosterEntry(
          profileId: item['profile_id'] as String? ?? '',
          roleNames: roleNames is List
              ? roleNames.whereType<String>().toList(growable: false)
              : const [],
          joinedAt: _spaceMemberJoinedAtFromJson(item['joined_at']) ??
              DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
          nickname: item['nickname'] as String?,
        ),
      );
    }
  }
  final nextCursor = list['next_cursor'] as String?;
  return SpaceMemberListData(
    members: members,
    nextCursor: nextCursor == null || nextCursor.isEmpty ? null : nextCursor,
  );
}

DateTime? _spaceMemberJoinedAtFromJson(dynamic raw) {
  if (raw is String && raw.isNotEmpty) {
    return DateTime.tryParse(raw);
  }
  if (raw is Map<String, dynamic>) {
    final seconds = raw['seconds'];
    if (seconds is num) {
      final nanos = (raw['nanos'] as num?)?.toInt() ?? 0;
      return DateTime.fromMillisecondsSinceEpoch(
        seconds.toInt() * 1000 + (nanos ~/ 1000000),
        isUtc: true,
      );
    }
  }
  return null;
}
