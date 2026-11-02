import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/call/stream_viewer_picker.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/voice_test_theme.dart';

const streams = [
  StreamViewerItem(profileId: 'alex', label: 'Alex', description: 'Screen 1'),
  StreamViewerItem(
    profileId: 'maria',
    label: 'Maria',
    description: 'Chrome',
    isPaused: true,
  ),
  StreamViewerItem(profileId: 'dima', label: 'Dima', description: 'Figma'),
];

void main() {
  testWidgets('H/V stream chooser capture uses shipped theme and fonts', (
    tester,
  ) async {
    final directory = Platform.environment['VOICE_STREAM_VIEWER_CAPTURE_DIR'];
    if (directory == null || directory.isEmpty) return;
    final theme = await tester.runAsync(() async {
      await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
          .load();
      final bytes = await File(
        '${Platform.environment['FLUTTER_ROOT']}/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      ).readAsBytes();
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
      final catalog = await VoiceTokenCatalog.load();
      return VoiceTheme.build(
        catalog: catalog,
        mode: VoiceThemeMode.light,
        profileAccent: catalog.profileAccentAt(0),
      );
    });
    const rootKey = Key('stream_viewer_capture');
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    for (final size in [const Size(840, 525), const Size(390, 844)]) {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      await tester.pumpWidget(
        RepaintBoundary(
          key: rootKey,
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: theme,
            home: Scaffold(
              body: Center(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: StreamViewerPicker(
                    streams: streams,
                    selectedProfileId: 'alex',
                    onSelected: (_) {},
                    onClose: () {},
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(rootKey),
      );
      final written = await tester.runAsync(() async {
        final image = await boundary.toImage(pixelRatio: 1);
        try {
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          final file = File(
            '$directory/stream-viewer-${size.width > 600 ? 'h' : 'v'}.png',
          );
          await file.parent.create(recursive: true);
          await file.writeAsBytes(bytes!.buffer.asUint8List(), flush: true);
          return true;
        } finally {
          image.dispose();
        }
      });
      expect(written, isTrue);
    }
  });
  testWidgets(
    'selection updates the active stream and paused streams remain selectable',
    (tester) async {
      var selected = 'alex';
      await tester.pumpWidget(
        MaterialApp(
          theme: voiceTestTheme(),
          home: Scaffold(
            body: StatefulBuilder(
              builder: (context, setState) => StreamViewerPicker(
                streams: streams,
                selectedProfileId: selected,
                onSelected: (id) => setState(() => selected = id),
                onClose: () {},
              ),
            ),
          ),
        ),
      );
      expect(find.text('Watching'), findsOneWidget);
      expect(find.text('Paused'), findsOneWidget);
      expect(find.byKey(StreamViewerPicker.limitKey), findsOneWidget);
      await tester.tap(find.byKey(const Key('stream_viewer_maria')));
      await tester.pump();
      expect(selected, 'maria');
      final semantics = tester.widget<Semantics>(
        find.byKey(const Key('stream_viewer_semantics_maria')),
      );
      expect(semantics.properties.selected, isTrue);
    },
  );

  testWidgets(
    'empty and fewer-stream states do not claim the limit is reached',
    (tester) async {
      var items = streams.take(2).toList();
      late StateSetter rebuild;
      await tester.pumpWidget(
        MaterialApp(
          theme: voiceTestTheme(),
          home: Scaffold(
            body: StatefulBuilder(
              builder: (context, setState) {
                rebuild = setState;
                return StreamViewerPicker(
                  streams: items,
                  selectedProfileId: 'alex',
                  onSelected: (_) {},
                  onClose: () {},
                );
              },
            ),
          ),
        ),
      );
      expect(find.byKey(StreamViewerPicker.limitKey), findsNothing);
      rebuild(() => items = []);
      await tester.pump();
      expect(find.text('No active streams'), findsOneWidget);
      expect(find.byKey(const Key('stream_viewer_alex')), findsNothing);
    },
  );

  testWidgets('close and Escape dismiss the chooser and allow reopening', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () => showDialog<void>(
                context: context,
                builder: (context) => Dialog(
                  child: StreamViewerPicker(
                    streams: streams,
                    selectedProfileId: 'alex',
                    onSelected: (_) {},
                  ),
                ),
              ),
              child: const Text('Streams'),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Streams'));
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byType(StreamViewerPicker), findsNothing);
    await tester.tap(find.text('Streams'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Close'));
    await tester.pumpAndSettle();
    expect(find.byType(StreamViewerPicker), findsNothing);
  });

  for (final size in [
    const Size(840, 525),
    const Size(390, 844),
    const Size(320, 480),
  ]) {
    testWidgets('stream chooser fits $size with enlarged text', (tester) async {
      await tester.binding.setSurfaceSize(size);
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.pumpWidget(
        MaterialApp(
          theme: voiceTestTheme(),
          home: MediaQuery(
            data: MediaQueryData(
              size: size,
              textScaler: TextScaler.linear(1.5),
            ),
            child: Scaffold(
              body: Center(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: StreamViewerPicker(
                    streams: streams,
                    selectedProfileId: 'alex',
                    onSelected: (_) {},
                    onClose: () {},
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      expect(tester.takeException(), isNull);
      expect(find.byTooltip('Close'), findsOneWidget);
    });
  }
}
