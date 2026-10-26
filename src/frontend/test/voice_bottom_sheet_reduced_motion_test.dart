import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/settings/reduced_motion.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';

void main() {
  for (final mode in [
    'preference',
    'system',
    'default',
    'corrupt',
    'corrupt-system',
  ]) {
    testWidgets('shared sheet opens and closes with $mode motion', (
      tester,
    ) async {
      SharedPreferences.setMockInitialValues({
        'voice_reduced_motion': mode.startsWith('corrupt')
            ? 'invalid'
            : mode == 'preference',
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
              ).copyWith(disableAnimations: mode.endsWith('system')),
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
      if (mode == 'default' || mode == 'corrupt') {
        expect(route.animation!.value, lessThan(1));
        await tester.pumpAndSettle();
      } else {
        expect(route.animation!.value, 1);
      }
      expect(find.text('sheet content'), findsOneWidget);
      Navigator.of(homeContext).pop();
      await tester.pump();
      if (mode == 'default' || mode == 'corrupt') {
        expect(find.text('sheet content'), findsOneWidget);
        await tester.pumpAndSettle();
      }
      expect(find.text('sheet content'), findsNothing);
      if (mode.startsWith('corrupt')) {
        await tester.tap(find.text('open'));
        await tester.pumpAndSettle();
        expect(find.text('sheet content'), findsOneWidget);
        Navigator.of(homeContext).pop();
        await tester.pumpAndSettle();
        expect(find.text('sheet content'), findsNothing);
      }
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('caller removed while preference loads does not open a sheet', (
    tester,
  ) async {
    final ready = Completer<void>();
    final container = ProviderContainer(
      overrides: [
        reducedMotionEnabledProvider.overrideWith(
          () => _PendingMotion(ready.future),
        ),
      ],
    );
    addTearDown(container.dispose);
    late BuildContext caller;
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: Builder(
            builder: (context) {
              caller = context;
              return const SizedBox();
            },
          ),
        ),
      ),
    );
    final result = showVoiceBottomSheet<void>(
      context: caller,
      child: const Text('late sheet'),
    );
    await tester.pumpWidget(const SizedBox());
    ready.complete();
    await result;
    await tester.pump();
    expect(find.text('late sheet'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}

class _PendingMotion extends ReducedMotionNotifier {
  _PendingMotion(this.ready);
  final Future<void> ready;
  @override
  Future<void> get loaded => ready;
  @override
  bool build() => false;
}
