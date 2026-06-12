import '../backend/voice_client.dart';

/// Returns true when an incoming call for [roomId] is already shown.
bool shouldIgnoreDuplicateIncoming({
  required String roomId,
  required String? currentRoomId,
  required bool isIncomingPhase,
}) {
  return isIncomingPhase && currentRoomId == roomId;
}

/// Parses VoIP / CallKit payload into call fields; null when invalid.
Map<String, String>? parseIncomingCallPayload(Map<dynamic, dynamic> raw) {
  final data = raw.map((k, v) => MapEntry('$k', '$v'));
  final roomId = data['room_id'] ?? '';
  if (roomId.isEmpty) return null;
  return data;
}

/// Builds a [VoiceCallSession] from push payload for incoming DM calls.
VoiceCallSession? sessionFromVoIPPayload(Map<String, String> data) {
  final roomId = data['room_id'] ?? '';
  if (roomId.isEmpty) return null;
  return VoiceCallSession(
    roomId: roomId,
    livekitRoomName: data['livekit_room_name'] ?? '',
    chatId: data['chat_id'] ?? '',
    initiatorProfileId: data['initiator_profile_id'] ?? '',
    calleeProfileId: data['callee_profile_id'] ?? '',
    mediaKind: '${data['media_kind']}'.toLowerCase() == 'video'
        ? VoiceCallMediaKind.video
        : VoiceCallMediaKind.audio,
    status: VoiceCallStatus.ringing,
    sessionKind: VoiceSessionKind.dm,
    expiresAt: DateTime.tryParse(data['expires_at'] ?? ''),
  );
}
