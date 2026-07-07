import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/settings/reduced_motion.dart';

class _FixedReducedMotionNotifier extends ReducedMotionNotifier {
  _FixedReducedMotionNotifier(this.value);
  final bool value;
  @override
  bool build() => value;
}

/// app stack8 a11y: reduced motion setting short-circuits panel animations.
void main() {
  testWidgets('reduced motion uses instant transitions', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          reducedMotionEnabledProvider.overrideWith(
            () => _FixedReducedMotionNotifier(true),
          ),
        ],
        child: MaterialApp(
          home: Builder(
            builder: (context) {
              final duration = panelAnimationDurationOf(context);
              return Text('duration:${duration.inMilliseconds}');
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('duration:0'), findsOneWidget);
  });

  testWidgets('default motion keeps non-zero panel animation', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          reducedMotionEnabledProvider.overrideWith(
            () => _FixedReducedMotionNotifier(false),
          ),
        ],
        child: MaterialApp(
          home: Builder(
            builder: (context) {
              final duration = panelAnimationDurationOf(context);
              return Text('duration:${duration.inMilliseconds}');
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.textContaining('duration:'), findsOneWidget);
    expect(find.text('duration:0'), findsNothing);
  });
}
