import 'voice_client.dart';

bool _canonicalUuid(String value) =>
    RegExp(r'^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$').hasMatch(value) &&
    value != '00000000-0000-0000-0000-000000000000';

/// Public-profile attribution only. [sdkIdentity] remains the SDK resource key.
class OrdinaryVoiceMediaIdentity {
  const OrdinaryVoiceMediaIdentity._(
    this.sdkIdentity,
    this.profileId,
    this.membershipId,
  );

  final String sdkIdentity;
  final String profileId;
  final String membershipId;

  static OrdinaryVoiceMediaIdentity? parse(String value) {
    final parts = value.split(':');
    if (parts.length != 3 ||
        parts[0] != 'ov' ||
        !_canonicalUuid(parts[1]) ||
        !_canonicalUuid(parts[2])) {
      return null;
    }
    return OrdinaryVoiceMediaIdentity._(value, parts[1], parts[2]);
  }
}

/// Chooses a presentation decoder from the current API session, not authority.
/// Foreign lifecycle resources retain their existing identity handling.
bool usesOrdinaryVoicePresentation(VoiceCallSession session) {
  if (!_canonicalUuid(session.roomId) ||
      session.spaceId != null ||
      session.voiceRoomId != null ||
      session.matchId != null ||
      session.mediaEpoch != null) {
    return false;
  }
  return switch (session.sessionKind) {
    VoiceSessionKind.dm =>
      session.livekitRoomName == 'voice-dm-${session.roomId}',
    VoiceSessionKind.groupVoice =>
      session.livekitRoomName == 'voice-group-${session.roomId}',
    VoiceSessionKind.voiceRoom || VoiceSessionKind.unknown => false,
  };
}
