import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

String richPlainText(WidgetTester tester) {
  return tester
      .widgetList<RichText>(find.byType(RichText))
      .map((r) => r.text.toPlainText())
      .join();
}

void expectMessagePlainText(WidgetTester tester, String text) {
  expect(richPlainText(tester), contains(text));
}

Finder messagePlainTextFinder(String text) {
  return find.byWidgetPredicate(
    (w) => w is RichText && w.text.toPlainText().contains(text),
  );
}
