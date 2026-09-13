import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/core/voice_state_panel.dart';

import 'support/voice_test_theme.dart';

void main() {
  Widget testApp(Widget child) {
    return MaterialApp(
      theme: voiceTestTheme(),
      home: Scaffold(body: SizedBox(width: 400, height: 600, child: child)),
    );
  }

  testWidgets('VoiceListSkeleton renders configured placeholder rows', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(const VoiceListSkeleton(rowCount: 3)));

    expect(
      find.descendant(
        of: find.byType(VoiceListSkeleton),
        matching: find.byType(Row),
      ),
      findsNWidgets(3),
    );
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('VoiceStatePanel presents an empty state as a named region', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      await tester.pumpWidget(
        testApp(
          const VoiceStatePanel(
            title: 'Nothing here yet',
            icon: Icons.inbox_outlined,
          ),
        ),
      );

      expect(find.text('Nothing here yet'), findsOneWidget);
      expect(find.byIcon(Icons.inbox_outlined), findsOneWidget);
      expect(
        tester.getSemantics(find.byType(VoiceStatePanel)),
        matchesSemantics(label: 'Nothing here yet\nNothing here yet'),
      );
      expect(find.byType(OutlinedButton), findsNothing);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('VoiceStatePanel exposes an error retry action to the keyboard', (
    tester,
  ) async {
    var retries = 0;

    await tester.pumpWidget(
      testApp(
        VoiceStatePanel(
          title: 'Could not load messages',
          message: 'Check your connection and try again.',
          icon: Icons.cloud_off_outlined,
          actionLabel: 'Retry',
          onAction: () => retries += 1,
        ),
      ),
    );

    expect(find.text('Could not load messages'), findsOneWidget);
    expect(find.text('Check your connection and try again.'), findsOneWidget);
    expect(find.byIcon(Icons.cloud_off_outlined), findsOneWidget);
    expect(find.bySemanticsLabel('Retry'), findsOneWidget);

    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(retries, 1);
  });

  testWidgets('VoiceStatePanel hides an incomplete retry action', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        const VoiceStatePanel(
          title: 'Could not load messages',
          actionLabel: 'Retry',
        ),
      ),
    );

    expect(find.byType(OutlinedButton), findsNothing);
  });
}
