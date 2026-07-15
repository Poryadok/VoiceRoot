import 'dart:convert';

/// Product notification event types (docs/features/notifications.md).
abstract final class NotificationEventTypes {
  static const all = <String>[
    newMessage,
    mention,
    reply,
    reaction,
    friendRequest,
    matchFound,
    system,
  ];

  static const newMessage = 'new_message';
  static const mention = 'mention';
  static const reply = 'reply';
  static const reaction = 'reaction';
  static const friendRequest = 'friend_request';
  static const matchFound = 'match_found';
  static const system = 'system';
}

class VoiceNotificationSettings {
  const VoiceNotificationSettings({
    required this.profileId,
    required this.scopeType,
    this.scopeId,
    required this.enabled,
    required this.suppressedTypes,
    this.muteUntil,
  });

  final String profileId;
  final String scopeType;
  final String? scopeId;
  final bool enabled;
  final Set<String> suppressedTypes;
  final DateTime? muteUntil;

  bool isTypeEnabled(String type) => !suppressedTypes.contains(type);

  VoiceNotificationSettings copyWith({
    String? profileId,
    String? scopeType,
    String? scopeId,
    bool? enabled,
    Set<String>? suppressedTypes,
    DateTime? muteUntil,
    bool clearMuteUntil = false,
  }) {
    return VoiceNotificationSettings(
      profileId: profileId ?? this.profileId,
      scopeType: scopeType ?? this.scopeType,
      scopeId: scopeId ?? this.scopeId,
      enabled: enabled ?? this.enabled,
      suppressedTypes: suppressedTypes ?? this.suppressedTypes,
      muteUntil: clearMuteUntil ? null : (muteUntil ?? this.muteUntil),
    );
  }

  VoiceNotificationSettings withTypeEnabled(String type, bool enabled) {
    final next = Set<String>.from(suppressedTypes);
    if (enabled) {
      next.remove(type);
    } else {
      next.add(type);
    }
    return copyWith(suppressedTypes: next);
  }

  String get suppressTypesJson => jsonEncode(suppressedTypes.toList()..sort());

  static VoiceNotificationSettings defaults({
    required String profileId,
    String scopeType = 'global',
    String? scopeId,
  }) {
    return VoiceNotificationSettings(
      profileId: profileId,
      scopeType: scopeType,
      scopeId: scopeId,
      enabled: true,
      suppressedTypes: const {},
    );
  }

  static Set<String> parseSuppressedTypes(String? json) {
    if (json == null || json.trim().isEmpty) return {};
    try {
      final decoded = jsonDecode(json);
      if (decoded is! List) return {};
      return decoded.whereType<String>().toSet();
    } catch (_) {
      return {};
    }
  }
}

class VoiceQuietHours {
  const VoiceQuietHours({
    required this.enabled,
    required this.startTime,
    required this.endTime,
    required this.timezone,
    required this.overrideMentions,
  });

  final bool enabled;
  final String startTime;
  final String endTime;
  final String timezone;
  final bool overrideMentions;

  static const defaults = VoiceQuietHours(
    enabled: false,
    startTime: '23:00',
    endTime: '08:00',
    timezone: 'UTC',
    overrideMentions: true,
  );

  VoiceQuietHours copyWith({
    bool? enabled,
    String? startTime,
    String? endTime,
    String? timezone,
    bool? overrideMentions,
  }) {
    return VoiceQuietHours(
      enabled: enabled ?? this.enabled,
      startTime: startTime ?? this.startTime,
      endTime: endTime ?? this.endTime,
      timezone: timezone ?? this.timezone,
      overrideMentions: overrideMentions ?? this.overrideMentions,
    );
  }

  Map<String, dynamic> toJson() => {
    'enabled': enabled,
    'start_time': startTime,
    'end_time': endTime,
    'timezone': timezone,
    'override_mentions': overrideMentions,
  };

  static VoiceQuietHours fromJson(Map<String, dynamic> json) {
    return VoiceQuietHours(
      enabled: json['enabled'] == true,
      startTime: json['start_time'] as String? ?? defaults.startTime,
      endTime: json['end_time'] as String? ?? defaults.endTime,
      timezone: json['timezone'] as String? ?? defaults.timezone,
      overrideMentions: json['override_mentions'] as bool? ?? true,
    );
  }
}
