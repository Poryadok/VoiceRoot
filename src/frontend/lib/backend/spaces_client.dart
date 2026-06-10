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
}
