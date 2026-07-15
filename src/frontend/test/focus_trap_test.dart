import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/a11y/focus_trap.dart';
import 'package:voice_frontend/ui/auth/guest_convert_sheet.dart';
import 'package:voice_frontend/ui/call/call_modal_overlay.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('VoiceFocusTrap creates an autofocus scope', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: VoiceFocusTrap(
            child: Column(
              children: [
                TextButton(
                  onPressed: () {},
                  child: const Text('inside'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final scope = tester.widget<FocusScope>(
      find.descendant(
        of: find.byType(VoiceFocusTrap),
        matching: find.byType(FocusScope),
      ),
    );
    expect(scope.autofocus, isTrue);
  });

  testWidgets('CallModalOverlay traps focus inside overlay card', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Stack(
            children: [
              TextButton(
                onPressed: () {},
                child: const Text('background'),
              ),
              CallModalOverlay(
                overlayKey: const Key('trap_overlay'),
                title: 'Incoming call',
                subtitle: 'Audio',
                avatarLabel: 'Peer',
                actions: [
                  FilledButton(
                    onPressed: () {},
                    child: const Text('Accept'),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(VoiceFocusTrap), findsOneWidget);
    expect(find.text('Accept'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('GuestConvertSheet modal content is wrapped in focus trap', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) {
            return FilledButton(
              onPressed: () => GuestConvertSheet.show(context),
              child: const Text('open'),
            );
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.byKey(GuestConvertSheet.modalKey), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(BottomSheet),
        matching: find.byType(VoiceFocusTrap),
      ),
      findsOneWidget,
    );
  });
}
