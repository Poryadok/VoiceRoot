import '../gen/voice/user/v1/user.pb.dart' as user_pb;
import 'api_result.dart';
import 'gateway_http.dart';

const String kUserPrivacyMissingBaseUrlDetail = 'missing base URL';

sealed class UserPrivacyApiResult<T> {
  const UserPrivacyApiResult();
}

final class UserPrivacyApiOk<T> extends UserPrivacyApiResult<T> {
  const UserPrivacyApiOk(this.data);
  final T data;
}

final class UserPrivacyApiFailure extends UserPrivacyApiResult<Never> {
  const UserPrivacyApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class VoicePrivacySettings {
  const VoicePrivacySettings({
    required this.profileId,
    required this.preset,
    required this.showOnline,
    required this.showGameStatus,
    required this.showMmRating,
    required this.showPhone,
    required this.showStories,
    required this.allowDm,
    required this.allowFriendRequests,
    required this.allowGuestDm,
    this.showOnlineIncludeGuests = false,
  });

  final String profileId;
  final String preset;
  final String showOnline;
  final String showGameStatus;
  final String showMmRating;
  final String showPhone;
  final String showStories;
  final String allowDm;
  final String allowFriendRequests;
  final bool allowGuestDm;
  final bool showOnlineIncludeGuests;

  user_pb.PrivacySettings toProto() {
    return user_pb.PrivacySettings(
      profileId: profileId,
      preset: preset,
      showOnline: showOnline,
      showGameStatus: showGameStatus,
      showMmRating: showMmRating,
      showPhone: showPhone,
      showStories: showStories,
      allowDm: allowDm,
      allowFriendRequests: allowFriendRequests,
      allowGuestDm: allowGuestDm,
      showOnlineIncludeGuests: showOnlineIncludeGuests,
    );
  }

  VoicePrivacySettings copyWith({
    String? preset,
    String? showOnline,
    String? showGameStatus,
    String? showMmRating,
    String? showPhone,
    String? showStories,
    String? allowDm,
    String? allowFriendRequests,
    bool? allowGuestDm,
    bool? showOnlineIncludeGuests,
  }) {
    return VoicePrivacySettings(
      profileId: profileId,
      preset: preset ?? this.preset,
      showOnline: showOnline ?? this.showOnline,
      showGameStatus: showGameStatus ?? this.showGameStatus,
      showMmRating: showMmRating ?? this.showMmRating,
      showPhone: showPhone ?? this.showPhone,
      showStories: showStories ?? this.showStories,
      allowDm: allowDm ?? this.allowDm,
      allowFriendRequests: allowFriendRequests ?? this.allowFriendRequests,
      allowGuestDm: allowGuestDm ?? this.allowGuestDm,
      showOnlineIncludeGuests:
          showOnlineIncludeGuests ?? this.showOnlineIncludeGuests,
    );
  }
}

VoicePrivacySettings voicePrivacyFromProto(user_pb.PrivacySettings proto) {
  return VoicePrivacySettings(
    profileId: proto.profileId,
    preset: proto.preset.isEmpty ? 'gaming' : proto.preset,
    showOnline: proto.showOnline,
    showGameStatus: proto.showGameStatus,
    showMmRating: proto.showMmRating,
    showPhone: proto.showPhone,
    showStories: proto.showStories,
    allowDm: proto.allowDm,
    allowFriendRequests: proto.allowFriendRequests,
    allowGuestDm: proto.allowGuestDm,
    showOnlineIncludeGuests: proto.showOnlineIncludeGuests,
  );
}

/// HTTP client for privacy settings (`/api/v1/users/me/privacy`).
class VoiceUserPrivacyClient {
  VoiceUserPrivacyClient({required GatewayHttpClient gateway})
    : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<UserPrivacyApiResult<VoicePrivacySettings>> getPrivacy({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/users/me/privacy'),
      authorization: authorization,
      createEmpty: user_pb.GetPrivacySettingsResponse.create,
    );
    return _map(
      result,
      (data) => voicePrivacyFromProto(
        data.hasPrivacySettings()
            ? data.privacySettings
            : user_pb.PrivacySettings(),
      ),
    );
  }

  Future<UserPrivacyApiResult<VoicePrivacySettings>> updatePrivacy({
    required String authorization,
    required VoicePrivacySettings settings,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve('/api/v1/users/me/privacy'),
      authorization: authorization,
      body: user_pb.UpdatePrivacySettingsRequest(settings: settings.toProto()),
      createEmpty: user_pb.UpdatePrivacySettingsResponse.create,
    );
    return _map(
      result,
      (data) => voicePrivacyFromProto(
        data.hasPrivacySettings()
            ? data.privacySettings
            : user_pb.PrivacySettings(),
      ),
    );
  }

  UserPrivacyApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => UserPrivacyApiOk(parse(data)),
      GatewayHttpFailure(:final error) => UserPrivacyApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}
