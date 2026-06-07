import 'package:fixnum/fixnum.dart';

import '../gen/voice/user/v1/user.pb.dart' as user_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'presigned_upload.dart';
import 'proto_mappers.dart';

const String kUsersMissingBaseUrlDetail = 'missing base URL';
const int kProfileDisplayNameMaxLength = 32;

const int kProfileAvatarMaxBytes = 5 * 1024 * 1024;
const Set<String> kProfileAvatarContentTypes = {
  'image/jpeg',
  'image/png',
  'image/webp',
};

sealed class UsersApiResult<T> {
  const UsersApiResult();
}

final class UsersApiOk<T> extends UsersApiResult<T> {
  const UsersApiOk(this.data);
  final T data;
}

final class UsersApiFailure extends UsersApiResult<Never> {
  const UsersApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class VoiceProfile {
  const VoiceProfile({
    required this.id,
    required this.accountId,
    required this.username,
    required this.discriminator,
    required this.displayName,
    this.avatarUrl,
    this.bio,
    this.customStatus,
  });

  final String id;
  final String accountId;
  final String username;
  final String discriminator;
  final String displayName;
  final String? avatarUrl;
  final String? bio;
  final String? customStatus;

  String get handle => '@$username#$discriminator';
}

class VoicePresence {
  const VoicePresence({
    required this.profileId,
    required this.status,
    this.lastSeen,
  });

  final String profileId;
  final String status;
  final DateTime? lastSeen;

  bool get isOnline => status == 'online';

  bool get isIdle => status == 'idle';

  bool get isDnd => status == 'dnd';
}

class SearchProfilesData {
  const SearchProfilesData({
    required this.profiles,
    this.nextCursor,
    this.hasMore = false,
  });

  final List<VoiceProfile> profiles;
  final String? nextCursor;
  final bool hasMore;
}

class AvatarPresignedUpload {
  const AvatarPresignedUpload({
    required this.httpMethod,
    required this.uploadUrl,
    required this.requiredHeaders,
    required this.maxBytes,
    this.expiresAt,
    required this.publicUrl,
    required this.objectKey,
  });

  final String httpMethod;
  final String uploadUrl;
  final Map<String, String> requiredHeaders;
  final int maxBytes;
  final DateTime? expiresAt;
  final String publicUrl;
  final String objectKey;
}

/// HTTP client for User routes via API Gateway (`/api/v1/users/**`).
class VoiceUsersClient {
  VoiceUsersClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<UsersApiResult<VoiceProfile>> getMe({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/users/me'),
      authorization: authorization,
      createEmpty: user_pb.GetProfileResponse.create,
    );
    return _map(
      result,
      (data) => voiceProfileFromProto(
        data.hasProfile() ? data.profile : user_pb.Profile(),
      ),
    );
  }

  Future<UsersApiResult<SearchProfilesData>> searchProfiles({
    required String authorization,
    required String query,
    String? cursor,
    int? pageSize,
  }) async {
    final params = <String, String>{'q': query};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = _gateway.replace(
      path: '/api/v1/users/search',
      queryParameters: params,
    );
    final result = await _gateway.getProto(
      uri,
      authorization: authorization,
      createEmpty: user_pb.SearchProfilesResponse.create,
    );
    return _map(
      result,
      (data) => searchProfilesFromProto(data as user_pb.SearchProfilesResponse),
    );
  }

  Future<UsersApiResult<VoiceProfile>> getProfile({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/users/profiles/$profileId'),
      authorization: authorization,
      createEmpty: user_pb.GetProfileResponse.create,
    );
    return _map(
      result,
      (data) => voiceProfileFromProto(
        data.hasProfile() ? data.profile : user_pb.Profile(),
      ),
    );
  }

  Future<UsersApiResult<VoiceProfile>> updateProfile({
    required String authorization,
    String? displayName,
    String? bio,
    String? avatarUrl,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/users/me'),
      authorization: authorization,
      body: updateProfileRequestToProto(
        displayName: displayName,
        bio: bio,
        avatarUrl: avatarUrl,
      ),
      createEmpty: user_pb.UpdateProfileResponse.create,
    );
    return _map(
      result,
      (data) => voiceProfileFromProto(
        data.hasProfile() ? data.profile : user_pb.Profile(),
      ),
    );
  }

  Future<UsersApiResult<AvatarPresignedUpload>> createAvatarPresignedUpload({
    required String authorization,
    required String contentType,
    required int contentLength,
  }) async {
    final normalizedContentType = contentType.trim().toLowerCase();
    if (!kProfileAvatarContentTypes.contains(normalizedContentType)) {
      return const UsersApiFailure(
        message: 'unsupported_avatar_content_type',
        errorCode: 'unsupported_avatar_content_type',
      );
    }
    if (contentLength <= 0 || contentLength > kProfileAvatarMaxBytes) {
      return const UsersApiFailure(
        message: 'invalid_avatar_size',
        errorCode: 'invalid_avatar_size',
      );
    }
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/users/me/avatar/presigned-upload'),
      authorization: authorization,
      body: user_pb.CreateAvatarPresignedUploadRequest(
        contentType: normalizedContentType,
        contentLength: Int64(contentLength),
      ),
      createEmpty: user_pb.CreateAvatarPresignedUploadResponse.create,
    );
    return _map(
      result,
      (data) => avatarPresignedFromProto(
        data as user_pb.CreateAvatarPresignedUploadResponse,
      ),
    );
  }

  Future<UsersApiResult<void>> uploadAvatarBytes({
    required Uri uploadUrl,
    required Map<String, String> requiredHeaders,
    required List<int> bytes,
  }) async {
    final result = await putPresigned(
      gateway: _gateway,
      uploadUrl: uploadUrl,
      requiredHeaders: requiredHeaders,
      bytes: bytes,
    );
    return _mapEmpty(result);
  }

  Future<UsersApiResult<VoicePresence>> getPresence({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/users/profiles/$profileId/presence'),
      authorization: authorization,
      createEmpty: user_pb.GetPresenceResponse.create,
    );
    return _map(
      result,
      (data) => voicePresenceFromProto(
        data.hasPresenceStatus()
            ? data.presenceStatus
            : user_pb.PresenceStatus(),
      ),
    );
  }

  Future<UsersApiResult<Map<String, VoicePresence>>> getBulkPresence({
    required String authorization,
    required List<String> profileIds,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/users/presence/bulk'),
      authorization: authorization,
      body: bulkPresenceRequestToProto(profileIds),
      createEmpty: user_pb.GetBulkPresenceResponse.create,
    );
    return _map(result, (data) {
      return {
        for (final entry in data.byProfileId.entries)
          entry.key: voicePresenceFromProto(entry.value),
      };
    });
  }

  UsersApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => UsersApiOk(parse(data)),
      GatewayHttpFailure(:final error) => UsersApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  UsersApiResult<void> _mapEmpty(GatewayHttpResult<dynamic> result) {
    return switch (result) {
      GatewayHttpOk() => const UsersApiOk<void>(null),
      GatewayHttpFailure(:final error) => UsersApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
