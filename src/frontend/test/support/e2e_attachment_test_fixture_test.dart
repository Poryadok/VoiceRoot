import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_file_crypto.dart';
import 'package:voice_frontend/e2e/e2e_image_thumb.dart';

import 'e2e_attachment_test_fixture.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('fixture decrypt and thumb resize pipeline', () async {
    final pngBytes = await _solidPng(32, 32);
    final fixture = await E2eAttachmentTestFixture.create();
    final encrypted = await fixture.encryptFileForPeer(plaintext: pngBytes);
    const crypto = E2eFileCrypto();
    final decrypted = await crypto.decryptBytes(
      ciphertext: encrypted.ciphertext,
      keyWire: encrypted.keyWire,
      messageService: fixture.messageService,
      localProfileId: fixture.localProfileId,
      peerProfileId: fixture.peerProfileId,
    );
    expect(decrypted, pngBytes);

    late Uint8List? thumb;
    await TestAsyncUtils.guard(() async {
      thumb = await resizeImageBytesForThumb(decrypted);
    });
    expect(thumb, isNotNull);
  });
}

Future<Uint8List> _solidPng(int width, int height) async {
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
