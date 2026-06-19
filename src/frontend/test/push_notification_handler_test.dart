import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';
import 'package:voice_frontend/state/push_notification_handler.dart';

void main() {
  group('pushDataToDeepLinkTarget', () {
    test('prefers canonical deep_link', () {
      final target = pushDataToDeepLinkTarget({
        'deep_link': 'https://voice.gg/u/vanya',
        'chat_id': 'legacy-chat',
      });
      expect(target?.kind, DeepLinkKind.profile);
      expect(target?.username, 'vanya');
    });

    test('falls back to chat_id and message_id', () {
      final target = pushDataToDeepLinkTarget({
        'chat_id': 'chat-1',
        'message_id': 'msg-1',
      });
      expect(target?.kind, DeepLinkKind.chatMessage);
      expect(target?.chatId, 'chat-1');
      expect(target?.messageId, 'msg-1');
    });

    test('falls back to chat only', () {
      final target = pushDataToDeepLinkTarget({'chat_id': 'chat-1'});
      expect(target?.kind, DeepLinkKind.chat);
      expect(target?.chatId, 'chat-1');
    });
  });

  test('fcmDataToRealtimeNotification includes deep_link', () {
    final frame = fcmDataToRealtimeNotification({
      'type': 'new_message',
      'deep_link': 'https://voice.gg/ch/c1',
      'chat_id': 'c1',
    });
    expect(frame?.data?['deep_link'], 'https://voice.gg/ch/c1');
  });
}
