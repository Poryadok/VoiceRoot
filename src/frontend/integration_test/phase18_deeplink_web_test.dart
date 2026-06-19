import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';

void main() {
  testWidgets('web deep link parser accepts voice.gg paths', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Text('deeplink'))),
    );

    const cases = [
      ('https://voice.gg/invite/demo', DeepLinkKind.invite),
      ('https://voice.gg/s/space-1', DeepLinkKind.space),
      ('https://voice.gg/ch/chat-1/m/msg-1', DeepLinkKind.chatMessage),
      ('https://voice.gg/u/alice', DeepLinkKind.profile),
      ('https://voice.gg/dm/user-1', DeepLinkKind.dm),
    ];

    for (final entry in cases) {
      final target = parseDeepLinkUrl(entry.$1);
      expect(target.kind, entry.$2, reason: entry.$1);
    }
  });
}
