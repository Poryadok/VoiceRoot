import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/chat/composer_error_notice.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'copy preserves the full selectable message and recovery actions',
    (tester) async {
      const message =
          'Upload failed. Retry the same attachment.\nThe draft is preserved.';
      String? copied;
      var retries = 0;
      var dismissals = 0;
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async {
          if (call.method == 'Clipboard.setData') {
            copied = (call.arguments as Map)['text'] as String;
          }
          return null;
        },
      );
      addTearDown(
        () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
          SystemChannels.platform,
          null,
        ),
      );
      await tester.pumpWidget(
        MaterialApp(
          theme: voiceTestTheme(),
          home: Scaffold(
            body: ComposerErrorNotice(
              message: message,
              attachmentName: 'research-notes.pdf',
              actionLabel: 'Retry',
              onAction: () => retries++,
              dismissLabel: 'Remove',
              onDismiss: () => dismissals++,
            ),
          ),
        ),
      );
      expect(
        tester.widget<SelectableText>(find.byType(SelectableText)).data,
        message,
      );
      await tester.tap(find.byTooltip('Copy'));
      expect(copied, message);
      expect(retries, 0);
      expect(dismissals, 0);
      await tester.tap(find.text('Retry'));
      await tester.tap(find.text('Remove'));
      expect(retries, 1);
      expect(dismissals, 1);
      expect(find.text('research-notes.pdf'), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (widget) =>
              widget is Semantics && widget.properties.liveRegion == true,
        ),
        findsOneWidget,
      );
    },
  );

  for (final width in [320.0, 390.0, 900.0]) {
    testWidgets(
      'error notice wraps at $width without overflow and keeps disabled retry',
      (tester) async {
        tester.view.physicalSize = Size(width, 844);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        await tester.pumpWidget(
          MaterialApp(
            theme: voiceTestTheme(),
            home: const Scaffold(
              body: ComposerErrorNotice(
                message:
                    'The message could not be sent. Your draft is still available. Please try again.',
                actionLabel: 'Retry sending this message',
              ),
            ),
          ),
        );
        expect(tester.takeException(), isNull);
        expect(
          tester.widget<TextButton>(find.byType(TextButton)).onPressed,
          isNull,
        );
        expect(
          tester.getSize(find.byType(ComposerErrorNotice)).width,
          lessThanOrEqualTo(width),
        );
      },
    );
  }
}
