import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('extended layout exposes navigation, tree, chat, and side panel',
      (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: ThreeColumnShell(
            navigationChild: Text('Navigation'),
            middleChild: Text('Space tree'),
            mainChild: Text('Chat'),
            sidePanelChild: Text('Side panel'),
          ),
        ),
      ),
    );

    expect(find.byKey(ThreeColumnShell.navActiveRail), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navSpaceTree), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navSidePanel), findsOneWidget);
    expect(find.text('Navigation'), findsOneWidget);
    expect(find.text('Space tree'), findsOneWidget);
    expect(find.text('Chat'), findsOneWidget);
    expect(find.text('Side panel'), findsOneWidget);
  });

  testWidgets('home layout hides middle column when middleChild is null',
      (tester) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: ThreeColumnShell(
            navigationChild: Text('Navigation'),
            mainChild: Text('Chat'),
          ),
        ),
      ),
    );

    expect(find.byKey(ThreeColumnShell.navActiveRail), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navSpaceTree), findsNothing);
    expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
    expect(find.text('Navigation'), findsOneWidget);
    expect(find.text('Chat'), findsOneWidget);
  });
}
