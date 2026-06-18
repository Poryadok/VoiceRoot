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

/// Multiselect audience union aligned with [user_pb.PrivacyAudience].
class VoicePrivacyAudience {
  const VoicePrivacyAudience({
    this.friends = false,
    this.friendsOfFriends = false,
    this.spaceMembers = false,
    this.spaceIds = const [],
    this.includeGuests = false,
  });

  final bool friends;
  final bool friendsOfFriends;
  final bool spaceMembers;
  final List<String> spaceIds;
  final bool includeGuests;

  static const nobody = VoicePrivacyAudience();

  static const friendsOnly = VoicePrivacyAudience(friends: true);

  static const friendsAndFoF = VoicePrivacyAudience(
    friends: true,
    friendsOfFriends: true,
  );

  static const spaceMembersOnly = VoicePrivacyAudience(spaceMembers: true);

  static const spaceMembersAndFriends = VoicePrivacyAudience(
    spaceMembers: true,
    friends: true,
  );

  static const everyoneWithGuests = VoicePrivacyAudience(
    friends: true,
    friendsOfFriends: true,
    spaceMembers: true,
    includeGuests: true,
  );

  bool get isNobody =>
      !friends &&
      !friendsOfFriends &&
      !spaceMembers &&
      !includeGuests &&
      spaceIds.isEmpty;

  bool get isEveryoneShortcut =>
      friends &&
      friendsOfFriends &&
      spaceMembers &&
      includeGuests &&
      spaceIds.isEmpty;

  user_pb.PrivacyAudience toProto() {
    return user_pb.PrivacyAudience(
      friends: friends,
      friendsOfFriends: friendsOfFriends,
      spaceMembers: spaceMembers,
      spaceIds: spaceIds,
      includeGuests: includeGuests,
    );
  }

  VoicePrivacyAudience copyWith({
    bool? friends,
    bool? friendsOfFriends,
    bool? spaceMembers,
    List<String>? spaceIds,
    bool? includeGuests,
  }) {
    return VoicePrivacyAudience(
      friends: friends ?? this.friends,
      friendsOfFriends: friendsOfFriends ?? this.friendsOfFriends,
      spaceMembers: spaceMembers ?? this.spaceMembers,
      spaceIds: spaceIds ?? this.spaceIds,
      includeGuests: includeGuests ?? this.includeGuests,
    );
  }

  @override
  bool operator ==(Object other) {
    return other is VoicePrivacyAudience &&
        other.friends == friends &&
        other.friendsOfFriends == friendsOfFriends &&
        other.spaceMembers == spaceMembers &&
        _listEquals(other.spaceIds, spaceIds) &&
        other.includeGuests == includeGuests;
  }

  @override
  int get hashCode => Object.hash(
    friends,
    friendsOfFriends,
    spaceMembers,
    Object.hashAll(spaceIds),
    includeGuests,
  );
}

bool _listEquals(List<String> a, List<String> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

VoicePrivacyAudience voicePrivacyAudienceFromProto(user_pb.PrivacyAudience proto) {
  return VoicePrivacyAudience(
    friends: proto.friends,
    friendsOfFriends: proto.friendsOfFriends,
    spaceMembers: proto.spaceMembers,
    spaceIds: List.unmodifiable(proto.spaceIds),
    includeGuests: proto.includeGuests,
  );
}

VoicePrivacyAudience voicePrivacyAudienceFromJson(Map<String, dynamic>? json) {
  if (json == null) return VoicePrivacyAudience.nobody;
  return VoicePrivacyAudience(
    friends: json['friends'] == true,
    friendsOfFriends: json['friends_of_friends'] == true,
    spaceMembers: json['space_members'] == true,
    spaceIds: (json['space_ids'] as List<dynamic>?)
            ?.map((e) => e.toString())
            .toList() ??
        const [],
    includeGuests: json['include_guests'] == true,
  );
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
    required this.allowPhoneSearch,
    required this.allowCalls,
    required this.allowChatSpaceInvites,
    required this.allowFiles,
    required this.allowVoiceMessages,
  });

  final String profileId;
  final String preset;
  final VoicePrivacyAudience showOnline;
  final VoicePrivacyAudience showGameStatus;
  final VoicePrivacyAudience showMmRating;
  final VoicePrivacyAudience showPhone;
  final VoicePrivacyAudience showStories;
  final VoicePrivacyAudience allowDm;
  final VoicePrivacyAudience allowFriendRequests;
  final bool allowGuestDm;
  final VoicePrivacyAudience allowPhoneSearch;
  final VoicePrivacyAudience allowCalls;
  final VoicePrivacyAudience allowChatSpaceInvites;
  final VoicePrivacyAudience allowFiles;
  final VoicePrivacyAudience allowVoiceMessages;

  user_pb.PrivacySettings toProto() {
    return user_pb.PrivacySettings(
      profileId: profileId,
      preset: preset,
      showOnline: showOnline.toProto(),
      showGameStatus: showGameStatus.toProto(),
      showMmRating: showMmRating.toProto(),
      showPhone: showPhone.toProto(),
      showStories: showStories.toProto(),
      allowDm: allowDm.toProto(),
      allowFriendRequests: allowFriendRequests.toProto(),
      allowGuestDm: allowGuestDm,
      allowPhoneSearch: allowPhoneSearch.toProto(),
      allowCalls: allowCalls.toProto(),
      allowChatSpaceInvites: allowChatSpaceInvites.toProto(),
      allowFiles: allowFiles.toProto(),
      allowVoiceMessages: allowVoiceMessages.toProto(),
    );
  }

  VoicePrivacySettings copyWith({
    String? preset,
    VoicePrivacyAudience? showOnline,
    VoicePrivacyAudience? showGameStatus,
    VoicePrivacyAudience? showMmRating,
    VoicePrivacyAudience? showPhone,
    VoicePrivacyAudience? showStories,
    VoicePrivacyAudience? allowDm,
    VoicePrivacyAudience? allowFriendRequests,
    bool? allowGuestDm,
    VoicePrivacyAudience? allowPhoneSearch,
    VoicePrivacyAudience? allowCalls,
    VoicePrivacyAudience? allowChatSpaceInvites,
    VoicePrivacyAudience? allowFiles,
    VoicePrivacyAudience? allowVoiceMessages,
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
      allowPhoneSearch: allowPhoneSearch ?? this.allowPhoneSearch,
      allowCalls: allowCalls ?? this.allowCalls,
      allowChatSpaceInvites:
          allowChatSpaceInvites ?? this.allowChatSpaceInvites,
      allowFiles: allowFiles ?? this.allowFiles,
      allowVoiceMessages: allowVoiceMessages ?? this.allowVoiceMessages,
    );
  }
}

VoicePrivacySettings voicePrivacyFromProto(user_pb.PrivacySettings proto) {
  return VoicePrivacySettings(
    profileId: proto.profileId,
    preset: proto.preset.isEmpty ? 'gaming' : proto.preset,
    showOnline: proto.hasShowOnline()
        ? voicePrivacyAudienceFromProto(proto.showOnline)
        : VoicePrivacyAudience.nobody,
    showGameStatus: proto.hasShowGameStatus()
        ? voicePrivacyAudienceFromProto(proto.showGameStatus)
        : VoicePrivacyAudience.nobody,
    showMmRating: proto.hasShowMmRating()
        ? voicePrivacyAudienceFromProto(proto.showMmRating)
        : VoicePrivacyAudience.nobody,
    showPhone: proto.hasShowPhone()
        ? voicePrivacyAudienceFromProto(proto.showPhone)
        : VoicePrivacyAudience.nobody,
    showStories: proto.hasShowStories()
        ? voicePrivacyAudienceFromProto(proto.showStories)
        : VoicePrivacyAudience.nobody,
    allowDm: proto.hasAllowDm()
        ? voicePrivacyAudienceFromProto(proto.allowDm)
        : VoicePrivacyAudience.nobody,
    allowFriendRequests: proto.hasAllowFriendRequests()
        ? voicePrivacyAudienceFromProto(proto.allowFriendRequests)
        : VoicePrivacyAudience.nobody,
    allowGuestDm: proto.allowGuestDm,
    allowPhoneSearch: proto.hasAllowPhoneSearch()
        ? voicePrivacyAudienceFromProto(proto.allowPhoneSearch)
        : VoicePrivacyAudience.nobody,
    allowCalls: proto.hasAllowCalls()
        ? voicePrivacyAudienceFromProto(proto.allowCalls)
        : VoicePrivacyAudience.nobody,
    allowChatSpaceInvites: proto.hasAllowChatSpaceInvites()
        ? voicePrivacyAudienceFromProto(proto.allowChatSpaceInvites)
        : VoicePrivacyAudience.nobody,
    allowFiles: proto.hasAllowFiles()
        ? voicePrivacyAudienceFromProto(proto.allowFiles)
        : VoicePrivacyAudience.nobody,
    allowVoiceMessages: proto.hasAllowVoiceMessages()
        ? voicePrivacyAudienceFromProto(proto.allowVoiceMessages)
        : VoicePrivacyAudience.nobody,
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
