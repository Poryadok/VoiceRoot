import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/push_notification_handler.dart';

void main() {
  group('handlePushPayloadMap', () {
    test('invokes callback for valid notification data', () {
      Map<String, dynamic>? captured;
      handlePushPayloadMap(
        {'type': 'mention', 'chat_id': 'c1'},
        (data) => captured = data,
      );
      expect(captured?['type'], 'mention');
      expect(captured?['chat_id'], 'c1');
    });

    test('ignores payload without type', () {
      var called = false;
      handlePushPayloadMap({'chat_id': 'c1'}, (_) => called = true);
      expect(called, isFalse);
    });
  });

  group('fcmDataToRealtimeNotification extended types', () {
    test('maps reply payload', () {
      final frame = fcmDataToRealtimeNotification({
        'type': 'reply',
        'chat_id': 'chat-1',
        'message_id': 'msg-1',
      });
      expect(frame?.data?['type'], 'reply');
    });

    test('maps friend_request payload', () {
      final frame = fcmDataToRealtimeNotification({
        'type': 'friend_request',
        'friend_request_id': 'fr-1',
      });
      expect(frame?.data?['friend_request_id'], 'fr-1');
    });

    test('maps search_nudge payload', () {
      final frame = fcmDataToRealtimeNotification({
        'type': 'search_nudge',
        'session_id': 'sess-1',
      });
      expect(frame?.data?['session_id'], 'sess-1');
    });

    test('maps system payload', () {
      final frame = fcmDataToRealtimeNotification({'type': 'system'});
      expect(frame?.data?['type'], 'system');
    });
  });
}
