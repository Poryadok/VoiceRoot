import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/chat/music_flac_metadata.dart';

void main() {
  test('reads only unique music tags from native FLAC comments', () {
    final tags = nativeFlacMusicTags(_flac([
      'TITLE=Fixture title',
      'artist=Fixture artist',
      'ALBUM=Fixture album',
      'GENRE=ignored',
    ]));

    expect(tags, {
      'title': 'Fixture title',
      'artist': 'Fixture artist',
      'album': 'Fixture album',
    });
  });

  test('omits ambiguous duplicate fields and retains unique fields', () {
    final tags = nativeFlacMusicTags(_flac([
      'TITLE=first',
      'title=second',
      'ARTIST=Fixture artist',
      'ALBUM=Fixture album',
    ]));

    expect(tags, {
      'artist': 'Fixture artist',
      'album': 'Fixture album',
    });
  });

  test('ignores non-FLAC and malformed or truncated comment blocks', () {
    expect(nativeFlacMusicTags(Uint8List.fromList([1, 2, 3, 4])), isEmpty);

    final truncated = _flac(['TITLE=Fixture title']);
    truncated[46] = 0xff; // Comment vendor length exceeds the block.
    expect(nativeFlacMusicTags(truncated), isEmpty);

    final missingLastFlag = _flac(['TITLE=Fixture title']);
    missingLastFlag[4] &= 0x7f;
    expect(nativeFlacMusicTags(missingLastFlag), isEmpty);
  });
}

Uint8List _flac(List<String> fields) {
  final comments = BytesBuilder(copy: false)
    ..add(_le32(6))
    ..add(utf8.encode('Voice'))
    ..add(_le32(fields.length));
  for (final field in fields) {
    final bytes = utf8.encode(field);
    comments
      ..add(_le32(bytes.length))
      ..add(bytes);
  }
  final block = comments.takeBytes();
  final result = BytesBuilder(copy: false)
    ..add([0x66, 0x4c, 0x61, 0x43])
    ..add([0x00, 0x00, 0x00, 0x22]) // STREAMINFO, 34 bytes, not last.
    ..add(List<int>.filled(34, 0))
    ..add([0x84, (block.length >> 16) & 0xff, (block.length >> 8) & 0xff, block.length & 0xff])
    ..add(block);
  return result.takeBytes();
}

List<int> _le32(int value) => [
  value & 0xff,
  (value >> 8) & 0xff,
  (value >> 16) & 0xff,
  (value >> 24) & 0xff,
];
