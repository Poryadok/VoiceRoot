import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

void main() {
  group('RealtimeProtocol.parseFrame', () {
    test('parses server event with sequence', () {
      final frame = RealtimeProtocol.parseFrame(
        jsonEncode({'op': 'message_create', 's': 5, 'd': {'chat_id': 'c1'}}),
      );
      expect(frame.op, 'message_create');
      expect(frame.sequence, 5);
      expect(frame.data?['chat_id'], 'c1');
    });

    test('parses hello without sequence', () {
      final frame = RealtimeProtocol.parseFrame(
        jsonEncode({'op': 'hello', 'd': {}}),
      );
      expect(frame.op, 'hello');
      expect(frame.sequence, isNull);
    });
  });

  group('RealtimeProtocol.trackSequence', () {
    test('updates last known s', () {
      expect(RealtimeProtocol.trackSequence(null, 1), 1);
      expect(RealtimeProtocol.trackSequence(1, 3), 3);
      expect(RealtimeProtocol.trackSequence(3, null), 3);
    });
  });

  group('RealtimeProtocol.buildClientOp', () {
    test('resume includes last_s', () {
      final json = RealtimeProtocol.buildClientOp(
        'resume',
        {'last_s': 42},
      );
      final decoded = jsonDecode(json) as Map<String, dynamic>;
      expect(decoded['op'], 'resume');
      expect((decoded['d'] as Map)['last_s'], 42);
    });

    test('parses presence_update payload', () {
      final frame = RealtimeProtocol.parseFrame(
        jsonEncode({
          'op': 'presence_update',
          's': 2,
          'd': {'profile_id': 'p1', 'status': 'online', 'chat_id': 'c1'},
        }),
      );
      expect(frame.op, 'presence_update');
      expect(frame.data?['profile_id'], 'p1');
      expect(frame.data?['status'], 'online');
    });

    test('subscribe includes chat_id', () {
      final json = RealtimeProtocol.buildClientOp(
        'subscribe',
        {'chat_id': 'chat-uuid'},
      );
      final decoded = jsonDecode(json) as Map<String, dynamic>;
      expect(decoded['op'], 'subscribe');
      expect((decoded['d'] as Map)['chat_id'], 'chat-uuid');
    });
  });

  group('gatewayWebSocketUri', () {
    test('maps http base URL to ws /ws path', () {
      final uri = gatewayWebSocketUri('http://api.test');
      expect(uri.scheme, 'ws');
      expect(uri.path, '/ws');
      expect(uri.host, 'api.test');
    });

    test('maps https to wss', () {
      final uri = gatewayWebSocketUri('https://voice.example');
      expect(uri.scheme, 'wss');
    });
  });
}
