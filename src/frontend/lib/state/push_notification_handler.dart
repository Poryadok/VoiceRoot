import '../backend/realtime_client.dart';

/// Maps an FCM/APNs data payload to the canonical realtime `notification` frame model.
RealtimeFrame? fcmDataToRealtimeNotification(Map<String, String> data) {
  final type = data['type'];
  if (type == null || type.isEmpty) {
    return null;
  }
  final frameData = <String, dynamic>{'type': type};
  for (final key in const [
    'chat_id',
    'message_id',
    'sender_profile_id',
    'reactor_profile_id',
    'emoji',
    'match_id',
    'session_id',
    'friend_request_id',
    'game_id',
    'mode',
  ]) {
    final value = data[key];
    if (value != null && value.isNotEmpty) {
      frameData[key] = value;
    }
  }
  return RealtimeFrame(op: 'notification', data: frameData);
}

/// Processes a push payload map (foreground, background resume, or notification tap).
void handlePushPayloadMap(
  Map<String, String> data,
  void Function(Map<String, dynamic>? notificationData) onNotification,
) {
  final frame = fcmDataToRealtimeNotification(data);
  if (frame == null) return;
  onNotification(frame.data);
}
