import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/settings/reduced_motion.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';

void main() {
  for (final mode in ['preference', 'system', 'default']) {
    testWidgets('shared sheet opens and closes with $mode motion', (
      tester,
    ) async {
      SharedPreferences.setMockInitialValues({
        'voice_reduced_motion': mode == 'preference',
      });
      final container = ProviderContainer();
      addTearDown(container.dispose);
      late BuildContext homeContext;
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(disableAnimations: mode == 'system'),
              child: child!,
            ),
            home: Builder(
              builder: (context) {
                homeContext = context;
                return Scaffold(
                  body: TextButton(
                    onPressed: () => showVoiceBottomSheet<void>(
                      context: context,
                      child: const Text('sheet content'),
                    ),
                    child: const Text('open'),
                  ),
                );
              },
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pump();
      expect(
        container.read(reducedMotionEnabledProvider),
        mode == 'preference',
      );
      final sheetContext = tester.element(find.text('sheet content'));
      final route = ModalRoute.of(sheetContext)!;
      if (mode == 'default') {
        expect(route.animation!.value, lessThan(1));
        await tester.pumpAndSettle();
      } else {
        expect(route.animation!.value, 1);
      }
      expect(find.text('sheet content'), findsOneWidget);
      Navigator.of(homeContext).pop();
      await tester.pump();
      if (mode == 'default') {
        expect(find.text('sheet content'), findsOneWidget);
        await tester.pumpAndSettle();
      }
      expect(find.text('sheet content'), findsNothing);
      expect(tester.takeException(), isNull);
    });
  }
}
