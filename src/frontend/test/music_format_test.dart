import 'dart:typed_data';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/chat/music_format.dart';

void main() {
  test('admits supported signatures without trusting MIME or extension', () {
    for (final bytes in <List<int>>[
      [0xff, 0xfb, 0x90, 0],
      [0x49, 0x44, 0x33, 4, 0, 0, 0, 0, 0, 0, 0xff, 0xfb, 0x90, 0],
      [0x66, 0x4c, 0x61, 0x43],
    ]) {
      expect(hasSupportedMusicSignature(Uint8List.fromList(bytes)), isTrue);
    }
    final ogg = Uint8List(36)..setAll(0, [0x4f, 0x67, 0x67, 0x53]);
    ogg[26] = 1;
    ogg[27] = 8;
    ogg.setAll(28, [0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64]);
    expect(hasSupportedMusicSignature(ogg), isTrue);
    ogg[28] = 0;
    expect(hasSupportedMusicSignature(ogg), isFalse);
  });
  test('WAVE requires supported PCM/float instead of container alone', () {
    final wav = Uint8List(36)
      ..setAll(0, [0x52, 0x49, 0x46, 0x46])
      ..setAll(8, [0x57, 0x41, 0x56, 0x45])
      ..setAll(12, [0x66, 0x6d, 0x74, 0x20]);
    final data = ByteData.sublistView(wav);
    data.setUint32(16, 16, Endian.little);
    data.setUint16(20, 1, Endian.little);
    data.setUint16(34, 16, Endian.little);
    expect(hasSupportedMusicSignature(wav), isTrue);
    data.setUint16(20, 0xff, Endian.little); // AAC in WAVE
    expect(hasSupportedMusicSignature(wav), isFalse);
    data.setUint16(20, 3, Endian.little);
    data.setUint16(34, 32, Endian.little);
    expect(hasSupportedMusicSignature(wav), isTrue);
    data.setUint32(16, 0xffffffff, Endian.little);
    expect(hasSupportedMusicSignature(wav), isFalse);
  });
  test('rejects native fallback formats and forged/truncated ID3', () {
    for (final bytes in <List<int>>[
      [], [0xff, 0xf1, 0x50, 0x80], // AAC ADTS
      [0, 0, 0, 16, 0x66, 0x74, 0x79, 0x70], // MP4
      [0x49, 0x44, 0x33],
      [0x49, 0x44, 0x33, 4, 0, 0, 0, 0, 1, 0],
      [0x49, 0x44, 0x33, 4, 0, 0, 0, 0, 0, 0, 0xff, 0xf1, 0x50, 0x80],
    ]) {
      expect(hasSupportedMusicSignature(Uint8List.fromList(bytes)), isFalse);
    }
    expect(
      hasSupportedMusicSignature(Uint8List(musicMaxStoredBytes + 1)),
      isFalse,
    );
  });
}
