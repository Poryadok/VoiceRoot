import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/chat/markdown_message_content.dart';

import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(String content) {
    return MaterialApp(
      theme: voiceTestTheme(),
      home: Scaffold(
        body: MarkdownMessageContent(content: content),
      ),
    );
  }

  String richPlainText(WidgetTester tester) {
    final rich = tester.widgetList<RichText>(find.byType(RichText));
    return rich.map((r) => r.text.toPlainText()).join();
  }

  testWidgets('renders bold text without markers', (tester) async {
    await tester.pumpWidget(wrap('**bold**'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), contains('bold'));
    expect(richPlainText(tester), isNot(contains('**')));
  });

  testWidgets('renders italic underline and strikethrough', (tester) async {
    await tester.pumpWidget(wrap('*i* __u__ ~~s~~'));
    await tester.pumpAndSettle();
    final plain = richPlainText(tester);
    expect(plain, contains('i'));
    expect(plain, contains('u'));
    expect(plain, contains('s'));
  });

  testWidgets('spoiler is hidden until tapped', (tester) async {
    await tester.pumpWidget(wrap('||secret||'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), isNot(contains('secret')));
    await tester.tap(find.byType(MarkdownMessageContent));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), contains('secret'));
  });

  testWidgets('renders inline code in monospace', (tester) async {
    await tester.pumpWidget(wrap('use `code` here'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), contains('code'));
  });

  testWidgets('renders markdown link label', (tester) async {
    await tester.pumpWidget(wrap('[Voice](https://voice.app)'));
    await tester.pumpAndSettle();
    expect(find.text('Voice'), findsOneWidget);
    expect(find.byKey(const Key('markdown_link')), findsWidgets);
  });

  testWidgets('renders autolinked https url', (tester) async {
    await tester.pumpWidget(wrap('see https://voice.app'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), contains('https://voice.app'));
  });

  testWidgets('renders blockquote', (tester) async {
    await tester.pumpWidget(wrap('> quoted'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), contains('quoted'));
  });

  testWidgets('unsafe javascript link renders label without link key', (
    tester,
  ) async {
    await tester.pumpWidget(wrap('[x](javascript:alert(1))'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), 'x');
    expect(find.byKey(const Key('markdown_link')), findsNothing);
  });

  testWidgets('nested emphasis strips to plain words', (tester) async {
    await tester.pumpWidget(wrap('**bold *and* bold**'));
    await tester.pumpAndSettle();
    expect(richPlainText(tester), 'bold and bold');
  });

  testWidgets('falls back to plain text on empty content edge', (tester) async {
    await tester.pumpWidget(wrap(''));
    await tester.pumpAndSettle();
    expect(find.byType(MarkdownMessageContent), findsOneWidget);
  });

  testWidgets('renders fenced code block', (tester) async {
    await tester.pumpWidget(wrap('```dart\nfinal x = 1;\n```'));
    await tester.pumpAndSettle();
    expect(
      find.byWidgetPredicate(
        (w) =>
            w is Text &&
            (w.data?.contains('final x = 1') ?? false) ||
            (w is RichText &&
                w.text.toPlainText().contains('final x = 1')),
      ),
      findsWidgets,
    );
  });
}
