import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/stories/story_text_style.dart';
import 'package:voice_frontend/theme/voice_colors.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('storyTextBackgroundColor resolves accent/elevated/muted',
      (tester) async {
  late VoiceColors voice;
  await tester.pumpWidget(
    MaterialApp(
      theme: voiceTestTheme(),
      home: Builder(
        builder: (context) {
          voice = VoiceColors.of(context);
          return const SizedBox.shrink();
        },
      ),
    ),
  );

  expect(
    storyTextBackgroundColor(voice, '{"background":"elevated"}'),
    voice.elevated,
  );
  expect(
    storyTextBackgroundColor(voice, '{"background":"muted"}'),
    voice.muted,
  );
  expect(
    storyTextBackgroundColor(voice, '{"background":"accent"}'),
    voice.profileAccent,
  );
  });
}
