import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';

/// Web driver E2E placeholder: parser parity for voice.gg paths (full driver in CI).
void main() {
  testWidgets('phase18 deeplink web parser smoke', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('deeplink'))),
    );

    final target = parseDeepLinkUrl('https://voice.gg/ch/chat-1/m/msg-1');
    expect(target.kind, DeepLinkKind.chatMessage);
    expect(target.chatId, 'chat-1');
    expect(target.messageId, 'msg-1');
  });
}
