import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('wide layout exposes three column keys', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(900, 600));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(body: ThreeColumnShell()),
      ),
    );
    expect(find.byKey(ThreeColumnShell.navActiveRail), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navChatList), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navMobileStack), findsNothing);
  });

  testWidgets('narrow layout uses mobile stack', (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(400, 800));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(body: ThreeColumnShell()),
      ),
    );
    expect(find.byKey(ThreeColumnShell.navMobileStack), findsOneWidget);
  });
}
