import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';

void main() {
  group('VoiceMessage threadParentId', () {
    test('parses threadParentId from JSON wire', () {
      final msg = VoiceMessage.fromJson({
        'id': 'msg-reply',
        'chat': {'id': 'chat-1'},
        'sender_profile_id': 'profile-1',
        'content': 'reply body',
        'thread_parent_id': 'parent-msg',
      });

      expect(msg.toJson()['thread_parent_id'], 'parent-msg');
    });

    test('round-trips threadParentId in toJson', () {
      final msg = VoiceMessage.fromJson({
        'id': 'msg-reply',
        'chat': {'id': 'chat-1'},
        'sender_profile_id': 'profile-1',
        'content': 'reply body',
        'thread_parent_id': 'parent-msg',
      });

      final roundTrip = VoiceMessage.fromJson(msg.toJson());
      expect(roundTrip.toJson()['thread_parent_id'], 'parent-msg');
    });
  });
}
