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
