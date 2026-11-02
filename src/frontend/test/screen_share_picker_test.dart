import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/screen_share_sources.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/call/screen_share_picker.dart';

void main() {
  Future<void> open(WidgetTester tester, {bool system = false}) async {
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: const [
          AppLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: const [Locale('en')],
        home: Scaffold(
          body: Builder(
            builder: (context) => TextButton(
              onPressed: () async {
                final result = await showScreenSharePicker(
                  context,
                  sources: const [
                    ScreenShareSource(
                      id: 'screen:0',
                      name: 'Display 1',
                      type: ScreenShareSourceType.screen,
                    ),
                    ScreenShareSource(
                      id: 'window:1',
                      name: 'Editor',
                      type: ScreenShareSourceType.window,
                    ),
                  ],
                  captureSystemAudioSupported: false,
                  useSystemPicker: system,
                );
                if (context.mounted && result != null) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(
                        '${result.sourceId}:${result.maxFrameRate}:${result.captureSystemAudio}',
                      ),
                    ),
                  );
                }
              },
              child: const Text('Open'),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Open'));
    await tester.pumpAndSettle();
  }

  testWidgets('selection is explicit and source IDs survive tab changes', (
    tester,
  ) async {
    await open(tester);
    expect(
      tester
          .widget<FilledButton>(
            find.byKey(const Key('screen_share_picker_start')),
          )
          .onPressed,
      isNull,
    );
    await tester.tap(find.text('Display 1'));
    await tester.pump();
    await tester.tap(find.text('Window or application'));
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(
            find.byKey(const Key('screen_share_picker_start')),
          )
          .onPressed,
      isNull,
    );
    await tester.tap(find.text('Editor'));
    await tester.pump();
    await tester.tap(find.byKey(const Key('screen_share_picker_start')));
    await tester.pumpAndSettle();
    expect(find.text('window:1:30.0:false'), findsOneWidget);
  });

  testWidgets(
    'browser delegates source selection and keeps system audio disabled',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await open(tester, system: true);
      expect(tester.takeException(), isNull);
      expect(
        tester.widget<SwitchListTile>(find.byType(SwitchListTile)).onChanged,
        isNull,
      );
      await tester.tap(find.byKey(const Key('screen_share_picker_start')));
      await tester.pumpAndSettle();
      expect(find.text('null:30.0:false'), findsOneWidget);
    },
  );

  testWidgets('cancel closes without starting capture', (tester) async {
    await open(tester);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(find.byType(ScreenSharePicker), findsNothing);
    expect(find.byType(SnackBar), findsNothing);
  });

  testWidgets('empty catalog cannot start a fabricated source', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(
          body: ScreenSharePicker(
            sources: [],
            captureSystemAudioSupported: true,
          ),
        ),
      ),
    );
    expect(
      find.text('No screens or windows are available to share'),
      findsOneWidget,
    );
    expect(
      tester
          .widget<FilledButton>(
            find.byKey(const Key('screen_share_picker_start')),
          )
          .onPressed,
      isNull,
    );
  });
}
