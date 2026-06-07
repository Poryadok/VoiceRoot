import 'messages_client.dart';

/// Persists read state via Messaging REST, then fans out on Realtime WS.
///
/// WS `mark_read` does not write to Messaging DB — see Realtime `ws.go` and
/// [docs/ARCHITECTURE_REQUIREMENTS.md].
class MessagingReadSync {
  MessagingReadSync({
    required VoiceMessagesClient messagesClient,
    required void Function(String chatId, String messageId) realtimeMarkRead,
  }) : _messages = messagesClient,
       _realtimeMarkRead = realtimeMarkRead;

  final VoiceMessagesClient _messages;
  final void Function(String chatId, String messageId) _realtimeMarkRead;

  Future<bool> markRead({
    required String authorization,
    required String chatId,
    required String messageId,
  }) async {
    final result = await _messages.markRead(
      authorization: authorization,
      chatId: chatId,
      lastReadMessageId: messageId,
    );
    if (result is! MessagesApiOk<void>) return false;
    _realtimeMarkRead(chatId, messageId);
    return true;
  }
}
