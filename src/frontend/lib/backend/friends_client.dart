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

class ContactEntry {
  const ContactEntry({
    required this.profileId,
    required this.source,
    required this.isFavorite,
  });

  final String profileId;
  final String source;
  final bool isFavorite;
}

class ContactsListData {
  const ContactsListData({required this.contacts, this.nextCursor});

  final List<ContactEntry> contacts;
  final String? nextCursor;
}

class FavoritesListData {
  const FavoritesListData({required this.favorites});

  final List<String> favorites;
}

class BlockedEntry {
  const BlockedEntry({required this.blockedAccountId});

  final String blockedAccountId;
}

class BlockedListData {
  const BlockedListData({required this.blocked, this.nextCursor});

  final List<BlockedEntry> blocked;
  final String? nextCursor;
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
    final result = await _gateway.getJson(
      uri,
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => FriendsApiOk(_friendsListFromJson(data)),
      GatewayHttpFailure(:final error) => FriendsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  FriendsListData _friendsListFromJson(Map<String, dynamic> data) {
    final list = data['friend_list'] as Map<String, dynamic>? ?? data;
    final friendsRaw = list['friends'] as List<dynamic>?;
    if (friendsRaw != null) {
      final ids = <String>[
        for (final item in friendsRaw)
          if (item is Map<String, dynamic>)
            item['profile_id'] as String? ??
                item['profileId'] as String? ??
                '',
      ].where((id) => id.isNotEmpty).toList(growable: false);
      return FriendsListData(
        friends: ids,
        nextCursor: list['next_cursor'] as String? ?? list['nextCursor'] as String?,
      );
    }
    final profileIds = list['profile_ids'] as List<dynamic>? ??
        list['profileIds'] as List<dynamic>? ??
        const [];
    return FriendsListData(
      friends: profileIds.map((id) => id.toString()).toList(growable: false),
      nextCursor: list['next_cursor'] as String? ?? list['nextCursor'] as String?,
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

  Future<FriendsApiResult<void>> removeFriend({
    required String authorization,
    required String friendProfileId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/friends/$friendProfileId'),
      authorization: authorization,
    );
    return _mapEmpty(result);
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

  Future<FriendsApiResult<ContactsListData>> listContacts({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/friends/contacts',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return switch (result) {
      GatewayHttpOk(:final data) => FriendsApiOk(_contactsListFromJson(data)),
      GatewayHttpFailure(:final error) => FriendsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  ContactsListData _contactsListFromJson(Map<String, dynamic> data) {
    final list = data['contact_list'] as Map<String, dynamic>? ?? data;
    final contactsRaw = list['contacts'] as List<dynamic>? ?? const [];
    final contacts = <ContactEntry>[
      for (final item in contactsRaw)
        if (item is Map<String, dynamic>)
          ContactEntry(
            profileId:
                item['profile_id'] as String? ??
                item['profileId'] as String? ??
                '',
            source: item['source'] as String? ?? '',
            isFavorite:
                item['is_favorite'] == true || item['isFavorite'] == true,
          ),
    ].where((c) => c.profileId.isNotEmpty).toList(growable: false);
    return ContactsListData(
      contacts: contacts,
      nextCursor:
          list['next_cursor'] as String? ?? list['nextCursor'] as String?,
    );
  }

  Future<FriendsApiResult<void>> addContact({
    required String authorization,
    required String targetProfileId,
    String source = 'manual',
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/friends/contacts'),
      authorization: authorization,
      jsonBody: {
        'target_profile_id': targetProfileId,
        'source': source,
      },
    );
    return _mapEmpty(result);
  }

  Future<FriendsApiResult<FavoritesListData>> listFavorites({
    required String authorization,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/friends/favorites'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk(:final data) => FriendsApiOk(_favoritesListFromJson(data)),
      GatewayHttpFailure(:final error) => FriendsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  FavoritesListData _favoritesListFromJson(Map<String, dynamic> data) {
    final list = data['friend_list'] as Map<String, dynamic>? ?? data;
    final friendsRaw = list['friends'] as List<dynamic>?;
    if (friendsRaw != null) {
      final ids = <String>[
        for (final item in friendsRaw)
          if (item is Map<String, dynamic>)
            item['profile_id'] as String? ??
                item['profileId'] as String? ??
                '',
      ].where((id) => id.isNotEmpty).toList(growable: false);
      return FavoritesListData(favorites: ids);
    }
    final profileIds = list['profile_ids'] as List<dynamic>? ??
        list['profileIds'] as List<dynamic>? ??
        const [];
    return FavoritesListData(
      favorites: profileIds.map((id) => id.toString()).toList(growable: false),
    );
  }

  Future<FriendsApiResult<void>> setFavorite({
    required String authorization,
    required String friendProfileId,
    required bool favorite,
  }) async {
    final result = await _gateway.postEmpty(
      uri: _gateway.resolve('/api/v1/friends/favorites'),
      authorization: authorization,
      jsonBody: {
        'friend_profile_id': friendProfileId,
        'favorite': favorite,
      },
    );
    return _mapEmpty(result);
  }

  Future<FriendsApiResult<BlockedListData>> listBlocked({
    required String authorization,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/friends/blocks',
      queryParameters: params.isEmpty ? null : params,
    );
    final result = await _gateway.getJson(uri, authorization: authorization);
    return switch (result) {
      GatewayHttpOk(:final data) => FriendsApiOk(_blockedListFromJson(data)),
      GatewayHttpFailure(:final error) => FriendsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  BlockedListData _blockedListFromJson(Map<String, dynamic> data) {
    final list = data['blocked_list'] as Map<String, dynamic>? ?? data;
    final blockedRaw = list['blocked'] as List<dynamic>? ?? const [];
    final blocked = <BlockedEntry>[
      for (final item in blockedRaw)
        if (item is Map<String, dynamic>)
          BlockedEntry(
            blockedAccountId:
                item['blocked_account_id'] as String? ??
                item['blockedAccountId'] as String? ??
                '',
          ),
    ].where((b) => b.blockedAccountId.isNotEmpty).toList(growable: false);
    return BlockedListData(
      blocked: blocked,
      nextCursor:
          list['next_cursor'] as String? ?? list['nextCursor'] as String?,
    );
  }

  Future<FriendsApiResult<void>> unblockAccount({
    required String authorization,
    required String blockedAccountId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/friends/blocks/$blockedAccountId'),
      authorization: authorization,
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
