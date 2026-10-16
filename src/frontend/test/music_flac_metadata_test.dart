import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/chat/music_flac_metadata.dart';

void main() {
  test(
    'falls back past unusable decoder tags and preserves valid precedence',
    () {
      expect(
        firstUsableMusicTag(['', '  ', 'invalid\ntag', 'Native title']),
        'Native title',
      );
      expect(
        firstUsableMusicTag([' Decoder title ', 'Native title']),
        'Decoder title',
      );
      expect(firstUsableMusicTag([null, '', '  ']), isNull);
    },
  );

  test('reads only unique music tags from native FLAC comments', () {
    final tags = nativeFlacMusicTags(
      _flac([
        'TITLE=Fixture title',
        'artist=Fixture artist',
        'ALBUM=Fixture album',
        'GENRE=ignored',
      ]),
    );

    expect(tags, {
      'title': 'Fixture title',
      'artist': 'Fixture artist',
      'album': 'Fixture album',
    });
  });

  test('omits ambiguous duplicate fields and retains unique fields', () {
    final tags = nativeFlacMusicTags(
      _flac([
        'TITLE=first',
        'title=second',
        'ARTIST=Fixture artist',
        'ALBUM=Fixture album',
      ]),
    );

    expect(tags, {'artist': 'Fixture artist', 'album': 'Fixture album'});
  });

  test('ignores non-FLAC and malformed or truncated comment blocks', () {
    expect(nativeFlacMusicTags(Uint8List.fromList([1, 2, 3, 4])), isEmpty);

    final truncated = _flac(['TITLE=Fixture title']);
    truncated[46] = 0xff; // Comment vendor length exceeds the block.
    expect(nativeFlacMusicTags(truncated), isEmpty);

    final missingLastFlag = _flac(['TITLE=Fixture title']);
    missingLastFlag[42] &= 0x7f; // Clear the type-4 block's last flag.
    expect(nativeFlacMusicTags(missingLastFlag), isEmpty);
  });

  test('uses the checked Ogg-FLAC EOS granule for unknown sample totals', () {
    expect(unknownTotalOggFlacDuration(_oggFlac()), const Duration(seconds: 2));
    expect(
      unknownTotalOggFlacDuration(_oggFlac(unknownIntermediateGranule: true)),
      const Duration(seconds: 2),
    );
    expect(
      unknownTotalOggFlacDuration(
        _oggFlac(
          unknownIntermediateGranule: true,
          zeroIntermediateGranule: true,
        ),
      ),
      isNull,
    );
    expect(
      unknownTotalOggFlacDuration(
        _oggFlac(
          unknownIntermediateGranule: true,
          completeUnknownIntermediatePacket: true,
        ),
      ),
      isNull,
    );
    expect(
      unknownTotalOggFlacDuration(
        _oggFlac(unknownIntermediateGranule: true, mismatchContinuation: true),
      ),
      isNull,
    );
    expect(
      unknownTotalOggFlacDuration(
        _oggFlac(unknownIntermediateGranule: true, unfinishedEndPacket: true),
      ),
      isNull,
    );
    expect(unknownTotalOggFlacDuration(_oggFlac(totalSamples: 88200)), isNull);
    expect(unknownTotalOggFlacDuration(_oggFlac(headerPacketCount: 0)), isNull);
    expect(unknownTotalOggFlacDuration(_oggFlac(headerPacketCount: 2)), isNull);
    expect(
      unknownTotalOggFlacDuration(_oggFlac(invalidHeaderGranule: true)),
      isNull,
    );
    expect(
      unknownTotalOggFlacDuration(Uint8List.fromList([0x4f, 0x67, 0x67, 0x53])),
      isNull,
    );
  });

  test('rejects malformed, incomplete, and ambiguous Ogg-FLAC pages', () {
    final badCrc = _oggFlac()..[101] ^= 1;
    expect(unknownTotalOggFlacDuration(badCrc), isNull);

    final truncated = Uint8List.fromList(_oggFlac().toList()..removeLast());
    expect(unknownTotalOggFlacDuration(truncated), isNull);

    final chained = _oggFlac(secondSerial: 8);
    expect(unknownTotalOggFlacDuration(chained), isNull);

    expect(unknownTotalOggFlacDuration(_oggFlac(sampleRate: 0)), isNull);
  });
}

Uint8List _oggFlac({
  int secondSerial = 7,
  int sampleRate = 44100,
  int totalSamples = 0,
  int headerPacketCount = 1,
  bool unknownIntermediateGranule = false,
  bool zeroIntermediateGranule = false,
  bool completeUnknownIntermediatePacket = false,
  bool mismatchContinuation = false,
  bool unfinishedEndPacket = false,
  bool invalidHeaderGranule = false,
}) {
  final mapping = <int>[
    0x7f, 0x46, 0x4c, 0x41, 0x43, 1, 0, 0, 1, // Ogg-FLAC mapping v1.0.
    0x66, 0x4c, 0x61, 0x43, // Native FLAC marker.
    0, 0, 0, 34, // STREAMINFO metadata header.
    ...List<int>.filled(34, 0),
  ];
  mapping[7] = (headerPacketCount >> 8) & 0xff;
  mapping[8] = headerPacketCount & 0xff;
  final packedInfo = (sampleRate << 44) | (1 << 41) | (15 << 36) | totalSamples;
  for (var i = 0; i < 8; i++) {
    mapping[27 + i] = (packedInfo >> (56 - i * 8)) & 0xff;
  }
  return Uint8List.fromList([
    ..._oggPage(flags: 2, granule: 0, serial: 7, sequence: 0, body: mapping),
    ..._oggPage(
      flags: 0,
      granule: invalidHeaderGranule ? 1 : 0,
      serial: 7,
      sequence: 1,
      body: [0x84, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0],
      lacing: [12],
    ),
    if (unknownIntermediateGranule)
      ..._oggPage(
        flags: 0,
        granule: zeroIntermediateGranule ? 0 : -1,
        serial: 7,
        sequence: 2,
        body: List<int>.filled(
          completeUnknownIntermediatePacket ? 1 : 255,
          0xff,
        ),
        lacing: [completeUnknownIntermediatePacket ? 1 : 255],
      ),
    ..._oggPage(
      flags: mismatchContinuation ? 4 : (unknownIntermediateGranule ? 5 : 4),
      granule: 88200,
      serial: secondSerial,
      sequence: unknownIntermediateGranule ? 3 : 2,
      body: List<int>.filled(unfinishedEndPacket ? 255 : 1, 0xff),
      lacing: unfinishedEndPacket ? [255] : null,
    ),
  ]);
}

List<int> _oggPage({
  required int flags,
  required int granule,
  required int serial,
  required int sequence,
  required List<int> body,
  List<int>? lacing,
}) {
  final segments = lacing ?? [body.length];
  final page = <int>[
    0x4f, 0x67, 0x67, 0x53, 0, flags,
    ..._le64(granule),
    ..._le32(serial),
    ..._le32(sequence),
    0, 0, 0, 0, // CRC, filled below.
    segments.length, ...segments,
    ...body,
  ];
  final crc = _oggCrc(page);
  page[22] = crc & 0xff;
  page[23] = (crc >> 8) & 0xff;
  page[24] = (crc >> 16) & 0xff;
  page[25] = (crc >> 24) & 0xff;
  return page;
}

int _oggCrc(List<int> bytes) {
  var crc = 0;
  for (var offset = 0; offset < bytes.length; offset++) {
    final byte = offset >= 22 && offset < 26 ? 0 : bytes[offset];
    crc =
        (((crc << 8) & 0xffffffff) ^
            _oggCrcTable[((crc >> 24) ^ byte) & 0xff]) &
        0xffffffff;
  }
  return crc;
}

final List<int> _oggCrcTable = List<int>.generate(256, (index) {
  var crc = index << 24;
  for (var bit = 0; bit < 8; bit++) {
    crc = (crc & 0x80000000) != 0
        ? ((crc << 1) ^ 0x04c11db7) & 0xffffffff
        : (crc << 1) & 0xffffffff;
  }
  return crc;
}, growable: false);

List<int> _le64(int value) => [
  ..._le32(value & 0xffffffff),
  ..._le32(value >> 32),
];

Uint8List _flac(List<String> fields) {
  final comments = BytesBuilder(copy: false)
    ..add(_le32(5))
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
    ..add([
      0x84,
      (block.length >> 16) & 0xff,
      (block.length >> 8) & 0xff,
      block.length & 0xff,
    ])
    ..add(block);
  return result.takeBytes();
}

List<int> _le32(int value) => [
  value & 0xff,
  (value >> 8) & 0xff,
  (value >> 16) & 0xff,
  (value >> 24) & 0xff,
];
