import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/ui/settings/help_sheet.dart';

void main() {
  testWidgets('Help exposes shortcuts with accessible labels and closes', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final openHelpFocus = FocusNode();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          voiceInputSettingsProvider.overrideWith(
            _PttVoiceInputSettingsNotifier.new,
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                focusNode: openHelpFocus,
                onPressed: () => HelpSheet.show(context),
                child: const Text('Open help'),
              ),
            ),
          ),
        ),
      ),
    );

    openHelpFocus.requestFocus();
    await tester.pump();
    await tester.tap(find.text('Open help'));
    await tester.pumpAndSettle();

    expect(find.text('Keyboard shortcuts'), findsOneWidget);
    expect(find.text('Ctrl+K'), findsOneWidget);
    expect(find.text('Ctrl+,'), findsOneWidget);
    expect(find.text('Alt+↑ / Alt+↓'), findsOneWidget);
    expect(find.text('Escape'), findsOneWidget);
    expect(find.text('↑ / ↓'), findsOneWidget);
    expect(find.text('Enter'), findsOneWidget);
    expect(find.text('R'), findsOneWidget);
    expect(find.text('E'), findsOneWidget);
    expect(find.text('`'), findsOneWidget);
    expect(find.bySemanticsLabel('Ctrl+K, Open search'), findsOneWidget);
    expect(
      find.bySemanticsLabel('`, Hold to talk when push-to-talk is enabled'),
      findsOneWidget,
    );
    expect(find.byTooltip('Close help'), findsOneWidget);

    await tester.tap(find.byTooltip('Close help'));
    await tester.pumpAndSettle();
    expect(find.text('Keyboard shortcuts'), findsNothing);
    expect(FocusManager.instance.primaryFocus, same(openHelpFocus));

    await tester.tap(find.text('Open help'));
    await tester.pumpAndSettle();
    expect(find.text('Keyboard shortcuts'), findsOneWidget);
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('Keyboard shortcuts'), findsNothing);
    expect(FocusManager.instance.primaryFocus, same(openHelpFocus));

    openHelpFocus.dispose();
  });

  testWidgets('Help reference fits a narrow viewport without overflow', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(360, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          voiceInputSettingsProvider.overrideWith(
            _PttVoiceInputSettingsNotifier.new,
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () => HelpSheet.show(context),
                child: const Text('Open help'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Open help'));
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('Keyboard shortcuts'), findsOneWidget);
    expect(find.text('Ctrl+K'), findsOneWidget);
  });
}

class _PttVoiceInputSettingsNotifier extends VoiceInputSettingsNotifier {
  @override
  VoiceInputSettings build() => const VoiceInputSettings(
    mode: VoiceInputMode.ptt,
    pttKey: LogicalKeyboardKey.backquote,
  );
}
