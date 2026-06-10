import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/mention_parser.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/ui/chat/mention_message_content.dart';

import 'support/voice_test_theme.dart';

void main() {
  group('MessageMention', () {
    test('listFromWire parses user mention', () {
      final list = MessageMention.listFromWire(
        '[{"type":"user","target_id":"11111111-1111-1111-1111-111111111111"}]',
      );
      expect(list, hasLength(1));
      expect(list.single.type, 'user');
      expect(list.single.targetId, '11111111-1111-1111-1111-111111111111');
    });

    test('encodeJson round-trips everyone', () {
      final json = MessageMention.encodeJson([
        const MessageMention(type: 'everyone'),
      ]);
      expect(json, contains('"everyone"'));
    });
  });

  group('parseMentionsFromContent', () {
    test('detects broadcast tokens', () {
      final mentions = parseMentionsFromContent('hi @everyone and @here');
      expect(mentions.map((m) => m.type), ['everyone', 'here']);
    });
  });

  testWidgets('MentionMessageContent highlights @everyone', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: Scaffold(
          body: MentionMessageContent(
            content: 'ping @everyone',
            mentions: const [MessageMention(type: 'everyone')],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final rich = tester.widgetList<RichText>(find.byType(RichText));
    final plain = rich.map((r) => r.text.toPlainText()).join();
    expect(plain, contains('@everyone'));
  });
}
