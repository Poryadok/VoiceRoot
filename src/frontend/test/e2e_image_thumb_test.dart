import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_image_thumb.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  Future<Uint8List> solidPng(int width, int height) async {
    final recorder = ui.PictureRecorder();
    final canvas = Canvas(recorder);
    canvas.drawRect(
      Rect.fromLTWH(0, 0, width.toDouble(), height.toDouble()),
      Paint()..color = const Color(0xFF336699),
    );
    final picture = recorder.endRecording();
    final image = await picture.toImage(width, height);
    final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
    image.dispose();
    return byteData!.buffer.asUint8List();
  }

  Future<ui.Image> decodeImage(Uint8List bytes) async {
    final codec = await ui.instantiateImageCodec(bytes);
    final frame = await codec.getNextFrame();
    return frame.image;
  }

  testWidgets('resizeImageBytesForThumb downscales large images', (tester) async {
    late Uint8List thumbBytes;
    await tester.runAsync(() async {
      final source = await solidPng(640, 480);
      thumbBytes = (await resizeImageBytesForThumb(source))!;
    });
    await tester.runAsync(() async {
      final thumb = await decodeImage(thumbBytes);
      expect(thumb.width, lessThanOrEqualTo(kE2eImageThumbMaxWidth));
      expect(thumb.height, lessThanOrEqualTo(kE2eImageThumbMaxHeight));
      thumb.dispose();
    });
  });

  testWidgets('resizeImageBytesForThumb preserves small images', (tester) async {
    late Uint8List thumbBytes;
    await tester.runAsync(() async {
      final source = await solidPng(80, 60);
      thumbBytes = (await resizeImageBytesForThumb(source))!;
    });
    await tester.runAsync(() async {
      final thumb = await decodeImage(thumbBytes);
      expect(thumb.width, 80);
      expect(thumb.height, 60);
      thumb.dispose();
    });
  });

  test('resizeImageBytesForThumb returns null for invalid bytes', () async {
    final result = await resizeImageBytesForThumb(Uint8List.fromList([1, 2, 3]));
    expect(result, isNull);
  });
}
