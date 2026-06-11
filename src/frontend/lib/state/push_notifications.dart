import '../backend/realtime_client.dart';

/// Maps an FCM data payload to the canonical realtime `notification` frame model.
RealtimeFrame? fcmDataToRealtimeNotification(Map<String, String> data) {
  final type = data['type'];
  final chatId = data['chat_id'];
  if (type == null || type.isEmpty) {
    return null;
  }
  final frameData = <String, dynamic>{'type': type};
  if (chatId != null && chatId.isNotEmpty) {
    frameData['chat_id'] = chatId;
  }
  for (final key in const [
    'message_id',
    'sender_profile_id',
    'reactor_profile_id',
    'emoji',
    'match_id',
  ]) {
    final value = data[key];
    if (value != null && value.isNotEmpty) {
      frameData[key] = value;
    }
  }
  return RealtimeFrame(op: 'notification', data: frameData);
}
