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
  'image/gif',
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
    this.bannerUrl,
    this.bio,
    this.customStatus,
    this.isPrimary = false,
    this.verificationType = 'none',
    this.verificationBadge,
    this.accentColor,
    this.locale = '',
    this.frozenAt,
  });

  final String id;
  final String accountId;
  final String username;
  final String discriminator;
  final String displayName;
  final String? avatarUrl;
  final String? bannerUrl;
  final String? bio;
  final String? customStatus;
  final bool isPrimary;
  final String verificationType;
  final String? verificationBadge;
  final String? accentColor;
  final String locale;
  final DateTime? frozenAt;

  bool get isFrozen => frozenAt != null;

  String get handle => '@$username#$discriminator';
}

/// Session bar label: guest accounts show the chosen nickname; others show @handle.
String profileSessionBarLabel(VoiceProfile profile, {required bool isGuest}) {
  if (isGuest && profile.displayName.isNotEmpty) {
    return profile.displayName;
  }
  return profile.handle;
}

class VoicePresence {
  const VoicePresence({
    required this.profileId,
    required this.status,
    this.customStatus,
    this.gameTitle,
    this.lastSeen,
  });

  final String profileId;
  final String status;
  final String? customStatus;
  final String? gameTitle;
  final DateTime? lastSeen;

  bool get isOnline => status == 'online';

  bool get isIdle => status == 'idle';

  bool get isDnd => status == 'dnd';

  bool get isInvisible => status == 'invisible';

  bool get appearsOffline => status.isEmpty || status == 'offline';
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

class VerificationStatus {
  const VerificationStatus({
    required this.profileId,
    required this.verificationType,
    this.badge,
    this.pendingOrganizationChallenge,
  });

  final String profileId;
  final String verificationType;
  final String? badge;
  final PendingOrganizationChallenge? pendingOrganizationChallenge;
}

class PendingOrganizationChallenge {
  const PendingOrganizationChallenge({
    required this.domain,
    required this.txtRecord,
    required this.expiresAt,
  });

  final String domain;
  final String txtRecord;
  final DateTime expiresAt;
}

class OrganizationVerificationChallenge {
  const OrganizationVerificationChallenge({
    required this.domain,
    required this.txtRecord,
  });

  final String domain;
  final String txtRecord;
}

/// HTTP client for User routes via API Gateway (`/api/v1/users/**`).
class VoiceUsersClient {
  VoiceUsersClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<UsersApiResult<List<VoiceProfile>>> listMyProfiles({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/users/profiles'),
      authorization: authorization,
      createEmpty: user_pb.ListMyProfilesResponse.create,
    );
    return _map(
      result,
      (data) => voiceProfileListFromProto(
        data.hasProfileList() ? data.profileList : user_pb.ProfileList(),
      ),
    );
  }

  Future<UsersApiResult<void>> deleteProfile({
    required String authorization,
    required String profileId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/users/profiles/$profileId'),
      authorization: authorization,
    );
    return _mapEmpty(result);
  }

  Future<UsersApiResult<VoiceProfile>> createProfile({
    required String authorization,
    required String displayName,
    String? username,
    String preset = 'personal',
    String? accentColor,
  }) async {
    final body = user_pb.CreateProfileRequest(displayName: displayName)
      ..preset = preset;
    if (username != null && username.isNotEmpty) {
      body.username = username;
    }
    if (accentColor != null && accentColor.isNotEmpty) {
      body.accentColor = accentColor;
    }
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/users/profiles'),
      authorization: authorization,
      body: body,
      createEmpty: user_pb.CreateProfileResponse.create,
    );
    return _map(
      result,
      (data) => voiceProfileFromProto(
        data.hasProfile() ? data.profile : user_pb.Profile(),
      ),
    );
  }

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

  Future<UsersApiResult<VerificationStatus>> getVerificationStatus({
    required String authorization,
  }) async {
    final result = await _gateway.getJson(
      _gateway.resolve('/api/v1/users/me/verification'),
      authorization: authorization,
    );
    return _mapVerificationStatus(result);
  }

  Future<UsersApiResult<OrganizationVerificationChallenge>>
  startOrganizationVerification({
    required String authorization,
    required String domain,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/users/me/verification/organization'),
      authorization: authorization,
      body: {'domain': domain.trim()},
    );
    return switch (result) {
      GatewayHttpOk(:final data) => _organizationChallengeFromJson(data),
      GatewayHttpFailure(:final error) => UsersApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<UsersApiResult<VerificationStatus>> checkOrganizationVerification({
    required String authorization,
  }) async {
    final result = await _gateway.postJson(
      uri: _gateway.resolve('/api/v1/users/me/verification/organization/check'),
      authorization: authorization,
      body: const {},
    );
    return _mapVerificationStatus(result);
  }

  UsersApiResult<VerificationStatus> _mapVerificationStatus(
    GatewayHttpResult<Map<String, dynamic>> result,
  ) {
    return switch (result) {
      GatewayHttpFailure(:final error) => UsersApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
      GatewayHttpOk(:final data) => _verificationStatusFromJson(data),
    };
  }

  UsersApiResult<VerificationStatus> _verificationStatusFromJson(
    Map<String, dynamic> data,
  ) {
    final status = data['verification_status'];
    if (status is! Map<String, dynamic> ||
        status['profile_id'] is! String ||
        status['verification_type'] is! String ||
        ((status['badge'] != null) && status['badge'] is! String)) {
      return const UsersApiFailure(
        message: 'invalid_response',
        errorCode: 'invalid_response',
      );
    }
    final verificationType = status['verification_type'] as String;
    if (!const {
      'none',
      'personal',
      'organization',
    }.contains(verificationType)) {
      return const UsersApiFailure(
        message: 'invalid_response',
        errorCode: 'invalid_response',
      );
    }
    PendingOrganizationChallenge? pendingChallenge;
    final pendingJson = data['pending_organization_challenge'];
    if (pendingJson != null) {
      if (pendingJson is! Map<String, dynamic> ||
          pendingJson['domain'] is! String ||
          (pendingJson['domain'] as String).isEmpty ||
          pendingJson['txt_record'] is! String ||
          (pendingJson['txt_record'] as String).isEmpty ||
          pendingJson['expires_at'] is! String) {
        return const UsersApiFailure(
          message: 'invalid_response',
          errorCode: 'invalid_response',
        );
      }
      final expiresAt = DateTime.tryParse(pendingJson['expires_at'] as String);
      if (expiresAt == null || !expiresAt.isUtc) {
        return const UsersApiFailure(
          message: 'invalid_response',
          errorCode: 'invalid_response',
        );
      }
      pendingChallenge = PendingOrganizationChallenge(
        domain: pendingJson['domain'] as String,
        txtRecord: pendingJson['txt_record'] as String,
        expiresAt: expiresAt,
      );
    }
    return UsersApiOk(
      VerificationStatus(
        profileId: status['profile_id'] as String,
        verificationType: verificationType,
        badge: status['badge'] as String?,
        pendingOrganizationChallenge: pendingChallenge,
      ),
    );
  }

  UsersApiResult<OrganizationVerificationChallenge>
  _organizationChallengeFromJson(Map<String, dynamic> data) {
    final domain = data['domain'];
    final txtRecord = data['txt_record'];
    if (domain is! String ||
        domain.isEmpty ||
        txtRecord is! String ||
        txtRecord.isEmpty) {
      return const UsersApiFailure(
        message: 'invalid_response',
        errorCode: 'invalid_response',
      );
    }
    return UsersApiOk(
      OrganizationVerificationChallenge(domain: domain, txtRecord: txtRecord),
    );
  }

  Future<UsersApiResult<VoiceProfile>> updateProfile({
    required String authorization,
    String? displayName,
    String? bio,
    String? avatarUrl,
    String? accentColor,
    String? locale,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/users/me'),
      authorization: authorization,
      body: updateProfileRequestToProto(
        displayName: displayName,
        bio: bio,
        avatarUrl: avatarUrl,
        accentColor: accentColor,
        locale: locale,
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

  Future<UsersApiResult<void>> updatePresence({
    required String authorization,
    required String status,
    String? customStatus,
  }) async {
    final body = user_pb.UpdatePresenceRequest(status: status);
    if (customStatus != null) {
      body.customStatus = customStatus;
    }
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/users/me/presence'),
      authorization: authorization,
      body: body,
      createEmpty: user_pb.UpdatePresenceResponse.create,
    );
    return _mapEmpty(result);
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
