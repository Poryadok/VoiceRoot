import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/stories/story_ring_avatar.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('StoryRingAvatar shows gradient ring when hasActiveStory', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: StoryRingAvatar(
            displayName: 'Alice',
            hasActiveStory: true,
          ),
        ),
      ),
    );

    expect(find.byKey(StoryRingAvatar.storyRingKey), findsOneWidget);
  });

  testWidgets('StoryRingAvatar hides ring without active story', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: StoryRingAvatar(
            displayName: 'Bob',
            hasActiveStory: false,
          ),
        ),
      ),
    );

    expect(find.byKey(StoryRingAvatar.storyRingKey), findsNothing);
    expect(find.byType(CircleAvatar), findsOneWidget);
  });

  testWidgets('StoryRingAvatar announces active story to screen readers', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: StoryRingAvatar(
            displayName: 'Alice',
            hasActiveStory: true,
          ),
        ),
      ),
    );

    expect(find.bySemanticsLabel('Active story'), findsOneWidget);
  });
}
