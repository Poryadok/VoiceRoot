import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

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

  factory VoiceProfile.fromJson(Map<String, dynamic> json) {
    return VoiceProfile(
      id: json['id'] as String,
      accountId: json['account_id'] as String,
      username: json['username'] as String,
      discriminator: json['discriminator'] as String,
      displayName: json['display_name'] as String,
      avatarUrl: json['avatar_url'] as String?,
      bio: json['bio'] as String?,
      customStatus: json['custom_status'] as String?,
    );
  }
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

  factory VoicePresence.fromJson(Map<String, dynamic> json) {
    return VoicePresence(
      profileId: (json['profile_id'] ?? json['profileId']) as String,
      status: json['status'] as String? ?? 'invisible',
      lastSeen: _parseTimestamp(json['last_seen'] ?? json['lastSeen']),
    );
  }

  /// Parses gateway proto JSON (`presenceStatus`) and legacy test keys.
  static VoicePresence? fromGatewayBody(Map<String, dynamic> body) {
    final raw =
        body['presenceStatus'] ?? body['presence_status'] ?? body['presence'];
    if (raw is! Map<String, dynamic>) return null;
    return VoicePresence.fromJson(raw);
  }

  static Map<String, VoicePresence> bulkFromGatewayBody(
    Map<String, dynamic> body,
  ) {
    final raw = body['byProfileId'] ?? body['by_profile_id'];
    if (raw is! Map) return {};
    final out = <String, VoicePresence>{};
    for (final entry in raw.entries) {
      final value = entry.value;
      if (value is Map<String, dynamic>) {
        out[entry.key.toString()] = VoicePresence.fromJson(value);
      } else if (value is Map) {
        out[entry.key.toString()] = VoicePresence.fromJson(
          Map<String, dynamic>.from(value),
        );
      }
    }
    return out;
  }

  static DateTime? _parseTimestamp(Object? raw) {
    if (raw is! String || raw.isEmpty) return null;
    return DateTime.tryParse(raw)?.toUtc();
  }
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

  factory AvatarPresignedUpload.fromJson(Map<String, dynamic> json) {
    final rawHeaders = json['required_headers'] ?? json['requiredHeaders'];
    final headers = <String, String>{};
    if (rawHeaders is Map) {
      for (final entry in rawHeaders.entries) {
        headers[entry.key.toString()] = entry.value.toString();
      }
    }
    final rawExpiresAt = json['expires_at'] ?? json['expiresAt'];
    return AvatarPresignedUpload(
      httpMethod:
          (json['http_method'] ?? json['httpMethod']) as String? ?? 'PUT',
      uploadUrl: (json['upload_url'] ?? json['uploadUrl']) as String,
      requiredHeaders: headers,
      maxBytes: ((json['max_bytes'] ?? json['maxBytes']) as num).toInt(),
      expiresAt: rawExpiresAt is String
          ? DateTime.tryParse(rawExpiresAt)
          : null,
      publicUrl: (json['public_url'] ?? json['publicUrl']) as String,
      objectKey: (json['object_key'] ?? json['objectKey']) as String,
    );
  }
}

/// HTTP client for User routes via API Gateway (`/api/v1/users/**`).
class VoiceUsersClient {
  VoiceUsersClient({
    required http.Client httpClient,
    required GatewayConfig config,
  }) : _http = httpClient,
       _config = config;

  final http.Client _http;
  final GatewayConfig _config;

  Future<UsersApiResult<VoiceProfile>> getMe({
    required String authorization,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/users/me');
    return _get(uri, authorization, _parseProfile);
  }

  Future<UsersApiResult<SearchProfilesData>> searchProfiles({
    required String authorization,
    required String query,
    String? cursor,
    int? pageSize,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final params = <String, String>{'q': query};
    if (cursor != null && cursor.isNotEmpty) params['cursor'] = cursor;
    if (pageSize != null) params['page_size'] = '$pageSize';
    final uri = Uri.parse(
      _config.baseUrl,
    ).replace(path: '/api/v1/users/search', queryParameters: params);
    return _get(uri, authorization, (body) {
      final profileList = body['profile_list'] as Map<String, dynamic>? ?? {};
      final rawProfiles = profileList['profiles'] as List<dynamic>? ?? [];
      final page = body['page'] as Map<String, dynamic>? ?? {};
      return SearchProfilesData(
        profiles: rawProfiles
            .map((e) => VoiceProfile.fromJson(e as Map<String, dynamic>))
            .toList(),
        nextCursor: page['next_cursor'] as String?,
        hasMore: page['has_more'] as bool? ?? false,
      );
    });
  }

  Future<UsersApiResult<VoiceProfile>> getProfile({
    required String authorization,
    required String profileId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final uri = Uri.parse(
      _config.baseUrl,
    ).resolve('/api/v1/users/profiles/$profileId');
    return _get(uri, authorization, (body) {
      final profile = body['profile'] as Map<String, dynamic>;
      return VoiceProfile.fromJson(profile);
    });
  }

  Future<UsersApiResult<VoiceProfile>> updateProfile({
    required String authorization,
    String? displayName,
    String? bio,
    String? avatarUrl,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final body = <String, dynamic>{};
    if (displayName != null) body['display_name'] = displayName;
    if (bio != null) body['bio'] = bio;
    if (avatarUrl != null) body['avatar_url'] = avatarUrl;
    final uri = Uri.parse(_config.baseUrl).resolve('/api/v1/users/me');
    return _sendJson(
      method: 'PATCH',
      uri: uri,
      authorization: authorization,
      body: body,
      parse: _parseProfile,
    );
  }

  Future<UsersApiResult<AvatarPresignedUpload>> createAvatarPresignedUpload({
    required String authorization,
    required String contentType,
    required int contentLength,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
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
    final uri = Uri.parse(
      _config.baseUrl,
    ).resolve('/api/v1/users/me/avatar/presigned-upload');
    return _sendJson(
      method: 'POST',
      uri: uri,
      authorization: authorization,
      body: {
        'content_type': normalizedContentType,
        'content_length': contentLength,
      },
      parse: AvatarPresignedUpload.fromJson,
    );
  }

  Future<UsersApiResult<void>> uploadAvatarBytes({
    required Uri uploadUrl,
    required Map<String, String> requiredHeaders,
    required List<int> bytes,
  }) async {
    try {
      final res = await _http.put(
        uploadUrl,
        headers: requiredHeaders,
        body: bytes,
      );
      if (res.statusCode >= 200 && res.statusCode < 300) {
        return const UsersApiOk<void>(null);
      }
      return UsersApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return UsersApiFailure(message: '$e');
    }
  }

  Future<UsersApiResult<VoicePresence>> getPresence({
    required String authorization,
    required String profileId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final uri = Uri.parse(
      _config.baseUrl,
    ).resolve('/api/v1/users/profiles/$profileId/presence');
    return _get(uri, authorization, (body) {
      final presence = VoicePresence.fromGatewayBody(body);
      if (presence == null) {
        throw const FormatException('missing presence');
      }
      return presence;
    });
  }

  Future<UsersApiResult<Map<String, VoicePresence>>> getBulkPresence({
    required String authorization,
    required List<String> profileIds,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final uri = Uri.parse(
      _config.baseUrl,
    ).resolve('/api/v1/users/presence/bulk');
    try {
      final res = await _http.post(
        uri,
        headers: {
          'Authorization': authorization,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({'profileIds': profileIds}),
      );
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        return UsersApiOk(VoicePresence.bulkFromGatewayBody(decoded));
      }
      return UsersApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return UsersApiFailure(message: '$e');
    }
  }

  Future<UsersApiResult<T>> _get<T>(
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
        return UsersApiOk(parse(decoded));
      }
      return UsersApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return UsersApiFailure(message: '$e');
    }
  }

  Future<UsersApiResult<T>> _sendJson<T>({
    required String method,
    required Uri uri,
    required String authorization,
    required Map<String, dynamic> body,
    required T Function(Map<String, dynamic> body) parse,
  }) async {
    try {
      final req = http.Request(method, uri)
        ..headers['Authorization'] = authorization
        ..headers['Content-Type'] = 'application/json'
        ..body = jsonEncode(body);
      final streamed = await _http.send(req);
      final res = await http.Response.fromStream(streamed);
      if (res.statusCode == 200) {
        final decoded = jsonDecode(res.body) as Map<String, dynamic>;
        return UsersApiOk(parse(decoded));
      }
      return UsersApiFailure(
        message: _failureMessage(res),
        errorCode: _errorCode(res),
        statusCode: res.statusCode,
      );
    } catch (e) {
      return UsersApiFailure(message: '$e');
    }
  }

  static VoiceProfile _parseProfile(Map<String, dynamic> body) {
    final profile = body['profile'] as Map<String, dynamic>;
    return VoiceProfile.fromJson(profile);
  }

  static String _failureMessage(http.Response res) {
    final code = _errorCode(res);
    if (code != null) return code;
    try {
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic>) {
        final message = decoded['message'];
        if (message is String && message.isNotEmpty) return message;
      }
    } catch (_) {
      // ignore malformed body
    }
    return 'HTTP ${res.statusCode}';
  }

  static String? _errorCode(http.Response res) {
    try {
      final decoded = jsonDecode(res.body);
      if (decoded is Map<String, dynamic>) {
        final err = decoded['error'] ?? decoded['error_code'];
        if (err is String && err.isNotEmpty) return err;
      }
    } catch (_) {
      // ignore malformed body
    }
    return null;
  }
}
