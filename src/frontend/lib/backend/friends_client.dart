import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

const String kFriendsMissingBaseUrlDetail = 'missing base URL';

sealed class FriendsApiResult<T> {
  const FriendsApiResult();
}

final class FriendsApiOk<T> extends FriendsApiResult<T> {
  const FriendsApiOk(this.data);
  final T data;
}

final class FriendsApiFailure extends FriendsApiResult<Never> {
  const FriendsApiFailure({required this.message, this.errorCode, this.statusCode});

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
  const FriendRequestsData({
    required this.incoming,
    required this.outgoing,
  });

  final List<String> incoming;
  final List<String> outgoing;
}

/// HTTP client for Social friend routes (`/api/v1/friends/**`).
class VoiceFriendsClient {
  VoiceFriendsClient({
    required http.Client httpClient,
    required GatewayConfig config,
  })  : _http = httpClient,
        _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<FriendsApiResult<FriendsListData>> listFriends({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    if (!_config.hasBaseUrl) {
      return const FriendsApiFailure(message: kFriendsMissingBaseUrlDetail);
    }
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = Uri.parse(_config.baseUrl).replace(
      path: '/api/v1/friends',
      queryParameters: params.isEmpty ? null : params,
    );
    return _get(uri, authorization, (body) {
      final list = body['friend_list'] as Map<String, dynamic>? ?? {};
      final edges = list['friends'] as List<dynamic>? ?? [];
      return FriendsListData(
        friends: edges
            .map((e) => (e as Map<String, dynamic>)['profile_id'] as String)
            .toList(),
        nextCursor: list['next_cursor'] as String?,
      );
    });
  }

  Future<FriendsApiResult<FriendRequestsData>> listFriendRequests({
    required String authorization,
  }) async {
    if (!_config.hasBaseUrl) {
      return const FriendsApiFailure(message: kFriendsMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/friends/requests');
    return _get(uri, authorization, (body) {
      final list =
          body['friend_request_list'] as Map<String, dynamic>? ?? {};
      return FriendRequestsData(
        incoming: _profileIds(list['incoming']),
        outgoing: _profileIds(list['outgoing']),
      );
    });
  }

  Future<FriendsApiResult<void>> sendFriendInvitation({
    required String authorization,
    required String targetProfileId,
  }) =>
      _postEmpty(
        '/api/v1/friends/invitations',
        authorization,
        {'target_profile_id': targetProfileId},
      );

  Future<FriendsApiResult<void>> acceptFriendInvitation({
    required String authorization,
    required String requesterProfileId,
  }) =>
      _postEmpty(
        '/api/v1/friends/invitations/$requesterProfileId/accept',
        authorization,
        null,
      );

  Future<FriendsApiResult<void>> declineFriendInvitation({
    required String authorization,
    required String requesterProfileId,
  }) =>
      _postEmpty(
        '/api/v1/friends/invitations/$requesterProfileId/decline',
        authorization,
        null,
      );

  List<String> _profileIds(dynamic raw) {
    if (raw is! List) return const [];
    return raw
        .map((e) => (e as Map<String, dynamic>)['profile_id'] as String)
        .toList();
  }

  Future<FriendsApiResult<T>> _get<T>(
    Uri uri,
    String authorization,
    T Function(Map<String, dynamic> body) parse,
  ) async {
    try {
      final res = await _http.get(
        uri,
        headers: {'Authorization': authorization},
      );
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        return FriendsApiOk(parse(decoded));
      }
      return FriendsApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return FriendsApiFailure(message: '$e');
    }
  }

  Future<FriendsApiResult<void>> _postEmpty(
    String path,
    String authorization,
    Map<String, dynamic>? body,
  ) async {
    if (!_config.hasBaseUrl) {
      return const FriendsApiFailure(message: kFriendsMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve(path);
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          if (body != null) 'Content-Type': 'application/json',
        },
        body: body == null ? null : jsonEncode(body),
      );
      if (res.statusCode == 200) return const FriendsApiEmpty();
      return FriendsApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return FriendsApiFailure(message: '$e');
    }
  }

  static String _failureMessage(http.Response res) {
    final code = _errorCode(res);
    if (code != null) return code;
    return 'HTTP ${res.statusCode}';
  }

  static String? _errorCode(http.Response res) {
    try {
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic>) {
        final err = decoded['error'];
        if (err is String && err.isNotEmpty) return err;
      }
    } catch (_) {
      // ignore malformed body
    }
    return null;
  }
}
