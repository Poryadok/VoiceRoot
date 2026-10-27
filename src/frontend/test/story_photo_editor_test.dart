import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/stories/story_photo_editor.dart';

Uint8List samplePhoto() => base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAACAAAAAYCAYAAACbU/80AAAAK0lEQVR4nO3OIQEAAAgDMMLRibRkgRg3E/Ornb6kEhAQEBAQEBAQEBBIBx68a8Vqn9gBOwAAAABJRU5ErkJggg==',
);

Future<void> waitForPhoto(WidgetTester tester) async {
  await tester.pump();
  await tester.runAsync(() async {
    await Future<void>.delayed(const Duration(milliseconds: 100));
  });
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('photo editor returns original bytes without edits', (
    tester,
  ) async {
    final source = samplePhoto();
    Uint8List? result;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) {
            return TextButton(
              onPressed: () async =>
                  result = await Navigator.of(context).push<Uint8List>(
                    MaterialPageRoute(
                      builder: (_) => StoryPhotoEditor(source: source),
                    ),
                  ),
              child: const Text('Open'),
            );
          },
        ),
      ),
    );
    await tester.tap(find.text('Open'));
    await waitForPhoto(tester);
    await tester.tap(find.byKey(StoryPhotoEditor.originalKey));
    await tester.pumpAndSettle();
    expect(result, orderedEquals(source));
  });

  testWidgets('photo editor cancel does not return media', (tester) async {
    final source = samplePhoto();
    Uint8List? result;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) {
            return TextButton(
              onPressed: () async =>
                  result = await Navigator.of(context).push<Uint8List>(
                    MaterialPageRoute(
                      builder: (_) => StoryPhotoEditor(source: source),
                    ),
                  ),
              child: const Text('Open'),
            );
          },
        ),
      ),
    );
    await tester.tap(find.text('Open'));
    await waitForPhoto(tester);
    await tester.tap(find.byKey(StoryPhotoEditor.cancelKey));
    await tester.pumpAndSettle();
    expect(result, isNull);
  });

  testWidgets('filter, doodle and sticker flatten into exported PNG', (
    tester,
  ) async {
    final source = samplePhoto();
    Uint8List? result;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) {
            return TextButton(
              onPressed: () async =>
                  result = await Navigator.of(context).push<Uint8List>(
                    MaterialPageRoute(
                      builder: (_) => StoryPhotoEditor(source: source),
                    ),
                  ),
              child: const Text('Open'),
            );
          },
        ),
      ),
    );
    await tester.tap(find.text('Open'));
    await waitForPhoto(tester);
    await tester.tap(find.byKey(StoryPhotoEditor.grayscaleKey));
    await tester.tap(find.byKey(StoryPhotoEditor.stickerKey));
    await tester.pumpAndSettle();
    final canvas = find.byKey(StoryPhotoEditor.canvasKey);
    final rect = tester.getRect(canvas);
    final gesture = await tester.startGesture(
      rect.topLeft + const Offset(12, 12),
    );
    await gesture.moveTo(rect.topLeft + const Offset(22, 16));
    await gesture.up();
    await tester.tap(find.byKey(StoryPhotoEditor.applyKey));
    await tester.pump();
    await tester.runAsync(() async {
      await Future<void>.delayed(const Duration(milliseconds: 100));
    });
    await tester.pumpAndSettle();
    expect(result, isNotNull);
    expect(result, isNot(orderedEquals(source)));
    final codec = await tester.runAsync(
      () async => ui.instantiateImageCodec(result!),
    );
    final frame = await tester.runAsync(() async => codec!.getNextFrame());
    expect(frame!.image.width, greaterThan(0));
    expect(frame.image.height, greaterThan(0));
    final pixels = await tester.runAsync(
      () async => frame.image.toByteData(format: ui.ImageByteFormat.rawRgba),
    );
    final rgba = pixels!.buffer.asUint8List();
    final width = frame.image.width;
    final scale = width / rect.width;
    int channel(int x, int y, int c) => rgba[(y * width + x) * 4 + c];
    // The untouched corner is gray; the drawn line is white.
    expect(channel(0, 0, 0), closeTo(channel(0, 0, 1), 2));
    expect(channel(0, 0, 1), closeTo(channel(0, 0, 2), 2));
    final strokeX = (12 * scale).round();
    final strokeY = (12 * scale).round();
    expect(channel(strokeX, strokeY, 0), greaterThan(220));
    expect(channel(strokeX, strokeY, 1), greaterThan(220));
    expect(channel(strokeX, strokeY, 2), greaterThan(220));
    frame.image.dispose();
  });
}
