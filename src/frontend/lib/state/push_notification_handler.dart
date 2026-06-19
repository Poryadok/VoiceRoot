import '../backend/realtime_client.dart';
import '../routing/deep_link_parser.dart';

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
    'deep_link',
  ]) {
    final value = data[key];
    if (value != null && value.isNotEmpty) {
      frameData[key] = value;
    }
  }
  return RealtimeFrame(op: 'notification', data: frameData);
}

/// Parses optional canonical [deep_link] from push data; falls back to chat_id.
DeepLinkTarget? pushDataToDeepLinkTarget(Map<String, String> data) {
  final deepLink = data['deep_link']?.trim();
  if (deepLink != null && deepLink.isNotEmpty) {
    try {
      return parseDeepLinkUrl(deepLink);
    } catch (_) {
      // Fall through to legacy chat_id payload.
    }
  }
  final chatId = data['chat_id']?.trim();
  if (chatId == null || chatId.isEmpty) return null;
  final messageId = data['message_id']?.trim();
  if (messageId != null && messageId.isNotEmpty) {
    return DeepLinkTarget(
      kind: DeepLinkKind.chatMessage,
      chatId: chatId,
      messageId: messageId,
      rawUrl: 'https://voice.gg/ch/$chatId/m/$messageId',
    );
  }
  return DeepLinkTarget(
    kind: DeepLinkKind.chat,
    chatId: chatId,
    rawUrl: 'https://voice.gg/ch/$chatId',
  );
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
