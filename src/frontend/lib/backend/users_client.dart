import 'dart:convert';

import 'package:http/http.dart' as http;

import 'gateway_config.dart';

const String kUsersMissingBaseUrlDetail = 'missing base URL';

sealed class UsersApiResult<T> {
  const UsersApiResult();
}

final class UsersApiOk<T> extends UsersApiResult<T> {
  const UsersApiOk(this.data);
  final T data;
}

final class UsersApiFailure extends UsersApiResult<Never> {
  const UsersApiFailure({required this.message, this.errorCode, this.statusCode});

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
  const VoicePresence({required this.profileId, required this.status});

  final String profileId;
  final String status;

  bool get isOnline => status == 'online';

  bool get isIdle => status == 'idle';

  bool get isDnd => status == 'dnd';

  factory VoicePresence.fromJson(Map<String, dynamic> json) {
    return VoicePresence(
      profileId: json['profile_id'] as String,
      status: json['status'] as String? ?? 'invisible',
    );
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

/// HTTP client for User routes via API Gateway (`/api/v1/users/**`).
class VoiceUsersClient {
  VoiceUsersClient({
    required http.Client httpClient,
    required GatewayConfig config,
  })  : _http = httpClient,
        _config = config;

  final http.Client _http;
  final GatewayConfig _config;

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
    final uri = Uri.parse(_config.baseUrl).replace(
      path: '/api/v1/users/search',
      queryParameters: params,
    );
    return _get(
      uri,
      authorization,
      (body) {
        final profileList =
            body['profile_list'] as Map<String, dynamic>? ?? {};
        final rawProfiles = profileList['profiles'] as List<dynamic>? ?? [];
        final page = body['page'] as Map<String, dynamic>? ?? {};
        return SearchProfilesData(
          profiles: rawProfiles
              .map((e) => VoiceProfile.fromJson(e as Map<String, dynamic>))
              .toList(),
          nextCursor: page['next_cursor'] as String?,
          hasMore: page['has_more'] as bool? ?? false,
        );
      },
    );
  }

  Future<UsersApiResult<VoiceProfile>> getProfile({
    required String authorization,
    required String profileId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl)
        .resolve('/api/v1/users/profiles/$profileId');
    return _get(uri, authorization, (body) {
      final profile = body['profile'] as Map<String, dynamic>;
      return VoiceProfile.fromJson(profile);
    });
  }

  Future<UsersApiResult<VoicePresence>> getPresence({
    required String authorization,
    required String profileId,
  }) async {
    if (!_config.hasBaseUrl) {
      return const UsersApiFailure(message: kUsersMissingBaseUrlDetail);
    }
    final uri = Uri.parse(_config.baseUrl)
        .resolve('/api/v1/users/profiles/$profileId/presence');
    return _get(uri, authorization, (body) {
      final presence = body['presence'] as Map<String, dynamic>;
      return VoicePresence.fromJson(presence);
    });
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
