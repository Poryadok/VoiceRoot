import '../gen/voice/social/v1/social.pb.dart' as social_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

const String kFriendsMissingBaseUrlDetail = 'missing base URL';

sealed class FriendsApiResult<T> {
  const FriendsApiResult();
}

final class FriendsApiOk<T> extends FriendsApiResult<T> {
  const FriendsApiOk(this.data);
  final T data;
}

final class FriendsApiFailure extends FriendsApiResult<Never> {
  const FriendsApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

final class FriendsApiEmpty extends FriendsApiResult<void> {
  const FriendsApiEmpty();
}

class FriendsListData {
  const FriendsListData({required this.friends, this.nextCursor});

  final List<String> friends;
  final String? nextCursor;
}

class FriendRequestsData {
  const FriendRequestsData({required this.incoming, required this.outgoing});

  final List<String> incoming;
  final List<String> outgoing;
}

/// HTTP client for Social friend routes (`/api/v1/friends/**`).
class VoiceFriendsClient {
  VoiceFriendsClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<FriendsApiResult<FriendsListData>> listFriends({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/friends',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: social_pb.ListFriendsResponse.create,
    );
    return _map(
      result,
      (data) => friendsListFromProto(
        data.hasFriendList() ? data.friendList : social_pb.FriendList(),
      ),
    );
  }

  Future<FriendsApiResult<FriendRequestsData>> listFriendRequests({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/friends/requests'),
      authorization: authorization,
      createEmpty: social_pb.ListFriendRequestsResponse.create,
    );
    return _map(
      result,
      (data) => friendRequestsFromProto(
        data.hasFriendRequestList()
            ? data.friendRequestList
            : social_pb.FriendRequestList(),
      ),
    );
  }

  Future<FriendsApiResult<void>> sendFriendInvitation({
    required String authorization,
    required String targetProfileId,
  }) {
    return _postInvitation(
      '/api/v1/friends/invitations',
      authorization,
      social_pb.SendFriendInvitationRequest(
        targetProfileId: targetProfileId,
      ),
    );
  }

  Future<FriendsApiResult<void>> acceptFriendInvitation({
    required String authorization,
    required String requesterProfileId,
  }) {
    return _postEmpty(
      '/api/v1/friends/invitations/$requesterProfileId/accept',
      authorization,
    );
  }

  Future<FriendsApiResult<void>> declineFriendInvitation({
    required String authorization,
    required String requesterProfileId,
  }) {
    return _postEmpty(
      '/api/v1/friends/invitations/$requesterProfileId/decline',
      authorization,
    );
  }

  Future<FriendsApiResult<void>> blockAccount({
    required String authorization,
    required String blockedAccountId,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/friends/blocks'),
      authorization: authorization,
      jsonBody: {'blocked_account_id': blockedAccountId},
    );
    return _mapEmpty(result);
  }

  Future<FriendsApiResult<void>> _postInvitation(
    String path,
    String authorization,
    social_pb.SendFriendInvitationRequest body,
  ) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve(path),
      authorization: authorization,
      body: body,
      createEmpty: social_pb.SendFriendInvitationResponse.create,
      allowNoContent: true,
    );
    return _mapEmpty(result);
  }

  Future<FriendsApiResult<void>> _postEmpty(
    String path,
    String authorization,
  ) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve(path),
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  FriendsApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => FriendsApiOk(parse(data)),
      GatewayHttpFailure(:final error) => FriendsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  FriendsApiResult<void> _mapEmpty(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const FriendsApiEmpty(),
      GatewayHttpFailure(:final error) => FriendsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
