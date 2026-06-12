import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/ui/shell/mobile_chat_strip.dart';

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

  testWidgets('narrow layout shows rail and list before a chat is selected', (
    tester,
  ) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(400, 800));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: ThreeColumnShell(
            railChild: Text('Mobile rail'),
            listChild: Text('Chat list'),
            mainChild: Text('Open chat'),
          ),
        ),
      ),
    );

    expect(find.byKey(ThreeColumnShell.navMobileStack), findsOneWidget);
    expect(find.text('Mobile rail'), findsOneWidget);
    expect(find.text('Chat list'), findsOneWidget);
    expect(find.text('Open chat'), findsNothing);
  });

  testWidgets('narrow layout can focus the open chat with a compact strip', (
    tester,
  ) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(400, 800));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: ThreeColumnShell(
            showMainOnlyOnNarrow: true,
            mobileRailChild: Text('Mini strip'),
            railChild: Text('Mobile rail'),
            listChild: Text('Chat list'),
            mainChild: Text('Open chat'),
          ),
        ),
      ),
    );

    expect(find.byKey(ThreeColumnShell.navMobileStack), findsOneWidget);
    expect(find.text('Mini strip'), findsOneWidget);
    expect(find.text('Open chat'), findsOneWidget);
    expect(find.text('Chat list'), findsNothing);
  });

  testWidgets('narrow open chat uses mobile chat strip key when provided', (
    tester,
  ) async {
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.binding.setSurfaceSize(const Size(400, 800));
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: ThreeColumnShell(
            showMainOnlyOnNarrow: true,
            mobileRailChild: SizedBox(key: MobileChatStrip.stripKey),
            mainChild: Text('Open chat'),
          ),
        ),
      ),
    );

    expect(find.byKey(MobileChatStrip.stripKey), findsOneWidget);
    expect(find.byKey(ThreeColumnShell.navOpenChat), findsOneWidget);
  });
}
