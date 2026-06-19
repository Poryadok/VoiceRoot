import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/theme/voice_emoji_style.dart';

void main() {
  test('VoiceEmojiStyle uses bundled Noto Color Emoji family', () {
    final style = VoiceEmojiStyle.textStyle(fontSize: 28);
    expect(style.fontFamily, 'Noto Color Emoji');
    expect(style.fontSize, 28);
  });

  testWidgets('reaction emoji Text uses bundled font family', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Text('👍', style: VoiceEmojiStyle.textStyle(fontSize: 14)),
        ),
      ),
    );

    final text = tester.widget<Text>(find.text('👍'));
    expect(text.style?.fontFamily, 'Noto Color Emoji');
  });
}
